package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/config"
)

// vaultModel is a project that is an Obsidian vault.
func vaultModel(t *testing.T) (Model, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "Work")
	if err := os.MkdirAll(filepath.Join(root, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "00 - Inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "00 - Inbox", "note.md"), []byte("# Note\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewWithInitial(root, filepath.Join(root, "00 - Inbox", "note.md"))
	m.Width, m.Height = 100, 30
	return m, root
}

func TestVaultIsDetected(t *testing.T) {
	m, root := vaultModel(t)
	v, ok := m.obsidianVault()
	if !ok {
		t.Fatal("no vault detected in an Obsidian project")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if v.Name != "Work" || v.Path != resolved {
		t.Fatalf("vault = %+v, want Work at %q", v, resolved)
	}
}

func TestNoVaultInAPlainProject(t *testing.T) {
	m, _ := projectModel(t)
	if v, ok := m.obsidianVault(); ok {
		t.Fatalf("detected %+v in a plain project", v)
	}
}

// A configured vault name wins over the detected one, so a symlinked or
// renamed directory can still be addressed.
func TestConfiguredVaultNameWins(t *testing.T) {
	m, _ := vaultModel(t)
	m.Cfg.Connections.Obsidian.Vault = "Privat"
	v, ok := m.obsidianVault()
	if !ok {
		t.Fatal("no vault")
	}
	if v.Name != "Privat" {
		t.Fatalf("Name = %q, want the configured name", v.Name)
	}
}

func TestVaultIsIgnoredWhenTheConnectionIsOff(t *testing.T) {
	m, _ := vaultModel(t)
	m.Cfg.Connections.Obsidian.Enabled = false
	if v, ok := m.obsidianVault(); ok {
		t.Fatalf("detected %+v although the connection is off", v)
	}
}

// --- backups stay out of the vault ---

func TestSavingInAVaultKeepsTheBackupOutside(t *testing.T) {
	m, root := vaultModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m.saveEditor()
	if m.StatusKind == "error" {
		t.Fatalf("save failed: %q", m.Status)
	}

	if _, err := os.Stat(filepath.Join(root, "00 - Inbox", "note.md.bak")); err == nil {
		t.Fatal("a .bak was written into the vault")
	}
	state := filepath.Join(os.Getenv("HOME"), ".local", "state", "glowed", "backups", "Work", "00 - Inbox", "note.md.bak")
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("no backup outside the vault: %v", err)
	}
	// The status says where it went.
	if !strings.Contains(m.Status, "backup") {
		t.Fatalf("Status = %q, want it to mention the backup", m.Status)
	}
}

func TestSavingOutsideAVaultKeepsTheBackupBeside(t *testing.T) {
	m, root := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m.saveEditor()
	if _, err := os.Stat(filepath.Join(root, "alpha.md.bak")); err != nil {
		t.Fatalf("no backup beside the document: %v", err)
	}
}

func TestBackupModeVaultKeepsTheOldBehaviour(t *testing.T) {
	m, root := vaultModel(t)
	m.Cfg.Connections.Obsidian.Backups = config.BackupsVault
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m.saveEditor()
	if _, err := os.Stat(filepath.Join(root, "00 - Inbox", "note.md.bak")); err != nil {
		t.Fatalf("no backup beside the document: %v", err)
	}
}

func TestBackupModeOffWritesNone(t *testing.T) {
	m, root := vaultModel(t)
	m.Cfg.Connections.Obsidian.Backups = config.BackupsOff
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m.saveEditor()
	if m.StatusKind == "error" {
		t.Fatalf("save failed: %q", m.Status)
	}
	if _, err := os.Stat(filepath.Join(root, "00 - Inbox", "note.md.bak")); err == nil {
		t.Fatal("a .bak was written although backups are off")
	}
	state := filepath.Join(os.Getenv("HOME"), ".local", "state", "glowed")
	if _, err := os.Stat(state); err == nil {
		t.Fatal("a backup was written outside although backups are off")
	}
	// The document itself was saved.
	body, err := os.ReadFile(filepath.Join(root, "00 - Inbox", "note.md"))
	if err != nil || !strings.Contains(string(body), "x") {
		t.Fatalf("document = %q (%v), want the edit saved", body, err)
	}
}

// Deleting inside a vault must not leave a .bak in it either.
func TestDeletingInAVaultKeepsTheBackupOutside(t *testing.T) {
	m, root := vaultModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = selectMenuLabel(t, m, "delete file")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.StatusKind == "error" {
		t.Fatalf("delete failed: %q", m.Status)
	}

	if _, err := os.Stat(filepath.Join(root, "00 - Inbox", "note.md")); err == nil {
		t.Fatal("the document was not deleted")
	}
	if _, err := os.Stat(filepath.Join(root, "00 - Inbox", "note.md.bak")); err == nil {
		t.Fatal("a .bak was left in the vault")
	}
	state := filepath.Join(os.Getenv("HOME"), ".local", "state", "glowed", "backups", "Work", "00 - Inbox", "note.md.bak")
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("the deleted document was not backed up outside: %v", err)
	}
}

