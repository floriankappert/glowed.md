package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/config"
)

// configModel isolates HOME, so a saved default never touches the real config.
func configModel(t *testing.T) Model {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	m, _ := projectModel(t)
	m.Cfg = config.Default()
	return m
}

func openMenu(t *testing.T, m Model, labels ...string) Model {
	t.Helper()
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, label := range labels {
		m = selectMenuLabel(t, m, label)
		m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	}
	return m
}

func TestActionMenuOffersConfiguration(t *testing.T) {
	m := configModel(t)
	if !strings.Contains(menuText(t, m), "configuration") {
		t.Fatalf("menu has no configuration entry:\n%s", menuText(t, m))
	}
}

func TestConfigurationOpensASubmenu(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration")
	if !m.Menu.Active {
		t.Fatal("submenu closed the menu")
	}
	text := menuText(t, m)
	if !strings.Contains(text, "defaults") {
		t.Fatalf("configuration submenu missing defaults:\n%s", text)
	}
	// The breadcrumb says where you are.
	if !strings.Contains(text, "configuration") {
		t.Fatalf("submenu title does not show the path:\n%s", text)
	}
	// Top-level entries are gone at this level.
	if strings.Contains(text, "new file") {
		t.Fatalf("top-level entries still shown:\n%s", text)
	}
}

func TestDefaultsSubmenuShowsBothTogglesAndTheirState(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration", "defaults")
	text := menuText(t, m)
	for _, want := range []string{"edit mode as default", "sidebar visible as default", "on"} {
		if !strings.Contains(text, want) {
			t.Fatalf("defaults submenu missing %q:\n%s", want, text)
		}
	}
}

func TestTogglingEditModeDefaultPersistsIt(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration", "defaults")
	m = selectMenuLabel(t, m, "edit mode as default")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.Cfg.Defaults.EditMode {
		t.Fatal("edit mode default still on")
	}
	if m.StatusKind == "error" {
		t.Fatalf("save failed: %q", m.Status)
	}
	// The submenu stays open so a second toggle needs no re-navigation.
	if !m.Menu.Active {
		t.Fatal("menu closed after a toggle")
	}
	if !strings.Contains(menuText(t, m), "off") {
		t.Fatalf("toggle state not reflected:\n%s", menuText(t, m))
	}

	saved, errs := config.Load(t.TempDir())
	if len(errs) > 0 {
		t.Fatalf("reload: %v", errs)
	}
	if saved.Defaults.EditMode {
		t.Fatal("edit mode default was not written to the config file")
	}
	if !saved.Defaults.SidebarVisible {
		t.Fatal("the other default was changed too")
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".config", "glowed", "config.json")); err != nil {
		t.Fatalf("config file missing: %v", err)
	}
}

func TestTogglingSidebarDefaultPersistsIt(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration", "defaults")
	m = selectMenuLabel(t, m, "sidebar visible as default")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Cfg.Defaults.SidebarVisible {
		t.Fatal("the running model still has the sidebar default on")
	}

	saved, _ := config.Load(t.TempDir())
	if saved.Defaults.SidebarVisible {
		t.Fatal("sidebar default was not written")
	}
	if !saved.Defaults.EditMode {
		t.Fatal("edit mode default changed too")
	}
}

// The saved defaults must actually drive the next launch.
func TestSavedDefaultsDriveStartup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := config.SaveDefaults(config.DefaultsConfig{EditMode: false, SidebarVisible: false}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewWithInitial(root, filepath.Join(root, "a.md"))
	if m.SidebarVisible {
		t.Fatal("sidebar visible although the default is off")
	}
	if m.Mode == ModeEdit {
		t.Fatalf("Mode = %v, want preview because the edit default is off", modeName(m.Mode))
	}
}

