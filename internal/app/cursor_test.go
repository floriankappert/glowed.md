package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// The caret used to be inserted before the character it sits on, which pushed
// the rest of the line one column to the right.
func TestEditorCursorDoesNotShiftTheLine(t *testing.T) {
	const line = "abcdef"
	for _, at := range []int{0, 1, 3, 6} {
		out := renderEditorVisibleLine(line, 0, 20, at, true, 0, 0, false, nil)
		plain := strings.TrimRight(stripANSI(out), " ")
		if !strings.HasPrefix(plain, line) {
			t.Fatalf("cursor at %d rendered %q, want the line unshifted (%q)", at, plain, line)
		}
	}
}

func TestEditorCursorRendersTheCellItCovers(t *testing.T) {
	// Without a TTY lipgloss degrades to the ASCII profile and drops all styling.
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	out := renderEditorVisibleLine("abc", 0, 20, 1, true, 0, 0, false, nil)
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("cursor cell not styled: %q", out)
	}
	if got := strings.TrimRight(stripANSI(out), " "); got != "abc" {
		t.Fatalf("plain text = %q, want %q", got, "abc")
	}
}

// At the end of the line there is no character to cover, so the caret needs a
// blank cell of its own without widening the rendered width.
func TestEditorCursorPastLastCharacterStaysInWidth(t *testing.T) {
	out := renderEditorVisibleLine("abc", 0, 4, 3, true, 0, 0, false, nil)
	if got := stripANSI(out); len([]rune(got)) > 4 {
		t.Fatalf("rendered %q, wider than the 4 available columns", got)
	}
}

// A wide rune occupies two columns; covering it must not eat the next one.
func TestEditorCursorOnWideRuneKeepsFollowingText(t *testing.T) {
	out := renderEditorVisibleLine("한글x", 0, 20, 0, true, 0, 0, false, nil)
	if got := strings.TrimRight(stripANSI(out), " "); got != "한글x" {
		t.Fatalf("plain text = %q, want %q", got, "한글x")
	}
}
