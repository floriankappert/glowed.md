package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/clipboard"
	textedit "github.com/khw1031/glowed/internal/editor"
)

type selectionPoint struct {
	Line int
	Col  int
}

type selectionState struct {
	Active   bool
	Dragging bool
	Kind     string
	Start    selectionPoint
	End      selectionPoint
	File     string
}

type clipboardResultMsg struct {
	Bytes    int
	Fallback bool
	Err      error
}

func (m *Model) startEditorSelection(p selectionPoint) {
	m.Selection = selectionState{
		Active:   false,
		Dragging: true,
		Kind:     "raw",
		Start:    p,
		End:      p,
		File:     m.Editor.File,
	}
	m.Editor.CY = p.Line
	m.Editor.CX = p.Col
	m.ensureEditorVisible()
}

func (m *Model) updateEditorSelection(p selectionPoint) {
	m.Selection.End = p
	m.Selection.Active = compareSelectionPoint(m.Selection.Start, m.Selection.End) != 0
}

func (m *Model) finishEditorSelection(p selectionPoint) tea.Cmd {
	m.updateEditorSelection(p)
	m.Selection.Dragging = false
	if !m.Selection.Active {
		m.clearEditorSelection()
		return nil
	}

	selected, rng, ok := textedit.SliceMarkdown(
		m.Editor.Lines,
		toEditorPosition(m.Selection.Start),
		toEditorPosition(m.Selection.End),
	)
	if !ok {
		m.clearEditorSelection()
		return nil
	}
	payload := selected
	if m.Mode == ModeEdit {
		formatted, _, ok := textedit.FormatClipboard(
			m.Editor.File,
			m.Editor.Lines,
			toEditorPosition(m.Selection.Start),
			toEditorPosition(m.Selection.End),
		)
		if ok {
			payload = m.addSelectionContextMetadata(formatted, m.Editor.File, modeName(m.Mode))
		}
	}
	m.LastSelectionFile = m.Editor.File
	m.LastSelectionPayload = payload
	m.setStatus(fmt.Sprintf("copying selection %d:%d–%d:%d", rng.Start.Line+1, rng.Start.Col+1, rng.End.Line+1, rng.End.Col+1), "info")
	return copyToClipboardCmd(payload)
}

func (m *Model) startPreviewSelection(p selectionPoint) {
	doc := m.currentDoc()
	file := ""
	if doc != nil {
		file = doc.Abs
	}
	m.Selection = selectionState{Active: false, Dragging: true, Kind: "preview", Start: p, End: p, File: file}
}

func (m *Model) finishPreviewSelection(p selectionPoint) tea.Cmd {
	m.updateEditorSelection(p)
	m.Selection.Dragging = false
	if !m.Selection.Active {
		m.clearEditorSelection()
		return nil
	}
	payload, ok := m.formatPreviewSelection()
	if !ok {
		m.clearEditorSelection()
		return nil
	}
	m.LastSelectionFile = m.Selection.File
	m.LastSelectionPayload = payload
	m.setStatus("copying preview selection with context", "info")
	return copyToClipboardCmd(payload)
}

func (m *Model) clearEditorSelection() {
	m.Selection = selectionState{}
}

func (m Model) hasEditorSelection() bool {
	return m.Selection.Kind == "raw" && m.Selection.Active && m.Selection.File == m.Editor.File && compareSelectionPoint(m.Selection.Start, m.Selection.End) != 0
}

func (m Model) hasPreviewSelection() bool {
	doc := m.currentDoc()
	return doc != nil && m.Selection.Kind == "preview" && m.Selection.Active && m.Selection.File == doc.Abs && compareSelectionPoint(m.Selection.Start, m.Selection.End) != 0
}

func (m Model) previewSelectionForLine(line int) (start int, end int, ok bool) {
	if !m.hasPreviewSelection() {
		return 0, 0, false
	}
	rng := textedit.Normalize(toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End))
	if line < rng.Start.Line || line > rng.End.Line {
		return 0, 0, false
	}
	lineLen := lineLen(stripANSI(m.PreviewLines[line]))
	start = 0
	end = lineLen
	if line == rng.Start.Line {
		start = rng.Start.Col
	}
	if line == rng.End.Line {
		end = rng.End.Col
	}
	start = clamp(start, 0, lineLen)
	end = clamp(end, 0, lineLen)
	return start, end, start != end
}

func (m Model) editorSelectionForLine(line int) (start int, end int, ok bool) {
	if !m.hasEditorSelection() {
		return 0, 0, false
	}
	rng := textedit.Normalize(toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End))
	if line < rng.Start.Line || line > rng.End.Line {
		return 0, 0, false
	}
	lineLen := lineLen(m.Editor.Lines[line])
	start = 0
	end = lineLen
	if line == rng.Start.Line {
		start = rng.Start.Col
	}
	if line == rng.End.Line {
		end = rng.End.Col
	}
	start = clamp(start, 0, lineLen)
	end = clamp(end, 0, lineLen)
	return start, end, start != end
}

func (m Model) previewPointFromMouse(x, y int, allowClamp bool) (selectionPoint, bool) {
	if m.Mode != ModePreview || len(m.PreviewLines) == 0 {
		return selectionPoint{}, false
	}
	textTop := m.contentTop() + 1
	textBottom := m.contentTop() + m.contentHeight()
	rightStart := m.rightStartX()
	rightEnd := rightStart + m.rightWidth()
	if !allowClamp && (y < textTop || y >= textBottom || x < rightStart || x >= rightEnd) {
		return selectionPoint{}, false
	}
	row := clamp(y, textTop, textBottom-1) - m.contentTop()
	line := m.PreviewScroll + row - 1
	line = clamp(line, 0, len(m.PreviewLines)-1)
	visibleCol := clamp(x-rightStart, 0, m.rightWidth())
	plain := stripANSI(m.PreviewLines[line])
	col := indexFromDisplayColumn(plain, visibleCol)
	return selectionPoint{Line: line, Col: col}, true
}

