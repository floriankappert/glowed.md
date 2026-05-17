package editor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveFileAtomicWithBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte("old\ncontent"), 0640); err != nil {
		t.Fatal(err)
	}

	backup, err := SaveFileAtomicWithBackup(path, []byte("new\ncontent"))
	if err != nil {
		t.Fatalf("SaveFileAtomicWithBackup() error = %v", err)
	}
	if backup != path+".bak" {
		t.Fatalf("backup path = %q, want %q", backup, path+".bak")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\ncontent" {
		t.Fatalf("saved content = %q", got)
	}

	bak, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(bak) != "old\ncontent" {
		t.Fatalf("backup content = %q", bak)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if gotMode := info.Mode().Perm(); gotMode != 0640 {
			t.Fatalf("saved mode = %v, want 0640", gotMode)
		}
		backupInfo, err := os.Stat(backup)
		if err != nil {
			t.Fatal(err)
		}
		if gotMode := backupInfo.Mode().Perm(); gotMode != 0640 {
			t.Fatalf("backup mode = %v, want 0640", gotMode)
		}
	}
}

func TestSaveFileAtomicWithBackupRejectsMissingFile(t *testing.T) {
	_, err := SaveFileAtomicWithBackup(filepath.Join(t.TempDir(), "missing.md"), []byte("new"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
