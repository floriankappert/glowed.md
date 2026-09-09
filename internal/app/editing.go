package app

import (
	"hash/fnv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/clipboard"
	textedit "github.com/khw1031/glowed/internal/editor"
	"github.com/khw1031/glowed/internal/render"
)

// caret returns the current editor caret as an editor.Position.
func (m Model) caret() textedit.Position {
	return textedit.Position{Line: m.Editor.CY, Col: m.Editor.CX}
}

func (m *Model) setCaret(p textedit.Position) {
	if len(m.Editor.Lines) == 0 {
		m.Editor.Lines = []string{""}
	}
	m.Editor.CY = clamp(p.Line, 0, len(m.Editor.Lines)-1)
	m.Editor.CX = clamp(p.Col, 0, lineLen(m.Editor.Lines[m.Editor.CY]))
}

// selectedEditorText returns the raw text covered by the editor selection.
func (m Model) selectedEditorText() string {
	if !m.hasEditorSelection() {
		return ""
	}
	text, _, ok := textedit.SliceMarkdown(m.Editor.Lines, toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End))
	if !ok {
		return ""
	}
	return text
}

// moveCaretTo moves the caret to p. When extend is true the selection grows
// from its existing anchor, otherwise any selection is dropped.
func (m *Model) moveCaretTo(p textedit.Position, extend bool) {
	if extend {
		if !m.hasEditorSelection() {
			m.beginKeyboardSelection()
		}
		m.setCaret(p)
		m.updateEditorSelection(selectionPoint{Line: m.Editor.CY, Col: m.Editor.CX})
		return
	}
	m.clearEditorSelection()
	m.setCaret(p)
}

// collapseSelection drops the selection and parks the caret on the given edge.
func (m *Model) collapseSelection(toEnd bool) bool {
	if !m.hasEditorSelection() {
		return false
	}
	rng := textedit.Normalize(toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End))
	edge := rng.Start
	if toEnd {
		edge = rng.End
	}
	m.clearEditorSelection()
	m.setCaret(edge)
	return true
}

func (m *Model) beginKeyboardSelection() {
	anchor := selectionPoint{Line: m.Editor.CY, Col: m.Editor.CX}
	m.Selection = selectionState{
		Active: false,
		Kind:   "raw",
		Start:  anchor,
		End:    anchor,
		File:   m.Editor.File,
	}
}

func (m *Model) selectAllEditor() {
	if len(m.Editor.Lines) == 0 {
		m.Editor.Lines = []string{""}
	}
	last := len(m.Editor.Lines) - 1
	m.Selection = selectionState{
		Active: true,
		Kind:   "raw",
		Start:  selectionPoint{Line: 0, Col: 0},
		End:    selectionPoint{Line: last, Col: lineLen(m.Editor.Lines[last])},
		File:   m.Editor.File,
	}
	m.setCaret(textedit.Position{Line: last, Col: lineLen(m.Editor.Lines[last])})
	m.setStatus("selected whole buffer", "info")
}

// deleteEditorRange removes everything between the caret and target.
func (m *Model) deleteEditorRange(a, b textedit.Position) bool {
	lines, at, changed := textedit.DeleteRange(m.Editor.Lines, a, b)
	if !changed {
		return false
	}
	m.pushEditorUndo()
	m.Editor.Lines = lines
	m.clearEditorSelection()
	m.setCaret(at)
	m.Editor.Dirty = true
	return true
}

// deleteEditorSelection removes an active selection and reports whether it did.
func (m *Model) deleteEditorSelection() bool {
	if !m.hasEditorSelection() {
		return false
	}
	return m.deleteEditorRange(toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End))
}

// refreshHighlight recomputes the syntax spans for the raw buffer when its
// content changed. The fingerprint keeps ordinary messages (mouse motion,
// scrolling, ticks) from re-tokenizing an unchanged buffer.
func (m *Model) refreshHighlight() {
	if !m.rawBufferMode() {
		m.Highlight = nil
		m.HighlightKey = 0
		return
	}
	key := bufferFingerprint(m.Editor.Lines, m.Cfg.Preview.Style)
	if key == m.HighlightKey && m.Highlight != nil {
		return
	}
	m.Highlight = render.CodeSpans(m.Editor.Lines, m.Cfg.Preview.Style)
	m.HighlightKey = key
}

func bufferFingerprint(lines []string, style string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(style))
	for _, line := range lines {
		_, _ = h.Write([]byte(line))
		_, _ = h.Write([]byte{'\n'})
	}
	return h.Sum64()
}

// replaceSelection inserts text over an active selection, recording the
// deletion and the insertion as a single undoable edit. Without a selection it
// is a plain insert.
func (m *Model) replaceSelection(text string) {
	m.pushEditorUndo()
	if m.hasEditorSelection() {
		if lines, at, changed := textedit.DeleteRange(m.Editor.Lines, toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End)); changed {
			m.Editor.Lines = lines
			m.setCaret(at)
		}
		m.clearEditorSelection()
	}
	if text == "\n" {
		m.splitLine()
	} else {
		m.insertText(text)
	}
	m.Editor.Dirty = true
}

// pasteIntoEditor inserts text at the caret, replacing an active selection.
// The whole paste is one undoable edit and may span several lines.
func (m *Model) pasteIntoEditor(text string) {
	if text == "" {
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	m.pushEditorUndo()
	if m.hasEditorSelection() {
		if lines, at, changed := textedit.DeleteRange(m.Editor.Lines, toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End)); changed {
			m.Editor.Lines = lines
			m.setCaret(at)
		}
		m.clearEditorSelection()
	}
	for i, part := range strings.Split(text, "\n") {
		if i > 0 {
			m.splitLine()
		}
		if part != "" {
			m.insertText(part)
		}
	}
	m.Editor.Dirty = true
}

// copyEditorSelection puts the selected text on the clipboard as plain text.
// Unlike the mouse selection, which is meant as LLM context, this is the raw
// text so it can be pasted anywhere.
func (m *Model) copyEditorSelection() tea.Cmd {
	text := m.selectedEditorText()
	if text == "" {
		m.setStatus("nothing selected to copy", "warn")
		return nil
	}
	m.LastSelectionFile = m.Editor.File
	m.LastSelectionPayload = text
	return copyToClipboardCmd(text)
}

// pasteFromClipboard reads the system clipboard and inserts it at the caret.
func (m *Model) pasteFromClipboard() {
	text, err := clipboard.Paste()
	if err != nil {
		m.setStatus("paste failed: "+err.Error(), "error")
		return
	}
	if text == "" {
		m.setStatus("clipboard is empty", "warn")
		return
	}
	m.pasteIntoEditor(text)
	m.setStatus("pasted from clipboard", "info")
}
