package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/khw1031/glowed/internal/docs"
)

// promptKind is what the toolbar prompt is currently asking for.
type promptKind int

const (
	promptNone promptKind = iota
	promptNewFile
	promptRename
	promptDeleteConfirm
)

// promptState is a single-line input rendered in the toolbar row. It owns the
// keyboard while active, so nothing typed into it reaches the buffer.
type promptState struct {
	Active bool
	Kind   promptKind
	Input  string
	Target string // absolute path the prompt acts on, for rename and delete
}

// menuState is the action menu opened with ctrl+p.
type menuState struct {
	Active   bool
	Selected int
}

// menuAction is what an action-menu entry does. Most open a prompt, one goes
// back to the welcome screen.
type menuAction int

const (
	menuNewFile menuAction = iota
	menuRename
	menuDelete
	menuGoHome
)

type menuEntry struct {
	Label string
	Kind  menuAction
}

// menuEntries are the menu actions, in display order.
func menuEntries() []menuEntry {
	return []menuEntry{
		{Label: "new file", Kind: menuNewFile},
		{Label: "edit filename", Kind: menuRename},
		{Label: "delete file", Kind: menuDelete},
		{Label: "go home", Kind: menuGoHome},
	}
}

// --- prompt ---

func (m *Model) openNewFilePrompt() {
	m.Menu.Active = false
	m.Prompt = promptState{Active: true, Kind: promptNewFile}
	m.setStatus("new file — enter to create, esc to cancel", "info")
}

func (m *Model) openRenamePrompt() {
	m.Menu.Active = false
	doc := m.currentDoc()
	if doc == nil {
		m.setStatus("no document to rename", "warn")
		return
	}
	if m.Editor.Dirty {
		m.setStatus("unsaved changes in "+filepath.Base(m.Editor.File)+" — ctrl+s to save, esc to discard", "warn")
		return
	}
	m.Prompt = promptState{Active: true, Kind: promptRename, Input: filepath.Base(doc.Abs), Target: doc.Abs}
	m.setStatus("rename "+doc.Rel+" — enter to apply, esc to cancel", "info")
}

func (m *Model) openDeletePrompt() {
	m.Menu.Active = false
	doc := m.currentDoc()
	if doc == nil {
		m.setStatus("no document to delete", "warn")
		return
	}
	m.Prompt = promptState{Active: true, Kind: promptDeleteConfirm, Target: doc.Abs}
	m.setStatus("delete "+doc.Rel+" — y to confirm", "warn")
}

func (m *Model) closePrompt() {
	m.Prompt = promptState{}
}

// handlePromptKey drives the toolbar prompt. It runs before every other key
// handler while the prompt is active.
func (m *Model) handlePromptKey(msg tea.KeyMsg) tea.Cmd {
	key := normalizeKey(msg.String())
	if key == "esc" {
		m.closePrompt()
		m.setStatus("cancelled", "info")
		return nil
	}

	if m.Prompt.Kind == promptDeleteConfirm {
		switch key {
		case "y":
			m.deleteCurrentFile()
		default:
			m.closePrompt()
			m.setStatus("delete cancelled", "info")
		}
		return nil
	}

	switch key {
	case "enter":
		switch m.Prompt.Kind {
		case promptNewFile:
			m.createFileFromPrompt()
		case promptRename:
			m.renameFileFromPrompt()
		}
		return nil
	case "backspace":
		runes := []rune(m.Prompt.Input)
		if len(runes) > 0 {
			m.Prompt.Input = string(runes[:len(runes)-1])
		}
		return nil
	}

	if msg.Type == tea.KeyRunes && !msg.Alt {
		m.Prompt.Input += string(msg.Runes)
	} else if msg.Type == tea.KeySpace {
		m.Prompt.Input += " "
	}
	return nil
}

// renderPrompt is the toolbar line while a prompt is active.
func (m Model) renderPrompt() string {
	label := ""
	switch m.Prompt.Kind {
	case promptNewFile:
		label = "enter filename: "
	case promptRename:
		label = "rename to: "
	case promptDeleteConfirm:
		return fitANSI(" "+styleRed.Render("delete "+filepath.Base(m.Prompt.Target)+"? (y/N)"), m.Width)
	}
	return fitANSI(" "+styleCyan.Render(label)+m.Prompt.Input+styleCursor.Render(" "), m.Width)
}

