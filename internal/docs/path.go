package docs

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GuardExistingPath resolves root and path (including symlinks) and returns the
// resolved path only when it is contained by root. It is intended for open/save
// guards before touching files selected by the TUI.
func GuardExistingPath(root, path string) (string, error) {
	rootReal, err := resolveExisting(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %s: %w", root, err)
	}
	pathReal, err := resolveExisting(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %s: %w", path, err)
	}
	if !isWithinCleanRoot(rootReal, pathReal) {
		return "", fmt.Errorf("path is outside root: %s", pathReal)
	}
	return pathReal, nil
}

// IsWithinRoot reports whether an existing path resolves inside an existing root.
func IsWithinRoot(root, path string) (bool, error) {
	rootReal, err := resolveExisting(root)
	if err != nil {
		return false, err
	}
	pathReal, err := resolveExisting(path)
	if err != nil {
		return false, err
	}
	return isWithinCleanRoot(rootReal, pathReal), nil
}

func resolveExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(real), nil
}

func isWithinCleanRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
