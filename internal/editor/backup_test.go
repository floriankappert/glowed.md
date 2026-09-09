package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFileAtomicWithBackupAtPutsTheBackupWhereAsked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "elsewhere", "note.md.bak")

	got, err := SaveFileAtomicWithBackupAt(path, backup, []byte("new"))
	if err != nil {
		t.Fatalf("SaveFileAtomicWithBackupAt: %v", err)
	}
	if got != backup {
		t.Fatalf("backup path = %q, want %q", got, backup)
	}
	if body, err := os.ReadFile(backup); err != nil || string(body) != "old" {
		t.Fatalf("backup = %q (%v), want the previous content", body, err)
	}
	if body, err := os.ReadFile(path); err != nil || string(body) != "new" {
		t.Fatalf("file = %q (%v), want the new content", body, err)
	}
	// Nothing was left next to the document.
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Fatal("a .bak was written next to the document as well")
	}
}

// An empty backup path means: save without keeping one.
func TestSaveFileAtomicWithBackupAtCanSkipTheBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SaveFileAtomicWithBackupAt(path, "", []byte("new"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("backup path = %q, want none", got)
	}
	if body, _ := os.ReadFile(path); string(body) != "new" {
		t.Fatalf("file = %q", body)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries, want only the document", len(entries))
	}
}

// The old entry point keeps writing beside the document.
func TestSaveFileAtomicWithBackupStillWritesBeside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SaveFileAtomicWithBackup(path, []byte("new"))
	if err != nil {
		t.Fatal(err)
	}
	if got != path+".bak" {
		t.Fatalf("backup path = %q, want %q", got, path+".bak")
	}
}

// A save must not be lost because the backup directory could not be made.
func TestSaveFileAtomicWithBackupAtReportsAnUnusableBackupPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveFileAtomicWithBackupAt(path, filepath.Join(blocker, "note.md.bak"), []byte("new")); err == nil {
		t.Fatal("SaveFileAtomicWithBackupAt hid an unusable backup path")
	}
	// The document is untouched, so nothing was half-written.
	if body, _ := os.ReadFile(path); string(body) != "old" {
		t.Fatalf("file = %q, want the original content", body)
	}
}
