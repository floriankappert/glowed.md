package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/khw1031/glowed/internal/config"
	"github.com/khw1031/glowed/internal/docs"
	textedit "github.com/khw1031/glowed/internal/editor"
	llmclient "github.com/khw1031/glowed/internal/llm"
	"github.com/khw1031/glowed/internal/render"
	"github.com/khw1031/glowed/internal/search"
	filewatch "github.com/khw1031/glowed/internal/watch"
)

type Mode int

const (
	ModePreview Mode = iota
	ModeEdit
	ModeSource
)

type Focus int

const (
	FocusSearch Focus = iota
	FocusSidebar
	FocusPreview
	FocusEditor
	FocusChat
)

type editorState struct {
	Lines           []string
	CX              int
	CY              int
	ScrollY         int
	ScrollX         int
	Dirty           bool
	ExternalChanged bool
	File            string
	FileFingerprint string
	Undo            []editorSnapshot
	Redo            []editorSnapshot
}

type editorSnapshot struct {
	Lines   []string
	CX      int
	CY      int
	ScrollY int
	ScrollX int
	Dirty   bool
}

type chatState struct {
	Visible  bool
	Input    string
	Messages []llmclient.Message
	Scroll   int
	Loading  bool
	Err      string
}

type chatResultMsg struct {
	Message llmclient.Message
	Err     error
}

type sidebarRowKind int

const (
	sidebarRowDocument sidebarRowKind = iota
	sidebarRowDirectory
)

type sidebarRow struct {
	Kind     sidebarRowKind
	Rel      string
	Name     string
	Depth    int
	DocIndex int
	Expanded bool
}

type llmLaunchResultMsg struct {
	Result llmclient.LaunchResult
	Err    error
}

const maxEditorHistory = 100

type Model struct {
	Root string
	Cfg  config.Config

	Width  int
	Height int

	Docs       []docs.Document
	Results    []docs.Document
	ScanReport docs.ScanReport
	Query      string

	Selected        int
	ListScroll      int
	SidebarRows     []sidebarRow
	SidebarSelected int
	ExpandedDirs    map[string]bool
	PreviewScroll   int
	PreviewScrolls  map[string]int
	PreviewLines    []string
	PreviewRaw      string

	Mode           Mode
	Focus          Focus
	SidebarVisible bool
	PrefixPending  bool
	MouseEnabled   bool
	Splash         bool
	SplashSelected int

	Editor               editorState
	Highlight            map[int][]render.Span
	HighlightKey         uint64
	Selection            selectionState
	LastSelectionFile    string
	LastSelectionPayload string
	Chat                 chatState

	Prompt promptState
	Menu   menuState

	Status     string
	StatusKind string

	WatchFingerprint       string
	WatchIgnoreFingerprint string
	WatchDebounceGen       int
	WatchDebouncePending   bool
	WatchLastEvent         filewatch.Event
}

var (
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleTitle   = lipgloss.NewStyle().Bold(true)
	styleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	styleReverse = lipgloss.NewStyle().Reverse(true)
	styleCursor  = lipgloss.NewStyle().Reverse(true)
	styleHeading = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))

	// The action menu covers the content pane, so it carries its own backdrop.
	styleMenuBackdrop = lipgloss.NewStyle().Background(lipgloss.Color(menuBackdropColor))
	styleMenuTitle    = lipgloss.NewStyle().Background(lipgloss.Color(menuBackdropColor)).Foreground(lipgloss.Color("11")).Bold(true)
	styleMenuEntry    = lipgloss.NewStyle().Background(lipgloss.Color(menuBackdropColor)).Foreground(lipgloss.Color("15"))
	styleMenuSelected = lipgloss.NewStyle().Background(lipgloss.Color("12")).Foreground(lipgloss.Color("0")).Bold(true)
	styleMenuHint     = lipgloss.NewStyle().Background(lipgloss.Color(menuBackdropColor)).Foreground(lipgloss.Color("8"))

	stylePaneActive        = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	stylePaneCaptionActive = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
)

func New(root string) Model {
	return NewWithInitial(root, "")
}

