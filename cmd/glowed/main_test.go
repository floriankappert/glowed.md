package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIHelpSmoke(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run . --help failed: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{"glowed.md - Ghostty terminal Markdown browser/editor", "Usage:", "glowed [project-root]", "glowed --init-ignore [project-root]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help output missing %q:\n%s", want, text)
		}
	}
}

func TestCLIVersionSmoke(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run . --version failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "glowed.md dev") {
		t.Fatalf("version output missing expected value:\n%s", out)
	}
}

func TestCLIInitIgnoreCreatesTemplate(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("go", "run", ".", "--init-ignore", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run . --init-ignore failed: %v\n%s", err, out)
	}
	path := filepath.Join(root, ".glowedignore")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "created "+path) || !strings.Contains(string(b), "built-in default ignores") {
		t.Fatalf("init-ignore output/template unexpected\nout=%s\ntemplate=%s", out, b)
	}

	cmd = exec.Command("go", "run", ".", "--init-ignore", root)
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("second go run . --init-ignore succeeded unexpectedly:\n%s", out)
	}
	if !strings.Contains(string(out), ".glowedignore already exists") {
		t.Fatalf("second init-ignore output missing exists error:\n%s", out)
	}
}

func TestCLIBareInitIgnoreIsNotAccepted(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "init-ignore")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("go run . init-ignore succeeded unexpectedly:\n%s", out)
	}
	if !strings.Contains(string(out), "neither a directory nor markdown file") {
		t.Fatalf("bare init-ignore output missing path error:\n%s", out)
	}
}

func TestResolveArgsInitialMarkdownFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("# Doc"), 0644); err != nil {
		t.Fatal(err)
	}

	gotRoot, gotInitial, err := resolveArgs([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != root || gotInitial != path {
		t.Fatalf("resolveArgs(file) = (%q, %q), want (%q, %q)", gotRoot, gotInitial, root, path)
	}

	gotRoot, gotInitial, err = resolveArgs([]string{root, path})
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != root || gotInitial != path {
		t.Fatalf("resolveArgs(root,file) = (%q, %q), want (%q, %q)", gotRoot, gotInitial, root, path)
	}
}

func TestCLIInvalidRootSmoke(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "./definitely-missing-root")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("go run . missing-root succeeded unexpectedly:\n%s", out)
	}
	if !strings.Contains(string(out), "neither a directory nor markdown file") {
		t.Fatalf("invalid root output missing expected error:\n%s", out)
	}
}
