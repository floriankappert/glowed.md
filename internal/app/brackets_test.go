package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A bracket that arrives as a plain rune must land in the buffer.
func TestBracketsAreTypableAsPlainRunes(t *testing.T) {
	m, _ := projectModel(t)
	m.Editor = editorState{Lines: []string{""}, File: m.Editor.File}
	for _, r := range "[]{}()" {
		m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if got := m.Editor.Lines[0]; got != "[]{}()" {
		t.Fatalf("buffer = %q, want %q", got, "[]{}()")
	}
}

// On a German Mac layout the brackets are Option-composed. With Ghostty's
// macos-option-as-alt = true they arrive as alt+digit instead of as the
// composed rune, and glowed drops every alt combination it has no binding for.
// It must not type the bare digit, and it must say why nothing happened.
func TestUnboundAltKeyExplainsItselfInsteadOfBeingSilent(t *testing.T) {
	m, _ := projectModel(t)
	m.Editor = editorState{Lines: []string{""}, File: m.Editor.File}
	m = press(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}, Alt: true})

	if got := m.Editor.Lines[0]; got != "" {
		t.Fatalf("buffer = %q, want the alt combination dropped, not the bare digit", got)
	}
	if m.StatusKind != "warn" {
		t.Fatalf("StatusKind = %q, want warn", m.StatusKind)
	}
	if !strings.Contains(m.Status, "option-as-alt") {
		t.Fatalf("Status = %q, want a hint naming the Ghostty setting", m.Status)
	}
}

// Bindings that legitimately use alt keep working and must not warn.
func TestBoundAltKeysStaySilent(t *testing.T) {
	m, _ := projectModel(t)
	m = press(t, m, tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	if strings.Contains(m.Status, "option-as-alt") {
		t.Fatalf("Status = %q, want no hint for a bound alt binding", m.Status)
	}
}