func NewWithInitial(root string, initialPath string) Model {
	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}
	cfg, cfgErrs := config.Load(root)
	m := Model{
		Root:           root,
		Cfg:            cfg,
		Width:          100,
		Height:         30,
		Mode:           ModePreview,
		Focus:          FocusPreview,
		SidebarVisible: true,
		MouseEnabled:   true,
		Splash:         initialPath == "",
		ExpandedDirs:   map[string]bool{},
		PreviewScrolls: map[string]int{},
		StatusKind:     "info",
		Editor: editorState{
			Lines: []string{""},
		},
	}
	if len(cfgErrs) > 0 {
		m.setStatus(cfgErrs[0].Error(), "warn")
	}
	m.scan("ready")
	m.initializePollingRefresh()
	if initialPath != "" {
		m.selectInitialDocument(initialPath)
	}
	// Edit is the default mode; fall back to preview when there is nothing to
	// edit, for instance in an empty project. The startup notice about the scan
	// and polling refresh is more useful here than the edit banner.
	if m.currentDoc() != nil {
		status, kind := m.Status, m.StatusKind
		m.enterEditMode()
		if status != "" {
			m.Status, m.StatusKind = status, kind
		}
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return pollTickCmd(m.Root)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	next.refreshHighlight()
	return next, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	// The welcome screen owns the keyboard until a document is opened, so keys
	// meant for its file list never reach the buffer.
	if m.Splash {
		if key, ok := msg.(tea.KeyMsg); ok {
			name := normalizeKey(key.String())
			if name == "ctrl+c" {
				m.shutdown()
				return m, tea.Quit
			}
			if m.Prompt.Active {
				cmd := m.handlePromptKey(key)
				return m, cmd
			}
			if m.Menu.Active {
				cmd := m.handleMenuKey(name)
				return m, cmd
			}
			if name == "ctrl+n" {
				m.openNewFilePrompt()
				return m, nil
			}
			if name == "ctrl+p" {
				m.toggleActionMenu()
				return m, nil
			}
			m.handleWelcomeKey(name)
			return m, nil
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.reloadPreview()
		m.ensureEditorVisible()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		cmd := m.handleMouse(msg)
		return m, cmd
	case clipboardResultMsg:
		m.handleClipboardResult(msg)
		return m, nil
	case chatResultMsg:
		m.handleChatResult(msg)
		return m, nil
	case llmLaunchResultMsg:
		m.handleLLMLaunchResult(msg)
		return m, nil
	case watchDebouncedMsg:
		return m.handleWatchDebounced(msg)
	case pollTickMsg:
		return m.handlePollTick(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := normalizeKey(msg.String())

	if key == "ctrl+c" {
		m.shutdown()
		return m, tea.Quit
	}
	if key == normalizeKey(m.Cfg.Prefix) {
		m.PrefixPending = true
		m.setStatus("prefix "+m.Cfg.Prefix, "info")
		return m, nil
	}
	// The toolbar prompt and the action menu own the keyboard while they are
	// open, in every mode.
	if m.Prompt.Active {
		cmd := m.handlePromptKey(msg)
		return m, cmd
	}
	if m.Menu.Active {
		cmd := m.handleMenuKey(key)
		return m, cmd
	}
	if key == "ctrl+n" {
		m.openNewFilePrompt()
		return m, nil
	}
	if key == "ctrl+p" {
		m.toggleActionMenu()
		return m, nil
	}

	if m.PrefixPending {
		m.PrefixPending = false
		if action := m.actionForKey(key, m.Cfg.PrefixKeys); action != "" {
			return m.dispatch(action)
		}
		m.setStatus("unknown prefix key: "+key, "warn")
		return m, nil
	}

	action := m.actionForKey(key, m.Cfg.Keys)

	// The sidebar toggle and focus switch work in every mode, including while
	// editing, where the browse bindings are not available.
	if key == "ctrl+b" {
		return m.dispatch("toggleSidebar")
	}
	if key == "shift+tab" {
		m.toggleSidebarFocus()
		return m, nil
	}

	if m.Chat.Visible && m.Focus == FocusChat {
		if action == "nextFocus" {
			m.cycleFocus()
			return m, nil
		}
		cmd := m.handleChatKey(msg)
		return m, cmd
	}

	if m.Mode == ModeEdit {
		if action == "save" {
			m.saveEditor()
			return m, nil
		}
		if action == "undo" || key == "cmd+z" {
			m.editorUndo()
			return m, nil
		}
		if action == "redo" || key == "cmd+shift+z" || key == "shift+cmd+z" || key == "ctrl+shift+z" || key == "shift+ctrl+z" {
			m.editorRedo()
			return m, nil
		}
		if m.SidebarVisible && m.Focus == FocusSidebar {
			m.handleEditSidebarKey(key)
			return m, nil
		}
		if key == "esc" {
			if m.hasEditorSelection() {
				m.clearEditorSelection()
				m.setStatus("selection cleared", "info")
				return m, nil
			}
			m.cancelEdit()
			return m, nil
		}
		return m, m.handleEditorKey(msg)
	}

	if m.Mode == ModeSource {
		if key == "esc" {
			m.exitSourceMode()
			return m, nil
		}
		if action != "" && action != "save" && isDirectAction(action) {
			return m.dispatch(action)
		}
		m.handleSourceNavigation(key)
		return m, nil
	}

	if m.Focus == FocusSearch {
		m.handleSearchKey(msg)
		return m, nil
	}

	if m.SidebarVisible && m.Focus == FocusSidebar && m.sidebarToggleKey(key) {
		m.toggleSidebarDirectory()
		return m, nil
	}

	if action != "" && isDirectAction(action) {
		return m.dispatch(action)
	}

	if !m.SidebarVisible || m.Focus == FocusPreview {
		m.handlePreviewNavigation(key)
	} else {
		m.handleSidebarNavigation(key)
	}
	return m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) {
	key := normalizeKey(msg.String())
	switch key {
	case "esc", "enter":
		if m.SidebarVisible {
			m.Focus = FocusSidebar
		} else {
			m.Focus = FocusPreview
		}
		return
	case "backspace", "ctrl+h":
		r := []rune(m.Query)
		if len(r) > 0 {
			m.Query = string(r[:len(r)-1])
			m.applySearch(false)
		}
		return
	case "ctrl+u":
		m.Query = ""
		m.applySearch(false)
		return
	case "ctrl+w", "alt+backspace":
		m.Query = deletePreviousSearchWord(m.Query)
		m.applySearch(false)
		return
	case "tab":
		m.cycleFocus()
		return
	}
	if input, ok := searchInputFromKey(msg); ok {
		m.Query += input
		m.applySearch(false)
	}
}

func searchInputFromKey(msg tea.KeyMsg) (string, bool) {
	// alt-modified keys are bindings, not text.
	if msg.Alt {
		return "", false
	}
	if msg.Type == tea.KeySpace {
		return " ", true
	}
	if msg.Type != tea.KeyRunes || len(msg.Runes) == 0 {
		return "", false
	}
	for _, r := range msg.Runes {
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return "", false
		}
	}
	return string(msg.Runes), true
}

func deletePreviousSearchWord(query string) string {
	r := []rune(query)
	i := len(r)
	for i > 0 && unicode.IsSpace(r[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(r[i-1]) {
		i--
	}
	return string(r[:i])
}

func (m *Model) handleChatKey(msg tea.KeyMsg) tea.Cmd {
	key := normalizeKey(msg.String())
	switch key {
	case "esc":
		m.Chat.Visible = false
		m.Focus = FocusPreview
		m.reloadPreview()
		m.ensureEditorVisible()
		m.setStatus("chat panel closed", "info")
		return nil
	case "enter":
		return m.sendChat()
	case "ctrl+j", "alt+enter", "shift+enter":
		m.Chat.Input += "\n"
		return nil
	case "backspace", "ctrl+h":
		r := []rune(m.Chat.Input)
		if len(r) > 0 {
			m.Chat.Input = string(r[:len(r)-1])
		}
		return nil
	case "ctrl+u":
		m.Chat.Input = ""
		return nil
	case "tab":
		m.cycleFocus()
		return nil
	case "up":
		m.Chat.Scroll = max(0, m.Chat.Scroll-1)
		return nil
	case "down":
		m.Chat.Scroll = min(max(0, len(m.chatLines())-m.chatBodyHeight()), m.Chat.Scroll+1)
		return nil
	case "pgup":
		m.Chat.Scroll = max(0, m.Chat.Scroll-m.chatBodyHeight())
		return nil
	case "pgdown":
		m.Chat.Scroll = min(max(0, len(m.chatLines())-m.chatBodyHeight()), m.Chat.Scroll+m.chatBodyHeight())
		return nil
	case " ":
		m.Chat.Input += " "
		return nil
	}
	if msg.Type == tea.KeyRunes && !msg.Alt {
		m.Chat.Input += string(msg.Runes)
	}
	return nil
}

func (m *Model) handlePreviewNavigation(key string) {
	switch key {
	case "up", "k":
		m.scrollPreview(-1)
	case "down", "j":
		m.scrollPreview(1)
	case "pgup":
		m.scrollPreview(-m.previewBodyHeight())
	case "pgdown":
		m.scrollPreview(m.previewBodyHeight())
	case "home":
		m.PreviewScroll = 0
		m.rememberPreviewScroll()
	case "end":
		m.PreviewScroll = max(0, len(m.PreviewLines)-m.previewBodyHeight())
		m.rememberPreviewScroll()
	case "ctrl+u":
		m.scrollPreview(-max(1, m.previewBodyHeight()/2))
	case "ctrl+d":
		m.scrollPreview(max(1, m.previewBodyHeight()/2))
	}
}

func (m *Model) handleSidebarNavigation(key string) {
	switch key {
	case "up", "k":
		m.moveSidebarSelection(-1)
	case "down", "j":
		m.moveSidebarSelection(1)
	case "pgup":
		m.moveSidebarSelection(-max(1, m.contentHeight()-1))
	case "pgdown":
		m.moveSidebarSelection(max(1, m.contentHeight()-1))
	case "home":
		m.setSidebarSelection(0)
	case "end":
		m.setSidebarSelection(len(m.SidebarRows) - 1)
	case "right", "l":
		m.expandSidebarDirectory()
	case "left", "h":
		m.collapseSidebarDirectory()
	}
}

// handleEditorKey maps a key to an editing operation.
//
// Ghostty does not deliver cmd combinations as such: it rewrites cmd+left,
// cmd+right and cmd+backspace into ctrl+a, ctrl+e and ctrl+u, and opt+left /
// opt+right into alt+b / alt+f. The bindings below are therefore expressed in
// terms of what actually reaches the program.
func (m *Model) handleEditorKey(msg tea.KeyMsg) tea.Cmd {
	key := normalizeKey(msg.String())

	// Pasted text arrives as a single bracketed-paste event and may span lines.
	if msg.Paste {
		m.pasteIntoEditor(string(msg.Runes))
		m.ensureEditorVisible()
		return nil
	}

	lines := m.Editor.Lines
	at := m.caret()
	var cmd tea.Cmd

	switch key {
	// Character-wise motion. A plain arrow collapses an active selection.
	case "up":
		if !m.collapseSelection(false) {
			m.moveEditor(0, -1)
		}
	case "down":
		if !m.collapseSelection(true) {
			m.moveEditor(0, 1)
		}
	case "left":
		if !m.collapseSelection(false) {
			m.moveEditor(-1, 0)
		}
	case "right":
		if !m.collapseSelection(true) {
			m.moveEditor(1, 0)
		}

	// Word-wise motion (opt+arrow).
	case "alt+b", "alt+left":
		m.moveCaretTo(textedit.WordLeft(lines, at), false)
	case "alt+f", "alt+right":
		m.moveCaretTo(textedit.WordRight(lines, at), false)

	// Line-wise motion. Ghostty sends ctrl+a / ctrl+e for cmd+left / cmd+right.
	case "home", "ctrl+a":
		m.moveCaretTo(textedit.LineStart(lines, at), false)
	case "end", "ctrl+e":
		m.moveCaretTo(textedit.LineEnd(lines, at), false)

	// Selection: shift extends from the anchor.
	case "shift+left":
		m.moveCaretTo(textedit.ClampPosition(lines, textedit.Position{Line: at.Line, Col: at.Col - 1}), true)
	case "shift+right":
		m.moveCaretTo(textedit.ClampPosition(lines, textedit.Position{Line: at.Line, Col: at.Col + 1}), true)
	case "shift+up":
		m.moveCaretTo(textedit.ClampPosition(lines, textedit.Position{Line: at.Line - 1, Col: at.Col}), true)
	case "shift+down":
		m.moveCaretTo(textedit.ClampPosition(lines, textedit.Position{Line: at.Line + 1, Col: at.Col}), true)
	case "alt+shift+left":
		m.moveCaretTo(textedit.WordLeft(lines, at), true)
	case "alt+shift+right":
		m.moveCaretTo(textedit.WordRight(lines, at), true)
	case "shift+home":
		m.moveCaretTo(textedit.LineStart(lines, at), true)
	case "shift+end":
		m.moveCaretTo(textedit.LineEnd(lines, at), true)
	case "alt+a":
		m.selectAllEditor()
	case "alt+c":
		cmd = m.copyEditorSelection()
	case "alt+v":
		m.pasteFromClipboard()

	case "pgup":
		m.moveCaretTo(textedit.ClampPosition(lines, textedit.Position{Line: at.Line - m.editorTextHeight(), Col: at.Col}), false)
	case "pgdown":
		m.moveCaretTo(textedit.ClampPosition(lines, textedit.Position{Line: at.Line + m.editorTextHeight(), Col: at.Col}), false)

	// Deletion. Each variant removes an active selection first.
	case "backspace", "ctrl+h":
		if !m.deleteEditorSelection() {
			m.editorBackspace()
		}
	case "delete":
		if !m.deleteEditorSelection() {
			m.editorDelete()
		}
	case "alt+backspace":
		if !m.deleteEditorSelection() {
			m.deleteEditorRange(at, textedit.WordLeft(lines, at))
		}
	case "alt+delete", "alt+d":
		if !m.deleteEditorSelection() {
			m.deleteEditorRange(at, textedit.WordRight(lines, at))
		}
	case "ctrl+u":
		if !m.deleteEditorSelection() {
			m.deleteEditorRange(at, textedit.LineStart(lines, at))
		}
	case "ctrl+k":
		if !m.deleteEditorSelection() {
			m.deleteEditorRange(at, textedit.LineEnd(lines, at))
		}

	case "enter":
		m.replaceSelection("\n")
	case "tab":
		m.replaceSelection("  ")
	default:
		if text, ok := editorInputFromKey(msg); ok {
			m.replaceSelection(text)
		} else if msg.Alt {
			// On layouts where brackets and friends are Option-composed, Ghostty's
			// macos-option-as-alt = true turns them into alt combinations that no
			// longer carry the composed rune. Say so instead of dropping silently.
			m.setStatus(key+" unbound — for [ ] { } set macos-option-as-alt = left in Ghostty", "warn")
		}
	}
	m.ensureEditorVisible()
	return cmd
}

// editorInputFromKey returns the text a key should insert into the buffer.
// Space arrives as its own key type rather than as runes, and unbound alt
// combinations and control sequences must never leak into the document.
func editorInputFromKey(msg tea.KeyMsg) (string, bool) {
	if msg.Alt {
		return "", false
	}
	if msg.Type == tea.KeySpace {
		return " ", true
	}
	if msg.Type != tea.KeyRunes || len(msg.Runes) == 0 {
		return "", false
	}
	for _, r := range msg.Runes {
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return "", false
		}
	}
	return string(msg.Runes), true
}

// toggleSidebarFocus moves the focus between the content pane and the sidebar,
// opening the sidebar first when it is hidden.
func (m *Model) toggleSidebarFocus() {
	if m.Focus == FocusSidebar {
		m.Focus = m.contentFocus()
		if m.Mode == ModeEdit {
			m.setStatus("editing "+filepath.Base(m.Editor.File), "info")
		} else {
			m.setStatus("preview", "info")
		}
		return
	}
	if !m.SidebarVisible {
		m.SidebarVisible = true
		m.reloadPreview()
		m.ensureEditorVisible()
	}
	m.ensureSidebarState()
	m.Focus = FocusSidebar
	if m.Mode == ModeEdit {
		m.setStatus("sidebar — up/down select, enter edits, shift+tab back", "info")
	} else {
		m.setStatus("sidebar — up/down select, enter opens, shift+tab back", "info")
	}
}

// contentFocus is the focus the content pane uses in the current mode.
func (m Model) contentFocus() Focus {
	if m.Mode == ModeEdit {
		return FocusEditor
	}
	return FocusPreview
}

// handleEditSidebarKey drives the sidebar while edit mode keeps the buffer open.
func (m *Model) handleEditSidebarKey(key string) {
	switch key {
	case "esc", "tab":
		m.Focus = FocusEditor
		return
	case "enter":
		if row := m.currentSidebarRow(); row != nil && row.Kind == sidebarRowDirectory {
			m.toggleSidebarDirectory()
			return
		}
		m.openSidebarSelectionForEditing()
		return
	}
	m.handleSidebarNavigation(key)
}

// openSidebarSelectionForEditing opens the selected document in edit mode.
func (m *Model) openSidebarSelectionForEditing() {
	row := m.currentSidebarRow()
	if row == nil || row.Kind != sidebarRowDocument {
		return
	}
	m.setSelection(row.DocIndex)
	m.enterEditMode()
}

func (m *Model) handleSourceNavigation(key string) {
	switch key {
	case "up", "k":
		m.Editor.ScrollY = clamp(m.Editor.ScrollY-1, 0, max(0, len(m.Editor.Lines)-m.editorTextHeight()))
	case "down", "j":
		m.Editor.ScrollY = clamp(m.Editor.ScrollY+1, 0, max(0, len(m.Editor.Lines)-m.editorTextHeight()))
	case "pgup":
		m.Editor.ScrollY = clamp(m.Editor.ScrollY-m.editorTextHeight(), 0, max(0, len(m.Editor.Lines)-m.editorTextHeight()))
	case "pgdown":
		m.Editor.ScrollY = clamp(m.Editor.ScrollY+m.editorTextHeight(), 0, max(0, len(m.Editor.Lines)-m.editorTextHeight()))
	case "home":
		m.Editor.ScrollY = 0
	case "end":
		m.Editor.ScrollY = max(0, len(m.Editor.Lines)-m.editorTextHeight())
	case "left", "h":
		m.Editor.ScrollX = max(0, m.Editor.ScrollX-1)
	case "right", "l":
		m.Editor.ScrollX++
	}
}

func (m Model) dispatch(action string) (Model, tea.Cmd) {
	switch action {
	case "quit":
		m.shutdown()
		return m, tea.Quit
	case "search":
		m.Focus = FocusSearch
		m.Mode = ModePreview
		m.setStatus("search: foo bar=AND, tag:foo, title/body/path", "info")
	case "edit":
		m.enterEditMode()
	case "sourceSelect":
		if m.Mode == ModeSource {
			m.exitSourceMode()
		} else {
			m.enterSourceMode()
		}
	case "openLLM":
		return m, m.launchExternalLLMSession()
	case "save":
		m.saveEditor()
	case "selectAll":
		if m.Mode == ModeEdit {
			m.selectAllEditor()
		}
	case "cancelEdit":
		if m.Mode == ModeEdit {
			m.cancelEdit()
		}
	case "refresh":
		m.scan("manual")
	case "nextFocus":
		m.cycleFocus()
	case "open":
		if m.Mode == ModeSource {
			m.exitSourceMode()
		} else {
			m.Focus = FocusPreview
		}
	case "toggleSidebar":
		m.SidebarVisible = !m.SidebarVisible
		if !m.SidebarVisible && m.Focus == FocusSidebar {
			m.Focus = FocusPreview
		}
		m.reloadPreview()
		m.ensureEditorVisible()
		m.setStatus(fmt.Sprintf("sidebar %s", map[bool]string{true: "shown", false: "hidden"}[m.SidebarVisible]), "info")
	}
	return m, nil
}

func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	mouse := tea.MouseEvent(msg)
	x, y := mouse.X, mouse.Y
	if !m.MouseEnabled {
		return nil
	}

	if m.Selection.Dragging {
		switch mouse.Action {
		case tea.MouseActionMotion:
			if m.Selection.Kind == "preview" {
				if p, ok := m.previewPointFromMouse(x, y, true); ok {
					m.updateEditorSelection(p)
				}
			} else if p, ok := m.editorPointFromMouse(x, y, true); ok {
				m.updateEditorSelection(p)
			}
			return nil
		case tea.MouseActionRelease:
			if m.Selection.Kind == "preview" {
				if p, ok := m.previewPointFromMouse(x, y, true); ok {
					return m.finishPreviewSelection(p)
				}
			} else if p, ok := m.editorPointFromMouse(x, y, true); ok {
				return m.finishEditorSelection(p)
			}
			m.clearEditorSelection()
			return nil
		}
	}

	if m.searchVisible() && y == m.searchRow() && mouse.Action == tea.MouseActionPress {
		m.Focus = FocusSearch
		return nil
	}

	if mouse.Button == tea.MouseButtonWheelUp {
		m.scrollAt(x, -3)
		return nil
	}
	if mouse.Button == tea.MouseButtonWheelDown {
		m.scrollAt(x, 3)
		return nil
	}
	if mouse.Button != tea.MouseButtonLeft || mouse.Action != tea.MouseActionPress {
		return nil
	}

	if y < m.contentTop() || y >= m.contentTop()+m.contentHeight() {
		return nil
	}
	row := y - m.contentTop()
	if m.Chat.Visible && x >= m.chatStartX() {
		m.Focus = FocusChat
		return nil
	}
	if m.SidebarVisible && x < m.leftWidth() {
		idx := m.ListScroll + row
		if idx >= 0 && idx < len(m.SidebarRows) {
			editing := m.Mode == ModeEdit
			if !editing {
				m.Mode = ModePreview
			}
			m.Focus = FocusSidebar
			m.setSidebarSelection(idx)
			if m.SidebarRows[idx].Kind == sidebarRowDirectory {
				m.toggleSidebarDirectory()
			} else if editing {
				// Edit is the default mode, so clicking a document opens it for
				// editing instead of dropping back into the preview.
				m.openSidebarSelectionForEditing()
			}
		}
		return nil
	}
	if m.rawBufferMode() {
		if m.Mode == ModeEdit {
			m.Focus = FocusEditor
		} else {
			m.Focus = FocusPreview
		}
		if p, ok := m.editorPointFromMouse(x, y, false); ok {
			m.startEditorSelection(p)
		}
	} else {
		m.Focus = FocusPreview
		if p, ok := m.previewPointFromMouse(x, y, false); ok {
			m.startPreviewSelection(p)
		}
	}
	return nil
}

func (m *Model) scrollAt(x int, delta int) {
	if m.Chat.Visible && x >= m.chatStartX() {
		m.Chat.Scroll = clamp(m.Chat.Scroll+delta, 0, max(0, len(m.chatLines())-m.chatBodyHeight()))
		return
	}
	if m.SidebarVisible && x < m.leftWidth() {
		m.ListScroll = clamp(m.ListScroll+delta, 0, max(0, len(m.SidebarRows)-m.contentHeight()))
		return
	}
	if m.rawBufferMode() {
		m.Editor.ScrollY = clamp(m.Editor.ScrollY+delta, 0, max(0, len(m.Editor.Lines)-m.editorTextHeight()))
		return
	}
	m.scrollPreview(delta)
}

func (m *Model) scan(reason string) {
	if err := m.scanAndApply(reason != "ready"); err != nil {
		m.setStatus(err.Error(), "error")
		return
	}
	m.setStatus(m.scanStatus(), "success")
}

func (m *Model) scanAndApply(keep bool) error {
	list, report, err := docs.ScanWithReport(m.Root, m.Cfg.Scan.MaxFileBytes)
	if err != nil {
		return err
	}
	m.Docs = list
	m.ScanReport = report
	m.applySearch(keep)
	return nil
}

func (m Model) scanStatus() string {
	status := fmt.Sprintf("%d markdown file(s) scanned", len(m.Docs))
	if len(m.ScanReport.Excluded) == 0 {
		return status
	}
	first := m.ScanReport.Excluded[0]
	kind := "file"
	if first.IsDir {
		kind = "dir"
	}
	extra := ""
	if len(m.ScanReport.Excluded) > 1 {
		extra = fmt.Sprintf(", +%d more", len(m.ScanReport.Excluded)-1)
	}
	return fmt.Sprintf("%s, %d hidden (first %s: %s by %s%s)", status, len(m.ScanReport.Excluded), kind, first.Rel, first.Reason, extra)
}

func (m *Model) applySearch(keep bool) {
	m.rememberPreviewScroll()
	old := ""
	oldIndex := m.Selected
	if doc := m.currentDoc(); doc != nil {
		old = doc.Abs
	}
	m.Results = search.Filter(m.Docs, m.Query)
	if keep {
		found := false
		if old != "" {
			for i, doc := range m.Results {
				if doc.Abs == old {
					m.Selected = i
					found = true
					break
				}
			}
		}
		if !found {
			m.Selected = clamp(oldIndex, 0, max(0, len(m.Results)-1))
		}
	} else {
		m.Selected = 0
		m.ListScroll = 0
		m.SidebarSelected = 0
	}
	if m.Selected >= len(m.Results) {
		m.Selected = max(0, len(m.Results)-1)
	}
	m.expandAncestorsForCurrentDoc()
	m.rebuildSidebarRows()
	m.syncSidebarSelectionToCurrentDoc()
	m.ensureSidebarSelectionVisible()
	m.restorePreviewScroll()
	m.reloadPreview()
}

func (m *Model) currentDoc() *docs.Document {
	if len(m.Results) == 0 || m.Selected < 0 || m.Selected >= len(m.Results) {
		return nil
	}
	return &m.Results[m.Selected]
}

func (m *Model) selectInitialDocument(path string) {
	target, err := docs.GuardExistingPath(m.Root, path)
	if err != nil {
		m.setStatus("initial file blocked: "+err.Error(), "warn")
		return
	}
	for i, doc := range m.Results {
		docPath, err := docs.GuardExistingPath(m.Root, doc.Abs)
		if err == nil && docPath == target {
			m.setSelection(i)
			m.setStatus("opened initial file "+doc.Rel, "success")
			return
		}
	}
	m.setStatus("initial file is not in markdown scan results: "+path, "warn")
}

func (m *Model) reloadPreview() {
	doc := m.currentDoc()
	if doc == nil {
		m.PreviewLines = []string{"No markdown document selected.", "", "Create or add .md files, then press r to refresh."}
		return
	}
	path, err := docs.GuardExistingPath(m.Root, doc.Abs)
	if err != nil {
		m.PreviewLines = []string{"read blocked: " + err.Error()}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		m.PreviewLines = []string{"read failed: " + err.Error()}
		return
	}
	m.PreviewRaw = string(raw)
	width := max(20, m.rightWidth()-2)
	rendered, err := render.Markdown(m.PreviewRaw, width, m.Cfg.Preview.Style, m.Cfg.Preview.PreserveNewLines)
	if err != nil {
		rendered = "render failed: " + err.Error() + "\n\n" + m.PreviewRaw
	}
	m.PreviewLines = strings.Split(strings.ReplaceAll(rendered, "\r\n", "\n"), "\n")
	m.PreviewScroll = clamp(m.PreviewScroll, 0, max(0, len(m.PreviewLines)-m.previewBodyHeight()))
}

func (m *Model) enterEditMode() {
	doc := m.currentDoc()
	if doc == nil {
		m.setStatus("no document selected", "warn")
		return
	}
	if m.Editor.Dirty && m.Editor.File != "" && !m.pathsMatch(m.Editor.File, doc.Abs) {
		m.setStatus("unsaved changes in "+filepath.Base(m.Editor.File)+" — ctrl+s to save, esc to discard", "warn")
		return
	}
	path, err := docs.GuardExistingPath(m.Root, doc.Abs)
	if err != nil {
		m.setStatus("open blocked: "+err.Error(), "error")
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		m.setStatus("open failed: "+err.Error(), "error")
		return
	}
	fingerprint, _ := filewatch.ContentFingerprint(path)
	m.Editor = editorState{Lines: splitEditorLines(string(raw)), File: path, FileFingerprint: fingerprint}
	m.clearEditorSelection()
	m.Mode = ModeEdit
	m.Focus = FocusEditor
	m.setStatus("editing "+doc.Rel+" — ctrl+s save, esc cancel", "info")
}

func (m *Model) saveEditor() {
	if m.Mode != ModeEdit || m.Editor.File == "" {
		m.setStatus("nothing to save", "warn")
		return
	}
	file, err := docs.GuardExistingPath(m.Root, m.Editor.File)
	if err != nil {
		m.setStatus("save blocked: "+err.Error(), "error")
		return
	}
	backup, err := textedit.SaveFileAtomicWithBackup(file, []byte(strings.Join(m.Editor.Lines, "\n")))
	if err != nil {
		m.setStatus("save failed: "+err.Error(), "error")
		return
	}
	m.Editor.File = file
	m.Editor.FileFingerprint, _ = filewatch.ContentFingerprint(file)
	m.Editor.Dirty = false
	m.Editor.ExternalChanged = false
	m.clearEditorSelection()
	// Edit is the default mode, so saving keeps the buffer open instead of
	// dropping back into the preview.
	m.Focus = FocusEditor
	m.scan("save")
	m.reloadPreview()
	m.setStatus("saved "+file+" (backup "+backup+")", "success")
}

func (m *Model) cancelEdit() {
	m.clearEditorSelection()
	m.Mode = ModePreview
	m.Focus = FocusPreview
	m.reloadPreview()
	m.setStatus("edit cancelled", "warn")
}

func (m *Model) enterSourceMode() {
	doc := m.currentDoc()
	if doc == nil {
		m.setStatus("no document selected", "warn")
		return
	}
	path, err := docs.GuardExistingPath(m.Root, doc.Abs)
	if err != nil {
		m.setStatus("source blocked: "+err.Error(), "error")
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		m.setStatus("source open failed: "+err.Error(), "error")
		return
	}
	fingerprint, _ := filewatch.ContentFingerprint(path)
	m.Editor = editorState{Lines: splitEditorLines(string(raw)), File: path, FileFingerprint: fingerprint}
	m.clearEditorSelection()
	m.Mode = ModeSource
	m.Focus = FocusPreview
	m.setStatus("source selection — drag raw markdown to copy, esc preview", "info")
}

func (m *Model) exitSourceMode() {
	m.clearEditorSelection()
	m.Mode = ModePreview
	m.Focus = FocusPreview
	m.reloadPreview()
	m.setStatus("source selection closed", "info")
}

func (m Model) rawBufferMode() bool {
	return m.Mode == ModeEdit || m.Mode == ModeSource
}

func (m *Model) launchExternalLLMSession() tea.Cmd {
	if !m.Cfg.LLM.Enabled {
		m.setStatus("llm disabled by config", "warn")
		return nil
	}
	req := llmclient.Request{Context: m.buildPathOnlyContext()}
	clipboardText := llmclient.BuildPrompt(req)
	cfg := llmclient.Config{
		Command:         m.Cfg.LLM.Command,
		TerminalCommand: m.Cfg.LLM.TerminalCommand,
		TerminalApp:     m.Cfg.LLM.TerminalApp,
		OpenMode:        m.Cfg.LLM.OpenMode,
	}
	m.setStatus("copied context to clipboard; opening external llm split", "info")
	return func() tea.Msg {
		if _, err := copyTextToClipboard(clipboardText); err != nil {
			return llmLaunchResultMsg{Err: err}
		}
		result, err := llmclient.LaunchExternal(context.Background(), cfg, req)
		return llmLaunchResultMsg{Result: result, Err: err}
	}
}

func (m *Model) handleLLMLaunchResult(msg llmLaunchResultMsg) {
	if msg.Err != nil {
		m.setStatus("llm launch failed: "+msg.Err.Error(), "error")
		return
	}
	m.setStatus("context copied. Paste it in the LLM split, add your question, then press Enter. ctx "+msg.Result.ContextFile, "success")
}

func (m *Model) sendChat() tea.Cmd {
	input := strings.TrimSpace(m.Chat.Input)
	if input == "" {
		return nil
	}
	if !m.Cfg.LLM.Enabled {
		m.setStatus("llm chat disabled by config", "warn")
		return nil
	}
	if m.Chat.Loading {
		m.setStatus("chat response already in progress", "warn")
		return nil
	}
	user := llmclient.Message{Role: "user", Content: input}
	m.Chat.Messages = append(m.Chat.Messages, user)
	m.Chat.Input = ""
	m.Chat.Loading = true
	m.Chat.Err = ""
	m.scrollChatToBottom()

	req := llmclient.Request{Messages: append([]llmclient.Message{}, m.Chat.Messages...), Context: m.buildChatContext()}
	provider := llmclient.NewProvider(llmclient.Config{Command: m.Cfg.LLM.Command})
	return func() tea.Msg {
		msg, err := provider.Send(context.Background(), req)
		return chatResultMsg{Message: msg, Err: err}
	}
}

func (m *Model) handleChatResult(msg chatResultMsg) {
	m.Chat.Loading = false
	if msg.Err != nil {
		m.Chat.Err = msg.Err.Error()
		m.setStatus("chat failed: "+msg.Err.Error(), "error")
		return
	}
	m.Chat.Messages = append(m.Chat.Messages, msg.Message)
	m.Chat.Err = ""
	m.scrollChatToBottom()
	m.setStatus("chat response received", "success")
}

func (m Model) buildPathOnlyContext() llmclient.FileContext {
	ctx := llmclient.FileContext{Root: m.Root, Mode: modeName(m.Mode)}
	doc := m.currentDoc()
	if doc != nil {
		ctx.AbsPath = doc.Abs
		ctx.RelPath = doc.Rel
	}
	if m.rawBufferMode() && m.Editor.File != "" {
		ctx.AbsPath = m.Editor.File
		if rel, err := filepath.Rel(m.Root, m.Editor.File); err == nil {
			ctx.RelPath = rel
		}
	}
	return ctx
}

func (m Model) buildChatContext() llmclient.FileContext {
	ctx := m.buildPathOnlyContext()
	if m.Cfg.LLM.IncludeSelection {
		if m.hasEditorSelection() {
			selected, _, ok := textedit.FormatClipboard(ctx.AbsPath, m.Editor.Lines, toEditorPosition(m.Selection.Start), toEditorPosition(m.Selection.End))
			if ok {
				ctx.SelectedMarkdown = selected
			}
		} else if m.LastSelectionPayload != "" && m.selectionFileMatchesContext(ctx.AbsPath) {
			ctx.SelectedMarkdown = m.LastSelectionPayload
		} else if payload := m.glowedSelectionFromClipboard(ctx.AbsPath); payload != "" {
			ctx.SelectedMarkdown = payload
		} else if payload := m.plainSelectionFromClipboard(ctx.AbsPath); payload != "" {
			ctx.SelectedMarkdown = payload
		}
	}
	if m.Cfg.LLM.IncludeCurrentFile {
		raw := m.currentRawMarkdownForChat(ctx.AbsPath)
		ctx.RawMarkdown, ctx.Truncated = limitContextBytes(raw, m.Cfg.LLM.MaxContextBytes)
	}
	return ctx
}

func (m Model) selectionFileMatchesContext(abs string) bool {
	return m.pathsMatch(m.LastSelectionFile, abs)
}

func (m Model) pathsMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	left, err := docs.GuardExistingPath(m.Root, a)
	if err != nil {
		left = a
	}
	right, err := docs.GuardExistingPath(m.Root, b)
	if err != nil {
		right = b
	}
	return left == right
}

func (m Model) currentRawMarkdownForChat(abs string) string {
	if m.rawBufferMode() && m.Editor.File == abs {
		return strings.Join(m.Editor.Lines, "\n")
	}
	if m.PreviewRaw != "" {
		return m.PreviewRaw
	}
	if abs == "" {
		return ""
	}
	path, err := docs.GuardExistingPath(m.Root, abs)
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func limitContextBytes(s string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len([]byte(s)) <= maxBytes {
		return s, false
	}
	used := 0
	var b strings.Builder
	for _, r := range s {
		rb := len(string(r))
		if used+rb > maxBytes {
			break
		}
		b.WriteRune(r)
		used += rb
	}
	return b.String(), true
}

func (m *Model) moveSidebarSelection(delta int) {
	m.setSidebarSelection(m.SidebarSelected + delta)
}

func (m *Model) setSidebarSelection(idx int) {
	if len(m.SidebarRows) == 0 {
		m.SidebarSelected = 0
		m.ListScroll = 0
		return
	}
	m.SidebarSelected = clamp(idx, 0, len(m.SidebarRows)-1)
	if row := m.currentSidebarRow(); row != nil && row.Kind == sidebarRowDocument {
		m.selectDocument(row.DocIndex, false)
	}
	m.ensureSidebarSelectionVisible()
}

func (m *Model) setSelection(idx int) {
	m.selectDocument(idx, true)
}

func (m *Model) selectDocument(idx int, syncSidebar bool) {
	m.clearEditorSelection()
	m.rememberPreviewScroll()
	if len(m.Results) == 0 {
		m.Selected = 0
		m.ListScroll = 0
		m.SidebarSelected = 0
		m.SidebarRows = nil
		m.PreviewScroll = 0
		m.reloadPreview()
		return
	}
	m.Selected = clamp(idx, 0, len(m.Results)-1)
	if syncSidebar {
		m.expandAncestorsForCurrentDoc()
		m.rebuildSidebarRows()
		m.syncSidebarSelectionToCurrentDoc()
	}
	m.ensureSidebarSelectionVisible()
	m.restorePreviewScroll()
	m.reloadPreview()
}

func (m *Model) ensureSidebarSelectionVisible() {
	h := m.contentHeight()
	if len(m.SidebarRows) == 0 {
		m.SidebarSelected = 0
		m.ListScroll = 0
		return
	}
	m.SidebarSelected = clamp(m.SidebarSelected, 0, len(m.SidebarRows)-1)
	if m.SidebarSelected < m.ListScroll {
		m.ListScroll = m.SidebarSelected
	}
	if m.SidebarSelected >= m.ListScroll+h {
		m.ListScroll = max(0, m.SidebarSelected-h+1)
	}
	m.ListScroll = clamp(m.ListScroll, 0, max(0, len(m.SidebarRows)-h))
}

func (m *Model) sidebarToggleKey(key string) bool {
	if key != "tab" && key != "enter" {
		return false
	}
	row := m.currentSidebarRow()
	return row != nil && row.Kind == sidebarRowDirectory
}

func (m *Model) currentSidebarRow() *sidebarRow {
	if len(m.SidebarRows) == 0 || m.SidebarSelected < 0 || m.SidebarSelected >= len(m.SidebarRows) {
		return nil
	}
	return &m.SidebarRows[m.SidebarSelected]
}

func (m *Model) toggleSidebarDirectory() {
	row := m.currentSidebarRow()
	if row == nil || row.Kind != sidebarRowDirectory {
		return
	}
	m.ensureSidebarState()
	m.ExpandedDirs[row.Rel] = !m.ExpandedDirs[row.Rel]
	m.rebuildSidebarRows()
	if idx := m.findSidebarRow(sidebarRowDirectory, row.Rel, -1); idx >= 0 {
		m.SidebarSelected = idx
	}
	m.ensureSidebarSelectionVisible()
	state := "collapsed"
	if m.ExpandedDirs[row.Rel] {
		state = "expanded"
	}
	m.setStatus(fmt.Sprintf("%s %s", row.Rel, state), "info")
}

func (m *Model) expandSidebarDirectory() {
	row := m.currentSidebarRow()
	if row == nil || row.Kind != sidebarRowDirectory || row.Expanded {
		return
	}
	m.toggleSidebarDirectory()
}

func (m *Model) collapseSidebarDirectory() {
	row := m.currentSidebarRow()
	if row == nil {
		return
	}
	m.ensureSidebarState()
	if row.Kind == sidebarRowDirectory && row.Expanded {
		m.toggleSidebarDirectory()
		return
	}
	parent := parentSlashRel(row.Rel)
	if parent == "" {
		return
	}
	if idx := m.findSidebarRow(sidebarRowDirectory, parent, -1); idx >= 0 {
		m.SidebarSelected = idx
		m.ensureSidebarSelectionVisible()
	}
}

func (m *Model) ensureSidebarState() {
	if m.ExpandedDirs == nil {
		m.ExpandedDirs = map[string]bool{}
	}
}

func (m *Model) expandAncestorsForCurrentDoc() {
	doc := m.currentDoc()
	if doc == nil || !m.sidebarTreeMode() {
		return
	}
	m.ensureSidebarState()
	for _, dir := range ancestorDirs(doc.Rel) {
		m.ExpandedDirs[dir] = true
	}
}

func (m *Model) rebuildSidebarRows() {
	m.ensureSidebarState()
	oldKind := sidebarRowDocument
	oldRel := ""
	oldDocIndex := -1
	if row := m.currentSidebarRow(); row != nil {
		oldKind = row.Kind
		oldRel = row.Rel
		oldDocIndex = row.DocIndex
	}
	m.SidebarRows = m.buildSidebarRows()
	if len(m.SidebarRows) == 0 {
		m.SidebarSelected = 0
		m.ListScroll = 0
		return
	}
	if oldRel != "" {
		if idx := m.findSidebarRow(oldKind, oldRel, oldDocIndex); idx >= 0 {
			m.SidebarSelected = idx
		} else {
			m.SidebarSelected = clamp(m.SidebarSelected, 0, len(m.SidebarRows)-1)
		}
	} else {
		m.SidebarSelected = clamp(m.SidebarSelected, 0, len(m.SidebarRows)-1)
	}
	m.ensureSidebarSelectionVisible()
}

func (m Model) buildSidebarRows() []sidebarRow {
	if strings.TrimSpace(m.Query) != "" {
		rows := make([]sidebarRow, 0, len(m.Results))
		for i, doc := range m.Results {
			rows = append(rows, sidebarRow{Kind: sidebarRowDocument, Rel: slashRel(doc.Rel), Name: doc.Name, DocIndex: i})
		}
		return rows
	}

	root := newSidebarTreeNode("", "")
	for i, doc := range m.Results {
		parts := splitSlashRel(doc.Rel)
		if len(parts) == 0 {
			continue
		}
		node := root
		for _, part := range parts[:len(parts)-1] {
			node = node.child(part)
		}
		node.docs = append(node.docs, i)
	}

	rows := []sidebarRow{}
	var walk func(node *sidebarTreeNode, depth int)
	walk = func(node *sidebarTreeNode, depth int) {
		for _, child := range node.sortedDirs() {
			rows = append(rows, sidebarRow{
				Kind:     sidebarRowDirectory,
				Rel:      child.rel,
				Name:     child.name,
				Depth:    depth,
				DocIndex: -1,
				Expanded: m.ExpandedDirs[child.rel],
			})
			if m.ExpandedDirs[child.rel] {
				walk(child, depth+1)
			}
		}
		for _, docIndex := range node.docs {
			doc := m.Results[docIndex]
			rows = append(rows, sidebarRow{Kind: sidebarRowDocument, Rel: slashRel(doc.Rel), Name: doc.Name, Depth: depth, DocIndex: docIndex})
		}
	}
	walk(root, 0)
	return rows
}

func (m *Model) syncSidebarSelectionToCurrentDoc() {
	if len(m.SidebarRows) == 0 || len(m.Results) == 0 {
		m.SidebarSelected = 0
		return
	}
	if idx := m.findSidebarRow(sidebarRowDocument, slashRel(m.Results[m.Selected].Rel), m.Selected); idx >= 0 {
		m.SidebarSelected = idx
	}
}

func (m Model) findSidebarRow(kind sidebarRowKind, rel string, docIndex int) int {
	rel = slashRel(rel)
	for i, row := range m.SidebarRows {
		if row.Kind != kind || row.Rel != rel {
			continue
		}
		if kind == sidebarRowDocument && docIndex >= 0 && row.DocIndex != docIndex {
			continue
		}
		return i
	}
	return -1
}

func (m Model) sidebarTreeMode() bool {
	return strings.TrimSpace(m.Query) == ""
}

type sidebarTreeNode struct {
	name string
	rel  string
	dirs map[string]*sidebarTreeNode
	docs []int
}

func newSidebarTreeNode(name, rel string) *sidebarTreeNode {
	return &sidebarTreeNode{name: name, rel: rel, dirs: map[string]*sidebarTreeNode{}}
}

func (n *sidebarTreeNode) child(name string) *sidebarTreeNode {
	if child, ok := n.dirs[name]; ok {
		return child
	}
	rel := name
	if n.rel != "" {
		rel = n.rel + "/" + name
	}
	child := newSidebarTreeNode(name, rel)
	n.dirs[name] = child
	return child
}

func (n *sidebarTreeNode) sortedDirs() []*sidebarTreeNode {
	names := make([]string, 0, len(n.dirs))
	for name := range n.dirs {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]*sidebarTreeNode, 0, len(names))
	for _, name := range names {
		out = append(out, n.dirs[name])
	}
	return out
}

func splitSlashRel(rel string) []string {
	rel = slashRel(rel)
	if rel == "" || rel == "." {
		return nil
	}
	parts := strings.Split(rel, "/")
	out := parts[:0]
	for _, part := range parts {
		if part != "" && part != "." {
			out = append(out, part)
		}
	}
	return out
}

func ancestorDirs(rel string) []string {
	parts := splitSlashRel(rel)
	if len(parts) <= 1 {
		return nil
	}
	out := make([]string, 0, len(parts)-1)
	cur := ""
	for _, part := range parts[:len(parts)-1] {
		if cur == "" {
			cur = part
		} else {
			cur += "/" + part
		}
		out = append(out, cur)
	}
	return out
}

func parentSlashRel(rel string) string {
	parts := splitSlashRel(rel)
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

func slashRel(rel string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(rel)), "./")
}

