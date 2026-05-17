package render

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkMarkdownPreviewRender(b *testing.B) {
	raw := benchmarkMarkdown(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := Markdown(raw, 100, "dark", true)
		if err != nil {
			b.Fatal(err)
		}
		if out == "" {
			b.Fatal("empty render output")
		}
	}
}

func benchmarkMarkdown(sections int) string {
	var b strings.Builder
	b.WriteString("---\ntitle: Benchmark Document\ntags: [bench, render]\n---\n\n")
	b.WriteString("# Benchmark Document\n\n")
	b.WriteString("> This document exercises headings, paragraphs, lists, tables, and code fences.\n\n")
	for i := 0; i < sections; i++ {
		fmt.Fprintf(&b, "## Section %03d\n\n", i)
		fmt.Fprintf(&b, "This paragraph contains enough text to wrap across multiple terminal columns. It mentions tag:bench-%03d and includes **bold**, _italic_, and `inline code`.\n\n", i)
		b.WriteString("- first item\n- second item with [a link](https://example.com)\n- third item\n\n")
		b.WriteString("| key | value |\n| --- | ----- |\n")
		fmt.Fprintf(&b, "| index | %03d |\n| status | ok |\n\n", i)
		b.WriteString("```go\n")
		fmt.Fprintf(&b, "fmt.Println(\"section %03d\")\n", i)
		b.WriteString("```\n\n")
	}
	return b.String()
}