// --- open in Obsidian ---

func TestActionMenuOffersOpenInObsidianInAVault(t *testing.T) {
	m, _ := vaultModel(t)
	if !strings.Contains(menuText(t, m), "open in Obsidian") {
		t.Fatalf("menu does not offer it:\n%s", menuText(t, m))
	}
	for _, entry := range m.menuActions() {
		if entry.Action == "openObsidian" {
			return
		}
	}
	t.Fatal("no runnable entry")
}

func TestActionMenuHidesOpenInObsidianWithoutAVault(t *testing.T) {
	m, _ := projectModel(t)
	if strings.Contains(menuText(t, m), "Obsidian") {
		t.Fatalf("menu offers Obsidian outside a vault:\n%s", menuText(t, m))
	}
}

func TestOpenInObsidianBuildsTheNoteURI(t *testing.T) {
	m, _ := vaultModel(t)
	uri, err := m.obsidianNoteURI()
	if err != nil {
		t.Fatalf("obsidianNoteURI: %v", err)
	}
	for _, want := range []string{"obsidian://open?", "vault=Work", "00+-+Inbox%2Fnote.md"} {
		if !strings.Contains(uri, want) {
			t.Fatalf("uri = %q, want it to contain %q", uri, want)
		}
	}
}

func TestOpenInObsidianReturnsACommand(t *testing.T) {
	m, _ := vaultModel(t)
	next, cmd := m.dispatch("openObsidian")
	if cmd == nil {
		t.Fatalf("no command returned: %q", next.Status)
	}
}

func TestOpenInObsidianWithoutAVaultWarns(t *testing.T) {
	m, _ := projectModel(t)
	next, cmd := m.dispatch("openObsidian")
	if cmd != nil {
		t.Fatal("a command was returned outside a vault")
	}
	if next.StatusKind != "warn" {
		t.Fatalf("StatusKind = %q, want warn: %q", next.StatusKind, next.Status)
	}
}

// --- configuration › connections › obsidian ---

func TestConfigurationOffersConnections(t *testing.T) {
	m, _ := vaultModel(t)
	m = openMenu(t, m, "configuration")
	text := menuText(t, m)
	for _, want := range []string{"defaults", "connections", "hotkeys"} {
		if !strings.Contains(text, want) {
			t.Fatalf("configuration submenu missing %q:\n%s", want, text)
		}
	}
}

func TestConnectionsSubmenuShowsObsidianAndItsState(t *testing.T) {
	m, _ := vaultModel(t)
	m = openMenu(t, m, "configuration", "connections")
	if !strings.Contains(menuText(t, m), "obsidian") {
		t.Fatalf("connections submenu missing obsidian:\n%s", menuText(t, m))
	}
	m = openMenu(t, m, "obsidian")
	text := menuText(t, m)
	for _, want := range []string{"enabled", "on", "vault", "Work", "backups", "outside"} {
		if !strings.Contains(text, want) {
			t.Fatalf("obsidian submenu missing %q:\n%s", want, text)
		}
	}
	// The detected vault is shown as such, so an empty setting is not confusing.
	if !strings.Contains(text, "detected") {
		t.Fatalf("the submenu does not say the vault was detected:\n%s", text)
	}
}

