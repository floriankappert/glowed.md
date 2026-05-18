package search

import (
	"sort"
	"strings"

	"github.com/khw1031/glowed/internal/docs"
)

const (
	rankTitle = iota
	rankBody
	rankFrontmatter
	rankPath
	rankTag
	rankNoMatch = 99
)

type matchInfo struct {
	Rank    int
	Source  string
	Snippet string
}

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

	type rankedDoc struct {
		Doc   docs.Document
		Rank  int
		Index int
	}
	matches := make([]rankedDoc, 0, len(input))
	for i, doc := range input {
		matched := true
		best := matchInfo{Rank: rankNoMatch}
		for _, token := range tokens {
			info := bestMatch(doc, token)
			if info.Rank == rankNoMatch {
				matched = false
				break
			}
			if info.Rank < best.Rank {
				best = info
			}
		}
		if matched {
			doc.Snippet = best.Snippet
			matches = append(matches, rankedDoc{Doc: doc, Rank: best.Rank, Index: i})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Rank != matches[j].Rank {
			return matches[i].Rank < matches[j].Rank
		}
		return matches[i].Index < matches[j].Index
	})
	out := make([]docs.Document, len(matches))
	for i, match := range matches {
		out[i] = match.Doc
	}
	return out
}

func bestMatch(doc docs.Document, token string) matchInfo {
	if strings.HasPrefix(token, "tag:") {
		return tagMatch(doc, strings.TrimPrefix(token, "tag:"))
	}
	if containsFold(doc.Title, token) {
		return matchInfo{Rank: rankTitle, Source: "title", Snippet: "title: " + trimSnippet(doc.Title, 80)}
	}
	if containsFold(doc.Body, token) {
		return matchInfo{Rank: rankBody, Source: "body", Snippet: "body: " + contextualSnippet(doc.Body, token, 80)}
	}
	if snippet := frontmatterSnippet(doc.FrontmatterRaw, token); snippet != "" {
		return matchInfo{Rank: rankFrontmatter, Source: "frontmatter", Snippet: "frontmatter: " + snippet}
	}
	if containsFold(doc.Rel, token) || containsFold(doc.Name, token) {
		return matchInfo{Rank: rankPath, Source: "path", Snippet: "path: " + trimSnippet(doc.Rel, 80)}
	}
	if info := tagMatch(doc, token); info.Rank != rankNoMatch {
		return info
	}
	return matchInfo{Rank: rankNoMatch}
}

func tagMatch(doc docs.Document, want string) matchInfo {
	if want == "" {
		return matchInfo{Rank: rankNoMatch}
	}
	for _, tag := range doc.Tags {
		if tag == want || strings.Contains(tag, want) {
			return matchInfo{Rank: rankTag, Source: "tag", Snippet: "tag:" + tag}
		}
	}
	return matchInfo{Rank: rankNoMatch}
}

func frontmatterSnippet(raw string, token string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && containsFold(line, token) {
			return trimSnippet(line, 80)
		}
	}
	return ""
}

func contextualSnippet(s string, token string, maxRunes int) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && containsFold(line, token) {
			return trimAroundToken(line, token, maxRunes)
		}
	}
	return trimAroundToken(s, token, maxRunes)
}

func trimAroundToken(s string, token string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 1 {
		return "…"
	}
	lowerRunes := []rune(strings.ToLower(s))
	tokenRunes := []rune(token)
	idx := runeIndex(lowerRunes, tokenRunes)
	if idx < 0 {
		return string(runes[:maxRunes-1]) + "…"
	}
	tokenEnd := idx + len(tokenRunes)
	prefix := idx > 0
	suffix := tokenEnd < len(runes)
	budget := maxRunes
	if prefix {
		budget--
	}
	if suffix {
		budget--
	}
	if budget <= 0 {
		return "…"
	}
	if len(tokenRunes) > budget {
		return ellipsis(prefix) + string(runes[idx:min(len(runes), idx+budget)]) + ellipsis(suffix)
	}

	start := idx - max(0, (budget-len(tokenRunes))/3)
	start = clampInt(start, 0, idx)
	if start+budget < tokenEnd {
		start = tokenEnd - budget
	}
	if start+budget > len(runes) {
		start = max(0, len(runes)-budget)
	}
	end := min(len(runes), start+budget)
	prefix = start > 0
	suffix = end < len(runes)
	if prefix && suffix && end-start > maxRunes-2 {
		end = start + maxRunes - 2
	} else if prefix && !suffix && end-start > maxRunes-1 {
		start = end - (maxRunes - 1)
	} else if !prefix && suffix && end-start > maxRunes-1 {
		end = start + maxRunes - 1
	}
	return ellipsis(prefix) + string(runes[start:end]) + ellipsis(suffix)
}

func ellipsis(show bool) string {
	if show {
		return "…"
	}
	return ""
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func runeIndex(haystack []rune, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		matched := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

func containsFold(s string, token string) bool {
	return strings.Contains(strings.ToLower(s), token)
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
