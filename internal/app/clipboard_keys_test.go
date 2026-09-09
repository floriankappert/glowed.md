package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- paste ---

func TestBracketedPasteInsertsText(t *testing.T) {
	m := editModel([]string{"hello "}, 0, 6)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world"), Paste: true})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello world" {
		t.Fatalf("lines = %q", got)
	}
	if m.Editor.CX != 11 {
		t.Fatalf("CX = %d, want 11", m.Editor.CX)
	}
}

func TestBracketedPasteKeepsNewlines(t *testing.T) {
	m := editModel([]string{"start", "end"}, 0, 5)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\nmiddle\nmore"), Paste: true})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "start\nmiddle\nmore\nend" {
		t.Fatalf("lines = %q", got)
	}
	if m.Editor.CY != 2 || m.Editor.CX != 4 {
		t.Fatalf("caret = %d:%d, want 2:4", m.Editor.CY, m.Editor.CX)
	}
}

func TestBracketedPasteNormalizesCarriageReturns(t *testing.T) {
	m := editModel([]string{""}, 0, 0)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\r\nb"), Paste: true})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "a\nb" {
		t.Fatalf("lines = %q", got)
	}
}

func TestPasteReplacesSelectionInOneUndoStep(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bye"), Paste: true})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "bye world" {
		t.Fatalf("lines = %q", got)
	}
	m.editorUndo()
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello world" {
		t.Fatalf("after undo = %q", got)
	}
}

func TestPasteIntoSidebarFocusDoesNotTouchBuffer(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	before := strings.Join(m.Editor.Lines, "\n")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("junk"), Paste: true})
	if got := strings.Join(m.Editor.Lines, "\n"); got != before {
		t.Fatalf("paste reached the buffer while the sidebar had focus: %q", got)
	}
}

// --- copy ---

func TestCopyKeyCopiesPlainSelection(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	out, cmd := m.handleKey(altRune('c'))
	if cmd == nil {
		t.Fatal("copy produced no command")
	}
	if out.LastSelectionPayload != "hello" {
		t.Fatalf("payload = %q, want plain %q", out.LastSelectionPayload, "hello")
	}
	if !out.hasEditorSelection() {
		t.Fatal("copy dropped the selection")
	}
}

func TestCopyWithoutSelectionWarns(t *testing.T) {
	m := editModel([]string{"hello"}, 0, 0)
	out, cmd := m.handleKey(altRune('c'))
	if cmd != nil {
		t.Fatal("copy without a selection still touched the clipboard")
	}
	if out.StatusKind != "warn" {
		t.Fatalf("StatusKind = %q, want warn", out.StatusKind)
	}
}

func TestCopyDoesNotTypeIntoBuffer(t *testing.T) {
	m := press(t, editModel([]string{"hello"}, 0, 5), tea.KeyMsg{Type: tea.KeyShiftLeft})
	before := strings.Join(m.Editor.Lines, "\n")
	out, _ := m.handleKey(altRune('c'))
	if got := strings.Join(out.Editor.Lines, "\n"); got != before {
		t.Fatalf("buffer changed: %q", got)
	}
}

// --- alt-modified keys must not leak into the other text inputs ---

func TestSearchIgnoresAltModifiedRunes(t *testing.T) {
	m := Model{Width: 80, Height: 12, Focus: FocusSearch}
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})
	m.handleSearchKey(altRune('a'))
	m.handleSearchKey(altRune('c'))
	if m.Query != "foo" {
		t.Fatalf("Query = %q, want %q", m.Query, "foo")
	}
}

func TestChatIgnoresAltModifiedRunes(t *testing.T) {
	m := Model{Width: 80, Height: 12, Focus: FocusChat}
	m.Chat.Visible = true
	m.handleChatKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi")})
	m.handleChatKey(altRune('a'))
	if m.Chat.Input != "hi" {
		t.Fatalf("Chat.Input = %q, want %q", m.Chat.Input, "hi")
	}
}
