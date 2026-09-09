package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func typePrompt(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// Whatever is typed into the filename prompt, the result is either a file
// inside the project or a reported error — never a panic or an escape.
func TestNewFilePromptSurvivesAwkwardNames(t *testing.T) {
	for _, name := range []string{
		"plain",
		"with space.md",
		"한글.md",
		"dots...md",
		".hidden",
		"UPPER.MD",
		"sub/../sibling.md",
		"../escape.md",
		"/absolute.md",
		"..",
		".",
		"   ",
		strings.Repeat("x", 300),
		"a\tb",
		"trailing/",
	} {
		m, root := projectModel(t)
		m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
		m = typePrompt(t, m, name)
		m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

		if m.StatusKind == "error" || m.StatusKind == "warn" {
			// Refused: nothing may have been written outside the project.
			outside := filepath.Join(filepath.Dir(root), "escape.md")
			if _, err := os.Stat(outside); err == nil {
				t.Fatalf("%q wrote outside the project root", name)
			}
			continue
		}
		// Accepted: the file has to be inside the root.
		if m.Editor.File == "" {
			t.Fatalf("%q reported success without opening a file: %q", name, m.Status)
		}
		real, _ := filepath.EvalSymlinks(root)
		if !strings.HasPrefix(m.Editor.File, real) {
			t.Fatalf("%q created %q outside %q", name, m.Editor.File, real)
		}
		if _, err := os.Stat(m.Editor.File); err != nil {
			t.Fatalf("%q reported success but the file is missing: %v", name, err)
		}
	}
}

func TestRenameSurvivesAwkwardNames(t *testing.T) {
	for _, name := range []string{"", "   ", "../escape.md", "beta.md", "renamed.md", strings.Repeat("y", 300), "한글.md"} {
		m, root := projectModel(t)
		m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
		m = selectMenuLabel(t, m, "edit filename")
		m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		for range m.Prompt.Input {
			m = press(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
		}
		m = typePrompt(t, m, name)
		m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})

		if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape.md")); err == nil {
			t.Fatalf("%q renamed outside the project root", name)
		}
		// beta.md must survive a rename that would have replaced it.
		body, err := os.ReadFile(filepath.Join(root, "beta.md"))
		if err != nil || !strings.Contains(string(body), "# Beta") {
			t.Fatalf("%q damaged beta.md: %q (%v)", name, body, err)
		}
	}
}

// Deleting the last document must leave a usable frame, not a broken one.
func TestDeletingEveryDocumentLeavesAUsableFrame(t *testing.T) {
	m, root := projectModel(t)
	for i := 0; i < 5; i++ {
		if m.currentDoc() == nil {
			break
		}
		m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
		m = selectMenuLabel(t, m, "delete file")
		m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
		m.Width, m.Height = 90, 24
		rows := strings.Split(m.View(), "\n")
		if len(rows) != m.Height {
			t.Fatalf("round %d: %d rows", i, len(rows))
		}
	}
	if m.currentDoc() != nil {
		t.Fatalf("documents left: %+v", m.Results)
	}
	// Every markdown file is gone, and a backup was kept for each.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	baks := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			t.Fatalf("%q survived the delete", e.Name())
		}
		if strings.HasSuffix(e.Name(), ".md.bak") {
			baks++
		}
	}
	if baks != 3 {
		t.Fatalf("%d backups kept, want one per deleted file", baks)
	}
	// The empty project still renders and still offers a way out.
	if view := stripANSI(m.View()); !strings.Contains(view, "glowed.md") {
		t.Fatalf("frame lost its header:\n%s", view)
	}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlN})
	if !m.Prompt.Active {
		t.Fatal("ctrl+n no longer works in an empty project")
	}
}

// A file that disappears behind glowed's back must not take it down.
func TestOperationsSurviveAVanishedFile(t *testing.T) {
	m, root := projectModel(t)
	if err := os.Remove(filepath.Join(root, "alpha.md")); err != nil {
		t.Fatal(err)
	}
	// The scan results still name it.
	m.saveEditor()
	if m.StatusKind != "error" && m.StatusKind != "warn" {
		t.Fatalf("saving a vanished file reported %q: %q", m.StatusKind, m.Status)
	}

	m = press(t, m, tea.KeyMsg{Type: tea.KeyCtrlP})
	m = selectMenuLabel(t, m, "delete file")
	m = press(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.StatusKind != "error" {
		t.Fatalf("deleting a vanished file reported %q: %q", m.StatusKind, m.Status)
	}
	_ = m.View()
}

// Pasted junk must not reach the buffer, and pasted text must not break it.
func TestPasteHandlesControlCharacters(t *testing.T) {
	m, _ := projectModel(t)
	m.Editor = editorState{Lines: []string{""}, File: m.Editor.File}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\x00b\x1b[31mc\r\nd\te"), Paste: true})
	joined := strings.Join(m.Editor.Lines, "\n")
	if !strings.Contains(joined, "d  e") {
		t.Fatalf("a pasted tab should become the two spaces the tab key inserts: %q", joined)
	}
	for _, forbidden := range []string{"\x00", "\x1b"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("buffer took a control character: %q", joined)
		}
	}
	if !strings.Contains(joined, "a") || !strings.Contains(joined, "d") {
		t.Fatalf("paste lost its text: %q", joined)
	}
	if strings.Contains(joined, "\r") {
		t.Fatalf("buffer kept a carriage return: %q", joined)
	}
}

// A control rune that came from a file on disk must not reach the terminal.
func TestControlRunesFromDiskAreNotEmitted(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(root, "hostile.md")
	body := "# Title\n\nbefore\x1b[2J\x1b[Hafter\x00nul\x07bell\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewWithInitial(root, path)
	m.Width, m.Height = 90, 24
	m.dispatch("edit")
	next, _ := m.dispatch("edit")
	m = next

	view := m.View()
	// The frame's own styling uses escapes, so check the text of each row.
	for i, row := range strings.Split(view, "\n") {
		plain := stripANSI(row)
		for _, forbidden := range []rune{0x00, 0x07, 0x1b} {
			if strings.ContainsRune(plain, forbidden) {
				t.Fatalf("row %d emitted control rune %#U: %q", i, forbidden, plain)
			}
		}
	}
	if !strings.Contains(stripANSI(view), "before") {
		t.Fatalf("the line's text was lost:\n%s", stripANSI(view))
	}
	// The buffer itself keeps the file faithful, so saving cannot corrupt it.
	if !strings.Contains(strings.Join(m.Editor.Lines, "\n"), "\x1b") {
		t.Fatal("the buffer was altered; a save would rewrite the file")
	}
}

func TestSanitizePastedKeepsTextAndDropsControls(t *testing.T) {
	got := sanitizePasted("a\x00b\x1bc\r\nd\te\x07")
	if want := "abc\nd  e"; got != want {
		t.Fatalf("sanitizePasted = %q, want %q", got, want)
	}
	if sanitizePasted("\x00\x1b\x07") != "" {
		t.Fatal("a paste of only control characters should come out empty")
	}
	// Printable exotica survives: no-break space and a zero-width joiner.
	if got := sanitizePasted("a\u00a0b\u200dc"); got != "a\u00a0b\u200dc" {
		t.Fatalf("sanitizePasted dropped printable runes: %q", got)
	}
}
