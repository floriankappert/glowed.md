package search

import (
	"strings"
	"testing"

	"github.com/khw1031/glowed/internal/docs"
)

func TestFilterTagAndFrontmatter(t *testing.T) {
	input := []docs.Document{
		{Rel: "README.md", Name: "README.md", Tags: []string{"intro"}, FrontmatterRaw: "title: hello"},
		{Rel: "notes/ai.md", Name: "ai.md", Tags: []string{"ai"}, FrontmatterRaw: "title: agent"},
	}
	if got := Filter(input, "tag:ai"); len(got) != 1 || got[0].Rel != "notes/ai.md" || got[0].Snippet != "tag:ai" {
		t.Fatalf("tag filter = %#v", got)
	}
	if got := Filter(input, "hello"); len(got) != 1 || got[0].Rel != "README.md" || got[0].Snippet != "frontmatter: title: hello" {
		t.Fatalf("frontmatter filter = %#v", got)
	}
}

func TestFilterUsesANDAcrossPathFrontmatterAndTags(t *testing.T) {
	input := []docs.Document{
		{Rel: "notes/ai.md", Name: "ai.md", Tags: []string{"ai"}, FrontmatterRaw: "title: Agent Notes"},
		{Rel: "notes/cooking.md", Name: "cooking.md", Tags: []string{"home"}, FrontmatterRaw: "title: Agent Recipes"},
	}
	if got := Filter(input, "notes tag:ai agent"); len(got) != 1 || got[0].Rel != "notes/ai.md" {
		t.Fatalf("AND path/tag/frontmatter filter = %#v", got)
	}
	if got := Filter(input, "notes tag:missing agent"); len(got) != 0 {
		t.Fatalf("missing tag AND filter = %#v, want empty", got)
	}
}

func TestFilterSearchesBodyAndShowsSourceAwareSnippet(t *testing.T) {
	input := []docs.Document{
		{Rel: "notes/a.md", Name: "a.md", Body: "This body contains vector search details."},
	}
	got := Filter(input, "vector")
	if len(got) != 1 || got[0].Rel != "notes/a.md" {
		t.Fatalf("body filter = %#v", got)
	}
	if !strings.HasPrefix(got[0].Snippet, "body: ") || !strings.Contains(got[0].Snippet, "vector search") {
		t.Fatalf("body snippet = %q", got[0].Snippet)
	}
}

func TestFilterRanksTitleBeforeBodyFrontmatterPathAndTag(t *testing.T) {
	input := []docs.Document{
		{Rel: "04-tag.md", Name: "04-tag.md", Tags: []string{"agent"}},
		{Rel: "03-agent-path.md", Name: "03-agent-path.md"},
		{Rel: "02-frontmatter.md", Name: "02-frontmatter.md", FrontmatterRaw: "summary: agent"},
		{Rel: "01-body.md", Name: "01-body.md", Body: "agent appears in body"},
		{Rel: "00-title.md", Name: "00-title.md", Title: "Agent Title"},
	}
	got := Filter(input, "agent")
	want := []string{"00-title.md", "01-body.md", "02-frontmatter.md", "03-agent-path.md", "04-tag.md"}
	if len(got) != len(want) {
		t.Fatalf("ranked len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i, rel := range want {
		if got[i].Rel != rel {
			t.Fatalf("ranked[%d] = %q, want %q; got %#v", i, got[i].Rel, rel, got)
		}
	}
	if got[0].Snippet != "title: Agent Title" {
		t.Fatalf("title snippet = %q", got[0].Snippet)
	}
}

func TestFilterRanksBodyBeforeFrontmatter(t *testing.T) {
	input := []docs.Document{
		{Rel: "frontmatter.md", Name: "frontmatter.md", FrontmatterRaw: "summary: needle"},
		{Rel: "body.md", Name: "body.md", Body: "needle in body"},
	}
	got := Filter(input, "needle")
	if len(got) != 2 || got[0].Rel != "body.md" || got[1].Rel != "frontmatter.md" {
		t.Fatalf("body/frontmatter rank = %#v", got)
	}
}

func TestFilterPreservesPathFilenameSearch(t *testing.T) {
	input := []docs.Document{{Rel: "notes/project-plan.md", Name: "project-plan.md"}}
	got := Filter(input, "project")
	if len(got) != 1 || got[0].Snippet != "path: notes/project-plan.md" {
		t.Fatalf("path filter = %#v", got)
	}
}

func TestTrimAroundTokenKeepsTokenNearEnd(t *testing.T) {
	got := trimAroundToken("0123456789 endpoint", "endpoint", 12)
	if !strings.Contains(got, "endpoint") {
		t.Fatalf("trimAroundToken() = %q, want token preserved", got)
	}
	if len([]rune(got)) > 12 {
		t.Fatalf("trimAroundToken() len = %d, want <= 12: %q", len([]rune(got)), got)
	}
}

func TestTrimAroundTokenRecomputesSuffixAfterBudget(t *testing.T) {
	got := trimAroundToken("prefix words final", "final", 10)
	if !strings.Contains(got, "final") {
		t.Fatalf("trimAroundToken() = %q, want final token", got)
	}
	if strings.HasSuffix(got, "…") {
		t.Fatalf("trimAroundToken() = %q, should not show suffix when window reaches end", got)
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
