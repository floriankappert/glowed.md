package docs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScanUsesBuiltInDefaultIgnores(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"README.md":              "# keep",
		".git/hidden.md":         "# git",
		"node_modules/pkg/a.md":  "# dependency",
		"build/root.md":          "# root build",
		"notes/build/visible.md": "# nested build",
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

	gotDocs, report, err := ScanWithReport(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, doc := range gotDocs {
		got = append(got, filepath.ToSlash(doc.Rel))
	}
	want := []string{"README.md", "notes/build/visible.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan() rels = %#v, want %#v", got, want)
	}
	gotReasons := map[string]string{}
	for _, excluded := range report.Excluded {
		gotReasons[filepath.ToSlash(excluded.Rel)] = excluded.Reason
	}
	for _, rel := range []string{".git", "build", "node_modules"} {
		if gotReasons[rel] != IgnoreReasonDefault {
			t.Fatalf("excluded[%q] reason = %q, want %q; all reasons %#v", rel, gotReasons[rel], IgnoreReasonDefault, gotReasons)
		}
	}
}

func TestGlowedIgnoreCanOverrideBuiltInDefaultIgnores(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".glowedignore"), []byte("!/build/\n!vendor/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"build/root.md":    "# root build",
		"vendor/readme.md": "# vendor docs",
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

	gotDocs, err := Scan(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, doc := range gotDocs {
		got = append(got, filepath.ToSlash(doc.Rel))
	}
	want := []string{"build/root.md", "vendor/readme.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan() rels = %#v, want %#v", got, want)
	}
}

func TestInitGlowedIgnoreCreatesTemplateWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	path, err := InitGlowedIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "built-in default ignores") || !strings.Contains(string(b), "!/build/") {
		t.Fatalf("generated .glowedignore template = %q", string(b))
	}
	if _, err := InitGlowedIgnore(root); !os.IsExist(err) {
		t.Fatalf("second InitGlowedIgnore() err = %v, want exists", err)
	}
}

func TestScanHonorsRootGlowedIgnore(t *testing.T) {
	root := t.TempDir()
	glowedignore := `# comments and blanks are ignored

drafts/
*.ignore.md
!keep.ignore.md
/root-only.md
nested/skip.md
`
	if err := os.WriteFile(filepath.Join(root, ".glowedignore"), []byte(glowedignore), 0644); err != nil {
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

	gotDocs, err := Scan(root, 0)
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

func TestScanWithReportListsExcludedPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".glowedignore"), []byte("ignored/\nsecret.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"keep.md":            "# keep",
		"ignored/a.md":       "# ignored dir",
		"secret.md":          "# ignored file",
		"large.md":           strings.Repeat("x", 16),
		"ignored/skip.txt":   "not markdown",
		"unrelated/file.txt": "not markdown",
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

	gotDocs, report, err := ScanWithReport(root, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotDocs) != 1 || filepath.ToSlash(gotDocs[0].Rel) != "keep.md" {
		t.Fatalf("ScanWithReport() docs = %#v, want keep.md", gotDocs)
	}
	got := map[string]string{}
	for _, excluded := range report.Excluded {
		got[filepath.ToSlash(excluded.Rel)] = excluded.Reason
	}
	want := map[string]string{"ignored": IgnoreReasonProject, "large.md": "maxFileBytes", "secret.md": IgnoreReasonProject}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("excluded = %#v, want %#v", got, want)
	}
}

func TestScanIgnoresGitIgnoreWithoutGlowedIgnore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "ignored", "visible.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# visible"), 0644); err != nil {
		t.Fatal(err)
	}

	gotDocs, err := Scan(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotDocs) != 1 || filepath.ToSlash(gotDocs[0].Rel) != "ignored/visible.md" {
		t.Fatalf("Scan() docs = %#v, want ignored/visible.md", gotDocs)
	}
}

func TestGlowedIgnoreRootBuildDoesNotHideNestedBuild(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".glowedignore"), []byte("/build/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"build/root.md":       "# root build",
		"notes/build/note.md": "# nested build",
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

	gotDocs, err := Scan(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotDocs) != 1 || filepath.ToSlash(gotDocs[0].Rel) != "notes/build/note.md" {
		t.Fatalf("Scan() docs = %#v, want notes/build/note.md", gotDocs)
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
