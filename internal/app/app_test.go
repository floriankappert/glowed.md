package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/khw1031/glowed/internal/config"
	"github.com/khw1031/glowed/internal/docs"
	llmclient "github.com/khw1031/glowed/internal/llm"
	filewatch "github.com/khw1031/glowed/internal/watch"
)

func TestNormalizeKeyCtrlSpaceAliases(t *testing.T) {
	aliases := []string{"ctrl+space", "ctrl+ ", "ctrl+@", "ctrl+`", "nul"}
	for _, alias := range aliases {
		if got := normalizeKey(alias); got != "ctrl+@" {
			t.Fatalf("normalizeKey(%q) = %q, want ctrl+@", alias, got)
		}
	}
}

func TestEditorUndoRedo(t *testing.T) {
	m := Model{
		Width:  80,
		Height: 12,
		Mode:   ModeEdit,
		Editor: editorState{Lines: []string{"hello"}},
	}

	m.Editor.CX = 5
	m.editorInsert("!")
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello!" {
		t.Fatalf("after insert = %q", got)
	}
	if !m.Editor.Dirty {
		t.Fatal("Dirty = false after insert")
	}

	m.editorUndo()
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello" {
		t.Fatalf("after undo = %q", got)
	}
	if m.Editor.Dirty {
		t.Fatal("Dirty = true after undo to clean initial state")
	}

	m.editorRedo()
	if got := strings.Join(m.Editor.Lines, "\n"); got != "hello!" {
		t.Fatalf("after redo = %q", got)
	}
	if !m.Editor.Dirty {
		t.Fatal("Dirty = false after redo")
	}
}

func TestEditorRedoClearedAfterNewEdit(t *testing.T) {
	m := Model{Width: 80, Height: 12, Mode: ModeEdit, Editor: editorState{Lines: []string{"a"}, CX: 1}}
	m.editorInsert("b")
	m.editorUndo()
	m.editorInsert("c")
	m.editorRedo()
	if got := strings.Join(m.Editor.Lines, "\n"); got != "ac" {
		t.Fatalf("redo after new edit changed buffer = %q, want ac", got)
	}
	if m.StatusKind != "warn" || !strings.Contains(m.Status, "nothing to redo") {
		t.Fatalf("status = (%q, %q), want nothing to redo warning", m.StatusKind, m.Status)
	}
}

func TestEditorUndoHistoryCap(t *testing.T) {
	m := Model{Width: 80, Height: 12, Mode: ModeEdit, Editor: editorState{Lines: []string{""}}}
	for i := 0; i < maxEditorHistory+5; i++ {
		m.editorInsert("x")
	}
	if len(m.Editor.Undo) != maxEditorHistory {
		t.Fatalf("undo len = %d, want %d", len(m.Editor.Undo), maxEditorHistory)
	}
}

func TestLLMPromptOmitsEmptyConversationTag(t *testing.T) {
	prompt := llmclient.BuildPrompt(llmclient.Request{Context: llmclient.FileContext{AbsPath: "/tmp/doc.md", RelPath: "doc.md"}})
	if strings.Contains(prompt, "<conversation>") || strings.Contains(prompt, "</conversation>") {
		t.Fatalf("prompt contains obsolete conversation tag:\n%s", prompt)
	}
}

func TestChatContextIncludesCurrentFilePathAndSelection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	raw := "alpha\nbravo\ncharlie"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.LLM.IncludeCurrentFile = true
	cfg.LLM.MaxContextBytes = 8
	m := Model{
		Root:    root,
		Cfg:     cfg,
		Width:   80,
		Height:  12,
		Mode:    ModeSource,
		Results: []docs.Document{{Abs: path, Rel: "doc.md", Name: "doc.md"}},
		Editor:  editorState{Lines: splitEditorLines(raw), File: path},
		Selection: selectionState{
			Active: true,
			Kind:   "raw",
			Start:  selectionPoint{Line: 0, Col: 1},
			End:    selectionPoint{Line: 1, Col: 2},
			File:   path,
		},
	}

	ctx := m.buildChatContext()
	if ctx.AbsPath != path || ctx.RelPath != "doc.md" {
		t.Fatalf("context path = (%q, %q), want (%q, doc.md)", ctx.AbsPath, ctx.RelPath, path)
	}
	if ctx.Mode != "source" {
		t.Fatalf("context mode = %q, want source", ctx.Mode)
	}
	if !strings.Contains(ctx.SelectedMarkdown, "path: "+path) || !strings.Contains(ctx.SelectedMarkdown, "lpha\nbr") {
		t.Fatalf("selected markdown = %q", ctx.SelectedMarkdown)
	}
	if ctx.RawMarkdown != "alpha\nbr" || !ctx.Truncated {
		t.Fatalf("raw context = %q truncated=%v, want alpha\\nbr truncated", ctx.RawMarkdown, ctx.Truncated)
	}
}

