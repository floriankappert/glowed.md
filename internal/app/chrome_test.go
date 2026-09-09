package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestSidebarIsVisibleOnLaunch(t *testing.T) {
	m, _ := projectModel(t)
	if !m.SidebarVisible {
		t.Fatal("SidebarVisible = false, want true on launch")
	}
}

// The search row sat permanently below the header, spending a content row on a
// placeholder. It now appears only while the search is in use.
func TestSearchRowHiddenWhileInactive(t *testing.T) {
	m := layoutModel(t, 90, 16)
	rows := viewRows(t, m)
	if m.searchVisible() {
		t.Fatal("searchVisible = true for an unfocused, empty search")
	}
	if strings.Contains(strings.Join(rows, "\n"), "tag:foo") {
		t.Fatalf("the frame still shows the search placeholder:\n%s", strings.Join(rows, "\n"))
	}
	if m.contentTop() != headerRows+1 {
		t.Fatalf("contentTop = %d, want %d with the search row hidden", m.contentTop(), headerRows+1)
	}
}

func TestSearchRowShownWhileFocused(t *testing.T) {
	m := layoutModel(t, 90, 16)
	m.Focus = FocusSearch
	if !m.searchVisible() {
		t.Fatal("searchVisible = false while the search has focus")
	}
	if m.contentTop() != headerRows+2 {
		t.Fatalf("contentTop = %d, want %d with the search row shown", m.contentTop(), headerRows+2)
	}
	if !strings.Contains(viewRows(t, m)[m.searchRow()], "/") {
		t.Fatal("search row missing while focused")
	}
}

func TestSearchRowShownWhileQueryIsSet(t *testing.T) {
	m := layoutModel(t, 90, 16)
	m.Query = "tag:note"
	if !m.searchVisible() {
		t.Fatal("searchVisible = false with a non-empty query")
	}
	if !strings.Contains(viewRows(t, m)[m.searchRow()], "tag:note") {
		t.Fatal("active query not shown")
	}
}

// Hiding a row must not desynchronise the mouse mapping from the frame.
func TestSearchRowHiddenKeepsRowCountAndMouseMapping(t *testing.T) {
	for _, focus := range []Focus{FocusEditor, FocusSearch} {
		m := layoutModel(t, 90, 16)
		m.Focus = focus
		rows := viewRows(t, m)
		if len(rows) != m.Height {
			t.Fatalf("focus %v: %d rows, want %d", focus, len(rows), m.Height)
		}
		if m.toolbarRow() != len(rows)-1 {
			t.Fatalf("focus %v: toolbar at %d, want the last row %d", focus, m.toolbarRow(), len(rows)-1)
		}
	}
}

func TestClickOnHiddenSearchRowDoesNotFocusSearch(t *testing.T) {
	m := layoutModel(t, 90, 16)
	m.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: 4, Y: m.searchRow(), Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if m.Focus == FocusSearch {
		t.Fatal("clicking the top border focused the hidden search row")
	}
}

// --- header with the lightbulb above the panes ---

func TestHeaderShowsLogoModeAndCurrentFile(t *testing.T) {
	m := layoutModel(t, 90, 20)
	rows := viewRows(t, m)
	for i, want := range splashLogo {
		if !strings.Contains(rows[i], strings.TrimSpace(want)) {
			t.Fatalf("header row %d = %q, want the logo line %q", i, rows[i], want)
		}
	}
	if !strings.Contains(rows[0], "glowed") || !strings.Contains(rows[0], "EDIT") {
		t.Fatalf("header row 0 = %q, want the name and the mode", rows[0])
	}
	if !strings.Contains(rows[1], "alpha.md") {
		t.Fatalf("header row 1 = %q, want the current file", rows[1])
	}
	if m.contentTop() != headerRows+1 {
		t.Fatalf("contentTop = %d, want %d", m.contentTop(), headerRows+1)
	}
}

