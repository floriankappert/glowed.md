package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// projectModel builds a model over a small project with three markdown files.
func projectModel(t *testing.T) (Model, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{
		"alpha.md": "# Alpha\n\nfirst\n",
		"beta.md":  "# Beta\n\nsecond\n",
		"gamma.md": "# Gamma\n\nthird\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := NewWithInitial(root, filepath.Join(root, "alpha.md"))
	m.Width, m.Height = 100, 20
	return m, root
}

func currentFile(m Model) string {
	return filepath.Base(m.Editor.File)
}

// --- Punkt 5: edit is the default mode ---

func TestStartsInEditMode(t *testing.T) {
	m, _ := projectModel(t)
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
	if m.Focus != FocusEditor {
		t.Fatalf("Focus = %v, want editor", focusName(m.Focus))
	}
	if currentFile(m) != "alpha.md" {
		t.Fatalf("editing %q, want alpha.md", currentFile(m))
	}
}

func TestStartsInEditModeWithoutInitialPath(t *testing.T) {
	m, root := projectModel(t)
	m = NewWithInitial(root, "")
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
	if m.Editor.File == "" {
		t.Fatal("no buffer loaded on startup")
	}
}

func TestEmptyProjectStaysInPreview(t *testing.T) {
	m := NewWithInitial(t.TempDir(), "")
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview when there is nothing to edit", modeName(m.Mode))
	}
}

func TestEscFromDefaultEditModeGoesToPreview(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview after esc", modeName(m.Mode))
	}
}

// --- Punkt 1: ctrl+b toggles the sidebar in edit mode ---

func TestCtrlBTogglesSidebarInEditMode(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlB}) // the sidebar starts visible
	if m.SidebarVisible {
		t.Fatal("sidebar visible before the toggle")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})
	if !m.SidebarVisible {
		t.Fatal("ctrl+b did not show the sidebar")
	}
	if m.Mode != ModeEdit {
		t.Fatal("ctrl+b left edit mode")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.SidebarVisible {
		t.Fatal("ctrl+b did not hide the sidebar again")
	}
}

func TestCtrlBDoesNotTypeIntoTheBuffer(t *testing.T) {
	m, _ := projectModel(t)
	before := strings.Join(m.Editor.Lines, "\n")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})
	if got := strings.Join(m.Editor.Lines, "\n"); got != before {
		t.Fatalf("buffer changed: %q", got)
	}
}

// --- Punkt 2: shift+tab focuses the sidebar ---

func TestShiftTabFocusesSidebarInEditMode(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.Focus != FocusSidebar {
		t.Fatalf("Focus = %v, want sidebar", focusName(m.Focus))
	}
	if m.Mode != ModeEdit {
		t.Fatal("focusing the sidebar left edit mode")
	}
}

func TestShiftTabOpensHiddenSidebar(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if !m.SidebarVisible || m.Focus != FocusSidebar {
		t.Fatalf("visible=%v focus=%v", m.SidebarVisible, focusName(m.Focus))
	}
}

func TestShiftTabReturnsFocusToEditor(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.Focus != FocusEditor {
		t.Fatalf("Focus = %v, want editor", focusName(m.Focus))
	}
}

// --- Punkt 3: up/down select rows while the sidebar has focus ---

func TestSidebarArrowsSelectWhileEditing(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	start := m.SidebarSelected
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.SidebarSelected != start+1 {
		t.Fatalf("SidebarSelected = %d, want %d", m.SidebarSelected, start+1)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.SidebarSelected != start {
		t.Fatalf("SidebarSelected = %d, want %d", m.SidebarSelected, start)
	}
}

func TestSidebarArrowsDoNotMoveTheCaret(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	cy := m.Editor.CY
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.Editor.CY != cy {
		t.Fatalf("caret moved to line %d while the sidebar had focus", m.Editor.CY)
	}
}

func TestSidebarKeysDoNotTypeIntoTheBuffer(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	before := strings.Join(m.Editor.Lines, "\n")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if got := strings.Join(m.Editor.Lines, "\n"); got != before {
		t.Fatalf("typing reached the buffer while the sidebar had focus: %q", got)
	}
}

// --- Punkt 4: enter opens the selected file for editing ---

func TestEnterOpensSelectedFileInEditMode(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	target := m.SidebarRows[m.SidebarSelected]
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
	if m.Focus != FocusEditor {
		t.Fatalf("Focus = %v, want editor", focusName(m.Focus))
	}
	if filepath.Base(m.Editor.File) != filepath.Base(target.Rel) {
		t.Fatalf("editing %q, want %q", currentFile(m), target.Rel)
	}
	if m.Editor.Dirty {
		t.Fatal("freshly opened buffer is marked dirty")
	}
}

func TestEnterRefusesToLeaveUnsavedChanges(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	if !m.Editor.Dirty {
		t.Fatal("buffer not dirty after typing")
	}
	opened := currentFile(m)

	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if currentFile(m) != opened {
		t.Fatalf("switched to %q despite unsaved changes", currentFile(m))
	}
	if !strings.Contains(strings.ToLower(m.Status), "unsaved") {
		t.Fatalf("status %q does not mention unsaved changes", m.Status)
	}
	if m.StatusKind != "warn" {
		t.Fatalf("StatusKind = %q, want warn", m.StatusKind)
	}
}

func TestEnterAfterSavingSwitchesFile(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.Editor.Dirty {
		t.Fatal("still dirty after save")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
}

func TestEnterOnDirectoryTogglesInsteadOfOpening(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "top.md"), []byte("# Top\n"), 0o644)
	os.WriteFile(filepath.Join(root, "sub", "inner.md"), []byte("# Inner\n"), 0o644)

	m := NewWithInitial(root, filepath.Join(root, "top.md"))
	m.Width, m.Height = 100, 20
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})

	dirRow := -1
	for i, row := range m.SidebarRows {
		if row.Kind == sidebarRowDirectory {
			dirRow = i
			break
		}
	}
	if dirRow < 0 {
		t.Skip("no directory row in this layout")
	}
	m.setSidebarSelection(dirRow)
	expanded := m.SidebarRows[dirRow].Expanded
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.SidebarRows[dirRow].Expanded == expanded {
		t.Fatal("enter did not toggle the directory")
	}
	if m.Focus != FocusSidebar {
		t.Fatal("toggling a directory moved the focus away from the sidebar")
	}
}

