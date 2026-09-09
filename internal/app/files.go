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
	Query    string // filter, typed straight into the menu
}

// menuAction is what an action-menu entry does: the file actions, going back to
// the welcome screen, or dispatching one of the mode's own actions.
type menuAction int

const (
	menuNewFile menuAction = iota
	menuRename
	menuDelete
	menuGoHome
	menuOpenSelection
	menuDispatch
)

type menuEntry struct {
	Label  string
	Key    string
	Kind   menuAction
	Action string // dispatch action, for menuDispatch entries
	Danger bool   // destructive: rendered apart, in red
	Gap    bool   // preceded by a blank row
}

// menuEntries are the app actions, in display order. The welcome screen gets
// its own short list: renaming or deleting a file that is not open makes no
// sense there, and neither do the mode actions.
func (m Model) menuEntries() []menuEntry {
	if m.Splash {
		return []menuEntry{
			{Label: "open", Key: "enter", Kind: menuOpenSelection},
			{Label: "new file", Key: "ctrl+n", Kind: menuNewFile},
			{Label: "quit", Key: m.footerKey("quit"), Kind: menuDispatch, Action: "quit"},
		}
	}
	return []menuEntry{
		{Label: "new file", Key: "ctrl+n", Kind: menuNewFile},
		{Label: "edit filename", Kind: menuRename},
		{Label: "<> sidebar", Key: "ctrl+t", Kind: menuDispatch, Action: "toggleSidebar"},
		{Label: "<> edit/preview", Kind: menuDispatch, Action: "toggleMode"},
		{Label: "go home", Kind: menuGoHome, Gap: true},
	}
}

// deleteEntry is destructive, so it sits at the bottom, separated from the rest.
func deleteEntry() menuEntry {
	return menuEntry{Label: "delete file", Kind: menuDelete, Danger: true}
}

// menuActions are the selectable entries that match the filter: the app
// actions, the actions the current mode offers, and the destructive one last.
// The mode's hints used to sit in a footer bar; the ones that can be run are
// runnable here.
func (m Model) menuActions() []menuEntry {
	entries := m.menuEntries()
	if !m.Splash {
		offered := map[string]bool{}
		for _, entry := range entries {
			if entry.Action != "" {
				offered[entry.Action] = true
			}
		}
		for _, hint := range m.modeEntries() {
			if hint.Action == "" || offered[hint.Action] {
				continue
			}
			entries = append(entries, menuEntry{
				Label:  hint.Label,
				Key:    hint.Key,
				Kind:   menuDispatch,
				Action: hint.Action,
			})
		}
		entries = append(entries, deleteEntry())
	}
	return filterMenuEntries(entries, m.Menu.Query)
}

// filterMenuEntries keeps the entries whose label or key contains the query.
func filterMenuEntries(entries []menuEntry, query string) []menuEntry {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return entries
	}
	out := entries[:0:0]
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Label), query) ||
			strings.Contains(strings.ToLower(entry.Key), query) {
			// A filtered list has no groups left to separate.
			entry.Gap = false
			out = append(out, entry)
		}
	}
	return out
}

// menuHints are the mode's remaining hints: keys worth knowing that the menu
// cannot run, such as the word-motion bindings.
func (m Model) menuHints() []footerEntry {
	if m.Splash {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(m.Menu.Query))
	hints := []footerEntry{}
	for _, hint := range m.modeEntries() {
		if hint.Action != "" {
			continue
		}
		// The sidebar hint would repeat the runnable "<> sidebar" entry.
		if hint.Label == "sidebar" {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(hint.Label), query) &&
			!strings.Contains(strings.ToLower(hint.Key), query) {
			continue
		}
		hints = append(hints, hint)
	}
	return hints
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
	// Opening a document in the editor is what leaves the welcome screen.
	m.Splash = false
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
	m.setStatus("actions — type to filter, ↑↓ select, enter run, esc close", "info")
}

