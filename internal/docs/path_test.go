package docs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGuardExistingPathAllowsRootChildAndCleansTraversal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := GuardExistingPath(root, filepath.Join(root, "sub", "..", "doc.md"))
	if err != nil {
		t.Fatalf("GuardExistingPath() error = %v", err)
	}
	want, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("GuardExistingPath() = %q, want %q", got, want)
	}
}

func TestGuardExistingPathRejectsPrefixSibling(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(parent, "root-sibling.md")
	if err := os.WriteFile(sibling, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}

	if ok, err := IsWithinRoot(root, sibling); err != nil {
		t.Fatalf("IsWithinRoot() error = %v", err)
	} else if ok {
		t.Fatal("IsWithinRoot() = true, want false for prefix sibling")
	}
	if _, err := GuardExistingPath(root, sibling); err == nil {
		t.Fatal("GuardExistingPath() error = nil, want outside-root error")
	}
}

func TestGuardExistingPathRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation often requires elevated privileges on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.md")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Fatal(err)
	}

	if ok, err := IsWithinRoot(root, link); err != nil {
		t.Fatalf("IsWithinRoot() error = %v", err)
	} else if ok {
		t.Fatal("IsWithinRoot() = true, want false for symlink escape")
	}
	if _, err := GuardExistingPath(root, link); err == nil {
		t.Fatal("GuardExistingPath() error = nil, want symlink escape error")
	}
}

func TestGuardNewPathAllowsNotYetExistingFileInRoot(t *testing.T) {
	root := t.TempDir()
	got, err := GuardNewPath(root, filepath.Join(root, "note.md"))
	if err != nil {
		t.Fatalf("GuardNewPath: %v", err)
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(real, "note.md"); got != want {
		t.Fatalf("GuardNewPath = %q, want %q", got, want)
	}
}

func TestGuardNewPathRejectsTraversalOutsideRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := GuardNewPath(root, filepath.Join(root, "..", "escape.md")); err == nil {
		t.Fatal("GuardNewPath accepted a path outside the root")
	}
}

func TestGuardNewPathRejectsMissingParentDirectory(t *testing.T) {
	root := t.TempDir()
	if _, err := GuardNewPath(root, filepath.Join(root, "nope", "note.md")); err == nil {
		t.Fatal("GuardNewPath accepted a missing parent directory")
	}
}
