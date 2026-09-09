package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/config"
)

func editModel(lines []string, cy, cx int) Model {
	m := Model{
		Width:  80,
		Height: 12,
		Mode:   ModeEdit,
		Focus:  FocusEditor,
		Cfg:    config.Default(),
		Editor: editorState{Lines: append([]string{}, lines...), CY: cy, CX: cx},
	}
	return m
}

func press(t *testing.T, m Model, msg tea.KeyMsg) Model {
	t.Helper()
	out, _ := m.handleKey(msg)
	return out
}

func altRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true}
}

// --- Punkt 3: word- and line-wise navigation ---

func TestEditorWordNavigation(t *testing.T) {
	cases := []struct {
		name   string
		key    tea.KeyMsg
		startX int
		wantX  int
	}{
		{"alt+b moves a word left", altRune('b'), 11, 6},
		{"alt+left moves a word left", tea.KeyMsg{Type: tea.KeyLeft, Alt: true}, 11, 6},
		{"alt+f moves a word right", altRune('f'), 0, 5},
		{"alt+right moves a word right", tea.KeyMsg{Type: tea.KeyRight, Alt: true}, 0, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := press(t, editModel([]string{"hello world"}, 0, tc.startX), tc.key)
			if m.Editor.CX != tc.wantX {
				t.Fatalf("CX = %d, want %d", m.Editor.CX, tc.wantX)
			}
		})
	}
}

func TestEditorLineNavigation(t *testing.T) {
	// Ghostty maps cmd+left/cmd+right to ctrl+a/ctrl+e.
	cases := []struct {
		name  string
		key   tea.KeyMsg
		wantX int
	}{
		{"ctrl+a goes to line start", tea.KeyMsg{Type: tea.KeyCtrlA}, 0},
		{"home goes to line start", tea.KeyMsg{Type: tea.KeyHome}, 0},
		{"ctrl+e goes to line end", tea.KeyMsg{Type: tea.KeyCtrlE}, 11},
		{"end goes to line end", tea.KeyMsg{Type: tea.KeyEnd}, 11},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := press(t, editModel([]string{"hello world"}, 0, 5), tc.key)
			if m.Editor.CX != tc.wantX {
				t.Fatalf("CX = %d, want %d", m.Editor.CX, tc.wantX)
			}
		})
	}
}

// --- Punkt 2: word- and line-wise deletion ---

func TestEditorWordDeletion(t *testing.T) {
	t.Run("alt+backspace deletes the word before the caret", func(t *testing.T) {
		m := press(t, editModel([]string{"hello world"}, 0, 11), tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
		if m.Editor.Lines[0] != "hello " || m.Editor.CX != 6 {
			t.Fatalf("lines=%q CX=%d", m.Editor.Lines, m.Editor.CX)
		}
		if !m.Editor.Dirty {
			t.Fatal("Dirty = false")
		}
	})
	t.Run("alt+delete deletes the word after the caret", func(t *testing.T) {
		m := press(t, editModel([]string{"hello world"}, 0, 5), tea.KeyMsg{Type: tea.KeyDelete, Alt: true})
		if m.Editor.Lines[0] != "hello" || m.Editor.CX != 5 {
			t.Fatalf("lines=%q CX=%d", m.Editor.Lines, m.Editor.CX)
		}
	})
	t.Run("alt+d deletes the word after the caret", func(t *testing.T) {
		m := press(t, editModel([]string{"hello world"}, 0, 5), altRune('d'))
		if m.Editor.Lines[0] != "hello" {
			t.Fatalf("lines=%q", m.Editor.Lines)
		}
	})
}

func TestEditorLineDeletion(t *testing.T) {
	t.Run("ctrl+u deletes to line start", func(t *testing.T) {
		// Ghostty maps cmd+backspace to ctrl+u.
		m := press(t, editModel([]string{"hello world"}, 0, 6), tea.KeyMsg{Type: tea.KeyCtrlU})
		if m.Editor.Lines[0] != "world" || m.Editor.CX != 0 {
			t.Fatalf("lines=%q CX=%d", m.Editor.Lines, m.Editor.CX)
		}
	})
	t.Run("ctrl+k deletes to line end", func(t *testing.T) {
		m := press(t, editModel([]string{"hello world"}, 0, 5), tea.KeyMsg{Type: tea.KeyCtrlK})
		if m.Editor.Lines[0] != "hello" {
			t.Fatalf("lines=%q", m.Editor.Lines)
		}
	})
}

func TestEditorDeletionIsUndoable(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 11), tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
	m.editorUndo()
	if m.Editor.Lines[0] != "hello world" {
		t.Fatalf("after undo = %q", m.Editor.Lines)
	}
}

// --- Punkt 4: keyboard selection ---

func TestEditorShiftArrowSelectsCharacterwise(t *testing.T) {
	m := editModel([]string{"hello world"}, 0, 0)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftRight})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftRight})
	if !m.hasEditorSelection() {
		t.Fatal("no selection after shift+right")
	}
	if got := m.selectedEditorText(); got != "he" {
		t.Fatalf("selected = %q, want %q", got, "he")
	}
	if m.Editor.CX != 2 {
		t.Fatalf("CX = %d, want 2", m.Editor.CX)
	}
}

func TestEditorAltShiftArrowSelectsWordwise(t *testing.T) {
	m := editModel([]string{"hello world"}, 0, 0)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	if got := m.selectedEditorText(); got != "hello" {
		t.Fatalf("selected = %q, want %q", got, "hello")
	}
}

