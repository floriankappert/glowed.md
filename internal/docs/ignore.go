package docs

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

type IgnoreMatcher struct {
	rules ignoreRules
}

type ignoreRules struct {
	patterns []ignorePattern
}

type ignorePattern struct {
	Pattern  string
	Negated  bool
	DirOnly  bool
	Anchored bool
	HasSlash bool
}

func LoadIgnoreMatcher(root string) IgnoreMatcher {
	return IgnoreMatcher{rules: loadGlowedIgnore(root)}
}

func (m IgnoreMatcher) Ignored(rel string, isDir bool) bool {
	return m.rules.ignored(rel, isDir)
}

func loadGlowedIgnore(root string) ignoreRules {
	b, err := os.ReadFile(filepath.Join(root, ".glowedignore"))
	if err != nil {
		return ignoreRules{}
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	patterns := make([]ignorePattern, 0, len(lines))
	for _, line := range lines {
		if p, ok := parseIgnorePattern(line); ok {
			patterns = append(patterns, p)
		}
	}
	return ignoreRules{patterns: patterns}
}

func parseIgnorePattern(line string) (ignorePattern, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ignorePattern{}, false
	}

	p := ignorePattern{}
	if strings.HasPrefix(line, "!") {
		p.Negated = true
		line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
		if line == "" {
			return ignorePattern{}, false
		}
	}
	if strings.HasPrefix(line, "/") {
		p.Anchored = true
		line = strings.TrimPrefix(line, "/")
	}
	if strings.HasSuffix(line, "/") {
		p.DirOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	line = path.Clean(filepath.ToSlash(line))
	if line == "." || line == "" {
		return ignorePattern{}, false
	}
	p.Pattern = line
	p.HasSlash = strings.Contains(line, "/")
	return p, true
}

func (g ignoreRules) ignored(rel string, isDir bool) bool {
	rel = cleanSlashRel(rel)
	if rel == "." || rel == "" {
		return false
	}
	ignored := false
	for _, p := range g.patterns {
		if p.matches(rel, isDir) {
			ignored = !p.Negated
		}
	}
	return ignored
}

func (p ignorePattern) matches(rel string, isDir bool) bool {
	if p.DirOnly {
		return p.matchesDir(rel, isDir)
	}
	if !p.HasSlash {
		if p.Anchored {
			return matchPathPattern(p.Pattern, rel)
		}
		return matchBasePattern(p.Pattern, path.Base(rel))
	}
	return matchPathPattern(p.Pattern, rel)
}

func (p ignorePattern) matchesDir(rel string, isDir bool) bool {
	if !p.HasSlash {
		if p.Anchored {
			return rel == p.Pattern || strings.HasPrefix(rel, p.Pattern+"/")
		}
		parts := strings.Split(rel, "/")
		limit := len(parts)
		if !isDir && limit > 0 {
			limit--
		}
		for i := 0; i < limit; i++ {
			if matchBasePattern(p.Pattern, parts[i]) {
				return true
			}
		}
		return false
	}
	if p.Anchored {
		return rel == p.Pattern || strings.HasPrefix(rel, p.Pattern+"/")
	}
	if rel == p.Pattern || strings.HasPrefix(rel, p.Pattern+"/") {
		return true
	}
	return strings.Contains(rel, "/"+p.Pattern+"/") || strings.HasSuffix(rel, "/"+p.Pattern)
}

func matchBasePattern(pattern, name string) bool {
	ok, err := path.Match(pattern, name)
	return err == nil && ok
}

func matchPathPattern(pattern, rel string) bool {
	ok, err := path.Match(pattern, rel)
	return err == nil && ok
}

func cleanSlashRel(rel string) string {
	rel = filepath.ToSlash(filepath.Clean(rel))
	rel = strings.TrimPrefix(rel, "./")
	return rel
}
