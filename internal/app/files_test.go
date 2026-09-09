package app

import (
	"fmt"
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
	m = selectMenuLabel(t, m, "edit filename")
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
	m = selectMenuLabel(t, m, "edit filename")
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
	m = selectMenuLabel(t, m, "delete file")
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
	m = selectMenuLabel(t, m, "go home")
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
	for _, want := range []string{"actions", "open", "new file", "quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("welcome screen menu missing %q:\n%s", want, view)
		}
	}
	// Renaming or deleting a file you have not opened makes no sense here, and
	// the mode actions have no meaning either.
	for _, unwanted := range []string{"edit filename", "delete file", "go home", "<> edit/preview", "save"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("welcome screen menu still offers %q:\n%s", unwanted, view)
		}
	}
}

func TestWelcomeMenuOpenOpensTheHighlightedFile(t *testing.T) {
	m := welcomeModel(t, "older.md", "newer.md")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown}) // second entry: older.md
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m = selectMenuLabel(t, m, "open")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Splash {
		t.Fatal("open did not leave the welcome screen")
	}
	if filepath.Base(m.Editor.File) != "older.md" {
		t.Fatalf("opened %q, want older.md", m.Editor.File)
	}
}

func TestWelcomeMenuQuitReturnsACommand(t *testing.T) {
	m := welcomeMenuModel(t, "one.md")
	m = selectMenuLabel(t, m, "quit")
	_, cmd := m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("quit from the welcome menu returned no command")
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
	if m.Menu.Selected != 0 {
		t.Fatalf("Menu.Selected = %d, want the first entry", m.Menu.Selected)
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

func TestWelcomeMenuNewFileOpensTheEditor(t *testing.T) {
	m := welcomeMenuModel(t, "one.md")
	m = selectMenuLabel(t, m, "new file")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
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
		m.handleMenuKey(tea.KeyMsg{Type: tea.KeyDown})
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
	m.handleMenuKey(tea.KeyMsg{Type: tea.KeyEnter})
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
	m := layoutModel(t, 90, 26)
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
	m.Menu = menuState{Active: true}
	m = selectMenuLabel(t, m, "cancel")
	fitted := fitMenuBlock(m.menuBlock(), 7, m.Menu.Selected)
	if len(fitted) > 7 {
		t.Fatalf("fitted block has %d rows, want at most 7", len(fitted))
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

// The menu box carries slack, so a label and its key never collide — some keys
// use glyphs the terminal draws wider than their reported width.
func TestActionMenuKeepsLabelAndKeyApart(t *testing.T) {
	m := layoutModel(t, 120, 30)
	m.Menu = menuState{Active: true}

	checked := 0
	for _, row := range m.menuBlock() {
		if row.Key == "" {
			continue
		}
		rendered := ""
		for _, r := range menuRows(t, m) {
			plain := stripANSI(r)
			if strings.Contains(plain, row.Label) && strings.Contains(plain, row.Key) {
				rendered = plain
				break
			}
		}
		if rendered == "" {
			t.Fatalf("row %q / %q not rendered", row.Label, row.Key)
		}
		start := strings.Index(rendered, row.Label) + len(row.Label)
		keyAt := strings.LastIndex(rendered, row.Key)
		gap := runewidth.StringWidth(rendered[start:keyAt])
		if gap < menuKeyGap {
			t.Fatalf("row %q: only %d columns between label and key %q, want at least %d",
				strings.TrimSpace(rendered), gap, row.Key, menuKeyGap)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no row with a key was checked")
	}
}

// The box is wider than its widest content row, so nothing sits flush against
// the backdrop edge.
func TestActionMenuBoxIsWiderThanItsContent(t *testing.T) {
	m := layoutModel(t, 120, 30)
	m.Menu = menuState{Active: true}

	widest := 0
	for _, row := range m.menuBlock() {
		w := runewidth.StringWidth(row.Label)
		if row.Key != "" {
			w += menuKeyGap + runewidth.StringWidth(row.Key)
		}
		widest = max(widest, w)
	}
	for _, r := range menuRows(t, m) {
		plain := stripANSI(r)
		if !strings.Contains(plain, "select all") {
			continue
		}
		start := strings.Index(plain, "select all")
		// The row's own padding starts menuPadX columns before the label.
		if start < menuPadX {
			t.Fatalf("label starts at column %d, want at least %d of padding", start, menuPadX)
		}
		return
	}
	t.Fatalf("no entry row rendered for a %d-column block", widest)
}

// --- delete sits apart at the bottom ---

func TestActionMenuPutsDeleteLastBehindABlankRow(t *testing.T) {
	m := layoutModel(t, 90, 30)
	m.Menu = menuState{Active: true}

	actions := m.menuActions()
	if second := actions[len(actions)-2]; second.Kind != menuDelete {
		t.Fatalf("second to last entry is %q, want delete file", second.Label)
	}

	block := m.menuBlock()
	deleteAt := -1
	for i, row := range block {
		if row.Kind == menuRowAction && row.Danger {
			deleteAt = i
		}
	}
	if deleteAt < 1 {
		t.Fatalf("delete row not found in %+v", block)
	}
	if block[deleteAt-1].Kind != menuRowBlank {
		t.Fatalf("row above delete is %v, want a blank row", block[deleteAt-1].Kind)
	}
	// Only the configuration submenu sits below it.
	for _, row := range block[deleteAt+1:] {
		if row.Kind == menuRowAction && row.Label != "configuration" {
			t.Fatalf("action %q rendered below delete file", row.Label)
		}
	}
}

func TestActionMenuRendersDeleteInRed(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := layoutModel(t, 90, 30)
	m.Menu = menuState{Active: true}
	danger := styleMenuDanger.Render("x")
	prefix := danger[:strings.Index(danger, "x")]
	for _, row := range menuRows(t, m) {
		if !strings.Contains(stripANSI(row), "delete file") {
			continue
		}
		if !strings.Contains(row, prefix) {
			t.Fatalf("delete row is not rendered in the danger colour: %q", row)
		}
		return
	}
	t.Fatal("delete row not rendered")
}

func TestActionMenuHighlightsSelectedDeleteAsDangerous(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := layoutModel(t, 90, 30)
	m.Menu = menuState{Active: true}
	m = selectMenuLabel(t, m, "delete file")
	selected := styleMenuDangerSelected.Render("x")
	prefix := selected[:strings.Index(selected, "x")]
	for _, row := range menuRows(t, m) {
		if strings.Contains(stripANSI(row), "delete file") && strings.Contains(row, prefix) {
			return
		}
	}
	t.Fatal("selected delete row does not use the danger highlight")
}

// Running it from the bottom of the list still asks for confirmation.
func TestActionMenuDeleteFromTheBottomStillConfirms(t *testing.T) {
	m, _ := projectModel(t)
	m.Menu = menuState{Active: true}
	m = selectMenuLabel(t, m, "delete file")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Prompt.Active || m.Prompt.Kind != promptDeleteConfirm {
		t.Fatalf("no delete confirmation: prompt=%+v status=%q", m.Prompt, m.Status)
	}
}

// --- toggle entries ---

func TestActionMenuHasToggleEntries(t *testing.T) {
	m, _ := projectModel(t)
	text := menuText(t, m)
	for _, want := range []string{"<> sidebar", "<> edit/preview"} {
		if !strings.Contains(text, want) {
			t.Fatalf("menu missing %q:\n%s", want, text)
		}
	}
	// The old hint row for the sidebar would now say the same thing twice.
	if strings.Count(text, "sidebar") != 1 {
		t.Fatalf("sidebar mentioned more than once:\n%s", text)
	}
}

// selectMenuLabel highlights an entry without disturbing the menu's level or
// filter.
func selectMenuLabel(t *testing.T, m Model, label string) Model {
	t.Helper()
	m.Menu.Active = true
	for i, entry := range m.menuActions() {
		if entry.Label == label {
			m.Menu.Selected = i
			return m
		}
	}
	t.Fatalf("menu entry %q not found", label)
	return m
}

func TestActionMenuTogglesTheSidebar(t *testing.T) {
	m, _ := projectModel(t)
	before := m.SidebarVisible
	m = selectMenuLabel(t, m, "<> sidebar")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.SidebarVisible == before {
		t.Fatalf("SidebarVisible still %v", m.SidebarVisible)
	}
}

func TestActionMenuTogglesEditAndPreview(t *testing.T) {
	m, _ := projectModel(t)
	if m.Mode != ModeEdit {
		t.Fatalf("setup: mode = %v", modeName(m.Mode))
	}
	m = selectMenuLabel(t, m, "<> edit/preview")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview", modeName(m.Mode))
	}
	m = selectMenuLabel(t, m, "<> edit/preview")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit again", modeName(m.Mode))
	}
}

// cancelEdit discards the buffer silently, so the toggle must not use it blindly.
func TestActionMenuToggleRefusesToDropUnsavedChanges(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	if !m.Editor.Dirty {
		t.Fatal("setup: buffer not dirty")
	}
	m = selectMenuLabel(t, m, "<> edit/preview")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want the switch refused", modeName(m.Mode))
	}
	if m.StatusKind != "warn" {
		t.Fatalf("StatusKind = %q, want warn: %q", m.StatusKind, m.Status)
	}
	if !m.Editor.Dirty {
		t.Fatal("unsaved changes were dropped")
	}
}

// Browse mode had no reference keys at all, so the "keys" section vanished.
func TestActionMenuShowsKeysSectionInEveryMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode Mode
	}{{"edit", ModeEdit}, {"preview", ModePreview}} {
		m, _ := projectModel(t)
		m.Mode = tc.mode
		if tc.mode == ModePreview {
			m.Focus = FocusPreview
		}
		text := menuText(t, m)
		if !strings.Contains(text, "keys") {
			t.Fatalf("%s mode: menu has no keys section:\n%s", tc.name, text)
		}
		if len(m.menuHints()) == 0 {
			t.Fatalf("%s mode: no reference keys", tc.name)
		}
	}
}

