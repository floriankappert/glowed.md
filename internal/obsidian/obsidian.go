// Package obsidian recognises an Obsidian vault around a project root and
// builds the URIs that open notes in the Obsidian app. It deliberately needs no
// external CLI: the obsidian:// scheme is handled by the app itself.
package obsidian

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// MarkerDir is the directory Obsidian keeps its vault settings in.
const MarkerDir = ".obsidian"

// Vault is a detected or configured Obsidian vault. Name is what the
// obsidian:// scheme identifies it by, which is the vault directory's name.
type Vault struct {
	Name string
	Path string
}

// Detect walks up from dir looking for a vault marker and returns the first
// vault it finds.
func Detect(dir string) (Vault, bool) {
	if dir == "" {
		return Vault{}, false
	}
	current, err := filepath.Abs(dir)
	if err != nil {
		return Vault{}, false
	}
	for {
		info, err := os.Stat(filepath.Join(current, MarkerDir))
		if err == nil && info.IsDir() {
			// Resolve symlinks: document paths are resolved as well, and on
			// macOS a temp or home path can run through one.
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				resolved = current
			}
			return Vault{Name: filepath.Base(resolved), Path: resolved}, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			// The filesystem root: there is nowhere left to walk.
			return Vault{}, false
		}
		current = parent
	}
}

// Relative returns path as Obsidian addresses it: relative to the vault, with
// forward slashes. It fails for anything outside the vault.
//
// The path is resolved first, because glowed carries both kinds: paths from the
// scan, which are built from the project root as given, and paths from the
// open/save guard, which are symlink-resolved. Without this they would not
// compare on macOS, where /var is a symlink to /private/var.
func (v Vault) Relative(path string) (string, error) {
	if v.Path == "" || path == "" {
		return "", fmt.Errorf("no vault or no path")
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	rel, err := filepath.Rel(v.Path, path)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the vault %s", path, v.Path)
	}
	return filepath.ToSlash(rel), nil
}

// NoteURI is the obsidian:// URI that opens one note.
func (v Vault) NoteURI(path string) (string, error) {
	rel, err := v.Relative(path)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("vault", v.Name)
	q.Set("file", rel)
	return "obsidian://open?" + q.Encode(), nil
}

// URI opens the vault itself.
func (v Vault) URI() string {
	q := url.Values{}
	q.Set("vault", v.Name)
	return "obsidian://open?" + q.Encode()
}
