package app

import (
	"os"
	"sort"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/khw1031/glowed/internal/docs"
)

// AppName is the project's display name. The command itself stays "glowed".
const AppName = "glowed.md"

// Version is the build version shown on the welcome screen. cmd/glowed sets it
// from the value stamped into the binary via ldflags.
var Version = "dev"

// maxRecentDocs is how many recently modified documents the welcome screen
// offers.
const maxRecentDocs = 7

// splashLogo is the lightbulb mark. All lines share one display width so the
// text column next to it stays aligned.
var splashLogo = []string{
	"▄███▄",
	"▜███▛",
	" ▐█▌ ",
}

// splashIndent is the left margin of the welcome block.
const splashIndent = 2

// splashGap separates the logo from the text column.
const splashGap = 3

// recentDocs returns the most recently modified documents, newest first.
func (m Model) recentDocs() []docs.Document {
	list := make([]docs.Document, len(m.Docs))
	copy(list, m.Docs)
	// Rel breaks ties so the order stays stable for equal timestamps.
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].ModTime.Equal(list[j].ModTime) {
			return list[i].Rel < list[j].Rel
		}
		return list[i].ModTime.After(list[j].ModTime)
	})
	if len(list) > maxRecentDocs {
		list = list[:maxRecentDocs]
	}
	return list
}

// handleWelcomeKey drives the welcome screen. It stays up until a document is
// opened, so only the recent-file list, ctrl+n and ctrl+c act on it.
func (m *Model) handleWelcomeKey(key string) {
	recent := m.recentDocs()
	switch key {
	case "up", "k", "shift+tab":
		m.SplashSelected = clamp(m.SplashSelected-1, 0, max(0, len(recent)-1))
	case "down", "j", "tab":
		m.SplashSelected = clamp(m.SplashSelected+1, 0, max(0, len(recent)-1))
	case "enter":
		m.openWelcomeSelection()
	}
}

// openWelcomeSelection opens the highlighted recent document and retires the
// welcome screen.
func (m *Model) openWelcomeSelection() {
	recent := m.recentDocs()
	if len(recent) == 0 {
		m.setStatus("no markdown documents — ctrl+n to create one", "warn")
		return
	}
	doc := recent[clamp(m.SplashSelected, 0, len(recent)-1)]
	if !m.selectPath(doc.Abs) {
		m.setStatus("cannot open "+doc.Rel, "error")
		return
	}
	m.Splash = false
	m.enterEditMode()
}

// renderSplash draws the welcome screen: the lightbulb next to the version and
// project root, with the recent documents underneath.
func (m Model) renderSplash() string {
	// The action menu owns the whole screen here, the way it owns the content
	// pane in the main layout.
	if m.Menu.Active {
		rows := make([]string, 0, m.Height)
		for i := 0; i < m.Height; i++ {
			row, _ := m.renderMenuRow(m.Width, m.Height, i)
			rows = append(rows, row)
		}
		return strings.Join(rows, "\n")
	}

	info := []string{
		styleTitle.Render(AppName) + " " + styleDim.Render(Version),
		styleDim.Render("Markdown browser/editor · Ghostty-first"),
		styleDim.Render(shortenHome(m.Root)),
	}
	logoWidth := 0
	for _, line := range splashLogo {
		logoWidth = max(logoWidth, runewidth.StringWidth(line))
	}

	block := make([]string, 0, len(splashLogo)+maxRecentDocs+4)
	for i, line := range splashLogo {
		row := strings.Repeat(" ", splashIndent) + styleYellow.Render(line)
		row += strings.Repeat(" ", logoWidth-runewidth.StringWidth(line)+splashGap)
		if i < len(info) {
			row += info[i]
		}
		block = append(block, row)
	}

	pad := strings.Repeat(" ", splashIndent)
	recent := m.recentDocs()
	block = append(block, "")
	if len(recent) == 0 {
		block = append(block, pad+styleDim.Render("no markdown documents in this project"))
		block = append(block, "", pad+styleDim.Render("ctrl+n new file · ctrl+c quit"))
		return m.frameSplash(block)
	}

	block = append(block, pad+styleHeading.Render("Recent files"), "")
	selected := clamp(m.SplashSelected, 0, len(recent)-1)
	for i, doc := range recent {
		label := " " + doc.Rel + " "
		if i == selected {
			block = append(block, pad+styleReverse.Render(label))
			continue
		}
		block = append(block, pad+label)
	}
	block = append(block, "", pad+styleDim.Render("↑↓ select · enter open · ctrl+n new file · ctrl+c quit"))
	return m.frameSplash(block)
}

// frameSplash centers the block vertically and pads the frame to the terminal
// size, so the welcome screen owns exactly as many rows as the main layout.
func (m Model) frameSplash(block []string) string {
	top := max(0, (m.Height-len(block))/2)
	rows := make([]string, 0, m.Height)
	for i := 0; i < m.Height; i++ {
		line := ""
		if i >= top && i-top < len(block) {
			line = block[i-top]
		}
		rows = append(rows, fitANSI(line, m.Width))
	}
	return strings.Join(rows, "\n")
}

// shortenHome replaces the home directory prefix with "~".
func shortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}
