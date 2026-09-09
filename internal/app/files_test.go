package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func toolbarText(m Model) string {
	return stripANSI(m.renderToolbar())
}

// --- ctrl+n: new file ---

func TestCtrlNOpensFilenamePromptInToolbar(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if !m.Prompt.Active {
		t.Fatal("Prompt.Active = false after ctrl+n")
	}
	if !strings.Contains(toolbarText(m), "enter filename:") {
		t.Fatalf("toolbar = %q, want the filename prompt", toolbarText(m))
	}
}

func TestNewFilePromptCreatesFileAndOpensItForEditing(t *testing.T) {
	m, root := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = typeText(t, m, "note")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	created := filepath.Join(root, "note.md")
	body, err := os.ReadFile(created)
	if err != nil {
		t.Fatalf("new file not created: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("new file = %q, want empty", body)
	}
	if m.Prompt.Active {
		t.Fatal("prompt still active after creating the file")
	}
	if m.Mode != ModeEdit || filepath.Base(m.Editor.File) != "note.md" {
		t.Fatalf("mode=%v file=%q, want the new file open for editing", modeName(m.Mode), m.Editor.File)
	}
	found := false
	for _, d := range m.Docs {
		if d.Rel == "note.md" {
			found = true
		}
	}
	if !found {
		t.Fatal("new file missing from the scan results")
	}
}

func TestNewFileKeepsAnExplicitMarkdownExtension(t *testing.T) {
	m, root := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = typeText(t, m, "note.md")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if _, err := os.Stat(filepath.Join(root, "note.md")); err != nil {
		t.Fatalf("note.md not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "note.md.md")); err == nil {
		t.Fatal("extension appended twice")
	}
}

func TestNewFileIsCreatedNextToTheCurrentDocument(t *testing.T) {
	m, root := projectModel(t)
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.md"), []byte("# Deep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.scan("manual")
	m.selectInitialDocument(filepath.Join(sub, "deep.md"))
	m.enterEditMode()

	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = typeText(t, m, "sibling")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if _, err := os.Stat(filepath.Join(sub, "sibling.md")); err != nil {
		t.Fatalf("file not created next to the current document: %v", err)
	}
}

func TestNewFileRefusesAnExistingFile(t *testing.T) {
	m, root := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = typeText(t, m, "alpha.md")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.StatusKind != "error" {
		t.Fatalf("StatusKind = %q, want error: %q", m.StatusKind, m.Status)
	}
	body, err := os.ReadFile(filepath.Join(root, "alpha.md"))
	if err != nil || !strings.Contains(string(body), "# Alpha") {
		t.Fatalf("existing file was overwritten: %q (%v)", body, err)
	}
}

func TestNewFileRefusesAPathOutsideTheRoot(t *testing.T) {
	m, root := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = typeText(t, m, "../escape.md")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.StatusKind != "error" {
		t.Fatalf("StatusKind = %q, want error: %q", m.StatusKind, m.Status)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape.md")); err == nil {
		t.Fatal("file created outside the project root")
	}
}

func TestNewFileRefusesAnEmptyName(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Prompt.Active {
		t.Fatal("prompt closed on an empty name")
	}
	if m.StatusKind != "warn" {
		t.Fatalf("StatusKind = %q, want warn", m.StatusKind)
	}
}

func TestPromptEscCancelsWithoutCreatingAFile(t *testing.T) {
	m, root := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = typeText(t, m, "gone")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Prompt.Active {
		t.Fatal("prompt still active after esc")
	}
	if _, err := os.Stat(filepath.Join(root, "gone.md")); err == nil {
		t.Fatal("esc created the file anyway")
	}
}

func TestPromptKeysDoNotReachTheEditor(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	m = typeText(t, m, "abc")
	if m.Editor.Dirty {
		t.Fatal("prompt input leaked into the buffer")
	}
	if m.Prompt.Input != "abc" {
		t.Fatalf("Prompt.Input = %q, want %q", m.Prompt.Input, "abc")
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Prompt.Input != "ab" {
		t.Fatalf("Prompt.Input = %q after backspace, want %q", m.Prompt.Input, "ab")
	}
}

// --- ctrl+p: action menu ---

func TestCtrlPOpensTheActionMenu(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if !m.Menu.Active {
		t.Fatal("Menu.Active = false after ctrl+p")
	}
	view := stripANSI(m.View())
	for _, want := range []string{"new file", "edit filename", "delete file"} {
		if !strings.Contains(view, want) {
			t.Fatalf("menu missing %q:\n%s", want, view)
		}
	}
}

func TestActionMenuEscClosesIt(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Menu.Active {
		t.Fatal("menu still active after esc")
	}
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want the mode untouched", modeName(m.Mode))
	}
}

func TestActionMenuArrowMovesTheSelection(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.Menu.Selected != 1 {
		t.Fatalf("Menu.Selected = %d, want 1", m.Menu.Selected)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.Menu.Selected != 0 {
		t.Fatalf("Menu.Selected = %d, want 0", m.Menu.Selected)
	}
}

func TestActionMenuNewFileOpensThePrompt(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Menu.Active {
		t.Fatal("menu stayed open")
	}
	if !m.Prompt.Active || !strings.Contains(toolbarText(m), "enter filename:") {
		t.Fatalf("new file did not open the prompt: %q", toolbarText(m))
	}
}

func TestActionMenuRenamePrefillsTheCurrentName(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Prompt.Input != "alpha.md" {
		t.Fatalf("Prompt.Input = %q, want the current file name", m.Prompt.Input)
	}
	if !strings.Contains(toolbarText(m), "rename to:") {
		t.Fatalf("toolbar = %q, want the rename prompt", toolbarText(m))
	}
}

func renamePrompt(t *testing.T, m Model) Model {
	t.Helper()
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	return press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
}

func TestRenameMovesTheFileAndKeepsItOpen(t *testing.T) {
	m, root := projectModel(t)
	m = renamePrompt(t, m)
	for range "alpha.md" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m = typeText(t, m, "renamed.md")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if _, err := os.Stat(filepath.Join(root, "renamed.md")); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha.md")); err == nil {
		t.Fatal("old file still there")
	}
	if filepath.Base(m.Editor.File) != "renamed.md" {
		t.Fatalf("editing %q, want renamed.md", m.Editor.File)
	}
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
}

func TestRenameRefusesAnExistingTarget(t *testing.T) {
	m, root := projectModel(t)
	m = renamePrompt(t, m)
	for range "alpha.md" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m = typeText(t, m, "beta.md")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.StatusKind != "error" {
		t.Fatalf("StatusKind = %q, want error: %q", m.StatusKind, m.Status)
	}
	body, _ := os.ReadFile(filepath.Join(root, "beta.md"))
	if !strings.Contains(string(body), "# Beta") {
		t.Fatalf("beta.md was overwritten: %q", body)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha.md")); err != nil {
		t.Fatal("alpha.md disappeared on a refused rename")
	}
}

func TestRenameRefusesUnsavedChanges(t *testing.T) {
	m, root := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	m = renamePrompt(t, m)
	if m.Prompt.Active {
		t.Fatal("rename prompt opened with unsaved changes")
	}
	if m.StatusKind != "warn" {
		t.Fatalf("StatusKind = %q, want warn: %q", m.StatusKind, m.Status)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha.md")); err != nil {
		t.Fatal("alpha.md disappeared")
	}
}

// --- delete ---

func deletePrompt(t *testing.T, m Model) Model {
	t.Helper()
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	return press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
}

func TestDeleteAsksForConfirmationFirst(t *testing.T) {
	m, root := projectModel(t)
	m = deletePrompt(t, m)
	if !m.Prompt.Active {
		t.Fatal("delete did not open a confirmation prompt")
	}
	if !strings.Contains(toolbarText(m), "alpha.md") || !strings.Contains(toolbarText(m), "(y/N)") {
		t.Fatalf("toolbar = %q, want a y/N confirmation naming the file", toolbarText(m))
	}
	if _, err := os.Stat(filepath.Join(root, "alpha.md")); err != nil {
		t.Fatal("file deleted before the confirmation")
	}
}

func TestDeleteConfirmedRemovesTheFileAndKeepsABackup(t *testing.T) {
	m, root := projectModel(t)
	m = deletePrompt(t, m)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if _, err := os.Stat(filepath.Join(root, "alpha.md")); err == nil {
		t.Fatal("file still there after a confirmed delete")
	}
	if _, err := os.Stat(filepath.Join(root, "alpha.md.bak")); err != nil {
		t.Fatalf("no backup kept: %v", err)
	}
	for _, d := range m.Docs {
		if d.Rel == "alpha.md" {
			t.Fatal("deleted file still in the scan results")
		}
	}
	if m.Editor.File != "" && filepath.Base(m.Editor.File) == "alpha.md" {
		t.Fatal("editor still holds the deleted file")
	}
}

func TestDeleteDeclinedKeepsTheFile(t *testing.T) {
	m, root := projectModel(t)
	m = deletePrompt(t, m)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.Prompt.Active {
		t.Fatal("prompt still active")
	}
	if _, err := os.Stat(filepath.Join(root, "alpha.md")); err != nil {
		t.Fatalf("file deleted despite declining: %v", err)
	}
}

func TestDeleteEscKeepsTheFile(t *testing.T) {
	m, root := projectModel(t)
	m = deletePrompt(t, m)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if _, err := os.Stat(filepath.Join(root, "alpha.md")); err != nil {
		t.Fatalf("file deleted on esc: %v", err)
	}
}

// --- caret focus ---

// While a prompt owns the keyboard the buffer must not keep drawing its own
// caret, otherwise two carets sit on screen and neither looks focused.
func TestEditorCaretMovesToThePromptLine(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m, _ := projectModel(t)
	m.SidebarVisible = false
	caretRow := m.Editor.CY - m.Editor.ScrollY
	if !strings.Contains(m.renderEditorLine(caretRow), "\x1b[7m") {
		t.Fatal("setup: the buffer does not draw a caret")
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if strings.Contains(m.renderEditorLine(caretRow), "\x1b[7m") {
		t.Fatal("buffer still draws its caret while the prompt is open")
	}
	if !strings.Contains(m.renderToolbar(), "\x1b[7m") {
		t.Fatalf("prompt line has no caret: %q", m.renderToolbar())
	}
}

func TestEditorCaretHiddenWhileTheMenuIsOpen(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m, _ := projectModel(t)
	m.SidebarVisible = false
	caretRow := m.Editor.CY - m.Editor.ScrollY
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if strings.Contains(m.renderEditorLine(caretRow), "\x1b[7m") {
		t.Fatal("buffer still draws its caret while the action menu is open")
	}
}

func TestEditorCaretReturnsAfterTheMenuCloses(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m, _ := projectModel(t)
	m.SidebarVisible = false
	caretRow := m.Editor.CY - m.Editor.ScrollY
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(m.renderEditorLine(caretRow), "\x1b[7m") {
		t.Fatal("caret did not return to the buffer after closing the menu")
	}
}

// --- action menu layout ---

// menuRows returns the content-pane rows of the frame, without the borders.
// Escapes are kept, so callers that only want text strip them themselves.
func menuRows(t *testing.T, m Model) []string {
	t.Helper()
	rows := strings.Split(m.View(), "\n")
	out := make([]string, 0, m.contentHeight())
	for row := m.contentTop(); row < m.paneBottomRow(); row++ {
		out = append(out, rows[row])
	}
	return out
}

func TestActionMenuFillsTheContentPaneWithABackground(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := layoutModel(t, 90, 20)
	m.Menu = menuState{Active: true}
	// The backdrop paints every content row, so its background SGR has to show
	// up on all of them, not just the ones carrying an entry.
	backdrop := "\x1b[48;5;" + menuBackdropColor + "m"
	rows := strings.Split(m.View(), "\n")
	for row := m.contentTop(); row < m.paneBottomRow(); row++ {
		if !strings.Contains(rows[row], backdrop) {
			t.Fatalf("content row %d has no menu background: %q", row, rows[row])
		}
	}
}

func TestActionMenuBlockIsCenteredWithLeftAlignedText(t *testing.T) {
	m := layoutModel(t, 90, 20)
	m.Menu = menuState{Active: true}
	rows := menuRows(t, m)
	for i, row := range rows {
		rows[i] = stripANSI(row)
	}

	labelRow := func(label string) (int, int) {
		for i, row := range rows {
			if idx := strings.Index(row, label); idx >= 0 {
				return i, idx
			}
		}
		t.Fatalf("menu label %q not rendered:\n%s", label, strings.Join(rows, "\n"))
		return -1, -1
	}

	titleRow, titleCol := labelRow("actions")
	_, newCol := labelRow("new file")
	_, editCol := labelRow("edit filename")
	deleteRow, deleteCol := labelRow("delete file")

	if titleCol != newCol || newCol != editCol || editCol != deleteCol {
		t.Fatalf("entries are not left-aligned on one column: %d/%d/%d/%d", titleCol, newCol, editCol, deleteCol)
	}
	// Left-aligned inside a block that is itself centered in the pane.
	if titleCol <= 2 {
		t.Fatalf("block starts at column %d, want it centered in the pane", titleCol)
	}
	above := titleRow
	below := len(rows) - 1 - deleteRow
	if diff := above - below; diff > 1 || diff < -1 {
		t.Fatalf("block not centered vertically: %d rows above, %d below", above, below)
	}
}

func TestActionMenuMarksTheSelectedEntry(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := layoutModel(t, 90, 20)
	m.Menu = menuState{Active: true, Selected: 1}
	// Take the highlight's escape prefix from the style itself rather than
	// hardcoding an SGR sequence.
	rendered := styleMenuSelected.Render("x")
	highlight := rendered[:strings.Index(rendered, "x")]
	if highlight == "" {
		t.Fatal("setup: the selected style renders no escape sequence")
	}
	for _, row := range menuRows(t, m) {
		if !strings.Contains(row, "edit filename") {
			continue
		}
		if !strings.Contains(row, highlight) {
			t.Fatalf("selected entry is not highlighted: %q", row)
		}
		return
	}
	t.Fatal("selected entry not rendered")
}

func TestActionMenuLeavesTheSidebarVisible(t *testing.T) {
	m := layoutModel(t, 90, 20)
	m.Menu = menuState{Active: true}
	view := stripANSI(m.View())
	if !strings.Contains(view, "alpha.md") {
		t.Fatalf("sidebar hidden behind the menu:\n%s", view)
	}
}