func TestChatMockSendMentionsCurrentPath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("# Doc"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	// The launcher is off by default; this test exercises it deliberately.
	cfg.LLM.Enabled = true
	cfg.LLM.Command = "mock"
	m := Model{
		Root:    root,
		Cfg:     cfg,
		Width:   80,
		Height:  12,
		Results: []docs.Document{{Abs: path, Rel: "doc.md", Name: "doc.md"}},
		Chat:    chatState{Visible: true, Input: "이 파일 요약해줘"},
	}
	m.reloadPreview()
	cmd := m.sendChat()
	if cmd == nil {
		t.Fatal("sendChat() returned nil command")
	}
	msg, ok := cmd().(chatResultMsg)
	if !ok {
		t.Fatalf("chat command returned %T", msg)
	}
	m.handleChatResult(msg)
	if len(m.Chat.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(m.Chat.Messages))
	}
	assistant := m.Chat.Messages[1].Content
	if !strings.Contains(assistant, path) || !strings.Contains(assistant, "doc.md") {
		t.Fatalf("assistant response missing current path context:\n%s", assistant)
	}
}

func TestSourceSelectionModeLoadsRawMarkdown(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	raw := "# Title\n\nraw **markdown**"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}

	m := Model{
		Root:           root,
		Cfg:            config.Default(),
		Width:          80,
		Height:         12,
		Docs:           []docs.Document{{Abs: path, Rel: "doc.md", Name: "doc.md"}},
		Results:        []docs.Document{{Abs: path, Rel: "doc.md", Name: "doc.md"}},
		SidebarVisible: true,
		PreviewScrolls: map[string]int{},
	}
	m.enterSourceMode()
	if m.Mode != ModeSource {
		t.Fatalf("Mode = %v, want ModeSource", m.Mode)
	}
	wantPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Editor.File != wantPath {
		t.Fatalf("Editor.File = %q, want %q", m.Editor.File, wantPath)
	}
	if got := strings.Join(m.Editor.Lines, "\n"); got != raw {
		t.Fatalf("source lines = %q, want %q", got, raw)
	}
	if !strings.Contains(stripANSI(m.renderPaneBorder(true)), "source selection") {
		t.Fatalf("pane caption = %q, want source selection", stripANSI(m.renderPaneBorder(true)))
	}

	m.exitSourceMode()
	if m.Mode != ModePreview {
		t.Fatalf("Mode after exit = %v, want ModePreview", m.Mode)
	}
}

func TestSidebarDirectoryRowsExpandCollapse(t *testing.T) {
	documents := []docs.Document{
		{Abs: "/tmp/root/README.md", Rel: "README.md", Name: "README.md"},
		{Abs: "/tmp/root/notes/a.md", Rel: "notes/a.md", Name: "a.md"},
		{Abs: "/tmp/root/notes/build/b.md", Rel: "notes/build/b.md", Name: "b.md"},
	}
	m := Model{Width: 80, Height: 12, Results: documents, ExpandedDirs: map[string]bool{}}
	m.rebuildSidebarRows()

	if len(m.SidebarRows) != 2 || m.SidebarRows[0].Kind != sidebarRowDirectory || m.SidebarRows[0].Rel != "notes" {
		t.Fatalf("initial sidebar rows = %#v, want collapsed notes dir plus README", m.SidebarRows)
	}
	m.SidebarSelected = 0
	m.toggleSidebarDirectory()
	if !m.ExpandedDirs["notes"] {
		t.Fatal("notes directory was not expanded")
	}
	if idx := m.findSidebarRow(sidebarRowDocument, "notes/a.md", -1); idx < 0 {
		t.Fatalf("expanded sidebar rows = %#v, want notes/a.md visible", m.SidebarRows)
	}
	buildIdx := m.findSidebarRow(sidebarRowDirectory, "notes/build", -1)
	if buildIdx < 0 {
		t.Fatalf("expanded sidebar rows = %#v, want notes/build directory visible", m.SidebarRows)
	}
	m.SidebarSelected = buildIdx
	m.toggleSidebarDirectory()
	if idx := m.findSidebarRow(sidebarRowDocument, "notes/build/b.md", -1); idx < 0 {
		t.Fatalf("nested expanded sidebar rows = %#v, want notes/build/b.md visible", m.SidebarRows)
	}
}