// --- file operations ---

// newFileDir is where ctrl+n creates a file: next to the current document, and
// in the project root when nothing is open.
func (m Model) newFileDir() string {
	if doc := m.currentDoc(); doc != nil {
		return filepath.Dir(doc.Abs)
	}
	return m.Root
}

// markdownName appends the .md extension unless the name already carries one.
func markdownName(name string) string {
	if strings.EqualFold(filepath.Ext(name), ".md") {
		return name
	}
	return name + ".md"
}

func (m *Model) createFileFromPrompt() {
	name := strings.TrimSpace(m.Prompt.Input)
	if name == "" {
		m.setStatus("enter a file name, or esc to cancel", "warn")
		return
	}
	target, err := docs.GuardNewPath(m.Root, filepath.Join(m.newFileDir(), markdownName(name)))
	if err != nil {
		m.setStatus("create blocked: "+err.Error(), "error")
		return
	}
	// O_EXCL makes "does it already exist" part of the create itself, so an
	// existing file can never be truncated here.
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			m.setStatus("file already exists: "+m.relToRoot(target), "error")
			return
		}
		m.setStatus("create failed: "+err.Error(), "error")
		return
	}
	if err := f.Close(); err != nil {
		m.setStatus("create failed: "+err.Error(), "error")
		return
	}

	m.closePrompt()
	m.openPathForEditing(target, "created "+m.relToRoot(target))
}

func (m *Model) renameFileFromPrompt() {
	name := strings.TrimSpace(m.Prompt.Input)
	if name == "" {
		m.setStatus("enter a file name, or esc to cancel", "warn")
		return
	}
	source, err := docs.GuardExistingPath(m.Root, m.Prompt.Target)
	if err != nil {
		m.setStatus("rename blocked: "+err.Error(), "error")
		return
	}
	target, err := docs.GuardNewPath(m.Root, filepath.Join(filepath.Dir(source), markdownName(name)))
	if err != nil {
		m.setStatus("rename blocked: "+err.Error(), "error")
		return
	}
	if target == source {
		m.closePrompt()
		m.setStatus("name unchanged", "info")
		return
	}
	// os.Rename would silently replace an existing target on Unix.
	if _, err := os.Lstat(target); err == nil {
		m.setStatus("file already exists: "+m.relToRoot(target), "error")
		return
	}
	if err := os.Rename(source, target); err != nil {
		m.setStatus("rename failed: "+err.Error(), "error")
		return
	}

	m.closePrompt()
	m.openPathForEditing(target, "renamed to "+m.relToRoot(target))
}

func (m *Model) deleteCurrentFile() {
	path, err := docs.GuardExistingPath(m.Root, m.Prompt.Target)
	if err != nil {
		m.setStatus("delete blocked: "+err.Error(), "error")
		return
	}
	rel := m.relToRoot(path)
	// Keep a copy next to the file, matching the backup the editor writes on
	// save, so a delete stays recoverable.
	body, err := os.ReadFile(path)
	if err != nil {
		m.setStatus("delete failed: "+err.Error(), "error")
		return
	}
	backup := path + ".bak"
	if err := os.WriteFile(backup, body, 0o644); err != nil {
		m.setStatus("delete failed, backup not written: "+err.Error(), "error")
		return
	}
	if err := os.Remove(path); err != nil {
		m.setStatus("delete failed: "+err.Error(), "error")
		return
	}

	m.closePrompt()
	if m.Editor.File != "" && m.pathsMatch(m.Editor.File, path) {
		m.Editor = editorState{Lines: []string{""}}
		m.clearEditorSelection()
		m.Mode = ModePreview
		m.Focus = FocusPreview
	}
	if err := m.scanAndApply(true); err != nil {
		m.setStatus(err.Error(), "error")
		return
	}
	m.rebuildSidebarRows()
	if doc := m.currentDoc(); doc != nil {
		m.enterEditMode()
	} else {
		m.reloadPreview()
	}
	m.setStatus(fmt.Sprintf("deleted %s (backup %s)", rel, filepath.Base(backup)), "success")
}

