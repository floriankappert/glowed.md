package obsidian

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func vault(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(root, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDetectFindsAVaultAtTheRoot(t *testing.T) {
	root := vault(t, "Work")
	got, ok := Detect(root)
	if !ok {
		t.Fatal("Detect found no vault")
	}
	if got.Name != "Work" {
		t.Fatalf("Name = %q, want Work", got.Name)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != resolved {
		t.Fatalf("Path = %q, want %q", got.Path, resolved)
	}
}

// glowed can be started in a subdirectory of a vault.
func TestDetectWalksUp(t *testing.T) {
	root := vault(t, "Privat")
	deep := filepath.Join(root, "01 - Projects", "sub")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := Detect(deep)
	if !ok || got.Path != resolved {
		t.Fatalf("Detect(%q) = %+v, %v; want the vault at %q", deep, got, ok, resolved)
	}
}

func TestDetectReportsNoVault(t *testing.T) {
	if got, ok := Detect(t.TempDir()); ok {
		t.Fatalf("Detect found %+v in a plain directory", got)
	}
	if got, ok := Detect(filepath.Join(t.TempDir(), "does-not-exist")); ok {
		t.Fatalf("Detect found %+v below a missing directory", got)
	}
	if got, ok := Detect(""); ok {
		t.Fatalf("Detect found %+v for an empty path", got)
	}
}

// A file called .obsidian is not a vault marker.
func TestDetectIgnoresAFileNamedObsidian(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".obsidian"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := Detect(root); ok {
		t.Fatalf("Detect accepted a file as the vault marker: %+v", got)
	}
}

// Detection must stop instead of walking to the filesystem root forever.
func TestDetectStopsAtTheFilesystemRoot(t *testing.T) {
	if got, ok := Detect(string(filepath.Separator)); ok {
		t.Fatalf("Detect found %+v at the filesystem root", got)
	}
}

func TestNoteURIEncodesVaultAndFile(t *testing.T) {
	v := Vault{Name: "My Vault", Path: "/vaults/My Vault"}
	uri, err := v.NoteURI(filepath.Join(v.Path, "00 - Inbox", "a note & more.md"))
	if err != nil {
		t.Fatalf("NoteURI: %v", err)
	}
	if !strings.HasPrefix(uri, "obsidian://open?") {
		t.Fatalf("uri = %q", uri)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("uri is not parseable: %v", err)
	}
	q := parsed.Query()
	if q.Get("vault") != "My Vault" {
		t.Fatalf("vault = %q", q.Get("vault"))
	}
	if want := "00 - Inbox/a note & more.md"; q.Get("file") != want {
		t.Fatalf("file = %q, want %q", q.Get("file"), want)
	}
}

func TestNoteURIRefusesAPathOutsideTheVault(t *testing.T) {
	v := Vault{Name: "Work", Path: "/vaults/Work"}
	for _, path := range []string{"/etc/passwd", "/vaults/Other/note.md", ""} {
		if uri, err := v.NoteURI(path); err == nil {
			t.Fatalf("NoteURI(%q) = %q, want an error", path, uri)
		}
	}
}

func TestVaultURIOpensTheVaultItself(t *testing.T) {
	v := Vault{Name: "Work", Path: "/vaults/Work"}
	uri := v.URI()
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("vault") != "Work" || parsed.Query().Get("file") != "" {
		t.Fatalf("uri = %q", uri)
	}
}

// The relative path is what Obsidian expects, with forward slashes.
func TestRelativeUsesForwardSlashes(t *testing.T) {
	v := Vault{Name: "Work", Path: filepath.Join("/vaults", "Work")}
	got, err := v.Relative(filepath.Join("/vaults", "Work", "a", "b.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "a/b.md" {
		t.Fatalf("Relative = %q, want a/b.md", got)
	}
}

// Document paths are symlink-resolved elsewhere in glowed, so the vault path
// has to be too — on macOS /var is a symlink to /private/var, and without this
// nothing inside the vault would look like it belongs to it.
func TestDetectResolvesSymlinks(t *testing.T) {
	real := filepath.Join(t.TempDir(), "Work")
	if err := os.MkdirAll(filepath.Join(real, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	got, ok := Detect(link)
	if !ok {
		t.Fatal("Detect found no vault through a symlink")
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != resolved {
		t.Fatalf("Path = %q, want the resolved %q", got.Path, resolved)
	}
	// A document addressed by its resolved path belongs to the vault.
	if _, err := got.Relative(filepath.Join(resolved, "note.md")); err != nil {
		t.Fatalf("Relative on a resolved path: %v", err)
	}
}