func TestSidebarTabTogglesSelectedDirectory(t *testing.T) {
	documents := []docs.Document{{Abs: "/tmp/root/notes/a.md", Rel: "notes/a.md", Name: "a.md"}}
	m := Model{Width: 80, Height: 12, Results: documents, SidebarVisible: true, Focus: FocusSidebar, ExpandedDirs: map[string]bool{}}
	m.rebuildSidebarRows()
	if !m.sidebarToggleKey("tab") {
		t.Fatal("tab should toggle a selected sidebar directory")
	}
	m.toggleSidebarDirectory()
	if idx := m.findSidebarRow(sidebarRowDocument, "notes/a.md", -1); idx < 0 {
		t.Fatalf("rows after tab toggle = %#v, want notes/a.md visible", m.SidebarRows)
	}
}

func TestSearchInputSupportsSpaceKey(t *testing.T) {
	m := Model{Docs: []docs.Document{{Rel: "foo bar.md", Name: "foo bar.md"}}}
	m.applySearch(false)

	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bar")})

	if m.Query != "foo bar" {
		t.Fatalf("Query = %q, want foo bar", m.Query)
	}
	if len(m.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(m.Results))
	}
}

func TestSearchInputIgnoresSpecialKeyRunes(t *testing.T) {
	m := Model{}
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("foo")})
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\x1b', '[', 'D'}})
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyUp})

	if m.Query != "foo" {
		t.Fatalf("Query = %q, want foo", m.Query)
	}
}

func TestSearchInputDeletesPreviousWord(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "single word", input: "foo", want: ""},
		{name: "single word trailing space", input: "foo ", want: ""},
		{name: "two words", input: "foo bar", want: "foo "},
		{name: "two words trailing spaces", input: "foo bar  ", want: "foo "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{Query: tc.input}
			m.handleSearchKey(tea.KeyMsg{Type: tea.KeyCtrlW})
			if m.Query != tc.want {
				t.Fatalf("Query after ctrl+w = %q, want %q", m.Query, tc.want)
			}
		})
	}

	m := Model{Query: "foo "}
	m.handleSearchKey(tea.KeyMsg{Type: tea.KeyBackspace, Alt: true})
	if m.Query != "" {
		t.Fatalf("Query after alt+backspace = %q, want empty", m.Query)
	}
}

func TestRenderSearchShowsCursorWhenFocused(t *testing.T) {
	cursor := styleReverse.Render(" ")
	m := Model{Width: 80, Focus: FocusSearch, Query: "agent"}
	line := m.renderSearch()

	if !strings.Contains(line, "agent"+cursor) {
		t.Fatalf("focused search line %q does not show cursor after query", line)
	}
}

func TestRenderSearchShowsCursorBeforePlaceholderWhenEmptyFocused(t *testing.T) {
	cursor := styleReverse.Render(" ")
	m := Model{Width: 80, Focus: FocusSearch}
	line := m.renderSearch()

	if !strings.Contains(line, cursor+" ") || !strings.Contains(stripANSI(line), "foo bar = AND") {
		t.Fatalf("empty focused search line %q does not show cursor before placeholder", line)
	}
}

func TestPreviewScrollPreservedPerDocument(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.md")
	second := filepath.Join(root, "second.md")
	if err := os.WriteFile(first, []byte(longMarkdown("first")), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte(longMarkdown("second")), 0644); err != nil {
		t.Fatal(err)
	}

	documents := []docs.Document{
		{Abs: first, Rel: "first.md", Name: "first.md"},
		{Abs: second, Rel: "second.md", Name: "second.md"},
	}
	m := Model{
		Root:           root,
		Cfg:            config.Default(),
		Width:          80,
		Height:         12,
		Docs:           documents,
		Results:        documents,
		SidebarVisible: true,
		PreviewScrolls: map[string]int{},
	}
	m.reloadPreview()

	m.scrollPreview(6)
	firstScroll := m.PreviewScroll
	if firstScroll == 0 {
		t.Fatal("first document did not scroll; test fixture may be too short")
	}

	m.setSelection(1)
	if m.PreviewScroll != 0 {
		t.Fatalf("second document initial scroll = %d, want 0", m.PreviewScroll)
	}
	m.scrollPreview(3)
	secondScroll := m.PreviewScroll
	if secondScroll == 0 {
		t.Fatal("second document did not scroll; test fixture may be too short")
	}

	m.setSelection(0)
	if m.PreviewScroll != firstScroll {
		t.Fatalf("restored first scroll = %d, want %d", m.PreviewScroll, firstScroll)
	}
	m.setSelection(1)
	if m.PreviewScroll != secondScroll {
		t.Fatalf("restored second scroll = %d, want %d", m.PreviewScroll, secondScroll)
	}
}

