package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GlobalPath is the global config file the action menu writes to.
func GlobalPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "glowed", "config.json")
}

// SaveDefaults writes the startup defaults into the global config file and
// returns its path. Only the "defaults" object is touched: the file is merged
// as raw JSON, so settings this build does not know about survive, and the
// remaining defaults are not frozen into the file.
func SaveDefaults(d DefaultsConfig) (string, error) {
	path := GlobalPath()
	if path == "" {
		return "", fmt.Errorf("cannot locate the global config: HOME is not set")
	}

	raw := map[string]json.RawMessage{}
	switch body, err := os.ReadFile(path); {
	case err == nil:
		if len(body) > 0 {
			if err := json.Unmarshal(body, &raw); err != nil {
				return "", fmt.Errorf("parse %s: %w", path, err)
			}
		}
	case !os.IsNotExist(err):
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	encoded, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	raw["defaults"] = encoded

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return "", err
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	// Write via a temp file in the same directory so a failed write cannot
	// truncate an existing config.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.json.tmp-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}
