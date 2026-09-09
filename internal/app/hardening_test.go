package app

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"
)

// hardeningModel is a small project with the frame in its default state.
func hardeningModel(t *testing.T) Model {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"a.md":           "# A\n\nplain\n",
		"wide.md":        "# 한글 제목\n\n한글 본문 텍스트\n",
		"code.md":        "# Code\n\n```go\nfunc main() {}\n```\n",
		"deep/nested.md": "# Nested\n",
		"empty.md":       "",
		"long.md":        "# Long\n\n" + strings.Repeat("word ", 400) + "\n",
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := NewWithInitial(root, filepath.Join(root, "a.md"))
	m.Width, m.Height = 100, 30
	m.ensureSidebarState()
	m.rebuildSidebarRows()
	return m
}

// Every frame must be exactly Width columns by Height rows, at any size and in
// any state — a mismatch corrupts the alt-screen.
func TestFrameGeometryHoldsAtEverySize(t *testing.T) {
	base := hardeningModel(t)
	states := map[string]func(m Model) Model{
		"edit":          func(m Model) Model { return m },
		"preview":       func(m Model) Model { m.Mode, m.Focus = ModePreview, FocusPreview; return m },
		"source":        func(m Model) Model { m.Mode = ModeSource; return m },
		"search":        func(m Model) Model { m.Focus, m.Query = FocusSearch, "a"; return m },
		"no sidebar":    func(m Model) Model { m.SidebarVisible = false; return m },
		"chat":          func(m Model) Model { m.Chat.Visible = true; return m },
		"menu":          func(m Model) Model { m.Menu = menuState{Active: true, Selected: menuFilterFocus}; return m },
		"menu filtered": func(m Model) Model { m.Menu = menuState{Active: true, Query: "zzzz"}; return m },
		"hotkeys": func(m Model) Model {
			m.Menu = menuState{Active: true, Path: []string{"configuration", "hotkeys"}}
			return m
		},
		"prompt":       func(m Model) Model { m.Prompt = promptState{Active: true, Kind: promptNewFile, Input: "x"}; return m },
		"welcome":      func(m Model) Model { m.Splash = true; return m },
		"welcome menu": func(m Model) Model { m.Splash = true; m.Menu = menuState{Active: true}; return m },
		"no documents": func(m Model) Model { m.Docs, m.Results, m.SidebarRows = nil, nil, nil; return m },
	}
	sizes := []struct{ w, h int }{
		{1, 1}, {2, 2}, {4, 3}, {10, 5}, {20, 8}, {40, 9}, {60, 12}, {80, 24}, {120, 40}, {300, 60},
	}
	for name, prepare := range states {
		for _, size := range sizes {
			m := prepare(base)
			m.Width, m.Height = size.w, size.h
			view := m.View()
			if view == "" {
				continue
			}
			rows := strings.Split(view, "\n")
			if len(rows) != m.Height {
				t.Fatalf("%s at %dx%d: %d rows", name, size.w, size.h, len(rows))
			}
			for i, row := range rows {
				if w := xansi.StringWidth(row); w != m.Width {
					t.Fatalf("%s at %dx%d: row %d is %d columns: %q",
						name, size.w, size.h, i, w, stripANSI(row))
				}
			}
		}
	}
}

// A degenerate or negative size must not panic or produce output.
func TestViewSurvivesDegenerateSizes(t *testing.T) {
	m := hardeningModel(t)
	for _, size := range []struct{ w, h int }{{0, 0}, {-1, 10}, {10, -1}, {0, 30}, {30, 0}} {
		m.Width, m.Height = size.w, size.h
		if got := m.View(); got != "" {
			t.Fatalf("%dx%d rendered %q, want nothing", size.w, size.h, got)
		}
	}
}

// The menu overlay is drawn from measurements; none of them may go negative.
func TestMenuRowSurvivesTinyRegions(t *testing.T) {
	m := hardeningModel(t)
	m.Menu = menuState{Active: true}
	for _, w := range []int{-1, 0, 1, 2, 3, 5, 12} {
		for _, h := range []int{-1, 0, 1, 2, 5, 40} {
			for _, row := range []int{-1, 0, 1, 5, 100} {
				line, ok := m.renderMenuRow(w, h, row)
				if !ok {
					continue
				}
				if got := xansi.StringWidth(line); got != max(0, w) {
					t.Fatalf("renderMenuRow(%d,%d,%d) is %d columns", w, h, row, got)
				}
			}
		}
	}
}

