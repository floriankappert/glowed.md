package search

import (
	"testing"

	"github.com/khw1031/glowed/internal/docs"
)

func TestFilterTagAndHaystack(t *testing.T) {
	input := []docs.Document{
		{Rel: "README.md", Name: "README.md", Tags: []string{"intro"}, FrontmatterRaw: "title: hello", Haystack: "readme.md\ntitle: hello\ntag:intro"},
		{Rel: "notes/ai.md", Name: "ai.md", Tags: []string{"ai"}, FrontmatterRaw: "title: agent", Haystack: "notes/ai.md\ntitle: agent\ntag:ai"},
	}
	if got := Filter(input, "tag:ai"); len(got) != 1 || got[0].Rel != "notes/ai.md" || got[0].Snippet != "tag:ai" {
		t.Fatalf("tag filter = %#v", got)
	}
	if got := Filter(input, "hello"); len(got) != 1 || got[0].Rel != "README.md" || got[0].Snippet != "title: hello" {
		t.Fatalf("haystack filter = %#v", got)
	}
}

func TestFilterClearsSnippetForEmptyQuery(t *testing.T) {
	input := []docs.Document{{Rel: "README.md", Snippet: "stale"}}
	got := Filter(input, "")
	if len(got) != 1 || got[0].Snippet != "" {
		t.Fatalf("empty filter = %#v", got)
	}
	if input[0].Snippet != "stale" {
		t.Fatalf("Filter mutated input = %#v", input)
	}
}
