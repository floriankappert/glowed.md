package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
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
	m := layoutModel(t, 90, 30)
	m.Menu = menuState{Active: true}
	rows := menuRows(t, m)
	for i, row := range rows {
		rows[i] = stripANSI(row)
	}

	// Compare display columns, not byte offsets: the sidebar rows contain
	// multi-byte runes, which would shift a byte index.
	labelRow := func(label string) (int, int) {
		for i, row := range rows {
			if idx := strings.Index(row, label); idx >= 0 {
				return i, runewidth.StringWidth(row[:idx])
			}
		}
		t.Fatalf("menu label %q not rendered:\n%s", label, strings.Join(rows, "\n"))
		return -1, -1
	}

	titleRow, titleCol := labelRow("actions")
	_, newCol := labelRow("new file")
	_, editCol := labelRow("edit filename")
	_, deleteCol := labelRow("delete file")
	block := fitMenuBlock(m.menuBlock(), m.contentHeight(), m.Menu.Selected)
	if len(block) != len(m.menuBlock()) {
		t.Fatalf("setup: block does not fit, %d of %d rows", len(block), len(m.menuBlock()))
	}
	lastRow, lastCol := labelRow(block[len(block)-1].Label)

	if titleCol != newCol || newCol != editCol || editCol != deleteCol || deleteCol != lastCol {
		t.Fatalf("entries are not left-aligned on one column: %d/%d/%d/%d/%d", titleCol, newCol, editCol, deleteCol, lastCol)
	}
	// Left-aligned inside a block that is itself centered in the pane.
	if titleCol <= 2 {
		t.Fatalf("block starts at column %d, want it centered in the pane", titleCol)
	}
	above := titleRow
	below := len(rows) - 1 - lastRow
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

// --- go home ---

func TestActionMenuHasGoHome(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if !strings.Contains(stripANSI(m.View()), "go home") {
		t.Fatalf("action menu has no go home entry:\n%s", stripANSI(m.View()))
	}
}

func TestGoHomeShowsTheWelcomeScreen(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for i := 0; i < len(m.menuEntries()); i++ {
		if m.menuEntries()[m.Menu.Selected].Kind == menuGoHome {
			break
		}
		m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.Splash {
		t.Fatal("go home did not show the welcome screen")
	}
	if m.Menu.Active {
		t.Fatal("menu stayed open")
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "Recent files") {
		t.Fatalf("welcome screen not rendered:\n%s", view)
	}
}

func TestGoHomeRefusesUnsavedChanges(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	m.Menu = menuState{Active: true, Selected: goHomeIndex(t, m)}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Splash {
		t.Fatal("go home dropped an unsaved buffer")
	}
	if m.StatusKind != "warn" {
		t.Fatalf("StatusKind = %q, want warn: %q", m.StatusKind, m.Status)
	}
}

func goHomeIndex(t *testing.T, m Model) int {
	t.Helper()
	for i, entry := range m.menuEntries() {
		if entry.Kind == menuGoHome {
			return i
		}
	}
	t.Fatal("no go home entry")
	return -1
}

// Leaving to the welcome screen and coming back must land on a real document.
func TestGoHomeThenEnterReopensADocument(t *testing.T) {
	m, _ := projectModel(t)
	m.Menu = menuState{Active: true, Selected: goHomeIndex(t, m)}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Splash {
		t.Fatal("go home did not show the welcome screen")
	}
	// The welcome screen is gated in update, not in handleKey.
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Splash {
		t.Fatal("still on the welcome screen")
	}
	if m.Mode != ModeEdit || m.Editor.File == "" {
		t.Fatalf("mode=%v file=%q, want a document open for editing", modeName(m.Mode), m.Editor.File)
	}
}

// --- action menu on the welcome screen ---

func welcomeMenuModel(t *testing.T, names ...string) Model {
	t.Helper()
	m := welcomeModel(t, names...)
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if !m.Menu.Active {
		t.Fatal("ctrl+p did not open the action menu on the welcome screen")
	}
	return m
}

func TestActionMenuOpensOnTheWelcomeScreen(t *testing.T) {
	m := welcomeMenuModel(t, "one.md", "two.md")
	view := stripANSI(m.View())
	for _, want := range []string{"actions", "new file", "edit filename", "delete file"} {
		if !strings.Contains(view, want) {
			t.Fatalf("welcome screen menu missing %q:\n%s", want, view)
		}
	}
}

// "go home" is meaningless while already home.
func TestWelcomeMenuOmitsGoHome(t *testing.T) {
	m := welcomeMenuModel(t, "one.md")
	for _, entry := range m.menuEntries() {
		if entry.Kind == menuGoHome {
			t.Fatal("go home offered on the welcome screen")
		}
	}
	if strings.Contains(stripANSI(m.View()), "go home") {
		t.Fatal("go home rendered on the welcome screen")
	}
}

func TestWelcomeMenuArrowsAndEscWork(t *testing.T) {
	m := welcomeMenuModel(t, "one.md")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Menu.Selected != 1 {
		t.Fatalf("Menu.Selected = %d, want 1", m.Menu.Selected)
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Menu.Active {
		t.Fatal("esc did not close the menu")
	}
	if !m.Splash {
		t.Fatal("esc left the welcome screen")
	}
	// The recent-file list owns the arrows again.
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SplashSelected != 0 {
		t.Fatalf("SplashSelected = %d, want the single entry clamped at 0", m.SplashSelected)
	}
}

// On the welcome screen the actions target the highlighted recent file, not
// whatever the sidebar selection happens to be.
func TestWelcomeMenuRenameTargetsTheHighlightedFile(t *testing.T) {
	m := welcomeModel(t, "older.md", "newer.md")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown}) // second entry: older.md
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Prompt.Input != "older.md" {
		t.Fatalf("Prompt.Input = %q, want older.md", m.Prompt.Input)
	}
}