func (m *Model) scrollPreview(delta int) {
	m.PreviewScroll = clamp(m.PreviewScroll+delta, 0, max(0, len(m.PreviewLines)-m.previewBodyHeight()))
	m.rememberPreviewScroll()
}

func (m *Model) rememberPreviewScroll() {
	doc := m.currentDoc()
	if doc == nil {
		return
	}
	if m.PreviewScrolls == nil {
		m.PreviewScrolls = map[string]int{}
	}
	m.PreviewScrolls[doc.Abs] = m.PreviewScroll
}

func (m *Model) restorePreviewScroll() {
	m.PreviewScroll = 0
	doc := m.currentDoc()
	if doc == nil || m.PreviewScrolls == nil {
		return
	}
	m.PreviewScroll = m.PreviewScrolls[doc.Abs]
}

func (m *Model) cycleFocus() {
	if m.Mode == ModeEdit {
		if m.Chat.Visible && m.Focus == FocusEditor {
			m.Focus = FocusChat
		} else {
			m.Focus = FocusEditor
		}
		return
	}
	order := []Focus{FocusSearch}
	if m.SidebarVisible {
		order = append(order, FocusSidebar)
	}
	order = append(order, FocusPreview)
	if m.Chat.Visible {
		order = append(order, FocusChat)
	}
	for i, focus := range order {
		if m.Focus == focus {
			m.Focus = order[(i+1)%len(order)]
			return
		}
	}
	m.Focus = order[0]
}

