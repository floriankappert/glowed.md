package docs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanHonorsRootGitIgnore(t *testing.T) {
	root := t.TempDir()
	gitignore := `# comments and blanks are ignored

drafts/
*.ignore.md
!keep.ignore.md
/root-only.md
nested/skip.md
`
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(gitignore), 0644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"README.md":             "# readme",
		"drafts/a.md":           "# draft",
		"notes/drop.ignore.md":  "# drop",
		"keep.ignore.md":        "# keep",
		"root-only.md":          "# root only",
		"nested/root-only.md":   "# nested root only",
		"nested/skip.md":        "# nested skip",
		"nested/deep/keep.md":   "# deep keep",
		"nested/deep/skip.txt":  "not markdown",
		"nested/deep/other.mdx": "not markdown",
	}
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	gotDocs, err := Scan(root, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, doc := range gotDocs {
		got = append(got, filepath.ToSlash(doc.Rel))
	}
	want := []string{"README.md", "keep.ignore.md", "nested/deep/keep.md", "nested/root-only.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan() rels = %#v, want %#v", got, want)
	}
}

func TestParseIgnorePattern(t *testing.T) {
	p, ok := parseIgnorePattern("!/keep.md")
	if !ok {
		t.Fatal("parseIgnorePattern() ok = false")
	}
	if !p.Negated || !p.Anchored || p.Pattern != "keep.md" || p.DirOnly || p.HasSlash {
		t.Fatalf("pattern = %+v", p)
	}

	p, ok = parseIgnorePattern("build/")
	if !ok {
		t.Fatal("parseIgnorePattern() ok = false")
	}
	if !p.DirOnly || p.Pattern != "build" {
		t.Fatalf("pattern = %+v", p)
	}
}