func TestActionMenuShowsCtrlTForTheSidebar(t *testing.T) {
	m, _ := projectModel(t)
	text := menuText(t, m)
	if !strings.Contains(text, "ctrl+t") {
		t.Fatalf("menu does not offer ctrl+t:\n%s", text)
	}
}

// The label column is as wide as the longest label, so the keys line up in
// their own column instead of drifting to the far edge.
func TestActionMenuAlignsKeysInTheirOwnColumn(t *testing.T) {
	m := layoutModel(t, 120, 34)
	m.Menu = menuState{Active: true}

	longest := 0
	for _, row := range m.menuBlock() {
		longest = max(longest, runewidth.StringWidth(row.Label))
	}
	starts := map[int]bool{}
	for _, row := range m.menuBlock() {
		if row.Key == "" {
			continue
		}
		for _, r := range menuRows(t, m) {
			plain := stripANSI(r)
			if !strings.Contains(plain, row.Label) || !strings.Contains(plain, row.Key) {
				continue
			}
			keyAt := strings.LastIndex(plain, row.Key)
			end := runewidth.StringWidth(plain[:keyAt]) + runewidth.StringWidth(row.Key)
			starts[end] = true
			labelEnd := strings.Index(plain, row.Label) + len(row.Label)
			if gap := runewidth.StringWidth(plain[labelEnd:keyAt]); gap < menuKeyGap {
				t.Fatalf("row %q: gap %d < %d", strings.TrimSpace(plain), gap, menuKeyGap)
			}
			break
		}
	}
	if len(starts) != 1 {
		t.Fatalf("keys end at %d different columns, want one shared column: %v", len(starts), starts)
	}
	if longest < 15 {
		t.Fatalf("longest label is %d columns, expected the toggle entry", longest)
	}
}