func (m *Model) pushEditorUndo() {
	m.Editor.Undo = append(m.Editor.Undo, snapshotEditor(m.Editor))
	if len(m.Editor.Undo) > maxEditorHistory {
		m.Editor.Undo = append([]editorSnapshot{}, m.Editor.Undo[len(m.Editor.Undo)-maxEditorHistory:]...)
	}
	m.Editor.Redo = nil
}

func (m *Model) editorUndo() {
	if len(m.Editor.Undo) == 0 {
		m.setStatus("nothing to undo", "warn")
		return
	}
	last := m.Editor.Undo[len(m.Editor.Undo)-1]
	m.Editor.Undo = m.Editor.Undo[:len(m.Editor.Undo)-1]
	m.Editor.Redo = append(m.Editor.Redo, snapshotEditor(m.Editor))
	m.restoreEditorSnapshot(last)
	m.clearEditorSelection()
	m.ensureEditorVisible()
	m.setStatus("undo", "info")
}

func (m *Model) editorRedo() {
	if len(m.Editor.Redo) == 0 {
		m.setStatus("nothing to redo", "warn")
		return
	}
	last := m.Editor.Redo[len(m.Editor.Redo)-1]
	m.Editor.Redo = m.Editor.Redo[:len(m.Editor.Redo)-1]
	m.Editor.Undo = append(m.Editor.Undo, snapshotEditor(m.Editor))
	m.restoreEditorSnapshot(last)
	m.clearEditorSelection()
	m.ensureEditorVisible()
	m.setStatus("redo", "info")
}

