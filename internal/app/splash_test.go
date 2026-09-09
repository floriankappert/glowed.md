package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

func splashModel(t *testing.T) Model {
	t.Helper()
	return welcomeModel(t, "README.md")
}

// welcomeModel launches without an initial file, so the welcome screen is up.
// Files are created oldest-first, one minute apart, so the recent-file order is
// deterministic: the last name passed is the most recent.
func welcomeModel(t *testing.T, names ...string) Model {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, name := range names {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		at := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	m := New(root)
	m.Width, m.Height = 90, 24
	return m
}

func TestSplashIsShownOnStartup(t *testing.T) {
	m := splashModel(t)
	if !m.Splash {
		t.Fatal("Splash = false, want true on startup")
	}
	rows := viewRows(t, m)
	if len(rows) != m.Height {
		t.Fatalf("splash has %d rows, want %d", len(rows), m.Height)
	}
	view := strings.Join(rows, "\n")
	for _, want := range []string{"glowed", Version, splashLogo[0], "README.md"} {
		if !strings.Contains(view, want) {
			t.Fatalf("splash missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "glow preview") {
		t.Fatalf("splash should replace the main layout:\n%s", view)
	}
}

func TestSplashLogoLinesShareOneWidth(t *testing.T) {
	want := runewidth.StringWidth(splashLogo[0])
	for i, line := range splashLogo {
		if got := runewidth.StringWidth(line); got != want {
			t.Fatalf("splashLogo[%d] width = %d, want %d", i, got, want)
		}
	}
}

// The welcome screen stays up until a file is picked, so a stray key must not
// dismiss it.
func TestWelcomeStaysUpOnAnUnrelatedKey(t *testing.T) {
	m := splashModel(t)
	next, _ := m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !next.Splash {
		t.Fatal("Splash = false after an unrelated key, want the welcome screen to stay")
	}
	if next.Editor.Dirty {
		t.Fatal("Editor.Dirty = true, want the welcome screen to swallow the key")
	}
}

func TestWelcomeListsTheSevenMostRecentFiles(t *testing.T) {
	names := []string{"a.md", "b.md", "c.md", "d.md", "e.md", "f.md", "g.md", "h.md", "i.md"}
	m := welcomeModel(t, names...)
	recent := m.recentDocs()
	if len(recent) != maxRecentDocs {
		t.Fatalf("recentDocs = %d entries, want %d", len(recent), maxRecentDocs)
	}
	// Newest first: i.md was written last.
	want := []string{"i.md", "h.md", "g.md", "f.md", "e.md", "d.md", "c.md"}
	for i, w := range want {
		if recent[i].Rel != w {
			t.Fatalf("recentDocs[%d] = %q, want %q", i, recent[i].Rel, w)
		}
	}
	view := stripANSI(m.View())
	for _, w := range want {
		if !strings.Contains(view, w) {
			t.Fatalf("welcome screen missing %q:\n%s", w, view)
		}
	}
	if strings.Contains(view, "a.md") || strings.Contains(view, "b.md") {
		t.Fatalf("welcome screen lists more than %d files:\n%s", maxRecentDocs, view)
	}
}

func TestWelcomeArrowsMoveTheSelection(t *testing.T) {
	m := welcomeModel(t, "one.md", "two.md", "three.md")
	if m.SplashSelected != 0 {
		t.Fatalf("SplashSelected = %d, want 0", m.SplashSelected)
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	if m.SplashSelected != 1 {
		t.Fatalf("SplashSelected = %d after down, want 1", m.SplashSelected)
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyUp})
	if m.SplashSelected != 0 {
		t.Fatalf("SplashSelected = %d after up, want 0", m.SplashSelected)
	}
	// The selection is clamped at both ends.
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyUp})
	if m.SplashSelected != 0 {
		t.Fatalf("SplashSelected = %d, want it clamped at 0", m.SplashSelected)
	}
	for i := 0; i < 5; i++ {
		m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.SplashSelected != 2 {
		t.Fatalf("SplashSelected = %d, want it clamped at 2", m.SplashSelected)
	}
}

func TestWelcomeEnterOpensTheSelectedFile(t *testing.T) {
	m := welcomeModel(t, "old.md", "new.md")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown}) // second entry: old.md
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Splash {
		t.Fatal("welcome screen still up after enter")
	}
	if m.Mode != ModeEdit {
		t.Fatalf("Mode = %v, want ModeEdit", modeName(m.Mode))
	}
	if filepath.Base(m.Editor.File) != "old.md" {
		t.Fatalf("opened %q, want old.md", m.Editor.File)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "editor") {
		t.Fatalf("main layout not shown after opening:\n%s", view)
	}
}

// An explicit file on the command line is the selection, so no welcome screen.
func TestWelcomeSkippedForAnExplicitInitialFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(root, "given.md")
	if err := os.WriteFile(path, []byte("# Given\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewWithInitial(root, path)
	if m.Splash {
		t.Fatal("Splash = true, want the welcome screen skipped for an explicit file")
	}
}

func TestWelcomeWithoutDocumentsSaysSo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	m := New(root)
	m.Width, m.Height = 90, 24
	view := stripANSI(m.View())
	if !strings.Contains(view, "no markdown documents") {
		t.Fatalf("welcome screen does not mention the empty project:\n%s", view)
	}
	// ctrl+n is the way out of an empty project.
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyCtrlN})
	if !m.Prompt.Active {
		t.Fatal("ctrl+n did not open the filename prompt on the welcome screen")
	}
}

func TestWelcomeCtrlCStillQuits(t *testing.T) {
	m := splashModel(t)
	if _, cmd := m.update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Fatal("ctrl+c returned no command on the welcome screen")
	}
}

func TestSplashResizeIsStillApplied(t *testing.T) {
	m := splashModel(t)
	next, _ := m.update(tea.WindowSizeMsg{Width: 70, Height: 24})
	if !next.Splash {
		t.Fatal("Splash = false after resize, want it to stay up")
	}
	if next.Width != 70 || next.Height != 24 {
		t.Fatalf("size = %dx%d, want 70x24", next.Width, next.Height)
	}
}

func TestWelcomeRecentFilesIsAHeading(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := welcomeModel(t, "one.md", "two.md")
	view := m.View()
	if !strings.Contains(stripANSI(view), "Recent files") {
		t.Fatalf("welcome screen missing the heading:\n%s", stripANSI(view))
	}
	if !strings.Contains(view, styleHeading.Render("Recent files")) {
		t.Fatal("\"Recent files\" is not rendered as a heading")
	}
}

// The welcome list is ordered by modification time, so a save reorders it.
func TestWelcomeReordersAfterAFileIsUpdated(t *testing.T) {
	m := welcomeModel(t, "first.md", "second.md")
	if got := m.recentDocs()[0].Rel; got != "second.md" {
		t.Fatalf("recentDocs[0] = %q, want second.md", got)
	}
	touched := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(m.Root, "first.md"), touched, touched); err != nil {
		t.Fatal(err)
	}
	m.scan("manual")
	if got := m.recentDocs()[0].Rel; got != "first.md" {
		t.Fatalf("recentDocs[0] = %q after touching first.md, want first.md", got)
	}
	if got := stripANSI(m.View()); strings.Index(got, "first.md") > strings.Index(got, "second.md") {
		t.Fatalf("welcome screen order does not follow the modification time:\n%s", got)
	}
}