func TestExternalChangeAddsMarkdownDocument(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.md")
	if err := os.WriteFile(first, []byte("# First"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(root)

	second := filepath.Join(root, "second.md")
	if err := os.WriteFile(second, []byte("# Second"), 0644); err != nil {
		t.Fatal(err)
	}
	m.rescanAfterExternalChange(filewatch.Event{Path: second, Rel: "second.md", Reason: "CREATE"})

	if len(m.Results) != 2 {
		t.Fatalf("results len = %d, want 2", len(m.Results))
	}
	if idx := m.findSidebarRow(sidebarRowDocument, "second.md", -1); idx < 0 {
		t.Fatalf("sidebar rows = %#v, want second.md", m.SidebarRows)
	}
}

func TestExternalChangeReloadsCurrentPreview(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("# Old"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	m.reloadPreview()

	if err := os.WriteFile(path, []byte("# New"), 0644); err != nil {
		t.Fatal(err)
	}
	m.rescanAfterExternalChange(filewatch.Event{Path: path, Rel: "doc.md", Reason: "WRITE"})

	if m.PreviewRaw != "# New" {
		t.Fatalf("PreviewRaw = %q, want # New", m.PreviewRaw)
	}
}

func TestExternalChangeDeletedCurrentDocumentSelectsNearest(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "a.md"),
		filepath.Join(root, "b.md"),
		filepath.Join(root, "c.md"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m := New(root)
	m.setSelection(1)
	if got := m.currentDoc().Rel; got != "b.md" {
		t.Fatalf("selected = %q, want b.md", got)
	}

	if err := os.Remove(paths[1]); err != nil {
		t.Fatal(err)
	}
	m.rescanAfterExternalChange(filewatch.Event{Path: paths[1], Rel: "b.md", Reason: "REMOVE"})

	if doc := m.currentDoc(); doc == nil || doc.Rel != "c.md" {
		t.Fatalf("current doc after delete = %#v, want c.md", doc)
	}
}

func TestExternalChangeDoesNotOverwriteDirtyEditorBuffer(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	m.enterEditMode()
	m.Editor.CX = lineLen(m.Editor.Lines[0])
	m.editorInsert(" local")
	if got := strings.Join(m.Editor.Lines, "\n"); got != "original local" {
		t.Fatalf("editor buffer before external change = %q", got)
	}

	if err := os.WriteFile(path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	m.rescanAfterExternalChange(filewatch.Event{Path: path, Rel: "doc.md", Reason: "WRITE"})

	if got := strings.Join(m.Editor.Lines, "\n"); got != "original local" {
		t.Fatalf("editor buffer after external change = %q, want original local", got)
	}
	if !m.Editor.ExternalChanged {
		t.Fatal("Editor.ExternalChanged = false, want true")
	}
}

func TestWatchDebounceIgnoresStaleGeneration(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.md")
	second := filepath.Join(root, "second.md")
	if err := os.WriteFile(first, []byte("# First"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	if err := os.WriteFile(second, []byte("# Second"), 0644); err != nil {
		t.Fatal(err)
	}

	m.WatchDebounceGen = 2
	m.WatchLastEvent = filewatch.Event{Path: second, Rel: "second.md", Reason: "CREATE"}
	m, _ = m.handleWatchDebounced(watchDebouncedMsg{Generation: 1})
	if len(m.Results) != 1 {
		t.Fatalf("stale debounce changed results len to %d, want 1", len(m.Results))
	}

	m, _ = m.handleWatchDebounced(watchDebouncedMsg{Generation: 2})
	if len(m.Results) != 2 {
		t.Fatalf("current debounce results len = %d, want 2", len(m.Results))
	}
}

func TestWatchRescanDebounceKeepsSinglePendingTimer(t *testing.T) {
	m := Model{}
	first := m.queueWatchRescan(filewatch.Event{Rel: "first.md"})
	if first == nil {
		t.Fatal("first debounce command is nil")
	}
	second := m.queueWatchRescan(filewatch.Event{Rel: "second.md"})
	if second != nil {
		t.Fatal("second debounce command should be coalesced while timer is pending")
	}
	if m.WatchDebounceGen != 2 || m.WatchLastEvent.Rel != "second.md" {
		t.Fatalf("debounce state = gen %d event %#v, want latest event", m.WatchDebounceGen, m.WatchLastEvent)
	}

	m, rescheduled := m.handleWatchDebounced(watchDebouncedMsg{Generation: 1})
	if rescheduled == nil {
		t.Fatal("stale debounce did not schedule a replacement timer")
	}
	if !m.WatchDebouncePending {
		t.Fatal("WatchDebouncePending = false after rescheduling stale debounce")
	}
}

func TestNewEnablesPollingRefresh(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc.md"), []byte("# Doc"), 0644); err != nil {
		t.Fatal(err)
	}

	m := New(root)

	if m.WatchFingerprint == "" {
		t.Fatal("WatchFingerprint is empty")
	}
	if !strings.Contains(m.Status, "polling refresh every 5s") {
		t.Fatalf("Status = %q, want polling refresh notice", m.Status)
	}
}

func TestPollTickQueuesRescanAndMarksIgnoreChange(t *testing.T) {
	m := Model{Root: t.TempDir(), WatchFingerprint: "old", WatchIgnoreFingerprint: "ignore-old"}

	m, cmd := m.handlePollTick(pollTickMsg{Fingerprint: "new", IgnoreFingerprint: "ignore-new"})

	if cmd == nil {
		t.Fatal("poll tick command is nil")
	}
	if m.WatchDebounceGen != 1 {
		t.Fatalf("WatchDebounceGen = %d, want 1", m.WatchDebounceGen)
	}
	if !m.WatchLastEvent.IgnoreChanged {
		t.Fatal("WatchLastEvent.IgnoreChanged = false, want true")
	}
}

func TestPollTickMarksDirtyEditorExternalChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	m.enterEditMode()
	m.Editor.CX = lineLen(m.Editor.Lines[0])
	m.editorInsert(" local")

	if err := os.WriteFile(path, []byte("external longer"), 0644); err != nil {
		t.Fatal(err)
	}
	m, _ = m.handlePollTick(pollTickMsg{Fingerprint: "new", IgnoreFingerprint: m.WatchIgnoreFingerprint})

	if !m.Editor.ExternalChanged {
		t.Fatal("Editor.ExternalChanged = false, want true after polling detects current editor file change")
	}
	if got := strings.Join(m.Editor.Lines, "\n"); got != "original local" {
		t.Fatalf("editor buffer after poll = %q, want original local", got)
	}
}

func TestPollTickMarksSameSizeSameModTimeEditorChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	fixed := time.Unix(1700000000, 0)
	if err := os.WriteFile(path, []byte("alpha"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	m.enterEditMode()
	m.editorInsert(" local")

	if err := os.WriteFile(path, []byte("bravo"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, fixed, fixed); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := filewatch.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	m, _ = m.handlePollTick(pollTickMsg{Fingerprint: fingerprint, IgnoreFingerprint: m.WatchIgnoreFingerprint})

	if !m.Editor.ExternalChanged {
		t.Fatal("Editor.ExternalChanged = false, want true for same-size same-mtime content change")
	}
}

func TestGlowedIgnoreChangeRescansExcludedDocuments(t *testing.T) {
	root := t.TempDir()
	ignorePath := filepath.Join(root, ".glowedignore")
	hidden := filepath.Join(root, "hidden.md")
	if err := os.WriteFile(ignorePath, []byte("hidden.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hidden, []byte("# Hidden"), 0644); err != nil {
		t.Fatal(err)
	}
	m := New(root)
	if len(m.Results) != 0 {
		t.Fatalf("initial results len = %d, want 0", len(m.Results))
	}

	if err := os.WriteFile(ignorePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	m.rescanAfterExternalChange(filewatch.Event{Path: ignorePath, Rel: ".glowedignore", Reason: "WRITE", IgnoreChanged: true})

	if len(m.Results) != 1 || m.Results[0].Rel != "hidden.md" {
		t.Fatalf("results after .glowedignore change = %#v, want hidden.md", m.Results)
	}
}

func longMarkdown(title string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&b, "## Section %02d\n\nThis is a long paragraph for %s section %02d.\n\n", i, title, i)
	}
	return b.String()
}
