package render

import "testing"

func spansFor(t *testing.T, lines []string, line int) []Span {
	t.Helper()
	return CodeSpans(lines, "dark")[line]
}

func spanText(line string, s Span) string {
	return string([]rune(line)[s.Start:s.End])
}

func TestCodeSpansHighlightsFencedBlockWithLanguage(t *testing.T) {
	lines := []string{"# title", "", "```go", "func main() {}", "```", "done"}
	spans := spansFor(t, lines, 3)
	if len(spans) == 0 {
		t.Fatal("no spans for the code line")
	}
	var keyword *Span
	for i := range spans {
		if spanText(lines[3], spans[i]) == "func" {
			keyword = &spans[i]
		}
	}
	if keyword == nil {
		t.Fatalf("keyword span missing, got %+v", spans)
	}
	if keyword.Color == "" {
		t.Fatal("keyword span has no color")
	}
	for _, s := range spans {
		if s.Start < 0 || s.End > len([]rune(lines[3])) || s.Start >= s.End {
			t.Fatalf("span %+v out of bounds for %q", s, lines[3])
		}
	}
}

func TestCodeSpansLeavesProseAndFencesAlone(t *testing.T) {
	lines := []string{"# title", "", "```go", "func main() {}", "```", "done"}
	all := CodeSpans(lines, "dark")
	for _, line := range []int{0, 1, 2, 4, 5} {
		if len(all[line]) != 0 {
			t.Fatalf("line %d (%q) should not be highlighted, got %+v", line, lines[line], all[line])
		}
	}
}

func TestCodeSpansIgnoresBlockWithoutLanguage(t *testing.T) {
	lines := []string{"```", "func main() {}", "```"}
	if got := CodeSpans(lines, "dark"); len(got) != 0 {
		t.Fatalf("plain fence highlighted: %+v", got)
	}
}

func TestCodeSpansHandlesUnterminatedFence(t *testing.T) {
	lines := []string{"```go", "func main() {}"}
	spans := spansFor(t, lines, 1)
	if len(spans) == 0 {
		t.Fatal("unterminated fence should still highlight up to EOF")
	}
}

func TestCodeSpansHandlesTildeFencesAndIndentedContent(t *testing.T) {
	lines := []string{"~~~python", "  def f(): pass", "~~~"}
	spans := spansFor(t, lines, 1)
	if len(spans) == 0 {
		t.Fatal("no spans in tilde fence")
	}
	for _, s := range spans {
		if s.Start < 2 && spanText(lines[1], s) != "  " {
			continue
		}
	}
	var def *Span
	for i := range spans {
		if spanText(lines[1], spans[i]) == "def" {
			def = &spans[i]
		}
	}
	if def == nil {
		t.Fatalf("indent shifted the spans: %+v", spans)
	}
	if def.Start != 2 {
		t.Fatalf("def starts at %d, want 2", def.Start)
	}
}

func TestCodeSpansHandlesMultibyteBeforeToken(t *testing.T) {
	lines := []string{"```go", "// grüße", "func f() {}", "```"}
	spans := spansFor(t, lines, 2)
	var kw *Span
	for i := range spans {
		if spanText(lines[2], spans[i]) == "func" {
			kw = &spans[i]
		}
	}
	if kw == nil || kw.Start != 0 {
		t.Fatalf("multibyte line shifted rune offsets: %+v", spans)
	}
}

func TestCodeSpansUnknownLanguageDoesNotPanic(t *testing.T) {
	lines := []string{"```notalanguage", "whatever", "```"}
	_ = CodeSpans(lines, "dark")
}

func TestCodeSpansEmptyInput(t *testing.T) {
	if got := CodeSpans(nil, "dark"); len(got) != 0 {
		t.Fatalf("nil input produced %+v", got)
	}
}