func (m *Model) handleMenuKey(msg tea.KeyMsg) tea.Cmd {
	key := normalizeKey(msg.String())
	entries := m.menuActions()

	switch key {
	case "ctrl+p":
		m.Menu.Active = false
		m.setStatus("actions closed", "info")
		return nil
	case "esc":
		// A typo should not close the menu, so the filter goes first.
		if m.Menu.Query != "" {
			m.Menu.Query = ""
			m.Menu.Selected = 0
			return nil
		}
		m.Menu.Active = false
		m.setStatus("actions closed", "info")
		return nil
	case "up":
		m.Menu.Selected = clamp(m.Menu.Selected-1, 0, max(0, len(entries)-1))
		return nil
	case "down":
		m.Menu.Selected = clamp(m.Menu.Selected+1, 0, max(0, len(entries)-1))
		return nil
	case "backspace":
		runes := []rune(m.Menu.Query)
		if len(runes) > 0 {
			m.Menu.Query = string(runes[:len(runes)-1])
			m.Menu.Selected = 0
		}
		return nil
	case "enter":
		if len(entries) == 0 {
			m.setStatus("no action matches "+m.Menu.Query, "warn")
			return nil
		}
		entry := entries[clamp(m.Menu.Selected, 0, len(entries)-1)]
		switch entry.Kind {
		case menuNewFile:
			m.openNewFilePrompt()
		case menuRename:
			m.openRenamePrompt()
		case menuDelete:
			m.openDeletePrompt()
		case menuGoHome:
			m.goHome()
		case menuOpenSelection:
			m.Menu.Active = false
			m.openWelcomeSelection()
		case menuDispatch:
			m.Menu.Active = false
			next, cmd := m.dispatch(entry.Action)
			*m = next
			return cmd
		}
		return nil
	}

	// Everything else is typed into the filter, so no letter can be a
	// navigation key here.
	if msg.Type == tea.KeyRunes && !msg.Alt {
		m.Menu.Query += string(msg.Runes)
		m.Menu.Selected = 0
	} else if msg.Type == tea.KeySpace {
		m.Menu.Query += " "
		m.Menu.Selected = 0
	}
	return nil
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

// menuBackdropColor is the 256-color index the menu paints its pane with. It
// sits just above black so the overlay reads as dark as the editor rather than
// as a light grey slab.
const menuBackdropColor = "233"

// menuPadX is the horizontal padding inside a menu row, so the highlight of the
// selected entry does not sit flush against its label.
const menuPadX = 3

// menuSlack widens the box beyond its measured content. Some binding glyphs
// (⌫, ←→) are drawn wider than their reported width, which would otherwise
// squeeze the key against its label.
const menuSlack = 3

// menuRowKind decides how a menu row is styled and whether it can be selected.
type menuRowKind int

const (
	menuRowTitle menuRowKind = iota
	menuRowBlank
	menuRowFilter
	menuRowAction
	menuRowSection
	menuRowHint
	menuRowEmpty
)

// menuRow is one line of the menu block. Entry is the index into menuActions
// for selectable rows and -1 for everything else.
type menuRow struct {
	Kind   menuRowKind
	Label  string
	Key    string
	Entry  int
	Danger bool
}

// menuBlock is the menu text, top to bottom: the runnable actions first, then
// the keys the menu cannot run but that are worth knowing.
func (m Model) menuBlock() []menuRow {
	rows := []menuRow{
		{Kind: menuRowTitle, Label: "actions", Entry: -1},
		{Kind: menuRowBlank, Entry: -1},
		{Kind: menuRowFilter, Label: m.Menu.Query, Entry: -1},
		{Kind: menuRowBlank, Entry: -1},
	}

	entries := m.menuActions()
	if len(entries) == 0 {
		return append(rows, menuRow{Kind: menuRowEmpty, Label: "no match", Entry: -1})
	}
	for i, entry := range entries {
		if entry.Danger || entry.Gap {
			rows = append(rows, menuRow{Kind: menuRowBlank, Entry: -1})
		}
		rows = append(rows, menuRow{
			Kind:   menuRowAction,
			Label:  entry.Label,
			Key:    entry.Key,
			Entry:  i,
			Danger: entry.Danger,
		})
	}
	if hints := m.menuHints(); len(hints) > 0 {
		rows = append(rows,
			menuRow{Kind: menuRowBlank, Entry: -1},
			menuRow{Kind: menuRowSection, Label: "keys", Entry: -1},
		)
		for _, hint := range hints {
			rows = append(rows, menuRow{Kind: menuRowHint, Label: hint.Label, Key: hint.Key, Entry: -1})
		}
	}
	return rows
}

// fitMenuBlock trims the block to the rows that are available. The hint section
// goes first, because it is only reference material; if the runnable actions
// still do not fit, the list scrolls to keep the selected one visible.
func fitMenuBlock(rows []menuRow, height, selected int) []menuRow {
	if height <= 0 {
		return nil
	}
	// The reference section is the first thing to go: it is only material to
	// read, not to run.
	for len(rows) > height && rows[len(rows)-1].Kind != menuRowAction {
		rows = rows[:len(rows)-1]
	}
	if len(rows) <= height {
		return rows
	}

	// The title stays pinned at the top and the destructive entry at the
	// bottom, so it can never scroll out of sight; the entries between them
	// scroll to keep the selection visible.
	// The title and the filter row stay put: the filter has the keyboard.
	head := rows[:min(len(rows), 4)]
	if height <= len(head) {
		return rows[:height]
	}
	tail := []menuRow{}
	body := rows[len(head):]
	if last := body[len(body)-1]; last.Danger {
		tail = []menuRow{last}
		body = body[:len(body)-1]
		if len(body) > 0 && body[len(body)-1].Kind == menuRowBlank {
			body = body[:len(body)-1]
		}
	}

	avail := height - len(head) - len(tail)
	if avail < 1 {
		// Too short even for the pinned rows: the entries win.
		tail = nil
		avail = height - len(head)
	}
	sel := 0
	for i, row := range body {
		if row.Kind == menuRowAction && row.Entry == selected {
			sel = i
		}
	}
	start := clamp(sel-avail/2, 0, max(0, len(body)-avail))
	end := min(len(body), start+avail)

	out := make([]menuRow, 0, len(head)+end-start+len(tail))
	out = append(out, head...)
	out = append(out, body[start:end]...)
	return append(out, tail...)
}

// menuKeyGap separates a label from its key inside a menu row.
const menuKeyGap = 4

// renderMenuRow draws one row of the region the action menu covers: the content
// pane in the main layout, the whole screen on the welcome screen. Every row
// belongs to the menu, so the block of entries is centered in the region while
// its lines stay left-aligned with each other.
func (m Model) renderMenuRow(width, height, row int) (string, bool) {
	if !m.Menu.Active || width <= 0 {
		return "", false
	}
	block := fitMenuBlock(m.menuBlock(), height, m.Menu.Selected)
	if len(block) == 0 {
		return styleMenuBackdrop.Render(strings.Repeat(" ", width)), true
	}

	// The labels share one column and the keys another, so the keys line up
	// instead of drifting to the far edge of the box.
	labelWidth, keyWidth := 0, 0
	for _, line := range block {
		labelWidth = max(labelWidth, runewidth.StringWidth(line.Label))
		keyWidth = max(keyWidth, runewidth.StringWidth(line.Key))
	}
	keyColumn := labelWidth + menuKeyGap
	blockWidth := min(width, menuPadX+keyColumn+keyWidth+menuPadX+menuSlack)
	left := max(0, (width-blockWidth)/2)
	top := max(0, (height-len(block))/2)

	idx := row - top
	if idx < 0 || idx >= len(block) {
		return styleMenuBackdrop.Render(strings.Repeat(" ", width)), true
	}

	line := block[idx]
	style := styleMenuEntry
	switch line.Kind {
	case menuRowTitle:
		style = styleMenuTitle
	case menuRowSection, menuRowHint, menuRowEmpty:
		style = styleMenuHint
	}
	if line.Kind == menuRowFilter {
		// The filter owns the keyboard, so it carries the caret.
		field := styleMenuFilter.Render(strings.Repeat(" ", menuPadX)+"› "+line.Label) +
			styleMenuCaret.Render(" ")
		pad := max(0, blockWidth-menuPadX-2-runewidth.StringWidth(line.Label)-1)
		field += styleMenuFilter.Render(strings.Repeat(" ", pad))
		right := max(0, width-left-blockWidth)
		return styleMenuBackdrop.Render(strings.Repeat(" ", left)) +
			field +
			styleMenuBackdrop.Render(strings.Repeat(" ", right)), true
	}
	if line.Danger {
		style = styleMenuDanger
	}
	if line.Kind == menuRowAction && line.Entry == m.Menu.Selected {
		style = styleMenuSelected
		if line.Danger {
			style = styleMenuDangerSelected
		}
	}

	inner := strings.Repeat(" ", menuPadX) + line.Label
	if line.Key != "" {
		// Right-align the key inside its own column.
		pad := menuPadX + keyColumn + keyWidth - runewidth.StringWidth(inner) - runewidth.StringWidth(line.Key)
		if pad < 1 {
			pad = 1
		}
		inner += strings.Repeat(" ", pad) + line.Key
	}
	if pad := blockWidth - runewidth.StringWidth(inner); pad > 0 {
		inner += strings.Repeat(" ", pad)
	}
	inner = xansi.Truncate(inner, blockWidth, "")

	right := max(0, width-left-runewidth.StringWidth(inner))
	return styleMenuBackdrop.Render(strings.Repeat(" ", left)) +
		style.Render(inner) +
		styleMenuBackdrop.Render(strings.Repeat(" ", right)), true
}
