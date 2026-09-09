package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseMetaTags(t *testing.T) {
	raw := "---\ntitle: Hello\ntags: [ai, robot]\n---\n\nbody tag:note"
	meta := ParseMeta(raw)
	want := map[string]bool{"ai": true, "robot": true, "note": true}
	if len(meta.Tags) != len(want) {
		t.Fatalf("tags len = %d, want %d: %#v", len(meta.Tags), len(want), meta.Tags)
	}
	for _, tag := range meta.Tags {
		if !want[tag] {
			t.Fatalf("unexpected tag %q in %#v", tag, meta.Tags)
		}
	}
	if meta.Frontmatter["title"] != "Hello" {
		t.Fatalf("title = %#v", meta.Frontmatter["title"])
	}
}

func TestParseMetaYAMLQuotedArraysAndNestedObject(t *testing.T) {
	raw := `---
title: "Hello: Agent"
tags:
  - "AI-Agent"
  - topic/crypto
aliases: ['one', "two:with:colon"]
author:
  name: "Ada Lovelace"
  social:
    x: '@ada'
---

body tag:inline`
	meta := ParseMeta(raw)
	if got := meta.Frontmatter["title"]; got != "Hello: Agent" {
		t.Fatalf("title = %#v", got)
	}

	aliases, ok := meta.Frontmatter["aliases"].([]any)
	if !ok || len(aliases) != 2 || aliases[1] != "two:with:colon" {
		t.Fatalf("aliases = %#v", meta.Frontmatter["aliases"])
	}
	author, ok := meta.Frontmatter["author"].(map[string]any)
	if !ok {
		t.Fatalf("author = %#v", meta.Frontmatter["author"])
	}
	if author["name"] != "Ada Lovelace" {
		t.Fatalf("author.name = %#v", author["name"])
	}
	social, ok := author["social"].(map[string]any)
	if !ok || social["x"] != "@ada" {
		t.Fatalf("author.social = %#v", author["social"])
	}

	want := map[string]bool{"ai-agent": true, "topic/crypto": true, "inline": true}
	if len(meta.Tags) != len(want) {
		t.Fatalf("tags len = %d, want %d: %#v", len(meta.Tags), len(want), meta.Tags)
	}
	for _, tag := range meta.Tags {
		if !want[tag] {
			t.Fatalf("unexpected tag %q in %#v", tag, meta.Tags)
		}
	}
}

func TestParseMetaFallbackForInvalidYAML(t *testing.T) {
	raw := "---\ntitle: 'unterminated\ntags: [fallback, ok]\n---\n"
	meta := ParseMeta(raw)
	if meta.Frontmatter["tags"] == nil {
		t.Fatalf("fallback frontmatter = %#v", meta.Frontmatter)
	}
	want := map[string]bool{"fallback": true, "ok": true}
	for _, tag := range meta.Tags {
		delete(want, tag)
	}
	if len(want) != 0 {
		t.Fatalf("missing fallback tags: %#v from %#v", want, meta.Tags)
	}
}

func TestExtractTitlePrefersFirstH1Heading(t *testing.T) {
	raw := "---\ntitle: Frontmatter Title\n---\n\n## Not title\n\n# Body Title\n\nBody"
	meta := ParseMeta(raw)
	if got := ExtractTitle(raw, meta); got != "Body Title" {
		t.Fatalf("ExtractTitle() = %q, want Body Title", got)
	}
}

func TestExtractTitleFallsBackToFrontmatterTitle(t *testing.T) {
	raw := "---\ntitle: Frontmatter Title\n---\n\nNo h1"
	meta := ParseMeta(raw)
	if got := ExtractTitle(raw, meta); got != "Frontmatter Title" {
		t.Fatalf("ExtractTitle() = %q, want Frontmatter Title", got)
	}
}

func TestExtractTitleSkipsFencedCodeBlocks(t *testing.T) {
	raw := "---\ntitle: Frontmatter Title\n---\n\n```go\n# Fake Code Title\n```\n\nNo h1"
	meta := ParseMeta(raw)
	if got := ExtractTitle(raw, meta); got != "Frontmatter Title" {
		t.Fatalf("ExtractTitle() = %q, want Frontmatter Title", got)
	}
}

func TestExtractTitleFindsHeadingAfterFencedCodeBlock(t *testing.T) {
	raw := "```\n# Fake Code Title\n```\n\n# Real Title\n"
	meta := ParseMeta(raw)
	if got := ExtractTitle(raw, meta); got != "Real Title" {
		t.Fatalf("ExtractTitle() = %q, want Real Title", got)
	}
}

func TestExtractTitleSkipsIndentedCodeBlocks(t *testing.T) {
	raw := "---\ntitle: Frontmatter Title\n---\n\n    # Fake Indented Code Title\n\t# Fake Tab Code Title\n"
	meta := ParseMeta(raw)
	if got := ExtractTitle(raw, meta); got != "Frontmatter Title" {
		t.Fatalf("ExtractTitle() = %q, want Frontmatter Title", got)
	}
}

func TestExtractTitleDoesNotUseSetextHeading(t *testing.T) {
	raw := "---\ntitle: Frontmatter Title\n---\n\nSetext Title\n============\n"
	meta := ParseMeta(raw)
	if got := ExtractTitle(raw, meta); got != "Frontmatter Title" {
		t.Fatalf("ExtractTitle() = %q, want Frontmatter Title because Setext H1 is unsupported", got)
	}
}

func TestExtractBodyRemovesLeadingFrontmatterOnly(t *testing.T) {
	raw := "---\ntitle: Hidden\n---\n\n# Visible\n\nBody text\n\n```\ncode search term\n```"
	body := ExtractBody(raw)
	if strings.Contains(body, "title: Hidden") {
		t.Fatalf("ExtractBody() included frontmatter: %q", body)
	}
	if !strings.Contains(body, "# Visible") || !strings.Contains(body, "code search term") {
		t.Fatalf("ExtractBody() = %q, want markdown body including code block", body)
	}
}

func TestScanWithReportPopulatesTitleAndBody(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "doc.md")
	raw := "---\ntitle: Frontmatter Title\ntags: [ai]\n---\n\n# Heading Title\n\nBody needle"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}

	got, _, err := ScanWithReport(root, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("docs len = %d, want 1", len(got))
	}
	if got[0].Title != "Heading Title" {
		t.Fatalf("Title = %q, want Heading Title", got[0].Title)
	}
	if !strings.Contains(got[0].Body, "Body needle") || strings.Contains(got[0].Body, "Frontmatter Title") {
		t.Fatalf("Body = %q, want body without frontmatter", got[0].Body)
	}
}

func TestScanRecordsModificationTime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.md")
	if err := os.WriteFile(path, []byte("# Note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(path, want, want); err != nil {
		t.Fatal(err)
	}
	list, err := Scan(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("scanned %d files, want 1", len(list))
	}
	if !list[0].ModTime.Equal(want) {
		t.Fatalf("ModTime = %v, want %v", list[0].ModTime, want)
	}
}