func TestEscLeavesASubmenuBeforeClosingTheMenu(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration", "defaults")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Menu.Active {
		t.Fatal("esc closed the menu instead of going up one level")
	}
	if !strings.Contains(menuText(t, m), "defaults") {
		t.Fatalf("esc did not land on the configuration level:\n%s", menuText(t, m))
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Menu.Active {
		t.Fatal("esc closed the menu instead of returning to the top level")
	}
	if !strings.Contains(menuText(t, m), "new file") {
		t.Fatalf("esc did not land on the top level:\n%s", menuText(t, m))
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Menu.Active {
		t.Fatal("esc did not close the menu at the top level")
	}
}

// A submenu has no filter bar, so typing must not filter invisibly.
func TestSubmenuHasNoFilter(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration", "defaults")
	if m.menuShowsFilter() {
		t.Fatal("submenu still shows the filter row")
	}
	for _, r := range "sidebar" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.Menu.Query != "" {
		t.Fatalf("Menu.Query = %q, want typing ignored in a submenu", m.Menu.Query)
	}
	if len(m.menuActions()) != 2 {
		t.Fatalf("%d entries, want both toggles untouched", len(m.menuActions()))
	}
	for _, row := range m.menuBlock() {
		if row.Kind == menuRowFilter {
			t.Fatal("filter row rendered in a submenu")
		}
	}
}

// The top level keeps its filter.
func TestTopLevelKeepsTheFilter(t *testing.T) {
	m := configModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	if !m.menuShowsFilter() {
		t.Fatal("top level has no filter row")
	}
}

// Reopening the menu starts at the top again.
func TestReopeningTheMenuResetsTheLevel(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP}) // close
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP}) // open
	if !strings.Contains(menuText(t, m), "new file") {
		t.Fatalf("menu did not reopen at the top level:\n%s", menuText(t, m))
	}
}

// The status line must not promise a filter that a submenu does not have.
func TestSubmenuStatusDoesNotMentionTyping(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration")
	if strings.Contains(m.Status, "type to filter") {
		t.Fatalf("Status = %q, want no filter hint in a submenu", m.Status)
	}
	if !strings.Contains(m.Status, "configuration") {
		t.Fatalf("Status = %q, want it to name the level", m.Status)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(m.Status, "type to filter") {
		t.Fatalf("Status = %q, want the filter hint back at the top level", m.Status)
	}
}

// The welcome screen needs the settings and the filter as much as the main
// window does.
func TestWelcomeMenuHasConfigurationAndFilter(t *testing.T) {
	m := welcomeMenuModel(t, "one.md", "two.md")
	if !m.menuShowsFilter() {
		t.Fatal("welcome menu has no filter row")
	}
	text := menuText(t, m)
	if !strings.Contains(text, "configuration") {
		t.Fatalf("welcome menu has no configuration entry:\n%s", text)
	}
	if !strings.Contains(text, "›") {
		t.Fatalf("welcome menu shows no filter prompt:\n%s", text)
	}
}

func TestWelcomeMenuReachesTheDefaultsAndSavesThem(t *testing.T) {
	m := welcomeMenuModel(t, "one.md")
	m = selectMenuLabel(t, m, "configuration")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	m = selectMenuLabel(t, m, "defaults")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	m = selectMenuLabel(t, m, "sidebar visible as default")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.StatusKind == "error" {
		t.Fatalf("save failed: %q", m.Status)
	}
	saved, _ := config.Load(t.TempDir())
	if saved.Defaults.SidebarVisible {
		t.Fatal("the welcome screen could not change the default")
	}
	if !m.Splash {
		t.Fatal("configuring dropped the welcome screen")
	}
}

func TestWelcomeMenuFilterMatchesConfiguration(t *testing.T) {
	m := welcomeMenuModel(t, "one.md")
	for _, r := range "conf" {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	entries := m.menuActions()
	if len(entries) != 1 || entries[0].Kind != menuSubmenu {
		t.Fatalf("filtered entries = %+v, want the configuration submenu", entries)
	}
}

// --- the edit-mode default governs how documents open ---

// previewDefaultModel is a project whose edit-mode default is off, as if it had
// been switched off in the configuration menu and glowed restarted.
func previewDefaultModel(t *testing.T) (Model, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := config.SaveDefaults(config.DefaultsConfig{EditMode: false, SidebarVisible: true}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, name := range []string{"todo.md", "other.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("# "+name+"\n\nbody\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := New(root)
	m.Width, m.Height = 100, 30
	if m.Cfg.Defaults.EditMode {
		t.Fatal("setup: the edit-mode default is still on")
	}
	return m, root
}

// The reported case: switch the default off, restart, open todo.md from the
// welcome screen, expect the preview.
func TestWelcomeOpenHonoursTheEditModeDefault(t *testing.T) {
	m, _ := previewDefaultModel(t)
	if !m.Splash {
		t.Fatal("setup: no welcome screen")
	}
	m = selectRecent(t, m, "todo.md")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.Splash {
		t.Fatal("welcome screen still up")
	}
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview", modeName(m.Mode))
	}
	if m.Focus != FocusPreview {
		t.Fatalf("Focus = %v, want preview", focusName(m.Focus))
	}
	if doc := m.currentDoc(); doc == nil || doc.Rel != "todo.md" {
		t.Fatalf("opened %v, want todo.md", doc)
	}
}

func selectRecent(t *testing.T, m Model, rel string) Model {
	t.Helper()
	for i, doc := range m.recentDocs() {
		if doc.Rel == rel {
			m.SplashSelected = i
			return m
		}
	}
	t.Fatalf("%q is not among the recent documents", rel)
	return m
}

func TestMenuDocumentMatchHonoursTheEditModeDefault(t *testing.T) {
	m, _ := previewDefaultModel(t)
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlP})
	for _, r := range "todo" {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview", modeName(m.Mode))
	}
	if doc := m.currentDoc(); doc == nil || doc.Rel != "todo.md" {
		t.Fatalf("opened %v, want todo.md", doc)
	}
}

func TestSidebarOpenHonoursTheEditModeDefault(t *testing.T) {
	m, _ := previewDefaultModel(t)
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter}) // leave the welcome screen
	m.ensureSidebarState()
	m.rebuildSidebarRows()
	m.Focus = FocusSidebar
	for i, row := range m.SidebarRows {
		if row.Kind == sidebarRowDocument {
			m.setSidebarSelection(i)
			break
		}
	}
	m.openSidebarSelection()
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview", modeName(m.Mode))
	}
}