// --- the menu filter ---

func TestActionMenuStartsWithTheFilterFocused(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.Menu.Query != "" {
		t.Fatalf("Menu.Query = %q, want empty", m.Menu.Query)
	}
	if m.Menu.Selected != menuFilterFocus {
		t.Fatalf("Menu.Selected = %d, want the filter focused (%d)", m.Menu.Selected, menuFilterFocus)
	}
	// Nothing is highlighted while the filter has the keyboard.
	for _, row := range m.menuBlock() {
		if row.Kind == menuRowAction && row.Entry == m.Menu.Selected {
			t.Fatalf("entry %q is highlighted although the filter has focus", row.Label)
		}
	}
	if !strings.Contains(menuText(t, m), "actions") {
		t.Fatal("menu title missing")
	}
}

// down enters the list, up on the first entry comes back to the filter.
func TestActionMenuArrowsWalkBetweenFilterAndList(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})

	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.Menu.Selected != 0 {
		t.Fatalf("Menu.Selected = %d after down, want the first entry", m.Menu.Selected)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.Menu.Selected != 1 {
		t.Fatalf("Menu.Selected = %d after a second down, want 1", m.Menu.Selected)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.Menu.Selected != 0 {
		t.Fatalf("Menu.Selected = %d after up, want 0", m.Menu.Selected)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.Menu.Selected != menuFilterFocus {
		t.Fatalf("Menu.Selected = %d, want up on the first entry to return to the filter", m.Menu.Selected)
	}
	// It stops there instead of wrapping around.
	m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.Menu.Selected != menuFilterFocus {
		t.Fatalf("Menu.Selected = %d, want it to stay on the filter", m.Menu.Selected)
	}
}

// Typing anywhere puts the keyboard back on the filter.
func TestTypingReturnsFocusToTheFilter(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if m.Menu.Selected != menuFilterFocus {
		t.Fatalf("Menu.Selected = %d, want the filter focused again", m.Menu.Selected)
	}
	if m.Menu.Query != "u" {
		t.Fatalf("Menu.Query = %q, want %q", m.Menu.Query, "u")
	}
}

// The filter row shows where the keyboard is.
func TestFilterRowMarksItsFocus(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	caret := styleMenuCaret.Render(" ")
	filterRow := func(mm Model) string {
		for _, row := range menuRows(t, mm) {
			// The filter row starts with the prompt; other rows only carry a
			// "›" as their key, at the right.
			if strings.HasPrefix(strings.TrimSpace(stripANSI(row)), "│") {
				inner := strings.TrimSpace(strings.Trim(stripANSI(row), "│ "))
				if strings.HasPrefix(inner, "›") {
					return row
				}
			}
		}
		return ""
	}
	focused := filterRow(m)
	if !strings.Contains(focused, caret) {
		t.Fatalf("filter row has no caret while focused: %q", focused)
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	unfocused := filterRow(m)
	if strings.Contains(unfocused, caret) {
		t.Fatalf("filter row still carries the caret while the list has focus: %q", unfocused)
	}
}

// Typing goes straight into the filter, so no letter may be a navigation key.
func TestActionMenuTypingFiltersTheEntries(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "undo" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.Menu.Query != "undo" {
		t.Fatalf("Menu.Query = %q, want %q", m.Menu.Query, "undo")
	}
	matches := m.menuActions()
	if len(matches) != 1 || matches[0].Label != "undo" {
		t.Fatalf("filtered entries = %+v, want only undo", matches)
	}
	text := menuText(t, m)
	if strings.Contains(text, "new file") {
		t.Fatalf("unfiltered entry still shown:\n%s", text)
	}
	if !strings.Contains(text, "undo") {
		t.Fatalf("match not shown:\n%s", text)
	}
}

func TestActionMenuFilterIsCaseInsensitiveAndMatchesKeys(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "CTRL+N" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	matches := m.menuActions()
	if len(matches) != 1 || matches[0].Label != "new file" {
		t.Fatalf("filtered entries = %+v, want new file matched by its key", matches)
	}
}

func TestActionMenuEnterRunsTheFirstMatch(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "filename" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Prompt.Active || m.Prompt.Kind != promptRename {
		t.Fatalf("enter did not run the filtered match: prompt=%+v status=%q", m.Prompt, m.Status)
	}
}

func TestActionMenuBackspaceEditsTheFilter(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "undo" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.Menu.Query != "und" {
		t.Fatalf("Menu.Query = %q, want %q", m.Menu.Query, "und")
	}
}

// esc clears a filter first, so a typo does not close the menu.
func TestActionMenuEscClearsTheFilterBeforeClosing(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Menu.Active {
		t.Fatal("esc closed the menu instead of clearing the filter")
	}
	if m.Menu.Query != "" {
		t.Fatalf("Menu.Query = %q, want it cleared", m.Menu.Query)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Menu.Active {
		t.Fatal("esc did not close the menu on an empty filter")
	}
}

func TestActionMenuFilterWithoutMatchesSaysSo(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "zzz" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(m.menuActions()) != 0 {
		t.Fatalf("expected no matches, got %+v", m.menuActions())
	}
	if text := menuText(t, m); !strings.Contains(text, "no match") {
		t.Fatalf("menu does not report the empty result:\n%s", text)
	}
	// enter must not run anything.
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Prompt.Active {
		t.Fatal("enter ran an action although nothing matched")
	}
}

func TestActionMenuFilterWorksOnTheWelcomeScreen(t *testing.T) {
	m := welcomeMenuModel(t, "one.md")
	for _, r := range "new" {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.Menu.Query != "new" {
		t.Fatalf("Menu.Query = %q on the welcome screen", m.Menu.Query)
	}
	matches := m.menuActions()
	if len(matches) != 1 || matches[0].Kind != menuNewFile {
		t.Fatalf("filtered entries = %+v, want new file", matches)
	}
}

func TestActionMenuShowsTheFilterRow(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "sid" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	text := menuText(t, m)
	if !strings.Contains(text, "sid") {
		t.Fatalf("filter row does not show the query:\n%s", text)
	}
}

// --- the filter finds documents, not just actions ---

func TestActionMenuFilterFindsDocuments(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "beta" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	entries := m.menuActions()
	found := false
	for _, entry := range entries {
		if entry.Kind == menuOpenDoc && entry.Label == "beta.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("filter did not offer beta.md: %+v", entries)
	}
	text := menuText(t, m)
	if !strings.Contains(text, "beta.md") {
		t.Fatalf("document match not rendered:\n%s", text)
	}
	if strings.Contains(text, "no match") {
		t.Fatalf("menu still reports no match:\n%s", text)
	}
	if !strings.Contains(text, "files") {
		t.Fatalf("document matches are not labelled:\n%s", text)
	}
}

func TestActionMenuOpensAMatchedDocument(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "gamma" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Menu.Active {
		t.Fatal("menu stayed open")
	}
	if filepath.Base(m.Editor.File) != "gamma.md" {
		t.Fatalf("editing %q, want gamma.md", m.Editor.File)
	}
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
	if m.Menu.Query != "" {
		t.Fatalf("Menu.Query = %q, want it cleared after opening", m.Menu.Query)
	}
}

func TestActionMenuMatchesADocumentTitle(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	// alpha.md carries "# Alpha" as its title.
	for _, r := range "Alph" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	for _, entry := range m.menuActions() {
		if entry.Kind == menuOpenDoc && entry.Label == "alpha.md" {
			return
		}
	}
	t.Fatalf("title match not offered: %+v", m.menuActions())
}

// Without a query the menu is the action list, not a file browser.
func TestActionMenuListsNoDocumentsWithoutAQuery(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, entry := range m.menuActions() {
		if entry.Kind == menuOpenDoc {
			t.Fatalf("document %q offered without a query", entry.Label)
		}
	}
	if strings.Contains(menuText(t, m), "files") {
		t.Fatal("files section shown without a query")
	}
}

func TestActionMenuLimitsDocumentMatchesAndSaysHowMany(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	for i := 0; i < menuDocMatches+3; i++ {
		name := filepath.Join(root, fmt.Sprintf("note-%02d.md", i))
		if err := os.WriteFile(name, []byte("# Note\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := NewWithInitial(root, filepath.Join(root, "note-00.md"))
	m.Width, m.Height = 100, 40
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "note" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	docs := 0
	for _, entry := range m.menuActions() {
		if entry.Kind == menuOpenDoc {
			docs++
		}
	}
	if docs != menuDocMatches {
		t.Fatalf("%d document matches offered, want %d", docs, menuDocMatches)
	}
	if text := menuText(t, m); !strings.Contains(text, "3 more") {
		t.Fatalf("menu does not say how many matches were left out:\n%s", text)
	}
}

func TestActionMenuFilterFindsDocumentsOnTheWelcomeScreen(t *testing.T) {
	m := welcomeMenuModel(t, "alpha.md", "beta.md")
	for _, r := range "beta" {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Splash {
		t.Fatal("opening a match did not leave the welcome screen")
	}
	if filepath.Base(m.Editor.File) != "beta.md" {
		t.Fatalf("editing %q, want beta.md", m.Editor.File)
	}
}

func TestConfigurationSitsBelowDeleteBehindABlankRow(t *testing.T) {
	m := layoutModel(t, 90, 34)
	m.Menu = menuState{Active: true}

	actions := m.menuActions()
	if last := actions[len(actions)-1]; last.Label != "configuration" {
		t.Fatalf("last entry is %q, want configuration", last.Label)
	}
	if actions[len(actions)-2].Kind != menuDelete {
		t.Fatalf("entry above configuration is %q, want delete file", actions[len(actions)-2].Label)
	}

	block := m.menuBlock()
	for i, row := range block {
		if row.Kind == menuRowAction && row.Label == "configuration" {
			if block[i-1].Kind != menuRowBlank {
				t.Fatalf("row above configuration is %v, want a blank row", block[i-1].Kind)
			}
			return
		}
	}
	t.Fatal("configuration row not rendered")
}

// Both the destructive entry and the configuration below it stay visible.
func TestActionMenuPinsTheBottomGroupWhenItScrolls(t *testing.T) {
	m := layoutModel(t, 90, 34)
	m.Menu = menuState{Active: true, Selected: 0}
	fitted := fitMenuBlock(m.menuBlock(), 10, 0)
	if len(fitted) > 10 {
		t.Fatalf("fitted block has %d rows, want at most 10", len(fitted))
	}
	var danger, configuration bool
	for _, row := range fitted {
		if row.Danger {
			danger = true
		}
		if row.Label == "configuration" {
			configuration = true
		}
	}
	if !danger || !configuration {
		t.Fatalf("bottom group not pinned: danger=%v configuration=%v", danger, configuration)
	}
}

// The overlay covers both panes, so the sidebar does not sit beside it.
func TestActionMenuCoversTheWholeFrame(t *testing.T) {
	m := layoutModel(t, 90, 24)
	m.Menu = menuState{Active: true}
	rows := viewRows(t, m)

	frame := strings.Join(rows[m.contentTop():m.paneBottomRow()], "\n")
	if strings.Contains(frame, "alpha.md") {
		t.Fatalf("sidebar still visible beside the menu:\n%s", frame)
	}
	top := rows[m.contentTop()-1]
	bottom := rows[m.paneBottomRow()]
	if strings.Contains(top, "┬") || strings.Contains(bottom, "┴") {
		t.Fatalf("frame is still divided:\ntop=%q\nbottom=%q", top, bottom)
	}
	if !strings.HasPrefix(top, "┌") || !strings.HasSuffix(top, "┐") {
		t.Fatalf("top border = %q", top)
	}
	for row := m.contentTop(); row < m.paneBottomRow(); row++ {
		line := rows[row]
		if !strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│") {
			t.Fatalf("body row %d is not enclosed: %q", row, line)
		}
		if strings.Count(line, "│") != 2 {
			t.Fatalf("body row %d still carries a divider: %q", row, line)
		}
	}
	if len(rows) != m.Height {
		t.Fatalf("%d rows, want %d", len(rows), m.Height)
	}
}

func TestActionMenuOverlayKeepsEveryRowAtFullWidth(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 18}, {120, 30}, {60, 14}} {
		m := layoutModel(t, size.w, size.h)
		m.Menu = menuState{Active: true}
		for i, row := range viewRows(t, m) {
			if w := runewidth.StringWidth(row); w != m.Width {
				t.Fatalf("%dx%d: row %d has width %d, want %d (%q)", size.w, size.h, i, w, m.Width, row)
			}
		}
	}
}

// A click on the covered sidebar must not select a document behind the menu.
func TestClickIsIgnoredWhileTheMenuCoversTheFrame(t *testing.T) {
	m := layoutModel(t, 90, 24)
	m.Menu = menuState{Active: true}
	before := m.Selected
	m.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: 2, Y: m.contentTop() + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if m.Selected != before {
		t.Fatalf("Selected = %d, want %d: the click reached the covered sidebar", m.Selected, before)
	}
	if m.Focus == FocusSidebar {
		t.Fatal("focus moved to the covered sidebar")
	}
}