func snapshotEditor(e editorState) editorSnapshot {
	return editorSnapshot{
		Lines:   append([]string{}, e.Lines...),
		CX:      e.CX,
		CY:      e.CY,
		ScrollY: e.ScrollY,
		ScrollX: e.ScrollX,
		Dirty:   e.Dirty,
	}
}

func (m *Model) restoreEditorSnapshot(s editorSnapshot) {
	m.Editor.Lines = append([]string{}, s.Lines...)
	if len(m.Editor.Lines) == 0 {
		m.Editor.Lines = []string{""}
	}
	m.Editor.CY = clamp(s.CY, 0, len(m.Editor.Lines)-1)
	m.Editor.CX = clamp(s.CX, 0, lineLen(m.Editor.Lines[m.Editor.CY]))
	m.Editor.ScrollY = clamp(s.ScrollY, 0, max(0, len(m.Editor.Lines)-m.editorTextHeight()))
	m.Editor.ScrollX = max(0, s.ScrollX)
	m.Editor.Dirty = s.Dirty
}

func (m *Model) moveEditor(dx, dy int) {
	if len(m.Editor.Lines) == 0 {
		m.Editor.Lines = []string{""}
	}
	if dy != 0 {
		m.Editor.CY = clamp(m.Editor.CY+dy, 0, len(m.Editor.Lines)-1)
		m.Editor.CX = min(m.Editor.CX, lineLen(m.Editor.Lines[m.Editor.CY]))
	}
	if dx < 0 {
		if m.Editor.CX == 0 && m.Editor.CY > 0 {
			m.Editor.CY--
			m.Editor.CX = lineLen(m.Editor.Lines[m.Editor.CY])
		} else {
			m.Editor.CX = max(0, m.Editor.CX-1)
		}
	}
	if dx > 0 {
		if m.Editor.CX >= lineLen(m.Editor.Lines[m.Editor.CY]) && m.Editor.CY < len(m.Editor.Lines)-1 {
			m.Editor.CY++
			m.Editor.CX = 0
		} else {
			m.Editor.CX = min(lineLen(m.Editor.Lines[m.Editor.CY]), m.Editor.CX+1)
		}
	}
}

