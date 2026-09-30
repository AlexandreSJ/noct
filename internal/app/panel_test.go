package app

import (
	"fmt"
	"image"
	"slices"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// Moving down a long list, the view stays put until the selection is scrollMargin rows from its
// bottom, then follows it; moving back up, it stays until scrollMargin rows from the top.
func TestListFirstKeepsRowsAroundTheSelection(t *testing.T) {
	const n, height = 30, 10
	first := 0
	for sel := 0; sel < n; sel++ {
		first = listFirst(first, sel, n, height)
		want := min(max(sel+scrollMargin(height)-height+1, 0), n-height)
		if first != want {
			t.Fatalf("down to %d: first = %d, want %d", sel, first, want)
		}
	}
	for sel := n - 1; sel >= 0; sel-- {
		first = listFirst(first, sel, n, height)
		want := min(max(sel-scrollMargin(height), 0), n-height)
		if first != want {
			t.Fatalf("up to %d: first = %d, want %d", sel, first, want)
		}
	}
}

func TestListFirstEdges(t *testing.T) {
	cases := []struct {
		name                             string
		first, selected, n, height, want int
	}{
		{"fits", 0, 5, 8, 10, 0},
		{"list shrank under the view", 20, 6, 8, 5, 3},
		{"short view keeps the selection inside", 0, 3, 20, 3, 2},
		{"one row", 0, 7, 20, 1, 7},
		{"no room", 4, 2, 20, 0, 0},
	}
	for _, c := range cases {
		if got := listFirst(c.first, c.selected, c.n, c.height); got != c.want {
			t.Errorf("%s: listFirst(%d, %d, %d, %d) = %d, want %d", c.name, c.first, c.selected, c.n, c.height, got, c.want)
		}
	}
}

func TestScrollMarginGrowsWithTheList(t *testing.T) {
	for height, want := range map[int]int{3: 1, 9: 4, 12: 4, 20: 6, 30: 9, 40: 12} {
		if got := scrollMargin(height); got != want {
			t.Errorf("scrollMargin(%d) = %d, want %d", height, got, want)
		}
	}
}

func TestListJump(t *testing.T) {
	for _, c := range []struct {
		key                  string
		selected, n, h, want int
	}{
		{"pgdown", 0, 30, 10, 9}, {"pgdown", 25, 30, 10, 29}, {"pgup", 20, 30, 10, 11}, {"pgup", 3, 30, 10, 0},
		{"home", 17, 30, 10, 0}, {"end", 2, 30, 10, 29}, {"end", 0, 0, 10, 0}, {"pgdown", 0, 5, 1, 1},
	} {
		if got, ok := listJump(c.key, c.selected, c.n, c.h); !ok || got != c.want {
			t.Errorf("listJump(%s, %d, %d, %d) = %d, want %d", c.key, c.selected, c.n, c.h, got, c.want)
		}
	}
	if _, ok := listJump("down", 0, 30, 10); ok {
		t.Error("down jumped")
	}
}

var (
	pgUpPress   = uv.KeyPressEvent{Code: uv.KeyPgUp}
	pgDownPress = uv.KeyPressEvent{Code: uv.KeyPgDown}
	homePress   = uv.KeyPressEvent{Code: uv.KeyHome}
	endPress    = uv.KeyPressEvent{Code: uv.KeyEnd}
)

func TestConfigJumpsByPagesAndToTheEnds(t *testing.T) {
	a := openConfigAt(t, prefixRow)
	press(a, endPress)
	if a.configPanel.selected != resetRow {
		t.Fatalf("end selected %d, want the reset, the last row", a.configPanel.selected)
	}
	press(a, homePress)
	if a.configPanel.selected != prefixRow {
		t.Fatalf("home selected %d, want the first row", a.configPanel.selected)
	}
	a.size = image.Pt(100, 16) // a page shorter than the settings
	press(a, pgDownPress)
	rows := a.selectableConfigRows()
	if page := a.configLayout().visible() - 1; page >= len(rows) || slices.Index(rows, a.configPanel.selected) != page {
		t.Errorf("pgdown went to row %d, want a page down at %d of %d", slices.Index(rows, a.configPanel.selected), page, len(rows))
	}
}

func TestSearchJumpsToTheEnds(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	press(a, prefixPress, uv.KeyPressEvent{Code: uv.KeySpace}, endPress)
	if a.finder.selected != len(a.finder.matches)-1 {
		t.Errorf("end selected %d of %d", a.finder.selected, len(a.finder.matches))
	}
}

// In the logs, pgup and pgdown page the list while it has the focus, and change the month on the
// calendar, as [ and ] always do.
func TestLogsPageTheListOrChangeTheMonth(t *testing.T) {
	a := testApp(t)
	today := dateOf(time.Now()).Add(10 * time.Hour)
	for i := range 40 {
		writeTestLog(t, today.Add(time.Duration(i)*time.Second), fmt.Sprint("log", i), 10)
	}
	pressDrawn(a, prefixPress, letter('v'))
	p := a.currentTab().focusedView().(*logsPanel)
	month := p.day.Month()
	pressDrawn(a, pgDownPress)
	if p.day.Month() == month {
		t.Fatal("pgdown on the calendar kept the month")
	}
	pressDrawn(a, pgUpPress, enterPress, pgDownPress)
	if p.day.Month() != month || p.selected != p.list.Dy()-1 {
		t.Errorf("pgdown in the list: month %v, selected %d, want a page down at %d", p.day.Month(), p.selected, p.list.Dy()-1)
	}
	pressDrawn(a, endPress)
	if p.selected != 39 {
		t.Errorf("end selected %d, want the last of 40", p.selected)
	}
}

func TestScriptsPageDown(t *testing.T) {
	a := testApp(t)
	for i := range 30 {
		saveScript(t, fmt.Sprintf("s%02d", i), "run: true\n")
	}
	panel := openScriptsPanel(t, a)
	drawApp(a)
	press(a, pgDownPress)
	if panel.selected != panel.height-1 || panel.height == 0 {
		t.Errorf("pgdown selected %d with %d rows shown", panel.selected, panel.height)
	}
}
