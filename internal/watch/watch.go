package watch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/khw1031/glowed/internal/docs"
)

// Event describes a filesystem change that may affect glowed's scanned note set.
type Event struct {
	Path          string
	Rel           string
	Reason        string
	IgnoreChanged bool
}

// IgnoreFingerprint returns a content fingerprint for .glowedignore itself.
func IgnoreFingerprint(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Clean(absRoot), ".glowedignore")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "dir", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("file\x00%d\x00%d\x00%s", info.Size(), info.ModTime().UnixNano(), hex.EncodeToString(h[:])), nil
}

// FileFingerprint returns a lightweight metadata fingerprint for a single file path.
// It intentionally uses only size and modtime for project-wide polling.
func FileFingerprint(path string) (string, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "dir", nil
	}
	return fmt.Sprintf("file\x00%d\x00%d", info.Size(), info.ModTime().UnixNano()), nil
}

// ContentFingerprint returns a content hash fingerprint for a single file path.
// glowed uses this only for the active raw buffer, where the extra I/O is small
// and catching same-size/same-mtime external edits is more important.
func ContentFingerprint(path string) (string, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "dir", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("file\x00%d\x00%d\x00%s", info.Size(), info.ModTime().UnixNano(), hex.EncodeToString(h.Sum(nil))), nil
}

// Fingerprint returns a lightweight fingerprint of note-relevant filesystem state.
// It is used by polling refresh and intentionally avoids reading Markdown file
// contents so large projects remain stable and cheap to monitor.
func Fingerprint(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absRoot = filepath.Clean(absRoot)
	matcher := docs.LoadIgnoreMatcher(absRoot)
	entries := []string{}

	addStat := func(path string, rel string) {
		fingerprint, err := FileFingerprint(path)
		if err != nil || fingerprint == "missing" || fingerprint == "dir" {
			return
		}
		entries = append(entries, fmt.Sprintf("%s\x00%s", filepath.ToSlash(rel), fingerprint))
	}

	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(absRoot, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel != "." && matcher.Ignored(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if matcher.Ignored(rel, false) || !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		addStat(path, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(entries)
	h := sha256.New()
	for _, entry := range entries {
		_, _ = h.Write([]byte(entry))
		_, _ = h.Write([]byte("\n"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