func (m *Model) editorInsert(text string) {
	m.pushEditorUndo()
	m.insertText(text)
}

// insertText inserts at the caret without recording an undo step, so callers
// can combine it with a preceding deletion into a single undoable edit.
func (m *Model) insertText(text string) {
	line := []rune(m.Editor.Lines[m.Editor.CY])
	insert := []rune(text)
	out := append([]rune{}, line[:m.Editor.CX]...)
	out = append(out, insert...)
	out = append(out, line[m.Editor.CX:]...)
	m.Editor.Lines[m.Editor.CY] = string(out)
	m.Editor.CX += len(insert)
	m.Editor.Dirty = true
}

func (m *Model) editorEnter() {
	m.pushEditorUndo()
	m.splitLine()
}

// splitLine breaks the line at the caret without recording an undo step.
func (m *Model) splitLine() {
	line := []rune(m.Editor.Lines[m.Editor.CY])
	before := string(line[:m.Editor.CX])
	after := string(line[m.Editor.CX:])
	lines := append([]string{}, m.Editor.Lines[:m.Editor.CY]...)
	lines = append(lines, before, after)
	lines = append(lines, m.Editor.Lines[m.Editor.CY+1:]...)
	m.Editor.Lines = lines
	m.Editor.CY++
	m.Editor.CX = 0
	m.Editor.Dirty = true
}

func (m *Model) editorBackspace() {
	if m.Editor.CX > 0 {
		m.pushEditorUndo()
		line := []rune(m.Editor.Lines[m.Editor.CY])
		line = append(line[:m.Editor.CX-1], line[m.Editor.CX:]...)
		m.Editor.Lines[m.Editor.CY] = string(line)
		m.Editor.CX--
		m.Editor.Dirty = true
		return
	}
	if m.Editor.CY > 0 {
		m.pushEditorUndo()
		prevLen := lineLen(m.Editor.Lines[m.Editor.CY-1])
		m.Editor.Lines[m.Editor.CY-1] += m.Editor.Lines[m.Editor.CY]
		m.Editor.Lines = append(m.Editor.Lines[:m.Editor.CY], m.Editor.Lines[m.Editor.CY+1:]...)
		m.Editor.CY--
		m.Editor.CX = prevLen
		m.Editor.Dirty = true
	}
}

func (m *Model) editorDelete() {
	line := []rune(m.Editor.Lines[m.Editor.CY])
	if m.Editor.CX < len(line) {
		m.pushEditorUndo()
		line = append(line[:m.Editor.CX], line[m.Editor.CX+1:]...)
		m.Editor.Lines[m.Editor.CY] = string(line)
		m.Editor.Dirty = true
		return
	}
	if m.Editor.CY < len(m.Editor.Lines)-1 {
		m.pushEditorUndo()
		m.Editor.Lines[m.Editor.CY] += m.Editor.Lines[m.Editor.CY+1]
		m.Editor.Lines = append(m.Editor.Lines[:m.Editor.CY+1], m.Editor.Lines[m.Editor.CY+2:]...)
		m.Editor.Dirty = true
	}
}

func (m *Model) ensureEditorVisible() {
	if len(m.Editor.Lines) == 0 {
		return
	}
	h := m.editorTextHeight()
	if m.Editor.CY < m.Editor.ScrollY {
		m.Editor.ScrollY = m.Editor.CY
	}
	if m.Editor.CY >= m.Editor.ScrollY+h {
		m.Editor.ScrollY = max(0, m.Editor.CY-h+1)
	}
	cursorCol := displayColumnForIndex(m.Editor.Lines[m.Editor.CY], m.Editor.CX)
	w := m.editorTextWidth()
	if cursorCol < m.Editor.ScrollX {
		m.Editor.ScrollX = cursorCol
	}
	if cursorCol >= m.Editor.ScrollX+w {
		m.Editor.ScrollX = max(0, cursorCol-w+1)
	}
}

func (m Model) View() string {
	if m.Width <= 0 || m.Height <= 0 {
		return ""
	}
	if m.Splash {
		return m.renderSplash()
	}
	var b strings.Builder
	b.WriteString("\x1b[5 q") // steady bar cursor for Ghostty/xterm
	b.WriteString(m.renderHeader())
	b.WriteByte('\n')
	if m.searchVisible() {
		b.WriteString(m.renderSearch())
		b.WriteByte('\n')
	}
	if m.logoHeader() {
		// A blank row gives the header block room to breathe against the panes.
		b.WriteString(fitANSI("", m.Width))
		b.WriteByte('\n')
	}
	b.WriteString(m.renderPaneBorder(true))
	b.WriteByte('\n')
	for row := 0; row < m.contentHeight(); row++ {
		b.WriteString(m.renderPaneRow(row))
		b.WriteByte('\n')
	}
	b.WriteString(m.renderPaneBorder(false))
	b.WriteByte('\n')
	b.WriteString(m.renderToolbar())
	return b.String()
}

type paneKind int

const (
	paneSidebar paneKind = iota
	paneContent
	paneChat
)

// paneSpec describes one boxed column of the layout.
type paneSpec struct {
	Kind    paneKind
	Caption string
	Width   int // inner width; panes share their vertical borders
	Focused bool
}

// panes returns the visible panes from left to right. Their widths add up to
// the terminal width.
func (m Model) panes() []paneSpec {
	specs := make([]paneSpec, 0, 3)
	if m.SidebarVisible {
		specs = append(specs, paneSpec{
			Kind:    paneSidebar,
			Caption: "Files & Folders",
			Width:   m.leftInnerWidth(),
			Focused: m.Focus == FocusSidebar,
		})
	}
	caption := "glow preview"
	switch m.Mode {
	case ModeEdit:
		caption = "editor"
	case ModeSource:
		caption = "source selection"
	}
	specs = append(specs, paneSpec{
		Kind:    paneContent,
		Caption: caption,
		Width:   m.rightInnerWidth(),
		Focused: m.Focus == FocusEditor || m.Focus == FocusPreview,
	})
	if m.Chat.Visible {
		chatCaption := "llm chat"
		if m.Focus == FocusChat {
			chatCaption = "chat input"
		}
		specs = append(specs, paneSpec{
			Kind:    paneChat,
			Caption: chatCaption,
			Width:   m.chatInnerWidth(),
			Focused: m.Focus == FocusChat,
		})
	}
	return specs
}

// paneCaptionIndent is how far the caption sits inside the top border.
const paneCaptionIndent = 2

// renderPaneBorder draws the top or bottom border row. Neighbouring panes share
// a junction column, and every border segment is colored by the pane it belongs
// to so the focused pane stands out.
func (m Model) renderPaneBorder(top bool) string {
	panes := m.panes()
	var b strings.Builder
	for i, pane := range panes {
		border, caption := paneStyles(pane.Focused)

		corner := "└"
		if top {
			corner = "┌"
		}
		if i > 0 {
			corner = "┴"
			if top {
				corner = "┬"
			}
			// The junction belongs to whichever neighbour is focused.
			if !pane.Focused && panes[i-1].Focused {
				junction, _ := paneStyles(true)
				border = junction
			}
		}
		b.WriteString(border.Render(corner))

		border, _ = paneStyles(pane.Focused)
		label := ""
		if top && pane.Caption != "" {
			label = " " + pane.Caption + " "
			if runewidth.StringWidth(label)+paneCaptionIndent > pane.Width {
				label = ""
			}
		}
		if label == "" {
			b.WriteString(border.Render(strings.Repeat("─", pane.Width)))
		} else {
			rest := max(0, pane.Width-paneCaptionIndent-runewidth.StringWidth(label))
			b.WriteString(border.Render(strings.Repeat("─", paneCaptionIndent)))
			b.WriteString(caption.Render(label))
			b.WriteString(border.Render(strings.Repeat("─", rest)))
		}

		if i == len(panes)-1 {
			closing := "┘"
			if top {
				closing = "┐"
			}
			b.WriteString(border.Render(closing))
		}
	}
	return b.String()
}

// renderPaneRow draws one body row across all panes, sharing vertical borders.
func (m Model) renderPaneRow(row int) string {
	panes := m.panes()
	var b strings.Builder
	for i, pane := range panes {
		style, _ := paneStyles(pane.Focused)
		if i > 0 && !pane.Focused && panes[i-1].Focused {
			style, _ = paneStyles(true)
		}
		b.WriteString(style.Render("│"))
		b.WriteString(m.paneContent(pane, row))
		if i == len(panes)-1 {
			style, _ = paneStyles(pane.Focused)
			b.WriteString(style.Render("│"))
		}
	}
	return b.String()
}

func (m Model) paneContent(pane paneSpec, row int) string {
	switch pane.Kind {
	case paneSidebar:
		return m.renderListLine(row)
	case paneChat:
		return strings.Repeat(" ", panePadLeft) + m.renderChatLine(row)
	default:
		if line, ok := m.renderMenuRow(pane.Width, m.contentHeight(), row); ok {
			return line
		}
		if m.rawBufferMode() {
			return strings.Repeat(" ", panePadLeft) + m.renderEditorLine(row)
		}
		return strings.Repeat(" ", panePadLeft) + m.renderPreviewLine(row)
	}
}

