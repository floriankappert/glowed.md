package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemoveKnownDirRemovesDescendants(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp", "root")
	removed := filepath.Join(root, "notes")
	w := &Watcher{watchedDirs: map[string]bool{
		root:                            true,
		removed:                         true,
		filepath.Join(removed, "child"): true,
		filepath.Join(removed, "child", "nested"): true,
		filepath.Join(root, "other"):              true,
	}}

	w.removeKnownDir(removed)

	if w.watchedDirs[removed] || w.watchedDirs[filepath.Join(removed, "child")] || w.watchedDirs[filepath.Join(removed, "child", "nested")] {
		t.Fatalf("removed directory descendants still tracked: %#v", w.watchedDirs)
	}
	if !w.watchedDirs[root] || !w.watchedDirs[filepath.Join(root, "other")] {
		t.Fatalf("unrelated watched directories were removed: %#v", w.watchedDirs)
	}
}

func TestIgnoreFingerprintChangesForIgnoreFileWrites(t *testing.T) {
	root := t.TempDir()
	before, err := IgnoreFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != "missing" {
		t.Fatalf("initial ignore fingerprint = %q, want missing", before)
	}
	ignorePath := filepath.Join(root, ".glowedignore")
	if err := os.WriteFile(ignorePath, []byte("ignored.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	afterCreate, err := IgnoreFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if afterCreate == before {
		t.Fatal("ignore fingerprint did not change after creating .glowedignore")
	}
	if err := os.WriteFile(ignorePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	afterWrite, err := IgnoreFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if afterWrite == afterCreate {
		t.Fatal("ignore fingerprint did not change after writing .glowedignore")
	}
}

func TestFingerprintChangesForMarkdownWrites(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	after, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("fingerprint did not change after markdown write")
	}
}

func TestFingerprintChangesForSameSizeSameModTimeWrites(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	fixed := time.Unix(1700000000, 0)
	if err := os.WriteFile(path, []byte("alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	before, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("bravo"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	after, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("fingerprint did not change for same-size same-mtime markdown write")
	}
}

func TestFingerprintRespectsGlowedIgnoreAndIgnoreFileChanges(t *testing.T) {
	root := t.TempDir()
	ignored := filepath.Join(root, "ignored.md")
	ignoreFile := filepath.Join(root, ".glowedignore")
	if err := os.WriteFile(ignored, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignoreFile, []byte("ignored.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignored, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	afterIgnoredWrite, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != afterIgnoredWrite {
		t.Fatal("fingerprint changed for ignored markdown write")
	}
	if err := os.WriteFile(ignoreFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	afterIgnoreChange, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before == afterIgnoreChange {
		t.Fatal("fingerprint did not change after .glowedignore change")
	}
}