func TestTogglingTheObsidianConnectionPersists(t *testing.T) {
	m, _ := vaultModel(t)
	m = openMenu(t, m, "configuration", "connections", "obsidian")
	m = selectMenuLabel(t, m, "enabled")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.Cfg.Connections.Obsidian.Enabled {
		t.Fatal("the connection is still on")
	}
	saved, _ := config.Load(t.TempDir())
	if saved.Connections.Obsidian.Enabled {
		t.Fatal("the change was not written to the config file")
	}
	// Switching it off hides the vault-only entry again.
	if strings.Contains(menuText(t, m), "open in Obsidian") {
		t.Fatal("the menu still offers the Obsidian action")
	}
}

func TestBackupModeCyclesThroughItsThreeValues(t *testing.T) {
	m, _ := vaultModel(t)
	m = openMenu(t, m, "configuration", "connections", "obsidian")
	want := []string{config.BackupsVault, config.BackupsOff, config.BackupsOutside}
	for _, expected := range want {
		m = selectMenuLabel(t, m, "backups")
		m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		if m.Cfg.Connections.Obsidian.Backups != expected {
			t.Fatalf("Backups = %q, want %q", m.Cfg.Connections.Obsidian.Backups, expected)
		}
		saved, _ := config.Load(t.TempDir())
		if saved.Connections.Obsidian.Backups != expected {
			t.Fatalf("saved Backups = %q, want %q", saved.Connections.Obsidian.Backups, expected)
		}
	}
}

// The vault name is text, so the submenu needs an input row.
func TestVaultNameIsTypedIntoAPrompt(t *testing.T) {
	m, _ := vaultModel(t)
	m = openMenu(t, m, "configuration", "connections", "obsidian")
	m = selectMenuLabel(t, m, "vault")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.Prompt.Active || m.Prompt.Kind != promptObsidianVault {
		t.Fatalf("no vault prompt: %+v", m.Prompt)
	}
	if !strings.Contains(stripANSI(m.renderToolbar()), "obsidian vault:") {
		t.Fatalf("toolbar = %q", stripANSI(m.renderToolbar()))
	}
	for _, r := range "Privat" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.Cfg.Connections.Obsidian.Vault != "Privat" {
		t.Fatalf("Vault = %q, want Privat", m.Cfg.Connections.Obsidian.Vault)
	}
	saved, _ := config.Load(t.TempDir())
	if saved.Connections.Obsidian.Vault != "Privat" {
		t.Fatalf("saved Vault = %q", saved.Connections.Obsidian.Vault)
	}
	if m.Prompt.Active {
		t.Fatal("the prompt stayed open")
	}
	// It stays in the submenu, so a second setting needs no re-navigation.
	if m.menuLevel() != "configuration · connections · obsidian" {
		t.Fatalf("level = %q", m.menuLevel())
	}
}

// An empty input clears the override and goes back to detection.
func TestEmptyVaultNameReturnsToDetection(t *testing.T) {
	m, _ := vaultModel(t)
	m.Cfg.Connections.Obsidian.Vault = "Privat"
	m = openMenu(t, m, "configuration", "connections", "obsidian")
	m = selectMenuLabel(t, m, "vault")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	for range "Privat" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.Cfg.Connections.Obsidian.Vault != "" {
		t.Fatalf("Vault = %q, want it cleared", m.Cfg.Connections.Obsidian.Vault)
	}
	v, ok := m.obsidianVault()
	if !ok || v.Name != "Work" {
		t.Fatalf("vault = %+v, want the detected Work", v)
	}
}
