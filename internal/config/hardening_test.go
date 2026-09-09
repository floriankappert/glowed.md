package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A broken or hostile config file must not stop glowed from starting.
func TestLoadSurvivesBrokenConfigFiles(t *testing.T) {
	for _, body := range []string{
		"",
		"   ",
		"not json at all",
		"{",
		"[]",
		"null",
		`{"keys": "not an object"}`,
		`{"preview": 42}`,
		`{"scan": {"maxFileBytes": -5}}`,
		`{"scan": {"maxFileBytes": "big"}}`,
		`{"defaults": "nope"}`,
		`{"defaults": {"editMode": "yes"}}`,
		`{"llm": []}`,
		`{"prefix": null}`,
		strings.Repeat(`{"a":`, 200) + "1" + strings.Repeat("}", 200),
	} {
		home := t.TempDir()
		t.Setenv("HOME", home)
		dir := filepath.Join(home, ".config", "glowed")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, _ := Load(t.TempDir())
		// Whatever the file said, the result has to be usable.
		if cfg.Prefix == "" {
			t.Fatalf("%q left an empty prefix", body)
		}
		if cfg.Scan.MaxFileBytes <= 0 {
			t.Fatalf("%q left maxFileBytes at %d", body, cfg.Scan.MaxFileBytes)
		}
		if len(cfg.Footer.Actions) == 0 {
			t.Fatalf("%q left no footer actions", body)
		}
		if cfg.Preview.Style == "" {
			t.Fatalf("%q left an empty preview style", body)
		}
	}
}

// A directory where the config file belongs must be reported, not panicked on.
func TestSaveDefaultsReportsAnUnwritableTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "glowed", "config.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveDefaults(DefaultsConfig{}); err == nil {
		t.Fatal("SaveDefaults succeeded although the path is a directory")
	}
}

func TestSaveDefaultsFailsWithoutAHome(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := SaveDefaults(DefaultsConfig{}); err == nil {
		t.Fatal("SaveDefaults succeeded without HOME")
	}
}

// A malformed existing config must not be silently replaced.
func TestSaveDefaultsRefusesToOverwriteBrokenJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "glowed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	original := "{ this is not json"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveDefaults(DefaultsConfig{EditMode: true}); err == nil {
		t.Fatal("SaveDefaults overwrote a file it could not parse")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != original {
		t.Fatalf("the unreadable config was modified: %q", body)
	}
}

// Saving repeatedly must leave exactly one config file and no temp litter.
func TestSaveDefaultsLeavesNoTempFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for i := 0; i < 5; i++ {
		if _, err := SaveDefaults(DefaultsConfig{EditMode: i%2 == 0, SidebarVisible: true}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(home, ".config", "glowed"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("directory holds %v, want only config.json", names)
	}
}