// paneStyles returns the border and caption styles for a pane.
func paneStyles(focused bool) (border lipgloss.Style, caption lipgloss.Style) {
	if focused {
		return stylePaneActive, stylePaneCaptionActive
	}
	return styleDim, styleDim
}

// renderToolbar is the bottom row: the path of the current document, or the
// prompt while one is open. The right end points at the action menu, which is
// where the key hints live.
func (m Model) renderToolbar() string {
	if m.Prompt.Active {
		return m.renderPrompt()
	}
	hint := styleCyan.Render("ctrl+p") + styleDim.Render(" actions ")
	left := fitANSI(" "+styleDim.Render(m.pathLine()), max(1, m.Width-xansi.StringWidth(stripANSI(hint))))
	return left + hint
}

// renderHeader draws the rows above the panes: the lightbulb mark, the name and
// mode beside it, and the current file underneath. On a short terminal it
// collapses into the single row it used to be.
func (m Model) renderHeader() string {
	mode := styleGreen.Render("PREVIEW")
	if m.Mode == ModeEdit {
		mode = styleYellow.Render("EDIT")
	} else if m.Mode == ModeSource {
		mode = styleCyan.Render("SOURCE")
	}
	dirty := ""
	if m.Editor.Dirty {
		dirty = styleRed.Render("*")
	}
	doc := "no document"
	if d := m.currentDoc(); d != nil {
		doc = d.Rel
	}
	if !m.logoHeader() {
		compact := fmt.Sprintf("%s %s%s %s", styleTitle.Render(AppName), mode, dirty, styleDim.Render(doc))
		return fitANSI(compact, m.Width)
	}

	status := ""
	if m.Status != "" {
		status = statusStyle(m.StatusKind).Render(m.Status)
	}
	// The focus name would repeat the pane caption, and the file name would
	// repeat both the status line and the toolbar path.
	beside := []string{styleTitle.Render(AppName), mode + dirty, status}

	rows := make([]string, 0, headerRows+1)
	for i, line := range splashLogo {
		row := " " + styleYellow.Render(line) + "  "
		if i < len(beside) {
			row += beside[i]
		}
		rows = append(rows, fitANSI(row, m.Width))
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderSearch() string {
	prompt := "/"
	if m.Focus == FocusSearch {
		prompt = styleReverse.Render("/")
	}
	query := renderSearchQuery(m.Query, m.Focus == FocusSearch)
	right := styleDim.Render(fmt.Sprintf("%d/%d", len(m.Results), len(m.Docs)))
	left := " " + prompt + " " + query
	return fitANSI(left, max(1, m.Width-xansi.StringWidth(right))) + right
}

func renderSearchQuery(query string, focused bool) string {
	const placeholder = "foo bar = AND; tag:foo; title/body/path"
	if !focused {
		if query == "" {
			return styleDim.Render(placeholder)
		}
		return query
	}
	cursor := styleReverse.Render(" ")
	if query == "" {
		return cursor + " " + styleDim.Render(placeholder)
	}
	return query + cursor
}

func (m Model) renderListLine(row int) string {
	idx := m.ListScroll + row
	rows := m.SidebarRows
	if len(rows) == 0 && len(m.Results) > 0 {
		rows = m.buildSidebarRows()
	}
	if idx >= len(rows) {
		return fitPlain("", m.leftInnerWidth())
	}
	sidebarRow := rows[idx]
	prefix := "  "
	if idx == m.SidebarSelected {
		prefix = "› "
	}
	indent := strings.Repeat("  ", sidebarRow.Depth)
	lineText := ""
	if sidebarRow.Kind == sidebarRowDirectory {
		icon := "▸"
		if sidebarRow.Expanded {
			icon = "▾"
		}
		lineText = prefix + indent + icon + " " + sidebarRow.Name + "/"
	} else {
		doc := m.Results[sidebarRow.DocIndex]
		label := doc.Rel
		if m.sidebarTreeMode() {
			label = doc.Name
		}
		tags := ""
		if len(doc.Tags) > 0 {
			shown := doc.Tags
			if len(shown) > 2 {
				shown = shown[:2]
			}
			tags = " #" + strings.Join(shown, " #")
		}
		snippet := ""
		if m.Query != "" && doc.Snippet != "" {
			snippet = " · " + doc.Snippet
		}
		lineText = prefix + indent + "  " + label + styleDim.Render(tags+snippet)
	}
	line := fitPlain(lineText, m.leftInnerWidth())
	if idx == m.SidebarSelected {
		return styleReverse.Render(stripANSI(line))
	}
	return line
}

func (m Model) renderPreviewLine(row int) string {
	w := m.contentTextWidth()
	idx := m.PreviewScroll + row
	if idx >= len(m.PreviewLines) {
		return fitPlain("", w)
	}
	if selStart, selEnd, selected := m.previewSelectionForLine(idx); selected {
		return fitANSI(sliceByDisplayRangeStyled(stripANSI(m.PreviewLines[idx]), 0, w, selStart, selEnd), w)
	}
	return fitANSI(m.PreviewLines[idx], w)
}

func (m Model) renderEditorLine(row int) string {
	w := m.contentTextWidth()
	idx := m.Editor.ScrollY + row
	if idx >= len(m.Editor.Lines) {
		return fitPlain("", w)
	}
	line := m.Editor.Lines[idx]
	selStart, selEnd, selected := m.editorSelectionForLine(idx)
	// The prompt and the action menu take the caret with them, so the buffer
	// stops drawing its own while either is open.
	cursor := m.Mode == ModeEdit && m.Focus == FocusEditor && idx == m.Editor.CY &&
		!m.hasEditorSelection() && !m.Prompt.Active && !m.Menu.Active
	out := renderEditorVisibleLine(line, m.Editor.ScrollX, m.editorTextWidth(), m.Editor.CX, cursor, selStart, selEnd, selected, m.Highlight[idx])
	return fitPlain(out, w)
}

func (m Model) renderChatLine(row int) string {
	w := max(1, m.chatInnerWidth()-panePadLeft)
	if row == 0 {
		path := "no file"
		if d := m.currentDoc(); d != nil {
			path = d.Rel
		}
		return fitANSI(styleCyan.Render("ctx ")+path, w)
	}
	if row == m.contentHeight()-1 {
		prompt := ">"
		if m.Focus == FocusChat {
			prompt = styleReverse.Render(">")
		}
		input := renderChatInput(m.Chat.Input)
		if input == "" {
			input = styleDim.Render("ask about current file path…")
		}
		return fitANSI(prompt+" "+input, w)
	}
	lines := m.chatLines()
	bodyHeight := m.chatBodyHeight()
	maxScroll := max(0, len(lines)-bodyHeight)
	scroll := clamp(m.Chat.Scroll, 0, maxScroll)
	idx := scroll + row - 1
	if idx < 0 || idx >= len(lines) {
		return fitPlain("", w)
	}
	return fitANSI(lines[idx], w)
}

func renderChatInput(input string) string {
	return strings.ReplaceAll(input, "\n", " ⏎ ")
}

func (m Model) chatLines() []string {
	lines := []string{}
	for _, msg := range m.Chat.Messages {
		prefix := styleCyan.Render("you: ")
		if msg.Role != "user" {
			prefix = styleGreen.Render("llm: ")
		}
		for i, line := range strings.Split(strings.ReplaceAll(msg.Content, "\r\n", "\n"), "\n") {
			if i == 0 {
				lines = append(lines, prefix+line)
			} else {
				lines = append(lines, "     "+line)
			}
		}
	}
	if m.Chat.Loading {
		lines = append(lines, styleYellow.Render("llm: …"))
	}
	if m.Chat.Err != "" {
		lines = append(lines, styleRed.Render("error: ")+m.Chat.Err)
	}
	if len(lines) == 0 {
		lines = append(lines, styleDim.Render("Chat sends current markdown path with every message."))
		lines = append(lines, styleDim.Render("Press c or prefix+l to toggle; enter sends."))
	}
	return lines
}

func (m Model) chatBodyHeight() int {
	return max(1, m.contentHeight()-2)
}

func (m *Model) scrollChatToBottom() {
	m.Chat.Scroll = max(0, len(m.chatLines())-m.chatBodyHeight())
}

// footerEntry is one hint in the footer bar.
type footerEntry struct {
	Key    string
	Label  string
	Action string // empty for hints that are not clickable
}

func (e footerEntry) plain() string {
	return strings.TrimSpace(e.Key + " " + e.Label)
}

// modeEntries returns the hints for the current mode. Edit mode has its own
// set because the browse bindings are not active while editing.
func (m Model) modeEntries() []footerEntry {
	if m.Mode == ModeEdit {
		return m.editModeEntries()
	}
	entries := []footerEntry{}
	for _, action := range m.Cfg.Footer.Actions {
		label := labelForAction(action)
		if label == "" {
			continue
		}
		entries = append(entries, footerEntry{Key: m.footerKey(action), Label: label, Action: action})
	}
	return entries
}

func (m Model) editModeEntries() []footerEntry {
	if m.SidebarVisible && m.Focus == FocusSidebar {
		return []footerEntry{
			{Key: "↑↓", Label: "select"},
			{Key: "enter", Label: "open"},
			{Key: "shift+tab", Label: "editor"},
			{Key: "ctrl+b", Label: "hide sidebar", Action: "toggleSidebar"},
		}
	}
	entries := []footerEntry{
		{Key: m.Cfg.Keys["save"], Label: "save", Action: "save"},
		{Key: m.Cfg.Keys["undo"], Label: "undo", Action: "undo"},
		{Key: "opt+←→", Label: "word"},
		{Key: "cmd+⌫", Label: "del line"},
		{Key: "opt+a", Label: "select all", Action: "selectAll"},
		{Key: "opt+c", Label: "copy"},
		{Key: "ctrl+b", Label: "sidebar"},
		{Key: "esc", Label: "cancel", Action: "cancelEdit"},
	}
	out := entries[:0:0]
	for _, e := range entries {
		if e.Key == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// dropHintsToFit removes non-clickable hints from the end until the bar fits
// the terminal width, so the actionable entries stay readable on narrow panes.

func (m Model) footerKey(action string) string {
	if isDirectAction(action) {
		return m.Cfg.Keys[action]
	}
	if k := m.Cfg.PrefixKeys[action]; k != "" {
		return m.Cfg.Prefix + " " + k
	}
	return ""
}

func (m Model) pathLine() string {
	if d := m.currentDoc(); d != nil {
		return "Path: " + d.Abs
	}
	return "Path: " + m.Root
}

// Panes are drawn as boxes standing side by side, sharing their vertical edges:
// N panes occupy N+1 border columns. The *InnerWidth functions return the text
// area of a pane, the *Width functions the columns it owns including borders.

func (m Model) paneBorderColumns() int {
	n := 1 // the content pane is always there
	if m.SidebarVisible {
		n++
	}
	if m.Chat.Visible {
		n++
	}
	return n + 1
}

func (m Model) leftInnerWidth() int {
	if !m.SidebarVisible {
		return 0
	}
	return clamp(m.Width*34/100, 24, 52) - 2
}

func (m Model) chatInnerWidth() int {
	if !m.Chat.Visible {
		return 0
	}
	available := m.Width - m.leftWidth()
	return clamp(available*32/100, 28, min(60, max(28, available-12))) - 2
}

func (m Model) rightInnerWidth() int {
	return max(1, m.Width-m.paneBorderColumns()-m.leftInnerWidth()-m.chatInnerWidth())
}

// leftWidth is the column range the sidebar occupies, its left border and the
// shared divider on its right included.
func (m Model) leftWidth() int {
	if !m.SidebarVisible {
		return 0
	}
	return m.leftInnerWidth() + 2
}

func (m Model) rightWidth() int { return m.rightInnerWidth() + 2 }
func (m Model) chatWidth() int {
	if !m.Chat.Visible {
		return 0
	}
	return m.chatInnerWidth() + 2
}

// panePadLeft keeps the text of the content and chat panes off their border.
const panePadLeft = 1

// rightTextStartX is the first column of the content pane's text, the border
// and the padding column excluded.
func (m Model) rightTextStartX() int { return max(1, m.leftWidth()) + panePadLeft }

// contentTextWidth is the writable width of the content pane.
func (m Model) contentTextWidth() int { return max(1, m.rightInnerWidth()-panePadLeft) }

func (m Model) rightStartX() int { return m.rightTextStartX() }

// chatStartX is where the chat region begins, on its shared divider.
func (m Model) chatStartX() int {
	if !m.Chat.Visible {
		return m.Width
	}
	return m.rightTextStartX() + m.rightInnerWidth()
}

// Row layout: header, optional search, pane top border, content, pane bottom
// border, toolbar.
func (m Model) contentTop() int { return m.chromeTopRows() + 1 }
func (m Model) contentHeight() int {
	return max(1, m.Height-m.chromeTopRows()-3)
}

// headerRows is the height of the logo header: the lightbulb, with the name
// beside it, the mode underneath and the status message on the third row. A
// blank row follows it, which chromeTopRows accounts for.
const headerRows = 3

// minLogoHeaderHeight is the terminal height the logo header needs. Below it the
// header collapses to a single row, so a short split still renders a frame that
// fits on screen.
const minLogoHeaderHeight = 12

// logoHeader reports whether the tall header is in use.
func (m Model) logoHeader() bool { return m.Height >= minLogoHeaderHeight }

// chromeTopRows counts the rows above the pane top border: the header, the
// blank row that separates the logo block from the panes, and the search row
// while it is in use.
func (m Model) chromeTopRows() int {
	rows := 1
	if m.logoHeader() {
		rows = headerRows + 1
	}
	if m.searchVisible() {
		rows++
	}
	return rows
}

// searchRow is the row the search input sits on, directly below the header.
func (m Model) searchRow() int {
	if m.logoHeader() {
		return headerRows
	}
	return 1
}

// searchVisible reports whether the search row is part of the frame. It only
// shows while the search is in use, so an idle frame spends the row on content.
func (m Model) searchVisible() bool {
	return m.Focus == FocusSearch || m.Query != ""
}
func (m Model) paneBottomRow() int     { return m.contentTop() + m.contentHeight() }
func (m Model) toolbarRow() int        { return m.paneBottomRow() + 1 }
func (m Model) previewBodyHeight() int { return max(1, m.contentHeight()) }
func (m Model) editorTextHeight() int  { return max(1, m.contentHeight()) }
func (m Model) editorTextWidth() int   { return m.contentTextWidth() }

func (m *Model) setStatus(s, kind string) {
	m.Status = s
	m.StatusKind = kind
}

func (m Model) actionForKey(key string, keymap map[string]string) string {
	for action, mapped := range keymap {
		if normalizeKey(mapped) == key {
			return action
		}
	}
	return ""
}

func splitEditorLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if raw == "" {
		return []string{""}
	}
	lines := strings.Split(raw, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func fitPlain(s string, w int) string {
	return fitANSI(s, w)
}

func fitANSI(s string, w int) string {
	if w <= 0 {
		return ""
	}
	t := xansi.Truncate(s, w, "…")
	pad := w - xansi.StringWidth(t)
	if pad > 0 {
		t += strings.Repeat(" ", pad)
	}
	return t
}

func stripANSI(s string) string { return xansi.Strip(s) }

func normalizeKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	k = strings.ReplaceAll(k, "control+", "ctrl+")
	k = strings.ReplaceAll(k, "pageup", "pgup")
	k = strings.ReplaceAll(k, "pagedown", "pgdown")
	switch k {
	case "ctrl+space", "ctrl+ ", "ctrl+", "ctrl+@", "ctrl+`", "nul":
		// Ctrl+Space is encoded as NUL in terminals. Bubble Tea exposes it as
		// ctrl+@, so accept user-friendly aliases in config.
		return "ctrl+@"
	case "space":
		return " "
	}
	return k
}

func modeName(mode Mode) string {
	switch mode {
	case ModePreview:
		return "preview"
	case ModeEdit:
		return "edit"
	case ModeSource:
		return "source"
	default:
		return "unknown"
	}
}

func focusName(f Focus) string {
	switch f {
	case FocusSearch:
		return "search"
	case FocusSidebar:
		return "sidebar"
	case FocusPreview:
		return "preview"
	case FocusEditor:
		return "editor"
	case FocusChat:
		return "chat"
	default:
		return "unknown"
	}
}

func statusStyle(kind string) lipgloss.Style {
	switch kind {
	case "error":
		return styleRed
	case "warn":
		return styleYellow
	case "success":
		return styleGreen
	default:
		return styleDim
	}
}

func isDirectAction(action string) bool {
	switch action {
	case "quit", "search", "edit", "sourceSelect", "openLLM", "toggleSidebar", "save", "undo", "redo", "refresh", "nextFocus", "open":
		return true
	default:
		return false
	}
}

func labelForAction(action string) string {
	switch action {
	case "quit":
		return "quit"
	case "search":
		return "search"
	case "edit":
		return "edit"
	case "sourceSelect":
		return "source"
	case "openLLM":
		return "llm"
	case "save":
		return "save"
	case "undo":
		return "undo"
	case "redo":
		return "redo"
	case "refresh":
		return "refresh"
	case "open":
		return "open"
	case "nextFocus":
		return "focus"
	case "toggleSidebar":
		return "sidebar"
	default:
		return ""
	}
}

func renderEditorVisibleLine(line string, scrollX, width, cursorIndex int, cursor bool, selStart, selEnd int, selected bool, spans []render.Span) string {
	if selected {
		return sliceStyled(line, scrollX, width, spans, selStart, selEnd, true)
	}
	if !cursor {
		return sliceStyled(line, scrollX, width, spans, 0, 0, false)
	}
	cursorCol := displayColumnForIndex(line, cursorIndex)
	if cursorCol < scrollX || cursorCol >= scrollX+width {
		return sliceStyled(line, scrollX, width, spans, 0, 0, false)
	}
	before := sliceStyled(line, scrollX, max(0, cursorCol-scrollX), spans, 0, 0, false)
	// The caret covers the cell it sits on. Inserting a glyph before that cell
	// instead would push the rest of the line one column to the right.
	cell, cellWidth := cursorCell(line, cursorIndex)
	afterWidth := max(0, width-xansi.StringWidth(before)-cellWidth)
	after := sliceStyled(line, cursorCol+cellWidth, afterWidth, spans, 0, 0, false)
	return before + styleCursor.Render(cell) + after
}

// cursorCell returns the character the caret covers and its display width. Past
// the last character the caret gets a blank cell of its own.
func cursorCell(line string, cursorIndex int) (string, int) {
	runes := []rune(line)
	idx := clamp(cursorIndex, 0, len(runes))
	if idx >= len(runes) {
		return " ", 1
	}
	return string(runes[idx]), max(1, runewidth.RuneWidth(runes[idx]))
}

func displayColumnForIndex(line string, idx int) int {
	runes := []rune(line)
	idx = clamp(idx, 0, len(runes))
	col := 0
	for i := 0; i < idx; i++ {
		col += runewidth.RuneWidth(runes[i])
	}
	return col
}

func indexFromDisplayColumn(line string, target int) int {
	runes := []rune(line)
	col := 0
	for i, r := range runes {
		next := col + runewidth.RuneWidth(r)
		if target < next {
			return i
		}
		col = next
	}
	return len(runes)
}

func sliceByDisplayRange(line string, start, width int) string {
	return sliceStyled(line, start, width, nil, 0, 0, false)
}

func sliceByDisplayRangeStyled(line string, start, width int, selStart, selEnd int) string {
	return sliceStyled(line, start, width, nil, selStart, selEnd, true)
}

// sliceStyled renders the display columns [start, start+width) of a line.
// Syntax spans colorize runes; an active selection reverses them and wins over
// any span color. Runes with the same appearance are emitted as one segment so
// the output stays compact.
func sliceStyled(line string, start, width int, spans []render.Span, selStart, selEnd int, selected bool) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(line)
	if selected {
		selStart = clamp(selStart, 0, len(runes))
		selEnd = clamp(selEnd, 0, len(runes))
		if selEnd < selStart {
			selStart, selEnd = selEnd, selStart
		}
	}

	end := start + width
	col := 0
	var b strings.Builder
	var segment strings.Builder
	segmentStyle := ""
	segmentSelected := false
	segmentOpen := false
	flush := func() {
		if !segmentOpen {
			return
		}
		text := segment.String()
		switch {
		case segmentSelected:
			b.WriteString(styleReverse.Render(text))
		case segmentStyle != "":
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(segmentStyle)).Render(text))
		default:
			b.WriteString(text)
		}
		segment.Reset()
		segmentOpen = false
	}
	write := func(text string, sel bool, style string) {
		if segmentOpen && (segmentSelected != sel || segmentStyle != style) {
			flush()
		}
		segmentSelected = sel
		segmentStyle = style
		segmentOpen = true
		segment.WriteString(text)
	}

	for i, r := range runes {
		rw := runewidth.RuneWidth(r)
		next := col + rw
		if next <= start {
			col = next
			continue
		}
		if col >= end {
			break
		}
		sel := selected && i >= selStart && i < selEnd
		style := ""
		if !sel {
			style = spanColorAt(spans, i)
		}
		if col < start {
			write(" ", sel, "")
		} else if next <= end {
			write(string(r), sel, style)
		} else {
			break
		}
		col = next
	}
	flush()
	return b.String()
}

// spanColorAt returns the color of the span covering rune index i, if any.
func spanColorAt(spans []render.Span, i int) string {
	for _, s := range spans {
		if i >= s.Start && i < s.End {
			return s.Color
		}
	}
	return ""
}

func lineLen(s string) int { return len([]rune(s)) }
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
