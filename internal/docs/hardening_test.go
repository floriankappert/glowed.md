package docs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// GuardNewPath is the gate in front of every create and rename, so it has to
// hold for the shapes a filename prompt can produce.
func TestGuardNewPathRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Names typed into the prompt, joined onto the project root.
	for _, name := range []string{
		"../escape.md",
		"../../escape.md",
		"sub/../../escape.md",
		"/etc/passwd", // joined: a parent that does not exist
		"",            // the root itself
		".",
		"..",
		strings.Repeat("../", 40) + "escape.md",
	} {
		if got, err := GuardNewPath(root, filepath.Join(root, name)); err == nil {
			t.Fatalf("GuardNewPath accepted %q as %q", name, got)
		}
	}

	// An absolute path handed in directly.
	for _, path := range []string{"/etc/passwd", "/tmp", filepath.Dir(root)} {
		if got, err := GuardNewPath(root, path); err == nil {
			t.Fatalf("GuardNewPath accepted the absolute path %q as %q", path, got)
		}
	}
}

// A path that normalises back into the root is not an escape: "sub/" is the
// existing directory, and the caller's O_EXCL create is what refuses it.
func TestGuardNewPathTreatsATrailingSlashAsThatDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := GuardNewPath(root, filepath.Join(root, "sub/"))
	if err != nil {
		t.Fatalf("GuardNewPath: %v", err)
	}
	real, _ := filepath.EvalSymlinks(root)
	if got != filepath.Join(real, "sub") {
		t.Fatalf("GuardNewPath = %q, want the sub directory inside the root", got)
	}
	if _, err := os.OpenFile(got, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); err == nil {
		t.Fatal("creating over an existing directory succeeded")
	}
}

func TestGuardNewPathAcceptsOrdinaryNames(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"note.md", "sub/note.md", "sub/../note.md", "한글.md", "with space.md", ".hidden.md"} {
		got, err := GuardNewPath(root, filepath.Join(root, name))
		if err != nil {
			t.Fatalf("GuardNewPath(%q): %v", name, err)
		}
		real, _ := filepath.EvalSymlinks(root)
		if !strings.HasPrefix(got, real) {
			t.Fatalf("GuardNewPath(%q) = %q, outside %q", name, got, real)
		}
	}
}

// A symlink that points out of the project must not become a way out.
func TestGuardNewPathRejectsASymlinkedParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := GuardNewPath(root, filepath.Join(link, "note.md")); err == nil {
		t.Fatal("GuardNewPath followed a symlink out of the root")
	}
}

// Scanning must survive the odd things a project directory contains.
func TestScanSurvivesUnusualEntries(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("empty.md", "")
	write("only-frontmatter.md", "---\ntitle: x\n---\n")
	write("broken-frontmatter.md", "---\ntitle: [unclosed\n---\nbody\n")
	write("no-newline.md", "# No trailing newline")
	write("binary.md", "\x00\x01\x02\xff")
	write("deep/a/b/c/d/e.md", "# Deep\n")
	write("not-markdown.txt", "ignored")
	write("UPPER.MD", "# Upper\n")
	if err := os.MkdirAll(filepath.Join(root, "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	list, report, err := ScanWithReport(root, 1024*1024)
	if err != nil {
		t.Fatalf("ScanWithReport: %v", err)
	}
	for _, doc := range list {
		if doc.Abs == "" || doc.Rel == "" {
			t.Fatalf("document without a path: %+v", doc)
		}
		if strings.HasSuffix(doc.Rel, ".txt") {
			t.Fatalf("scanned a non-markdown file: %q", doc.Rel)
		}
		if doc.Rel == "dir.md" {
			t.Fatal("scanned a directory as a document")
		}
	}
	_ = report
}

func TestScanHonoursTheSizeLimit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big.md"), []byte(strings.Repeat("x", 5000)), 0o644); err != nil {
		t.Fatal(err)
	}
	list, report, err := ScanWithReport(root, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("scanned %d files, want the oversized one skipped", len(list))
	}
	if len(report.Excluded) != 1 || report.Excluded[0].Reason != "maxFileBytes" {
		t.Fatalf("report = %+v, want the size exclusion", report.Excluded)
	}
}