// An explicit request beats the default.
func TestExplicitEditStillEntersEditMode(t *testing.T) {
	m, _ := previewDefaultModel(t)
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	next, _ := m.dispatch("edit")
	if next.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit for an explicit edit", modeName(next.Mode))
	}
	next, _ = next.dispatch("toggleMode")
	if next.Mode != ModePreview {
		t.Fatalf("Mode = %v, want the toggle to work", modeName(next.Mode))
	}
}

// A brand new file has nothing to preview, so creating one still opens the editor.
func TestNewFileStillOpensTheEditor(t *testing.T) {
	m, root := previewDefaultModel(t)
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlN})
	for _, r := range "fresh" {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit for a new file", modeName(m.Mode))
	}
	if _, err := os.Stat(filepath.Join(root, "fresh.md")); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

// With the default on, everything still opens in the editor.
func TestEditModeDefaultOnStillOpensTheEditor(t *testing.T) {
	m := configModel(t)
	if !m.Cfg.Defaults.EditMode {
		t.Fatal("setup: the default is off")
	}
	m.Splash = true
	m.SplashSelected = 0
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
}

// Launching with an explicit file and the default off must still show that
// file — in the preview.
func TestExplicitFileWithEditDefaultOffShowsThePreview(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := config.SaveDefaults(config.DefaultsConfig{EditMode: false, SidebarVisible: true}); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "todo.md")
	if err := os.WriteFile(path, []byte("# Todo\n\nfirst item\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewWithInitial(root, path)
	m.Width, m.Height = 100, 30

	if m.Splash {
		t.Fatal("welcome screen shown for an explicit file")
	}
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview", modeName(m.Mode))
	}
	if doc := m.currentDoc(); doc == nil || doc.Rel != "todo.md" {
		t.Fatalf("current document = %v, want todo.md", doc)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "Todo") {
		t.Fatalf("the document is not rendered:\n%s", view)
	}
}

