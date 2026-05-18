package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelStartsWithSidebarHidden(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Hello"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	if m.SidebarVisible {
		t.Fatal("SidebarVisible = true, want false on startup")
	}
	if m.Focus != FocusPreview {
		t.Fatalf("Focus = %v, want FocusPreview", m.Focus)
	}
}

func TestNewWithInitialSelectsMarkdownFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	first := filepath.Join(root, "first.md")
	second := filepath.Join(root, "second.md")
	if err := os.WriteFile(first, []byte("# First"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("# Second"), 0644); err != nil {
		t.Fatal(err)
	}
	m := NewWithInitial(root, second)
	if doc := m.currentDoc(); doc == nil || doc.Rel != "second.md" {
		t.Fatalf("currentDoc = %#v, want second.md", doc)
	}
	if !strings.Contains(m.PreviewRaw, "# Second") {
		t.Fatalf("PreviewRaw = %q, want second content", m.PreviewRaw)
	}
}

func TestModelStartupSmokeLoadsProjectConfigAndMarkdown(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)

	config := `{
  "prefix": "ctrl+space",
  "preview": {"style": "dark"},
  "scan": {"maxFileBytes": 1048576},
  "mouse": {"enabled": false}
}`
	if err := os.WriteFile(filepath.Join(root, ".glowed.json"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("---\ntags: [smoke]\n---\n\n# Hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".glowedignore"), []byte("ignored/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "ignored"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored", "skip.md"), []byte("# Skip"), 0644); err != nil {
		t.Fatal(err)
	}

	m := New(root)
	if m.Root != root {
		t.Fatalf("Root = %q, want %q", m.Root, root)
	}
	if !m.MouseEnabled {
		t.Fatal("MouseEnabled = false, want always-on app-managed mouse selection")
	}
	if got := normalizeKey(m.Cfg.Prefix); got != "ctrl+@" {
		t.Fatalf("prefix = %q, normalized %q", m.Cfg.Prefix, got)
	}
	if len(m.Docs) != 1 || m.Docs[0].Rel != "README.md" {
		t.Fatalf("Docs = %#v, want only README.md", m.Docs)
	}
	if len(m.Results) != 1 || m.Results[0].Rel != "README.md" {
		t.Fatalf("Results = %#v, want README.md", m.Results)
	}
	if !strings.Contains(m.PreviewRaw, "# Hello") {
		t.Fatalf("PreviewRaw = %q, want markdown content", m.PreviewRaw)
	}
}