func TestSidebarClickKeepsEditMode(t *testing.T) {
	m, _ := projectModel(t)
	row := 1
	if len(m.SidebarRows) <= row {
		t.Skip("not enough sidebar rows")
	}
	m.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: 0, Y: m.contentTop() + row,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit after clicking a document", modeName(m.Mode))
	}
	if filepath.Base(m.Editor.File) != filepath.Base(m.SidebarRows[row].Rel) {
		t.Fatalf("editing %q, want %q", currentFile(m), m.SidebarRows[row].Rel)
	}
}

func TestSidebarClickRefusesToDropUnsavedChanges(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	opened := currentFile(m)
	row := 1
	if len(m.SidebarRows) <= row {
		t.Skip("not enough sidebar rows")
	}
	m.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: 0, Y: m.contentTop() + row,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if currentFile(m) != opened {
		t.Fatalf("click switched to %q despite unsaved changes", currentFile(m))
	}
}

func TestFooterShowsSidebarHintsWhenSidebarFocused(t *testing.T) {
	m, _ := projectModel(t)
	m.Width = 100
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	footer := stripANSI(m.renderFooter())
	for _, want := range []string{"open", "editor"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("sidebar footer %q missing %q", footer, want)
		}
	}
}

func TestEditFooterMentionsSidebarToggle(t *testing.T) {
	m, _ := projectModel(t)
	m.Width = 120
	footer := stripANSI(m.renderFooter())
	if !strings.Contains(footer, "ctrl+b") {
		t.Fatalf("edit footer %q does not mention ctrl+b", footer)
	}
}

// --- sidebar toggle and focus work outside edit mode too ---

func previewModel(t *testing.T) Model {
	t.Helper()
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc}) // edit is the default mode
	if m.Mode != ModePreview {
		t.Fatalf("setup: Mode = %v", modeName(m.Mode))
	}
	return m
}

func TestCtrlBTogglesSidebarInPreviewMode(t *testing.T) {
	m := previewModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.SidebarVisible {
		t.Fatal("ctrl+b did not hide the sidebar in preview mode")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})
	if !m.SidebarVisible {
		t.Fatal("ctrl+b did not show the sidebar again")
	}
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview", modeName(m.Mode))
	}
}

func TestShiftTabFocusesSidebarInPreviewMode(t *testing.T) {
	m := previewModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if !m.SidebarVisible || m.Focus != FocusSidebar {
		t.Fatalf("visible=%v focus=%v", m.SidebarVisible, focusName(m.Focus))
	}
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview", modeName(m.Mode))
	}
}

func TestShiftTabReturnsFocusToPreview(t *testing.T) {
	m := previewModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.Focus != FocusPreview {
		t.Fatalf("Focus = %v, want preview", focusName(m.Focus))
	}
}

func TestSidebarArrowsSelectInPreviewMode(t *testing.T) {
	m := previewModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	start := m.SidebarSelected
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.SidebarSelected != start+1 {
		t.Fatalf("SidebarSelected = %d, want %d", m.SidebarSelected, start+1)
	}
}

func TestCtrlBFromSearchFocusTogglesSidebar(t *testing.T) {
	m := previewModel(t)
	m.Focus = FocusSearch
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.SidebarVisible {
		t.Fatal("ctrl+b was swallowed by the search input")
	}
	if m.Query != "" {
		t.Fatalf("Query = %q, want empty", m.Query)
	}
}