// --- llm off by default, clearer source label ---

func TestLLMIsDisabledByDefault(t *testing.T) {
	if config.Default().LLM.Enabled {
		t.Fatal("LLM.Enabled = true, want it off by default")
	}
}

// A disabled feature must not sit in the menu as an entry that only warns.
func TestActionMenuHidesTheLLMEntryWhileItIsDisabled(t *testing.T) {
	m := configModel(t)
	m.Mode = ModePreview
	m.Focus = FocusPreview
	if m.Cfg.LLM.Enabled {
		t.Fatal("setup: llm is enabled")
	}
	if text := menuText(t, m); strings.Contains(text, "llm") {
		t.Fatalf("menu still offers llm:\n%s", text)
	}
	for _, entry := range m.menuActions() {
		if entry.Action == "openLLM" {
			t.Fatal("openLLM is still runnable from the menu")
		}
	}
}

func TestActionMenuShowsTheLLMEntryWhenEnabled(t *testing.T) {
	m := configModel(t)
	m.Mode = ModePreview
	m.Focus = FocusPreview
	m.Cfg.LLM.Enabled = true
	if text := menuText(t, m); !strings.Contains(text, "llm") {
		t.Fatalf("menu does not offer llm although it is enabled:\n%s", text)
	}
}

// "source" said nothing about what the mode does.
func TestSourceActionHasATellingLabel(t *testing.T) {
	label := labelForAction("sourceSelect")
	if label == "source" || label == "" {
		t.Fatalf("labelForAction(sourceSelect) = %q, want something that explains it", label)
	}
	m := configModel(t)
	m.Mode = ModePreview
	m.Focus = FocusPreview
	if text := menuText(t, m); !strings.Contains(text, label) {
		t.Fatalf("menu does not show %q:\n%s", label, text)
	}
}

// --- the mode entry names the mode it switches to ---

func TestModeEntryIsCalledPreviewWhileEditing(t *testing.T) {
	m := configModel(t)
	if m.Mode != ModeEdit {
		t.Fatalf("setup: mode = %v", modeName(m.Mode))
	}
	var found *menuEntry
	for i, entry := range m.menuActions() {
		if entry.Action == "toggleMode" {
			found = &m.menuActions()[i]
		}
	}
	if found == nil {
		t.Fatalf("no mode entry: %+v", m.menuActions())
	}
	if found.Label != "Preview" || found.Key != "esc" {
		t.Fatalf("mode entry = %q / %q, want Preview / esc", found.Label, found.Key)
	}
	if strings.Contains(menuText(t, m), "edit/preview") {
		t.Fatalf("the old label is still rendered:\n%s", menuText(t, m))
	}
	// The mode hint it replaces must not sit beside it.
	if strings.Contains(menuText(t, m), "cancel") {
		t.Fatalf("cancel duplicates the mode entry:\n%s", menuText(t, m))
	}
}

func TestModeEntryIsCalledEditWhilePreviewing(t *testing.T) {
	m := configModel(t)
	m.Mode = ModePreview
	m.Focus = FocusPreview
	var found *menuEntry
	entries := m.menuActions()
	for i, entry := range entries {
		if entry.Action == "toggleMode" {
			found = &entries[i]
		}
	}
	if found == nil {
		t.Fatalf("no mode entry: %+v", entries)
	}
	if found.Label != "Edit" || found.Key != "e" {
		t.Fatalf("mode entry = %q / %q, want Edit / e", found.Label, found.Key)
	}
	// "edit" from the mode actions would say the same thing twice.
	count := 0
	for _, entry := range entries {
		if entry.Action == "edit" || entry.Action == "toggleMode" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("%d entries switch to the editor, want 1: %+v", count, entries)
	}
}

func TestModeEntryStillSwitchesTheMode(t *testing.T) {
	m := configModel(t)
	m.Menu = menuState{Active: true}
	m = selectMenuLabel(t, m, "Preview")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModePreview {
		t.Fatalf("Mode = %v, want ModePreview", modeName(m.Mode))
	}
	m.Menu = menuState{Active: true}
	m = selectMenuLabel(t, m, "Edit")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
}

