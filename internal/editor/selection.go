package editor

import (
	"fmt"
	"strings"
)

// Position is a zero-based caret position in raw markdown text.
// Col is a rune index and End positions are treated as exclusive.
type Position struct {
	Line int
	Col  int
}

// Range is a normalized, half-open range [Start, End).
type Range struct {
	Start Position
	End   Position
}

// SliceMarkdown returns the exact raw markdown slice covered by a selection.
// Positions are clamped to the provided lines and the end position is exclusive.
func SliceMarkdown(lines []string, a, b Position) (string, Range, bool) {
	if len(lines) == 0 {
		lines = []string{""}
	}
	a = clampPosition(lines, a)
	b = clampPosition(lines, b)
	rng := Normalize(a, b)
	if comparePosition(rng.Start, rng.End) == 0 {
		return "", rng, false
	}

	if rng.Start.Line == rng.End.Line {
		runes := []rune(lines[rng.Start.Line])
		return string(runes[rng.Start.Col:rng.End.Col]), rng, true
	}

	parts := make([]string, 0, rng.End.Line-rng.Start.Line+1)
	first := []rune(lines[rng.Start.Line])
	parts = append(parts, string(first[rng.Start.Col:]))
	for line := rng.Start.Line + 1; line < rng.End.Line; line++ {
		parts = append(parts, lines[line])
	}
	last := []rune(lines[rng.End.Line])
	parts = append(parts, string(last[:rng.End.Col]))
	return strings.Join(parts, "\n"), rng, true
}

// FormatClipboard wraps a selected raw markdown slice with glowed metadata.
func FormatClipboard(path string, lines []string, a, b Position) (string, Range, bool) {
	selected, rng, ok := SliceMarkdown(lines, a, b)
	if !ok {
		return "", rng, false
	}
	payload := fmt.Sprintf("<!-- glowed\npath: %s\nstart_line: %d\nstart_col: %d\nend_line: %d\nend_col: %d\n-->\n\n%s",
		path,
		rng.Start.Line+1,
		rng.Start.Col+1,
		rng.End.Line+1,
		rng.End.Col+1,
		selected,
	)
	return payload, rng, true
}

// Normalize returns a range whose Start is before or equal to End.
func Normalize(a, b Position) Range {
	if comparePosition(a, b) <= 0 {
		return Range{Start: a, End: b}
	}
	return Range{Start: b, End: a}
}

func clampPosition(lines []string, p Position) Position {
	if len(lines) == 0 {
		return Position{}
	}
	p.Line = clamp(p.Line, 0, len(lines)-1)
	p.Col = clamp(p.Col, 0, len([]rune(lines[p.Line])))
	return p
}

func comparePosition(a, b Position) int {
	if a.Line < b.Line {
		return -1
	}
	if a.Line > b.Line {
		return 1
	}
	if a.Col < b.Col {
		return -1
	}
	if a.Col > b.Col {
		return 1
	}
	return 0
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