func TestWelcomeMenuRenameKeepsYouHome(t *testing.T) {
	m := welcomeModel(t, "one.md", "two.md")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	for range m.Prompt.Input {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "renamed.md" {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.Splash {
		t.Fatal("rename left the welcome screen")
	}
	if _, err := os.Stat(filepath.Join(m.Root, "renamed.md")); err != nil {
		t.Fatalf("file not renamed: %v", err)
	}
	if !strings.Contains(stripANSI(m.View()), "renamed.md") {
		t.Fatal("recent list not refreshed after the rename")
	}
}

func TestWelcomeMenuDeleteKeepsYouHome(t *testing.T) {
	m := welcomeModel(t, "one.md", "two.md")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(toolbarText(m), "two.md") {
		t.Fatalf("confirmation does not name the highlighted file: %q", toolbarText(m))
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if !m.Splash {
		t.Fatal("delete left the welcome screen")
	}
	if _, err := os.Stat(filepath.Join(m.Root, "two.md")); err == nil {
		t.Fatal("file not deleted")
	}
	if strings.Contains(stripANSI(m.View()), "two.md") {
		t.Fatal("deleted file still listed")
	}
}

func TestWelcomeMenuNewFileOpensTheEditor(t *testing.T) {
	m := welcomeMenuModel(t, "one.md")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter}) // new file
	for _, r := range "fresh" {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Splash {
		t.Fatal("creating a file did not leave the welcome screen")
	}
	if filepath.Base(m.Editor.File) != "fresh.md" {
		t.Fatalf("editing %q, want fresh.md", m.Editor.File)
	}
}

// --- the footer hints moved into the action menu ---

// menuText returns the plain text of the whole menu overlay.
func menuText(t *testing.T, m Model) string {
	t.Helper()
	m.Menu.Active = true
	rows := []string{}
	for i := 0; ; i++ {
		row, ok := m.renderMenuRow(60, 24, i)
		if !ok || i > 40 {
			break
		}
		rows = append(rows, stripANSI(row))
	}
	return strings.Join(rows, "\n")
}

func TestFooterRowIsGone(t *testing.T) {
	m := layoutModel(t, 90, 20)
	rows := viewRows(t, m)
	if len(rows) != m.Height {
		t.Fatalf("%d rows, want %d", len(rows), m.Height)
	}
	last := rows[len(rows)-1]
	if !strings.Contains(last, "Path:") {
		t.Fatalf("last row = %q, want the toolbar now that the footer is gone", last)
	}
	for _, row := range rows {
		if strings.Contains(row, "ctrl+s save") {
			t.Fatalf("footer hints still rendered: %q", row)
		}
	}
}

func TestToolbarKeepsTheActionMenuDiscoverable(t *testing.T) {
	m := layoutModel(t, 90, 20)
	toolbar := stripANSI(m.renderToolbar())
	if !strings.Contains(toolbar, "ctrl+p") || !strings.Contains(toolbar, "actions") {
		t.Fatalf("toolbar = %q, want a ctrl+p actions hint", toolbar)
	}
}

func TestActionMenuShowsEditHintsInEditMode(t *testing.T) {
	m := editModel([]string{"hello"}, 0, 0)
	text := menuText(t, m)
	for _, want := range []string{"save", "ctrl+s", "select all", "cancel", "word"} {
		if !strings.Contains(text, want) {
			t.Fatalf("edit menu missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "quit") {
		t.Fatalf("edit menu still offers quit:\n%s", text)
	}
}

func TestActionMenuShowsBrowseHintsInPreviewMode(t *testing.T) {
	m := editModel([]string{"hello"}, 0, 0)
	m.Mode = ModePreview
	m.Focus = FocusPreview
	text := menuText(t, m)
	for _, want := range []string{"quit", "search"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview menu missing %q:\n%s", want, text)
		}
	}
}

func TestActionMenuShowsSidebarHintsWhenSidebarFocused(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})
	text := menuText(t, m)
	for _, want := range []string{"open", "editor"} {
		if !strings.Contains(text, want) {
			t.Fatalf("sidebar menu missing %q:\n%s", want, text)
		}
	}
}

func TestActionMenuShowsTheKeyForAnEntry(t *testing.T) {
	m, _ := projectModel(t)
	text := menuText(t, m)
	if !strings.Contains(text, "ctrl+n") {
		t.Fatalf("menu does not show the new-file key:\n%s", text)
	}
}

// A hint without an action is not selectable, so the selection never lands on it.
func TestActionMenuSelectionOnlyVisitsRunnableEntries(t *testing.T) {
	m, _ := projectModel(t)
	m.Menu = menuState{Active: true}
	entries := m.menuActions()
	if len(entries) < 5 {
		t.Fatalf("only %d runnable entries, expected the mode actions too", len(entries))
	}
	for i := 0; i < len(entries)+3; i++ {
		if m.Menu.Selected < 0 || m.Menu.Selected >= len(entries) {
			t.Fatalf("selection %d out of range for %d entries", m.Menu.Selected, len(entries))
		}
		m.handleMenuKey("down")
	}
	for _, e := range entries {
		if e.Kind == menuDispatch && e.Action == "" {
			t.Fatalf("entry %q is selectable but has no action", e.Label)
		}
	}
}

func TestActionMenuRunsAModeAction(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if !m.Editor.Dirty {
		t.Fatal("setup: buffer not dirty")
	}
	m.Menu = menuState{Active: true}
	for i, entry := range m.menuActions() {
		if entry.Action == "save" {
			m.Menu.Selected = i
		}
	}
	m.handleMenuKey("enter")
	if m.Editor.Dirty {
		t.Fatalf("save from the menu did not save: %q", m.Status)
	}
	if m.Menu.Active {
		t.Fatal("menu stayed open after running an action")
	}
}

// A pane too short for the whole menu drops the reference hints first and never
// loses a runnable action.
func TestActionMenuFitsAShortPane(t *testing.T) {
	// Tall enough for every action, too short for the hint section.
	m := layoutModel(t, 90, 20)
	m.Menu = menuState{Active: true}
	full := m.menuBlock()
	fitted := fitMenuBlock(full, m.contentHeight(), m.Menu.Selected)

	if len(fitted) > m.contentHeight() {
		t.Fatalf("fitted block has %d rows, pane has %d", len(fitted), m.contentHeight())
	}
	if len(fitted) >= len(full) {
		t.Fatalf("setup: block already fits (%d rows in %d)", len(full), m.contentHeight())
	}
	for _, row := range full {
		if row.Kind != menuRowAction {
			continue
		}
		found := false
		for _, kept := range fitted {
			if kept.Entry == row.Entry && kept.Kind == menuRowAction {
				found = true
			}
		}
		if !found {
			t.Fatalf("runnable action %q was dropped", row.Label)
		}
	}
}

// When even the actions do not fit, the selected one stays on screen.
func TestActionMenuKeepsTheSelectionVisibleWhenScrolling(t *testing.T) {
	m := layoutModel(t, 90, 14)
	m.Menu = menuState{Active: true, Selected: len(m.menuActions()) - 1}
	fitted := fitMenuBlock(m.menuBlock(), 5, m.Menu.Selected)
	if len(fitted) > 5 {
		t.Fatalf("fitted block has %d rows, want at most 5", len(fitted))
	}
	visible := false
	for _, row := range fitted {
		if row.Kind == menuRowAction && row.Entry == m.Menu.Selected {
			visible = true
		}
	}
	if !visible {
		t.Fatalf("selected entry %d not visible in %+v", m.Menu.Selected, fitted)
	}
	if fitted[0].Kind != menuRowTitle {
		t.Fatal("title not pinned")
	}
}