// --- configuration › hotkeys ---

func TestConfigurationOffersHotkeys(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration")
	text := menuText(t, m)
	for _, want := range []string{"defaults", "hotkeys"} {
		if !strings.Contains(text, want) {
			t.Fatalf("configuration submenu missing %q:\n%s", want, text)
		}
	}
}

func TestHotkeysLevelListsTheBindings(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration", "hotkeys")
	if m.menuLevel() != "configuration · hotkeys" {
		t.Fatalf("level = %q", m.menuLevel())
	}
	hints := m.menuHints()
	if len(hints) < 20 {
		t.Fatalf("%d hotkeys listed, expected the full set", len(hints))
	}
	// Every row carries a key, and the sections group them.
	for _, hint := range hints {
		if hint.Label == "" {
			t.Fatalf("hotkey row without a label: %+v", hint)
		}
	}
	text := menuText(t, m)
	for _, want := range []string{"ctrl+p", "ctrl+n", "ctrl+t", "ctrl+s", "opt+←→", "esc"} {
		if !strings.Contains(text, want) {
			t.Fatalf("hotkey list missing %q:\n%s", want, text)
		}
	}
	// It is a reference list: nothing to run.
	if len(m.menuActions()) != 0 {
		t.Fatalf("hotkeys level offers runnable entries: %+v", m.menuActions())
	}
}

// A list that long has to scroll, and it has no selection to follow.
func TestHotkeysLevelScrolls(t *testing.T) {
	m := configModel(t)
	m.Width, m.Height = 90, 20
	m = openMenu(t, m, "configuration", "hotkeys")

	// Render into a window smaller than the list, the way a real pane is.
	window := func(mm Model) string {
		rows := []string{}
		for i := 0; i < 14; i++ {
			row, _ := mm.renderMenuRow(60, 14, i)
			rows = append(rows, stripANSI(row))
		}
		return strings.Join(rows, "\n")
	}

	first := window(m)
	for i := 0; i < 8; i++ {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.Menu.Scroll == 0 {
		t.Fatal("down did not scroll the list")
	}
	if window(m) == first {
		t.Fatalf("the rendered list did not change:\n%s", window(m))
	}
	// The title stays put while the body scrolls.
	if !strings.Contains(window(m), "hotkeys") {
		t.Fatalf("the title scrolled away:\n%s", window(m))
	}
	for i := 0; i < 40; i++ {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyUp})
	}
	if m.Menu.Scroll != 0 {
		t.Fatalf("Menu.Scroll = %d, want it back at the top", m.Menu.Scroll)
	}
	if window(m) != first {
		t.Fatal("scrolling back to the top did not restore the list")
	}
}

// Scrolling stops at the end instead of running past it.
func TestHotkeysScrollStopsAtTheEnd(t *testing.T) {
	m := configModel(t)
	m.Width, m.Height = 90, 20
	m = openMenu(t, m, "configuration", "hotkeys")
	for i := 0; i < 200; i++ {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	block := m.menuBlock()
	body := len(block) - leadingChromeRows(block)
	visible := m.menuHeight() - leadingChromeRows(block)
	if m.Menu.Scroll > max(0, body-visible) {
		t.Fatalf("Menu.Scroll = %d, want at most %d", m.Menu.Scroll, max(0, body-visible))
	}
	// The last binding is reachable.
	rows := fitMenuBlock(block, m.menuHeight(), m.Menu.Selected, m.Menu.Scroll)
	last := block[len(block)-1]
	if rows[len(rows)-1].Label != last.Label {
		t.Fatalf("bottom of the list = %q, want %q", rows[len(rows)-1].Label, last.Label)
	}
}

func TestHotkeysLevelEscGoesBack(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration", "hotkeys")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.menuLevel() != "configuration" {
		t.Fatalf("level = %q, want configuration", m.menuLevel())
	}
	if m.Menu.Scroll != 0 {
		t.Fatalf("Menu.Scroll = %d, want it reset when leaving", m.Menu.Scroll)
	}
}
