package editor

import "unicode"

// isWordRune reports whether r belongs to a word for word-wise motion.
// Punctuation separates words, matching macOS and readline behaviour.
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// WordLeft returns the position one word to the left of p.
// It skips any separators before the word, then the word itself.
// At the start of a line it wraps to the end of the previous line.
func WordLeft(lines []string, p Position) Position {
	lines = ensureLines(lines)
	p = clampPosition(lines, p)
	if p.Col == 0 {
		if p.Line == 0 {
			return p
		}
		prev := p.Line - 1
		return Position{Line: prev, Col: len([]rune(lines[prev]))}
	}
	runes := []rune(lines[p.Line])
	i := p.Col
	for i > 0 && !isWordRune(runes[i-1]) {
		i--
	}
	for i > 0 && isWordRune(runes[i-1]) {
		i--
	}
	return Position{Line: p.Line, Col: i}
}

// WordRight returns the position at the end of the next word to the right of p.
// It skips any separators after p, then the word itself.
// At the end of a line it wraps into the next line.
func WordRight(lines []string, p Position) Position {
	lines = ensureLines(lines)
	p = clampPosition(lines, p)
	runes := []rune(lines[p.Line])
	if p.Col >= len(runes) {
		if p.Line >= len(lines)-1 {
			return p
		}
		return WordRight(lines, Position{Line: p.Line + 1, Col: 0})
	}
	i := p.Col
	for i < len(runes) && !isWordRune(runes[i]) {
		i++
	}
	for i < len(runes) && isWordRune(runes[i]) {
		i++
	}
	return Position{Line: p.Line, Col: i}
}

// LineStart returns the first column of p's line.
func LineStart(lines []string, p Position) Position {
	lines = ensureLines(lines)
	p = clampPosition(lines, p)
	return Position{Line: p.Line, Col: 0}
}

// LineEnd returns the last column of p's line.
func LineEnd(lines []string, p Position) Position {
	lines = ensureLines(lines)
	p = clampPosition(lines, p)
	return Position{Line: p.Line, Col: len([]rune(lines[p.Line]))}
}

// DeleteRange removes the text between a and b and returns the resulting lines
// together with the caret position after the deletion. The input slice is left
// untouched. changed is false when the range is empty.
func DeleteRange(lines []string, a, b Position) (out []string, at Position, changed bool) {
	lines = ensureLines(lines)
	a = clampPosition(lines, a)
	b = clampPosition(lines, b)
	rng := Normalize(a, b)
	if comparePosition(rng.Start, rng.End) == 0 {
		return lines, rng.Start, false
	}

	head := string([]rune(lines[rng.Start.Line])[:rng.Start.Col])
	tail := string([]rune(lines[rng.End.Line])[rng.End.Col:])

	out = make([]string, 0, len(lines)-(rng.End.Line-rng.Start.Line))
	out = append(out, lines[:rng.Start.Line]...)
	out = append(out, head+tail)
	out = append(out, lines[rng.End.Line+1:]...)
	return out, rng.Start, true
}

func ensureLines(lines []string) []string {
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// ClampPosition limits p to the bounds of lines.
func ClampPosition(lines []string, p Position) Position {
	return clampPosition(ensureLines(lines), p)
}
