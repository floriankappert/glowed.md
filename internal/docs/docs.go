package docs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Document struct {
	Abs            string
	Rel            string
	Name           string
	Frontmatter    map[string]any
	FrontmatterRaw string
	Tags           []string
	Title          string
	Body           string
	Snippet        string
}

type Meta struct {
	Frontmatter    map[string]any
	FrontmatterRaw string
	Tags           []string
}

type ScanReport struct {
	Excluded []ExcludedPath
}

type ExcludedPath struct {
	Rel    string
	IsDir  bool
	Reason string
}

func Scan(root string, maxFileBytes int64) ([]Document, error) {
	out, _, err := ScanWithReport(root, maxFileBytes)
	return out, err
}

func ScanWithReport(root string, maxFileBytes int64) ([]Document, ScanReport, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, ScanReport{}, err
	}
	root = filepath.Clean(absRoot)

	ignore := loadGlowedIgnore(root)

	var out []Document
	report := ScanReport{}
	err = filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := entry.Name()
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil || rel == "." {
			rel = name
		}
		if entry.IsDir() {
			if ignored, reason := ignore.ignoredReason(rel, true); p != root && ignored {
				report.Excluded = append(report.Excluded, ExcludedPath{Rel: cleanSlashRel(rel), IsDir: true, Reason: reason})
				return filepath.SkipDir
			}
			return nil
		}
		if ignored, reason := ignore.ignoredReason(rel, false); ignored {
			if strings.ToLower(filepath.Ext(name)) == ".md" {
				report.Excluded = append(report.Excluded, ExcludedPath{Rel: cleanSlashRel(rel), Reason: reason})
			}
			return nil
		}
		if !entry.Type().IsRegular() || strings.ToLower(filepath.Ext(name)) != ".md" {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if maxFileBytes > 0 && info.Size() > maxFileBytes {
			report.Excluded = append(report.Excluded, ExcludedPath{Rel: cleanSlashRel(rel), Reason: "maxFileBytes"})
			return nil
		}

		raw, _ := os.ReadFile(p)
		rawText := string(raw)
		meta := ParseMeta(rawText)
		title := ExtractTitle(rawText, meta)
		body := ExtractBody(rawText)
		out = append(out, Document{
			Abs:            p,
			Rel:            rel,
			Name:           name,
			Frontmatter:    meta.Frontmatter,
			FrontmatterRaw: meta.FrontmatterRaw,
			Tags:           meta.Tags,
			Title:          title,
			Body:           body,
		})
		return nil
	})
	if err != nil {
		return nil, ScanReport{}, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	sort.Slice(report.Excluded, func(i, j int) bool { return report.Excluded[i].Rel < report.Excluded[j].Rel })
	return out, report, nil
}

func ParseMeta(raw string) Meta {
	meta := Meta{Frontmatter: map[string]any{}}
	tagSet := map[string]bool{}

	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				end = i
				break
			}
		}
		if end > 0 {
			meta.FrontmatterRaw = strings.Join(lines[1:end], "\n")
			meta.Frontmatter = parseFrontmatter(meta.FrontmatterRaw)
			for key, value := range meta.Frontmatter {
				if strings.EqualFold(key, "tag") || strings.EqualFold(key, "tags") {
					for _, tag := range tagsFromValue(value) {
						tagSet[tag] = true
					}
				}
			}
		}
	}

	re := regexp.MustCompile(`(?:^|\s)tag:([A-Za-z0-9_.-]+)`)
	for _, m := range re.FindAllStringSubmatch(raw, -1) {
		if len(m) > 1 {
			tagSet[strings.ToLower(m[1])] = true
		}
	}

	for tag := range tagSet {
		meta.Tags = append(meta.Tags, tag)
	}
	sort.Strings(meta.Tags)
	return meta
}

func parseFrontmatter(raw string) map[string]any {
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(raw), &parsed); err == nil && parsed != nil {
		return normalizeFrontmatterMap(parsed)
	}
	return parseFrontmatterFallback(raw)
}

func parseFrontmatterFallback(raw string) map[string]any {
	out := map[string]any{}
	for _, line := range strings.Split(raw, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, " \t") {
			continue
		}
		out[key] = parseFrontmatterValue(strings.TrimSpace(val))
	}
	return out
}

func normalizeFrontmatterMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = normalizeFrontmatterValue(value)
	}
	return out
}

func normalizeFrontmatterValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return normalizeFrontmatterMap(v)
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[fmt.Sprint(key)] = normalizeFrontmatterValue(value)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeFrontmatterValue(item)
		}
		return out
	default:
		return v
	}
}

func parseFrontmatterValue(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
		if body == "" {
			return []string{}
		}
		parts := strings.Split(body, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = stripQuotes(strings.TrimSpace(p))
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return stripQuotes(value)
}

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func tagsFromValue(v any) []string {
	var raw []string
	switch vv := v.(type) {
	case []string:
		raw = vv
	case []any:
		for _, item := range vv {
			raw = append(raw, fmt.Sprint(item))
		}
	case string:
		raw = strings.FieldsFunc(vv, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	default:
		if v != nil {
			raw = []string{fmt.Sprint(v)}
		}
	}
	out := make([]string, 0, len(raw))
	for _, tag := range raw {
		tag = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(tag, "tag:"), "#"))
		if tag != "" {
			out = append(out, strings.ToLower(tag))
		}
	}
	return out
}

func ExtractTitle(raw string, meta Meta) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	inFrontmatter := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
	inFence := false
	fenceMarker := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inFrontmatter {
			if i > 0 && trimmed == "---" {
				inFrontmatter = false
			}
			continue
		}
		// Title extraction intentionally supports ATX H1 headings only. Indented
		// code blocks and fenced code blocks are skipped; Setext headings are not
		// treated as titles in this MVP implementation.
		if isIndentedCodeLine(line) {
			continue
		}
		if marker, ok := fenceStart(trimmed); ok {
			if inFence && marker == fenceMarker {
				inFence = false
				fenceMarker = ""
			} else if !inFence {
				inFence = true
				fenceMarker = marker
			}
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			return cleanHeadingText(strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")))
		}
	}
	if title, ok := meta.Frontmatter["title"]; ok {
		return strings.TrimSpace(fmt.Sprint(title))
	}
	return ""
}

func fenceStart(trimmed string) (string, bool) {
	if strings.HasPrefix(trimmed, "```") {
		return "```", true
	}
	if strings.HasPrefix(trimmed, "~~~") {
		return "~~~", true
	}
	return "", false
}

func isIndentedCodeLine(line string) bool {
	return strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ")
}

func cleanHeadingText(s string) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), "#"))
}

func ExtractBody(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return normalized
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return normalized
}
