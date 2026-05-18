package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkScanLargeDirectory(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".glowedignore"), []byte("ignored/\n*.tmp.md\n"), 0644); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		dir := filepath.Join(root, fmt.Sprintf("group-%02d", i%20))
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatal(err)
		}
		body := fmt.Sprintf("---\ntitle: Document %04d\ntags: [bench, group-%02d]\n---\n\n# Document %04d\n\nBody tag:item-%04d\n", i, i%20, i, i)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("doc-%04d.md", i)), []byte(body), 0644); err != nil {
			b.Fatal(err)
		}
	}
	ignoredDir := filepath.Join(root, "ignored")
	if err := os.MkdirAll(ignoredDir, 0755); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(ignoredDir, fmt.Sprintf("skip-%04d.md", i)), []byte("# ignored"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := Scan(root, 1024*1024)
		if err != nil {
			b.Fatal(err)
		}
		if len(got) != 1000 {
			b.Fatalf("Scan() returned %d docs, want 1000", len(got))
		}
	}
}
