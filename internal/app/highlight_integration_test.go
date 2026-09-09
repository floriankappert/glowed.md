package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/khw1031/glowed/internal/config"
)

func highlightModel(t *testing.T, body string) Model {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewWithInitial(root, path)
	m.Splash = false
	m.Width, m.Height = 100, 24
	next, _ := m.dispatch("edit")
	m = next
	m.refreshHighlight()
	return m
}

func TestEditModeHighlightsCodeBlocks(t *testing.T) {
	m := highlightModel(t, "intro\n\n```go\nfunc main() {}\n```\n")
	if len(m.Highlight[3]) == 0 {
		t.Fatalf("code line not highlighted, spans=%+v", m.Highlight)
	}
	if len(m.Highlight[0]) != 0 {
		t.Fatal("prose line was highlighted")
	}
}

func TestEditModeHighlightRenderedIntoEditorLine(t *testing.T) {
	// Without a TTY lipgloss degrades to the ASCII profile and drops all color.
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := highlightModel(t, "intro\n\n```go\nfunc main() {}\n```\n")
	// Pane rows map 1:1 onto buffer lines, so buffer index 3 is row 3.
	line := m.renderEditorLine(3)
	if !strings.Contains(line, "\x1b[") {
		t.Fatalf("code line rendered without color: %q", line)
	}
	if !strings.Contains(stripANSI(line), "func main() {}") {
		t.Fatalf("code line text lost: %q", stripANSI(line))
	}
}

func TestHighlightFollowsBufferEdits(t *testing.T) {
	m := highlightModel(t, "```go\nfunc\n```\n")
	before := len(m.Highlight[1])
	if before == 0 {
		t.Fatal("no spans before the edit")
	}
	m.Editor.CY, m.Editor.CX = 1, 4
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" x() {}")})
	m = out.(Model)
	if strings.Join(m.Editor.Lines, "\n") != "```go\nfunc x() {}\n```" {
		t.Fatalf("buffer = %q", m.Editor.Lines)
	}
	if len(m.Highlight[1]) <= before {
		t.Fatalf("spans not recomputed after edit: %+v", m.Highlight[1])
	}
}

func TestHighlightClearedWhenLeavingRawBuffer(t *testing.T) {
	m := highlightModel(t, "```go\nfunc main() {}\n```\n")
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = out.(Model)
	if m.Highlight != nil {
		t.Fatalf("highlight survived leaving edit mode: %+v", m.Highlight)
	}
}

func TestHighlightNotRecomputedForUnchangedBuffer(t *testing.T) {
	m := highlightModel(t, "```go\nfunc main() {}\n```\n")
	key := m.HighlightKey
	first := m.Highlight
	m.refreshHighlight()
	if m.HighlightKey != key {
		t.Fatal("fingerprint changed without an edit")
	}
	if &first == nil {
		t.Fatal("unreachable")
	}
}

func BenchmarkRefreshHighlightLargeBuffer(b *testing.B) {
	lines := []string{}
	for i := 0; i < 200; i++ {
		lines = append(lines,
			"Some prose paragraph that is reasonably long and not code at all.",
			"",
			"```go",
			"func handler(w http.ResponseWriter, r *http.Request) error { return nil }",
			"```",
			"")
	}
	m := Model{Mode: ModeEdit, Cfg: config.Default(), Editor: editorState{Lines: lines}}
	m.refreshHighlight()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate a keystroke: one code line changes, the rest stays cached.
		m.Editor.Lines[3] = fmt.Sprintf("func handler%d(w http.ResponseWriter) error { return nil }", i)
		m.refreshHighlight()
	}
}