// openPathForEditing rescans, selects path and opens it in the editor.
func (m *Model) openPathForEditing(path, message string) {
	if err := m.scanAndApply(true); err != nil {
		m.setStatus(err.Error(), "error")
		return
	}
	m.rebuildSidebarRows()
	if !m.selectPath(path) {
		m.setStatus(message+", but it is not in the scan results", "warn")
		return
	}
	m.enterEditMode()
	m.setStatus(message, "success")
}

// selectPath selects the document at an absolute path, if it was scanned.
func (m *Model) selectPath(path string) bool {
	for i, doc := range m.Results {
		if m.pathsMatch(doc.Abs, path) {
			m.setSelection(i)
			m.syncSidebarSelectionToCurrentDoc()
			return true
		}
	}
	return false
}

func (m Model) relToRoot(path string) string {
	if rel, err := filepath.Rel(m.Root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// --- action menu ---

func (m *Model) toggleActionMenu() {
	if m.Menu.Active {
		m.Menu.Active = false
		return
	}
	m.Menu = menuState{Active: true}
	m.setStatus("actions — ↑↓ select, enter run, esc close", "info")
}

func (m *Model) handleMenuKey(key string) {
	entries := menuEntries()
	switch key {
	case "esc", "ctrl+p":
		m.Menu.Active = false
		m.setStatus("actions closed", "info")
	case "up", "k":
		m.Menu.Selected = clamp(m.Menu.Selected-1, 0, len(entries)-1)
	case "down", "j":
		m.Menu.Selected = clamp(m.Menu.Selected+1, 0, len(entries)-1)
	case "enter":
		switch entries[clamp(m.Menu.Selected, 0, len(entries)-1)].Kind {
		case menuNewFile:
			m.openNewFilePrompt()
		case menuRename:
			m.openRenamePrompt()
		case menuDelete:
			m.openDeletePrompt()
		case menuGoHome:
			m.goHome()
		}
	}
}

// goHome returns to the welcome screen, which then owns the keyboard again
// until a document is picked.
func (m *Model) goHome() {
	if m.Editor.Dirty {
		m.setStatus("unsaved changes in "+filepath.Base(m.Editor.File)+" — ctrl+s to save, esc to discard", "warn")
		return
	}
	m.Menu.Active = false
	m.closePrompt()
	m.clearEditorSelection()
	m.Splash = true
	m.SplashSelected = 0
	m.setStatus("home — ↑↓ select, enter open", "info")
}

// menuBackdropColor is the 256-color index the menu paints the content pane with.
const menuBackdropColor = "236"

// menuPadX is the horizontal padding inside a menu row, so the highlight of the
// selected entry does not sit flush against its label.
const menuPadX = 2

// menuBlock is the menu text, top to bottom. The empty line separates the title
// from the entries.
func (m Model) menuBlock() []string {
	block := []string{"actions", ""}
	for _, entry := range menuEntries() {
		block = append(block, entry.Label)
	}
	return block
}

// renderMenuRow draws one content-pane row while the action menu is open. The
// menu covers the whole pane, so every row belongs to it: the block of entries
// is centered in the pane, and its lines are left-aligned with each other.
func (m Model) renderMenuRow(width, row int) (string, bool) {
	if !m.Menu.Active || width <= 0 {
		return "", false
	}
	block := m.menuBlock()

	textWidth := 0
	for _, line := range block {
		textWidth = max(textWidth, runewidth.StringWidth(line))
	}
	blockWidth := min(width, textWidth+2*menuPadX)
	left := max(0, (width-blockWidth)/2)
	top := max(0, (m.contentHeight()-len(block))/2)

	idx := row - top
	if idx < 0 || idx >= len(block) {
		return styleMenuBackdrop.Render(strings.Repeat(" ", width)), true
	}

	label := block[idx]
	inner := strings.Repeat(" ", menuPadX) + label
	if pad := blockWidth - runewidth.StringWidth(inner); pad > 0 {
		inner += strings.Repeat(" ", pad)
	}
	inner = xansi.Truncate(inner, blockWidth, "")

	style := styleMenuEntry
	switch {
	case idx == 0:
		style = styleMenuTitle
	case idx-2 == m.Menu.Selected && idx >= 2:
		style = styleMenuSelected
	}

	right := max(0, width-left-runewidth.StringWidth(inner))
	return styleMenuBackdrop.Render(strings.Repeat(" ", left)) +
		style.Render(inner) +
		styleMenuBackdrop.Render(strings.Repeat(" ", right)), true
}
