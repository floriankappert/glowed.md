package editor

import (
	"strings"
	"testing"
)

func TestSliceMarkdownSameLine(t *testing.T) {
	got, rng, ok := SliceMarkdown([]string{"abcdef"}, Position{Line: 0, Col: 1}, Position{Line: 0, Col: 4})
	if !ok {
		t.Fatal("expected non-empty selection")
	}
	if got != "bcd" {
		t.Fatalf("SliceMarkdown() = %q, want %q", got, "bcd")
	}
	if rng.Start != (Position{Line: 0, Col: 1}) || rng.End != (Position{Line: 0, Col: 4}) {
		t.Fatalf("range = %+v", rng)
	}
}

func TestSliceMarkdownMultilineIncludesNewlines(t *testing.T) {
	lines := []string{"alpha", "bravo", "charlie"}
	got, _, ok := SliceMarkdown(lines, Position{Line: 0, Col: 2}, Position{Line: 2, Col: 4})
	if !ok {
		t.Fatal("expected non-empty selection")
	}
	want := "pha\nbravo\nchar"
	if got != want {
		t.Fatalf("SliceMarkdown() = %q, want %q", got, want)
	}
}

func TestSliceMarkdownReverseAndClamp(t *testing.T) {
	lines := []string{"abc", "de"}
	got, rng, ok := SliceMarkdown(lines, Position{Line: 10, Col: 20}, Position{Line: 0, Col: 2})
	if !ok {
		t.Fatal("expected non-empty selection")
	}
	want := "c\nde"
	if got != want {
		t.Fatalf("SliceMarkdown() = %q, want %q", got, want)
	}
	if rng.Start != (Position{Line: 0, Col: 2}) || rng.End != (Position{Line: 1, Col: 2}) {
		t.Fatalf("range = %+v", rng)
	}
}

func TestFormatClipboard(t *testing.T) {
	payload, rng, ok := FormatClipboard("/tmp/doc.md", []string{"hello world"}, Position{Line: 0, Col: 6}, Position{Line: 0, Col: 11})
	if !ok {
		t.Fatal("expected formatted payload")
	}
	if rng.Start != (Position{Line: 0, Col: 6}) || rng.End != (Position{Line: 0, Col: 11}) {
		t.Fatalf("range = %+v", rng)
	}
	for _, want := range []string{
		"<!-- glowed",
		"path: /tmp/doc.md",
		"start_line: 1",
		"start_col: 7",
		"end_line: 1",
		"end_col: 12",
		"\n\nworld",
	} {
		if !strings.Contains(payload, want) {
			t.Fatalf("payload missing %q:\n%s", want, payload)
		}
	}
}
