package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khw1031/glowed/internal/config"
	"github.com/khw1031/glowed/internal/docs"
	llmclient "github.com/khw1031/glowed/internal/llm"
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
	if !strings.Contains(m.renderSeparator(), "source selection") {
		t.Fatalf("separator = %q, want source selection", m.renderSeparator())
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

func longMarkdown(title string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&b, "## Section %02d\n\nThis is a long paragraph for %s section %02d.\n\n", i, title, i)
	}
	return b.String()
}
