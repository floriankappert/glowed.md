package editor

import (
	"fmt"
	"os"
	"path/filepath"
)

// SaveFileAtomicWithBackup writes data to path via a same-directory temp file
// and first stores the current on-disk contents at "<path>.bak".
func SaveFileAtomicWithBackup(path string, data []byte) (backupPath string, err error) {
	return SaveFileAtomicWithBackupAt(path, path+".bak", data)
}

// SaveFileAtomicWithBackupAt is SaveFileAtomicWithBackup with the backup
// written where the caller asks — outside an Obsidian vault, for instance, so
// the vault neither shows nor syncs it. An empty backupPath skips the backup.
// The document is only written once the backup is safely in place.
func SaveFileAtomicWithBackupAt(path, backupPath string, data []byte) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file: %s", path)
	}
	mode := info.Mode().Perm()

	if backupPath != "" {
		old, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if dir := filepath.Dir(backupPath); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("create backup directory %s: %w", dir, err)
			}
		}
		if err := writeFileAtomic(backupPath, old, mode); err != nil {
			return "", fmt.Errorf("write backup %s: %w", backupPath, err)
		}
	}
	if err := writeFileAtomic(path, data, mode); err != nil {
		return backupPath, err
	}
	return backupPath, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if err = tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		closed = true
		return err
	}
	closed = true
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	_ = syncDir(dir)
	return nil
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
