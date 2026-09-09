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

// A filter typed in a submenu filters that level, not the top one.
func TestFilterAppliesToTheCurrentLevel(t *testing.T) {
	m := configModel(t)
	m = openMenu(t, m, "configuration", "defaults")
	for _, r := range "sidebar" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	entries := m.menuActions()
	if len(entries) != 1 || !strings.Contains(entries[0].Label, "sidebar") {
		t.Fatalf("filtered entries = %+v, want the sidebar toggle only", entries)
	}
	// Documents are a top-level concern.
	for _, entry := range entries {
		if entry.Kind == menuOpenDoc {
			t.Fatalf("submenu offered a document: %q", entry.Label)
		}
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
