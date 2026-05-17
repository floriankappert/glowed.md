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
	Haystack       string
	Snippet        string
}

type Meta struct {
	Frontmatter    map[string]any
	FrontmatterRaw string
	Tags           []string
}

func Scan(root string, excludeDirs []string, maxFileBytes int64) ([]Document, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(absRoot)

	exclude := map[string]bool{}
	for _, d := range excludeDirs {
		exclude[d] = true
	}
	ignore := loadGitIgnore(root)

	var out []Document
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
			if p != root && (exclude[name] || ignore.ignored(rel, true)) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignore.ignored(rel, false) {
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
			return nil
		}

		raw, _ := os.ReadFile(p)
		meta := ParseMeta(string(raw))
		out = append(out, Document{
			Abs:            p,
			Rel:            rel,
			Name:           name,
			Frontmatter:    meta.Frontmatter,
			FrontmatterRaw: meta.FrontmatterRaw,
			Tags:           meta.Tags,
			Haystack:       buildHaystack(rel, meta),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
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

func buildHaystack(rel string, meta Meta) string {
	parts := []string{rel, filepath.Base(rel), meta.FrontmatterRaw}
	for _, tag := range meta.Tags {
		parts = append(parts, "tag:"+tag)
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}