func TestHeaderLogoIsYellow(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := layoutModel(t, 90, 20)
	first := strings.Split(m.View(), "\n")[0]
	if !strings.Contains(first, styleYellow.Render(splashLogo[0])) {
		t.Fatalf("header logo is not rendered yellow: %q", first)
	}
}

func TestSearchRowSitsBelowTheHeaderBlock(t *testing.T) {
	m := layoutModel(t, 90, 20)
	m.Focus = FocusSearch
	rows := viewRows(t, m)
	if !strings.Contains(rows[headerRows], "/") {
		t.Fatalf("row %d = %q, want the search row below the header", headerRows, rows[headerRows])
	}
	if m.contentTop() != headerRows+2 {
		t.Fatalf("contentTop = %d, want %d", m.contentTop(), headerRows+2)
	}
}

func TestClickOnTheSearchRowBelowTheHeaderFocusesSearch(t *testing.T) {
	m := layoutModel(t, 90, 20)
	m.Query = "note"
	m.handleMouse(tea.MouseMsg(tea.MouseEvent{
		X: 4, Y: headerRows, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	}))
	if m.Focus != FocusSearch {
		t.Fatalf("Focus = %v, want the search row click to still work", focusName(m.Focus))
	}
}

// The logo header only fits when there is room for it; a short split must not
// render more rows than the terminal has.
func TestShortTerminalFallsBackToASingleHeaderRow(t *testing.T) {
	m := layoutModel(t, 90, 9)
	if m.chromeTopRows() != 1 {
		t.Fatalf("chromeTopRows = %d, want 1 on a short terminal", m.chromeTopRows())
	}
	rows := viewRows(t, m)
	if !strings.Contains(rows[0], "glowed") || !strings.Contains(rows[0], "alpha.md") {
		t.Fatalf("compact header = %q, want the name, mode and file on one row", rows[0])
	}
}

func TestFrameFillsEveryTerminalHeight(t *testing.T) {
	for h := 8; h <= 24; h++ {
		m := layoutModel(t, 90, h)
		if rows := viewRows(t, m); len(rows) != h {
			t.Fatalf("height %d: rendered %d rows", h, len(rows))
		}
	}
}

// The project is branded glowed.md; the command stays "glowed".
func TestHeaderAndWelcomeShowTheProjectName(t *testing.T) {
	if AppName != "glowed.md" {
		t.Fatalf("AppName = %q, want glowed.md", AppName)
	}
	m := layoutModel(t, 90, 20)
	if !strings.Contains(viewRows(t, m)[0], AppName) {
		t.Fatalf("header = %q, want it to carry %q", viewRows(t, m)[0], AppName)
	}
}

// The third header row, next to the lightbulb, carries the status message.
func TestHeaderThirdRowShowsTheStatus(t *testing.T) {
	m := layoutModel(t, 90, 20)
	m.setStatus("notes updated: 1356 markdown file(s) scanned", "success")
	rows := viewRows(t, m)
	if !strings.Contains(rows[2], "notes updated: 1356 markdown file(s) scanned") {
		t.Fatalf("header row 2 = %q, want the status message", rows[2])
	}
	if !strings.Contains(rows[2], strings.TrimSpace(splashLogo[2])) {
		t.Fatalf("header row 2 lost the logo line: %q", rows[2])
	}
}

func TestHeaderThirdRowIsBlankWithoutAStatus(t *testing.T) {
	m := layoutModel(t, 90, 20)
	m.setStatus("", "info")
	rows := viewRows(t, m)
	if got := strings.TrimSpace(strings.ReplaceAll(rows[2], strings.TrimSpace(splashLogo[2]), "")); got != "" {
		t.Fatalf("header row 2 = %q, want only the logo line", rows[2])
	}
}

func TestHeaderStatusUsesTheStatusKindColor(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	m := layoutModel(t, 90, 20)
	m.setStatus("scanned", "success")
	row := strings.Split(m.View(), "\n")[2]
	if !strings.Contains(row, statusStyle("success").Render("scanned")) {
		t.Fatalf("status not rendered with its kind color: %q", row)
	}
}
