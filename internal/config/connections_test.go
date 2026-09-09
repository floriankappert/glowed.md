package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionDefaults(t *testing.T) {
	cfg := Default()
	if !cfg.Connections.Obsidian.Enabled {
		t.Fatal("the obsidian connection should be on: it needs nothing but a vault")
	}
	if cfg.Connections.Obsidian.Vault != "" {
		t.Fatalf("Vault = %q, want empty so the vault is detected", cfg.Connections.Obsidian.Vault)
	}
	if cfg.Connections.Obsidian.Backups != BackupsOutside {
		t.Fatalf("Backups = %q, want %q: a synced vault must not collect .bak files",
			cfg.Connections.Obsidian.Backups, BackupsOutside)
	}
}

func TestLoadReadsTheObsidianConnection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "glowed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"connections":{"obsidian":{"enabled":false,"vault":"Work","backups":"off"}}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, errs := Load(t.TempDir())
	if len(errs) > 0 {
		t.Fatalf("Load: %v", errs)
	}
	got := cfg.Connections.Obsidian
	if got.Enabled || got.Vault != "Work" || got.Backups != BackupsOff {
		t.Fatalf("connection = %+v", got)
	}
}

// An unknown or empty backup mode falls back to the safe one.
func TestNormalizeRepairsTheBackupMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "glowed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"connections":{"obsidian":{"backups":""}}}`,
		`{"connections":{"obsidian":{"backups":"nonsense"}}}`,
		`{"connections":{"obsidian":{}}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, _ := Load(t.TempDir())
		if cfg.Connections.Obsidian.Backups != BackupsOutside {
			t.Fatalf("%s left Backups = %q", body, cfg.Connections.Obsidian.Backups)
		}
	}
}

func TestSaveConnectionsRoundTripsAndKeepsOtherSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "glowed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"defaults":{"editMode":false},"somethingElse":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	want := ConnectionsConfig{Obsidian: ObsidianConfig{Enabled: true, Vault: "Privat", Backups: BackupsOff}}
	if _, err := SaveConnections(want); err != nil {
		t.Fatalf("SaveConnections: %v", err)
	}
	cfg, _ := Load(t.TempDir())
	if cfg.Connections != want {
		t.Fatalf("round trip = %+v, want %+v", cfg.Connections, want)
	}
	if cfg.Defaults.EditMode {
		t.Fatal("SaveConnections changed the defaults")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"defaults", "somethingElse", "connections"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("saved file lost %q:\n%s", key, raw)
		}
	}
}
