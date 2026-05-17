package docs

import "testing"

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