func TestEditorShiftDownSelectsLinewise(t *testing.T) {
	m := editModel([]string{"abc", "def"}, 0, 0)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftDown})
	if got := m.selectedEditorText(); got != "abc\n" {
		t.Fatalf("selected = %q, want %q", got, "abc\n")
	}
}

func TestEditorSelectAll(t *testing.T) {
	// cmd+a never reaches the program: macOS routes it to Ghostty's own menu
	// item. opt+a is the binding, and it needs no terminal configuration.
	m := press(t, editModel([]string{"abc", "def"}, 0, 1), altRune('a'))
	if got := m.selectedEditorText(); got != "abc\ndef" {
		t.Fatalf("selected = %q", got)
	}
}

// Ghostty sends ctrl+a for cmd+left, so it must move the caret, not select.
func TestCtrlADoesNotSelect(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 5), tea.KeyMsg{Type: tea.KeyCtrlA})
	if m.hasEditorSelection() {
		t.Fatal("ctrl+a selected the buffer instead of moving to line start")
	}
	if m.Editor.CX != 0 {
		t.Fatalf("CX = %d, want 0", m.Editor.CX)
	}
}

func TestEditorTypingReplacesSelection(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bye")})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "bye world" {
		t.Fatalf("lines = %q", got)
	}
	if m.hasEditorSelection() {
		t.Fatal("selection survived typing")
	}
}

func TestEditorBackspaceDeletesSelection(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if got := strings.Join(m.Editor.Lines, "\n"); got != " world" {
		t.Fatalf("lines = %q", got)
	}
	if m.Editor.CX != 0 {
		t.Fatalf("CX = %d, want 0", m.Editor.CX)
	}
}

func TestEditorSelectAllThenDeleteEmptiesBuffer(t *testing.T) {
	m := press(t, editModel([]string{"abc", "def"}, 0, 0), altRune('a'))
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDelete})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "" {
		t.Fatalf("lines = %q, want empty", got)
	}
	if len(m.Editor.Lines) != 1 {
		t.Fatalf("len(Lines) = %d, want 1", len(m.Editor.Lines))
	}
}

func TestEditorPlainArrowCollapsesSelection(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	if m.hasEditorSelection() {
		t.Fatal("selection survived a plain arrow key")
	}
	if m.Editor.CX != 0 {
		t.Fatalf("CX = %d, want 0 (collapsed to selection start)", m.Editor.CX)
	}
}

func TestEditorEscClearsSelectionBeforeCancelling(t *testing.T) {
	m := press(t, editModel([]string{"hello"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.hasEditorSelection() {
		t.Fatal("esc did not clear the selection")
	}
	if m.Mode != ModeEdit {
		t.Fatal("esc left edit mode while a selection was active")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Mode == ModeEdit {
		t.Fatal("second esc did not leave edit mode")
	}
}

// The mode-aware hints live in the action menu; see
// TestActionMenuShowsEditHintsInEditMode and its browse counterpart.

func TestReplacingSelectionIsOneUndoStep(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bye")})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "bye world" {
		t.Fatalf("lines = %q", got)
	}
	m.editorUndo()
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello world" {
		t.Fatalf("after one undo = %q, want the original buffer", got)
	}
}

func TestEnterOverSelectionIsOneUndoStep(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "\n world" {
		t.Fatalf("lines = %q", got)
	}
	m.editorUndo()
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello world" {
		t.Fatalf("after one undo = %q", got)
	}
}

func TestUnboundAltRuneDoesNotEnterBuffer(t *testing.T) {
	m := press(t, editModel([]string{"hello"}, 0, 5), altRune('q'))
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello" {
		t.Fatalf("alt+q leaked into the buffer: %q", got)
	}
}

func TestWordDeletionAcrossLineStartJoinsLines(t *testing.T) {
	m := press(t, editModel([]string{"abc", "def"}, 1, 0), tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "abcdef" {
		t.Fatalf("lines = %q", got)
	}
	if m.Editor.CY != 0 || m.Editor.CX != 3 {
		t.Fatalf("caret = %d:%d, want 0:3", m.Editor.CY, m.Editor.CX)
	}
}

func TestSelectionSurvivesAcrossKeystrokes(t *testing.T) {
	m := editModel([]string{"hello world"}, 0, 0)
	for i := 0; i < 3; i++ {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftRight})
	}
	if got := m.selectedEditorText(); got != "hel" {
		t.Fatalf("selected = %q, want %q", got, "hel")
	}
}

func TestEditorInsertsSpace(t *testing.T) {
	// Bubble Tea reports space as KeySpace, not KeyRunes.
	m := press(t, editModel([]string{"hello"}, 0, 5), tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("world")})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello world" {
		t.Fatalf("lines = %q, want %q", got, "hello world")
	}
}

func TestEditorSpaceReplacesSelection(t *testing.T) {
	m := press(t, editModel([]string{"hello world"}, 0, 0), tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	m = press(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "  world" {
		t.Fatalf("lines = %q", got)
	}
}

func TestEditorIgnoresControlRunes(t *testing.T) {
	m := press(t, editModel([]string{"hello"}, 0, 5), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\x1b', '['}})
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello" {
		t.Fatalf("control runes leaked into the buffer: %q", got)
	}
}
