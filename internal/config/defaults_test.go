package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsStartOn(t *testing.T) {
	cfg := Default()
	if !cfg.Defaults.EditMode || !cfg.Defaults.SidebarVisible {
		t.Fatalf("Defaults = %+v, want both on", cfg.Defaults)
	}
}

func TestLoadReadsDefaultsFromTheGlobalConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "glowed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"defaults":{"editMode":false,"sidebarVisible":false}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, errs := Load(t.TempDir())
	if len(errs) > 0 {
		t.Fatalf("Load: %v", errs)
	}
	if cfg.Defaults.EditMode || cfg.Defaults.SidebarVisible {
		t.Fatalf("Defaults = %+v, want both off", cfg.Defaults)
	}
}

// An absent key must keep the built-in default rather than reading as false.
func TestLoadKeepsUnsetDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "glowed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"defaults":{"editMode":false}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Load(t.TempDir())
	if cfg.Defaults.EditMode {
		t.Fatal("editMode = true, want the file's false")
	}
	if !cfg.Defaults.SidebarVisible {
		t.Fatal("sidebarVisible = false, want the built-in default to survive")
	}
}

func TestSaveDefaultsCreatesTheGlobalConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := SaveDefaults(DefaultsConfig{EditMode: false, SidebarVisible: true})
	if err != nil {
		t.Fatalf("SaveDefaults: %v", err)
	}
	if want := filepath.Join(home, ".config", "glowed", "config.json"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	cfg, errs := Load(t.TempDir())
	if len(errs) > 0 {
		t.Fatalf("Load: %v", errs)
	}
	if cfg.Defaults.EditMode || !cfg.Defaults.SidebarVisible {
		t.Fatalf("round trip lost the values: %+v", cfg.Defaults)
	}
}

// Saving must not throw away settings it does not know about.
func TestSaveDefaultsKeepsOtherSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "glowed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"preview":{"style":"light"},"keys":{"quit":"x"},"somethingElse":42}`
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SaveDefaults(DefaultsConfig{EditMode: false, SidebarVisible: false}); err != nil {
		t.Fatalf("SaveDefaults: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("saved file is not valid JSON: %v\n%s", err, raw)
	}
	for _, key := range []string{"preview", "keys", "somethingElse", "defaults"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("saved file lost %q:\n%s", key, raw)
		}
	}
	cfg, _ := Load(t.TempDir())
	if cfg.Preview.Style != "light" || cfg.Keys["quit"] != "x" {
		t.Fatalf("existing settings changed: style=%q quit=%q", cfg.Preview.Style, cfg.Keys["quit"])
	}
}

// A test that writes defaults must never reach the developer's own config.
// os.UserHomeDir cannot verify this on macOS, because it reads HOME itself, so
// check that HOME points somewhere temporary.
func TestSuiteRunsWithAnIsolatedHome(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Fatal("HOME is empty: SaveDefaults would fail rather than isolate")
	}
	if !strings.HasPrefix(home, os.TempDir()) {
		t.Fatalf("HOME = %q, want a directory under %q: see TestMain", home, os.TempDir())
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("HOME %q does not exist: %v", home, err)
	}
}
