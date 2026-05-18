package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
	fixed := time.Unix(1700000000, 0)
	if err := os.WriteFile(ignorePath, []byte("same-a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(ignorePath, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	sameSizeBefore, err := IgnoreFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignorePath, []byte("same-b\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(ignorePath, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	sameSizeAfter, err := IgnoreFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if sameSizeBefore == sameSizeAfter {
		t.Fatal("ignore fingerprint did not change for same-size same-mtime content write")
	}
}

func TestFileFingerprintChangesForMarkdownWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := FileFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("two longer"), 0644); err != nil {
		t.Fatal(err)
	}
	after, err := FileFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("file fingerprint did not change after markdown write")
	}
}

func TestFileFingerprintIgnoresSameSizeSameModTimeContentChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	fixed := time.Unix(1700000000, 0)
	if err := os.WriteFile(path, []byte("alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	before, err := FileFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("bravo"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	after, err := FileFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("lightweight file fingerprint changed for same-size same-mtime content-only write")
	}
}

func TestContentFingerprintChangesForSameSizeSameModTimeContentChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	fixed := time.Unix(1700000000, 0)
	if err := os.WriteFile(path, []byte("alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	before, err := ContentFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("bravo"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	after, err := ContentFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("content fingerprint did not change for same-size same-mtime content write")
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
	if err := os.WriteFile(path, []byte("two longer"), 0644); err != nil {
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

func TestFingerprintRespectsBuiltInDefaultIgnores(t *testing.T) {
	root := t.TempDir()
	ignored := filepath.Join(root, ".git", "hidden.md")
	if err := os.MkdirAll(filepath.Dir(ignored), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignored, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignored, []byte("two longer"), 0644); err != nil {
		t.Fatal(err)
	}
	after, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("fingerprint changed for built-in ignored markdown write")
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
	if err := os.WriteFile(ignored, []byte("two longer"), 0644); err != nil {
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
		t.Fatal("fingerprint did not change after .glowedignore made markdown visible")
	}
}