func TestFitMenuBlockNeverExceedsItsHeight(t *testing.T) {
	m := hardeningModel(t)
	for _, path := range [][]string{nil, {"configuration"}, {"configuration", "defaults"}, {"configuration", "hotkeys"}} {
		m.Menu = menuState{Active: true, Path: path}
		block := m.menuBlock()
		for h := -1; h <= len(block)+3; h++ {
			for _, selected := range []int{menuFilterFocus, 0, 1, len(block), 999} {
				for _, scroll := range []int{0, 3, 999} {
					got := fitMenuBlock(block, h, selected, scroll)
					if h <= 0 && len(got) != 0 {
						t.Fatalf("height %d returned %d rows", h, len(got))
					}
					if h > 0 && len(got) > h {
						t.Fatalf("height %d returned %d rows (path=%v)", h, len(got), path)
					}
				}
			}
		}
	}
}

// Random keys must never panic, and the frame must stay intact throughout.
func TestRandomKeysNeverPanic(t *testing.T) {
	keys := []tea.KeyMsg{
		{Type: tea.KeyUp}, {Type: tea.KeyDown}, {Type: tea.KeyLeft}, {Type: tea.KeyRight},
		{Type: tea.KeyEnter}, {Type: tea.KeyEsc}, {Type: tea.KeyTab}, {Type: tea.KeyShiftTab},
		{Type: tea.KeyBackspace}, {Type: tea.KeyDelete}, {Type: tea.KeySpace},
		{Type: tea.KeyHome}, {Type: tea.KeyEnd}, {Type: tea.KeyPgUp}, {Type: tea.KeyPgDown},
		{Type: tea.KeyCtrlP}, {Type: tea.KeyCtrlN}, {Type: tea.KeyCtrlT}, {Type: tea.KeyCtrlB},
		{Type: tea.KeyCtrlS}, {Type: tea.KeyCtrlZ}, {Type: tea.KeyCtrlY}, {Type: tea.KeyCtrlK},
		{Type: tea.KeyCtrlU}, {Type: tea.KeyCtrlA}, {Type: tea.KeyCtrlE},
		{Type: tea.KeyShiftUp}, {Type: tea.KeyShiftDown}, {Type: tea.KeyShiftLeft}, {Type: tea.KeyShiftRight},
		{Type: tea.KeyRunes, Runes: []rune("a")}, {Type: tea.KeyRunes, Runes: []rune("/")},
		{Type: tea.KeyRunes, Runes: []rune("e")}, {Type: tea.KeyRunes, Runes: []rune("v")},
		{Type: tea.KeyRunes, Runes: []rune("한")}, {Type: tea.KeyRunes, Runes: []rune("[")},
		{Type: tea.KeyRunes, Runes: []rune{'y'}, Alt: true},
		{Type: tea.KeyRunes, Runes: []rune("多行\n粘贴"), Paste: true},
	}
	sizes := []struct{ w, h int }{{100, 30}, {40, 10}, {12, 6}}

	rng := rand.New(rand.NewSource(1))
	m := hardeningModel(t)
	for step := 0; step < 4000; step++ {
		key := keys[rng.Intn(len(keys))]
		next, _ := m.update(key)
		m = next
		if step%7 == 0 {
			size := sizes[rng.Intn(len(sizes))]
			m, _ = m.update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		}
		view := m.View()
		rows := strings.Split(view, "\n")
		if len(rows) != m.Height {
			t.Fatalf("step %d (%v): %d rows, want %d", step, key, len(rows), m.Height)
		}
		for i, row := range rows {
			if w := xansi.StringWidth(row); w != m.Width {
				t.Fatalf("step %d (%v): row %d is %d columns, want %d: %q",
					step, key, i, w, m.Width, stripANSI(row))
			}
		}
	}
}

// Random mouse events must not panic either.
func TestRandomMouseNeverPanics(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	m := hardeningModel(t)
	buttons := []tea.MouseButton{tea.MouseButtonLeft, tea.MouseButtonRight, tea.MouseButtonWheelUp, tea.MouseButtonWheelDown, tea.MouseButtonNone}
	actions := []tea.MouseAction{tea.MouseActionPress, tea.MouseActionRelease, tea.MouseActionMotion}
	for step := 0; step < 3000; step++ {
		m.handleMouse(tea.MouseMsg(tea.MouseEvent{
			X:      rng.Intn(m.Width+4) - 2,
			Y:      rng.Intn(m.Height+4) - 2,
			Button: buttons[rng.Intn(len(buttons))],
			Action: actions[rng.Intn(len(actions))],
		}))
		if step%11 == 0 {
			m.Menu.Active = !m.Menu.Active
		}
		_ = m.View()
	}
}
