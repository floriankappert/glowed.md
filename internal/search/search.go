package search

import (
	"strings"

	"github.com/khw1031/glowed/internal/docs"
)

func Filter(input []docs.Document, query string) []docs.Document {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(tokens) == 0 {
		out := make([]docs.Document, len(input))
		copy(out, input)
		for i := range out {
			out[i].Snippet = ""
		}
		return out
	}

	out := make([]docs.Document, 0, len(input))
	for _, doc := range input {
		matched := true
		for _, token := range tokens {
			if !matches(doc, token) {
				matched = false
				break
			}
		}
		if matched {
			doc.Snippet = snippet(doc, tokens)
			out = append(out, doc)
		}
	}
	return out
}

func matches(doc docs.Document, token string) bool {
	if strings.HasPrefix(token, "tag:") {
		want := strings.TrimPrefix(token, "tag:")
		for _, tag := range doc.Tags {
			if tag == want || strings.Contains(tag, want) {
				return true
			}
		}
		return false
	}
	return strings.Contains(doc.Haystack, token)
}

func snippet(doc docs.Document, tokens []string) string {
	for _, token := range tokens {
		if strings.HasPrefix(token, "tag:") {
			want := strings.TrimPrefix(token, "tag:")
			for _, tag := range doc.Tags {
				if tag == want || strings.Contains(tag, want) {
					return "tag:" + tag
				}
			}
			continue
		}
		if strings.Contains(strings.ToLower(doc.Rel), token) {
			return trimSnippet(doc.Rel, 80)
		}
		if strings.Contains(strings.ToLower(doc.Name), token) {
			return trimSnippet(doc.Name, 80)
		}
		for _, line := range strings.Split(doc.FrontmatterRaw, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && strings.Contains(strings.ToLower(line), token) {
				return trimSnippet(line, 80)
			}
		}
	}
	return ""
}

func trimSnippet(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}
