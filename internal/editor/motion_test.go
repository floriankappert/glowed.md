package editor

import "testing"

func TestWordLeft(t *testing.T) {
	lines := []string{"hello world  foo", "second line"}
	cases := []struct {
		name string
		from Position
		want Position
	}{
		{"from end of word", Position{0, 16}, Position{0, 13}},
		{"from start of word skips gap", Position{0, 13}, Position{0, 6}},
		{"inside word", Position{0, 9}, Position{0, 6}},
		{"first word goes to col 0", Position{0, 6}, Position{0, 0}},
		{"col 0 wraps to end of previous line", Position{1, 0}, Position{0, 16}},
		{"start of buffer stays", Position{0, 0}, Position{0, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WordLeft(lines, tc.from); got != tc.want {
				t.Fatalf("WordLeft(%v) = %v, want %v", tc.from, got, tc.want)
			}
		})
	}
}

func TestWordRight(t *testing.T) {
	lines := []string{"hello world  foo", "second line"}
	cases := []struct {
		name string
		from Position
		want Position
	}{
		{"from col 0 to end of first word", Position{0, 0}, Position{0, 5}},
		{"inside word to its end", Position{0, 2}, Position{0, 5}},
		{"skips gap to end of next word", Position{0, 5}, Position{0, 11}},
		{"across double space", Position{0, 11}, Position{0, 16}},
		{"end of line wraps to next line", Position{0, 16}, Position{1, 6}},
		{"end of buffer stays", Position{1, 11}, Position{1, 11}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WordRight(lines, tc.from); got != tc.want {
				t.Fatalf("WordRight(%v) = %v, want %v", tc.from, got, tc.want)
			}
		})
	}
}

func TestWordMotionTreatsPunctuationAsSeparator(t *testing.T) {
	lines := []string{"foo.bar(baz)"}
	if got := WordRight(lines, Position{0, 0}); got != (Position{0, 3}) {
		t.Fatalf("WordRight over punctuation = %v, want {0 3}", got)
	}
	if got := WordLeft(lines, Position{0, 12}); got != (Position{0, 8}) {
		t.Fatalf("WordLeft over punctuation = %v, want {0 8}", got)
	}
}

func TestWordMotionHandlesMultibyte(t *testing.T) {
	lines := []string{"grüße welt"}
	if got := WordRight(lines, Position{0, 0}); got != (Position{0, 5}) {
		t.Fatalf("WordRight = %v, want {0 5}", got)
	}
	if got := WordLeft(lines, Position{0, 5}); got != (Position{0, 0}) {
		t.Fatalf("WordLeft = %v, want {0 0}", got)
	}
}

func TestLineStartEnd(t *testing.T) {
	lines := []string{"  indented text", ""}
	if got := LineStart(lines, Position{0, 9}); got != (Position{0, 0}) {
		t.Fatalf("LineStart = %v, want {0 0}", got)
	}
	if got := LineEnd(lines, Position{0, 2}); got != (Position{0, 15}) {
		t.Fatalf("LineEnd = %v, want {0 15}", got)
	}
	if got := LineEnd(lines, Position{1, 0}); got != (Position{1, 0}) {
		t.Fatalf("LineEnd on empty line = %v, want {1 0}", got)
	}
}

func TestDeleteRange(t *testing.T) {
	t.Run("within one line", func(t *testing.T) {
		lines, at, changed := DeleteRange([]string{"hello world"}, Position{0, 5}, Position{0, 11})
		if !changed || at != (Position{0, 5}) || len(lines) != 1 || lines[0] != "hello" {
			t.Fatalf("got %q at %v changed %v", lines, at, changed)
		}
	})
	t.Run("across lines joins", func(t *testing.T) {
		lines, at, changed := DeleteRange([]string{"abc", "def", "ghi"}, Position{0, 1}, Position{2, 2})
		if !changed || at != (Position{0, 1}) || len(lines) != 1 || lines[0] != "ai" {
			t.Fatalf("got %q at %v changed %v", lines, at, changed)
		}
	})
	t.Run("reversed range is normalized", func(t *testing.T) {
		lines, at, changed := DeleteRange([]string{"hello world"}, Position{0, 11}, Position{0, 5})
		if !changed || at != (Position{0, 5}) || lines[0] != "hello" {
			t.Fatalf("got %q at %v changed %v", lines, at, changed)
		}
	})
	t.Run("empty range changes nothing", func(t *testing.T) {
		_, _, changed := DeleteRange([]string{"hello"}, Position{0, 2}, Position{0, 2})
		if changed {
			t.Fatal("empty range reported a change")
		}
	})
	t.Run("does not alias the input slice", func(t *testing.T) {
		in := []string{"abc", "def"}
		DeleteRange(in, Position{0, 0}, Position{1, 3})
		if in[0] != "abc" || in[1] != "def" {
			t.Fatalf("input mutated: %q", in)
		}
	})
}