func (m Model) editorPointFromMouse(x, y int, allowClamp bool) (selectionPoint, bool) {
	if !m.rawBufferMode() || len(m.Editor.Lines) == 0 {
		return selectionPoint{}, false
	}
	textTop := m.contentTop() + 1
	textBottom := m.contentTop() + m.contentHeight()
	rightStart := m.rightStartX()
	rightEnd := rightStart + m.rightWidth()
	if !allowClamp {
		if y < textTop || y >= textBottom || x < rightStart || x >= rightEnd {
			return selectionPoint{}, false
		}
	}

	row := clamp(y, textTop, textBottom-1) - m.contentTop()
	line := m.Editor.ScrollY + row - 1
	line = clamp(line, 0, len(m.Editor.Lines)-1)

	visibleCol := clamp(x-rightStart, 0, m.editorTextWidth())
	displayCol := m.Editor.ScrollX + visibleCol
	col := indexFromDisplayColumn(m.Editor.Lines[line], displayCol)
	return selectionPoint{Line: line, Col: col}, true
}

func (m *Model) handleClipboardResult(msg clipboardResultMsg) {
	if msg.Err != nil {
		m.setStatus("copy failed: "+msg.Err.Error(), "error")
		return
	}
	via := "pbcopy"
	if msg.Fallback {
		via = "OSC52"
	}
	m.setStatus(fmt.Sprintf("copied selection to clipboard (%d bytes via %s)", msg.Bytes, via), "success")
}

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		fallback, err := copyTextToClipboard(text)
		if err != nil {
			return clipboardResultMsg{Err: err}
		}
		return clipboardResultMsg{Bytes: len([]byte(text)), Fallback: fallback}
	}
}

func copyTextToClipboard(text string) (bool, error) {
	if err := clipboard.Copy(text); err == nil {
		return false, nil
	} else if _, writeErr := fmt.Fprint(os.Stdout, clipboard.OSC52(text)); writeErr != nil {
		return false, fmt.Errorf("%w; OSC52 fallback failed: %v", err, writeErr)
	}
	return true, nil
}

func (m Model) addSelectionContextMetadata(payload, absPath, mode string) string {
	rel := ""
	if absPath != "" {
		if r, err := filepath.Rel(m.Root, absPath); err == nil {
			rel = r
		}
	}
	lines := strings.Split(payload, "\n")
	out := make([]string, 0, len(lines)+3)
	inserted := false
	for _, line := range lines {
		out = append(out, line)
		if !inserted && strings.HasPrefix(strings.TrimSpace(line), "path: ") {
			if rel != "" {
				out = append(out, "rel_path: "+rel)
			}
			if m.Root != "" {
				out = append(out, "root: "+m.Root)
			}
			if mode != "" {
				out = append(out, "mode: "+mode)
			}
			inserted = true
		}
	}
	return strings.Join(out, "\n")
}

func (m Model) formatPreviewSelection() (string, bool) {
	doc := m.currentDoc()
	if doc == nil || !m.hasPreviewSelection() {
		return "", false
	}
	plainLines := make([]string, len(m.PreviewLines))
	for i, line := range m.PreviewLines {
		plainLines[i] = stripANSI(line)
	}
	selected, rng, ok := textedit.SliceMarkdown(plainLines, toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End))
	if !ok {
		return "", false
	}
	payload := fmt.Sprintf("<!-- glowed\npath: %s\nrel_path: %s\nroot: %s\nmode: preview\nselection_type: rendered-preview\nstart_preview_line: %d\nstart_col: %d\nend_preview_line: %d\nend_col: %d\n-->\n\n%s",
		doc.Abs,
		doc.Rel,
		m.Root,
		rng.Start.Line+1,
		rng.Start.Col+1,
		rng.End.Line+1,
		rng.End.Col+1,
		selected,
	)
	return payload, true
}

func (m Model) glowedSelectionFromClipboard(currentAbs string) string {
	text, err := clipboard.Paste()
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "<!-- glowed") {
		return ""
	}
	path := glowedSelectionPath(trimmed)
	if path == "" || !m.pathsMatch(path, currentAbs) {
		return ""
	}
	return trimmed
}

func glowedSelectionPath(payload string) string {
	for _, line := range strings.Split(payload, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "path: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "path: "))
		}
		if line == "-->" {
			break
		}
	}
	return ""
}

func (m Model) plainSelectionFromClipboard(currentAbs string) string {
	if currentAbs == "" {
		return ""
	}
	text, err := clipboard.Paste()
	if err != nil {
		return ""
	}
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "<!-- glowed") {
		return ""
	}
	limited, truncated := limitContextBytes(text, m.Cfg.LLM.MaxContextBytes)
	truncatedLine := ""
	if truncated {
		truncatedLine = "truncated: true\n"
	}
	return fmt.Sprintf("<!-- glowed\npath: %s\nsource: terminal-clipboard\n%sline_metadata: unavailable\n-->\n\n%s", currentAbs, truncatedLine, limited)
}

func toEditorPosition(p selectionPoint) textedit.Position {
	return textedit.Position{Line: p.Line, Col: p.Col}
}

func compareSelectionPoint(a, b selectionPoint) int {
	if a.Line < b.Line {
		return -1
	}
	if a.Line > b.Line {
		return 1
	}
	if a.Col < b.Col {
		return -1
	}
	if a.Col > b.Col {
		return 1
	}
	return 0
}
