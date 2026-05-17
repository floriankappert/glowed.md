package render

import (
	"sync"

	"github.com/charmbracelet/glamour"
)

type rendererKey struct {
	Style            string
	Width            int
	PreserveNewLines bool
}

type rendererEntry struct {
	mu       sync.Mutex
	renderer *glamour.TermRenderer
}

var rendererCache sync.Map

func Markdown(raw string, width int, style string, preserveNewLines bool) (string, error) {
	entry, err := cachedRenderer(width, style, preserveNewLines)
	if err != nil {
		return "", err
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.renderer.Render(raw)
}

func cachedRenderer(width int, style string, preserveNewLines bool) (*rendererEntry, error) {
	if width < 20 {
		width = 20
	}
	if style == "" {
		style = "dark"
	}
	key := rendererKey{Style: style, Width: width, PreserveNewLines: preserveNewLines}
	if entry, ok := rendererCache.Load(key); ok {
		return entry.(*rendererEntry), nil
	}
	opts := []glamour.TermRendererOption{
		glamour.WithStylePath(style),
		glamour.WithWordWrap(width),
	}
	if preserveNewLines {
		opts = append(opts, glamour.WithPreservedNewLines())
	}
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return nil, err
	}
	entry := &rendererEntry{renderer: r}
	actual, _ := rendererCache.LoadOrStore(key, entry)
	return actual.(*rendererEntry), nil
}
