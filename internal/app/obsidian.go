package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/config"
	"github.com/khw1031/glowed/internal/obsidian"
)

// obsidianVault is the vault the project sits in, if the connection is on. A
// configured name overrides the detected one, so a renamed or symlinked
// directory can still be addressed by the name Obsidian knows it under.
func (m Model) obsidianVault() (obsidian.Vault, bool) {
	if !m.Cfg.Connections.Obsidian.Enabled {
		return obsidian.Vault{}, false
	}
	vault, ok := obsidian.Detect(m.Root)
	if !ok {
		return obsidian.Vault{}, false
	}
	if name := m.Cfg.Connections.Obsidian.Vault; name != "" {
		vault.Name = name
	}
	return vault, true
}

// backupPathFor decides where the backup of a document goes. Inside a vault it
// stays out of it, because Obsidian would show it in the file tree and sync it
// to every other device.
func (m Model) backupPathFor(path string) (string, error) {
	vault, ok := m.obsidianVault()
	if !ok {
		return path + ".bak", nil
	}
	switch m.Cfg.Connections.Obsidian.Backups {
	case config.BackupsOff:
		return "", nil
	case config.BackupsVault:
		return path + ".bak", nil
	}

	rel, err := vault.Relative(path)
	if err != nil {
		// Not in the vault after all: keep it beside the document.
		return path + ".bak", nil
	}
	dir, err := backupStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, vault.Name, filepath.FromSlash(rel)) + ".bak", nil
}

// backupStateDir is where backups go that must not live in the vault.
func backupStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("cannot locate a place for backups: no home directory")
	}
	return filepath.Join(home, ".local", "state", "glowed", "backups"), nil
}

// obsidianNoteURI is the URI that opens the current document in Obsidian.
func (m Model) obsidianNoteURI() (string, error) {
	vault, ok := m.obsidianVault()
	if !ok {
		return "", fmt.Errorf("this project is not an Obsidian vault")
	}
	doc := m.currentDoc()
	if doc == nil {
		return vault.URI(), nil
	}
	return vault.NoteURI(doc.Abs)
}

// openObsidianCmd hands a URI to the desktop, which is what starts Obsidian.
// No CLI is involved: the app registers the obsidian:// scheme itself.
func openObsidianCmd(uri string) tea.Cmd {
	return func() tea.Msg {
		name, args := uriOpener(uri)
		if name == "" {
			return obsidianOpenResultMsg{Err: fmt.Errorf("opening a URI is not supported on %s", runtime.GOOS)}
		}
		if err := exec.Command(name, args...).Start(); err != nil {
			return obsidianOpenResultMsg{Err: err}
		}
		return obsidianOpenResultMsg{URI: uri}
	}
}

// uriOpener is the platform's "open this URI" command.
func uriOpener(uri string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{uri}
	case "linux":
		return "xdg-open", []string{uri}
	default:
		return "", nil
	}
}

type obsidianOpenResultMsg struct {
	URI string
	Err error
}

func (m *Model) handleObsidianOpenResult(msg obsidianOpenResultMsg) {
	if msg.Err != nil {
		m.setStatus("open in Obsidian failed: "+msg.Err.Error(), "error")
		return
	}
	m.setStatus("opened in Obsidian", "success")
}
