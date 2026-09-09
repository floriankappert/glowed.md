package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func viewRows(t *testing.T, m Model) []string {
	t.Helper()
	rows := strings.Split(m.View(), "\n")
	for i, row := range rows {
		rows[i] = stripANSI(row)
	}
	// The first row carries the cursor-shape escape ahead of the header.
	return rows
}

func layoutModel(t *testing.T, w, h int) Model {
	t.Helper()
	m, _ := projectModel(t)
	m.Width, m.Height = w, h
	m.SidebarVisible = true
	m.ensureSidebarState()
	m.rebuildSidebarRows()
	return m
}

func TestLayoutRowsFillTheTerminalHeight(t *testing.T) {
	m := layoutModel(t, 90, 20)
	if got := len(viewRows(t, m)); got != m.Height {
		t.Fatalf("view has %d rows, want %d", got, m.Height)
	}
}

func TestLayoutRowsFillTheTerminalWidth(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 18}, {120, 30}, {60, 12}} {
		m := layoutModel(t, size.w, size.h)
		for i, row := range viewRows(t, m) {
			if w := xansi.StringWidth(row); w != m.Width {
				t.Fatalf("%dx%d: row %d has width %d, want %d (%q)", size.w, size.h, i, w, m.Width, row)
			}
		}
	}
}

func TestPanesAreFullyBordered(t *testing.T) {
	m := layoutModel(t, 90, 16)
	rows := viewRows(t, m)
	top := rows[m.contentTop()-1]
	bottom := rows[m.paneBottomRow()]

	if !strings.HasPrefix(top, "┌") || !strings.HasSuffix(top, "┐") {
		t.Fatalf("top border = %q", top)
	}
	if !strings.HasPrefix(bottom, "└") || !strings.HasSuffix(bottom, "┘") {
		t.Fatalf("bottom border = %q", bottom)
	}
	if !strings.Contains(top, "┬") || !strings.Contains(bottom, "┴") {
		t.Fatal("panes do not share a junction")
	}
	for row := m.contentTop(); row < m.paneBottomRow(); row++ {
		line := rows[row]
		if !strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│") {
			t.Fatalf("body row %d is not enclosed: %q", row, line)
		}
	}
}

func TestPaneCaptionIsIndented(t *testing.T) {
	m := layoutModel(t, 90, 16)
	top := viewRows(t, m)[m.contentTop()-1]
	if !strings.HasPrefix(top, "┌"+strings.Repeat("─", paneCaptionIndent)+" files ") {
		t.Fatalf("caption not indented by %d: %q", paneCaptionIndent, top)
	}
}

func TestActivePaneIsHighlighted(t *testing.T) {
	// Without a TTY lipgloss falls back to the ASCII profile and drops color.
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := layoutModel(t, 90, 16)

	m.Focus = FocusSidebar
	sidebarActive := m.renderPaneBorder(true)
	m.Focus = FocusEditor
	editorActive := m.renderPaneBorder(true)

	if sidebarActive == editorActive {
		t.Fatal("the top border looks the same no matter which pane has focus")
	}
	if stripANSI(sidebarActive) != stripANSI(editorActive) {
		t.Fatal("focus changed the border text, it should only change the styling")
	}
}

func TestPathMovedToToolbarAbovetheFooter(t *testing.T) {
	m := layoutModel(t, 90, 16)
	rows := viewRows(t, m)

	toolbar := rows[m.toolbarRow()]
	if !strings.Contains(toolbar, "Path: ") {
		t.Fatalf("toolbar %q does not carry the path", toolbar)
	}
	if m.toolbarRow() != len(rows)-2 {
		t.Fatalf("toolbar is at row %d, want directly above the footer at %d", m.toolbarRow(), len(rows)-2)
	}
	for row := m.contentTop(); row < m.paneBottomRow(); row++ {
		if strings.Contains(rows[row], "Path: ") {
			t.Fatalf("row %d still shows the path inside a pane: %q", row, rows[row])
		}
	}
	if strings.Contains(rows[0], m.Root) {
		t.Fatalf("header still repeats the path: %q", rows[0])
	}
}

func TestFirstBufferLineIsTheFirstPaneRow(t *testing.T) {
	m := layoutModel(t, 90, 16)
	m.Editor.Lines = []string{"first line", "second line"}
	m.Editor.ScrollY = 0
	rows := viewRows(t, m)
	if !strings.Contains(rows[m.contentTop()], "first line") {
		t.Fatalf("first pane row = %q, want the first buffer line", rows[m.contentTop()])
	}
}

func TestMouseMapsToTheClickedBufferLine(t *testing.T) {
	m := layoutModel(t, 90, 16)
	m.Editor.Lines = []string{"aaa", "bbb", "ccc", "ddd"}
	m.Editor.ScrollY = 0
	for want := 0; want < 4; want++ {
		p, ok := m.editorPointFromMouse(m.rightTextStartX(), m.contentTop()+want, false)
		if !ok {
			t.Fatalf("click on row %d was rejected", want)
		}
		if p.Line != want {
			t.Fatalf("click on row %d mapped to line %d", want, p.Line)
		}
		if p.Col != 0 {
			t.Fatalf("click on the first text column mapped to col %d", p.Col)
		}
	}
}

func TestMouseRejectsClicksOnThePaneBorder(t *testing.T) {
	m := layoutModel(t, 90, 16)
	m.Editor.Lines = []string{"aaa"}
	if _, ok := m.editorPointFromMouse(m.rightTextStartX()-1, m.contentTop(), false); ok {
		t.Fatal("a click on the border was treated as a text position")
	}
}
