package app

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

func TestHeatLevelSpreadsDaysOverTheSteps(t *testing.T) {
	for _, c := range []struct{ n, want int }{{0, 0}, {1, 1}, {10, 2}, {50, 3}, {100, 4}} {
		if got := heatLevel(c.n, 100); got != c.want {
			t.Errorf("heatLevel(%d, 100) = %d, want %d", c.n, got, c.want)
		}
	}
}

// Calendar days take the color of their logs across the whole cell, the selected one is only
// underlined, the legend sits between the weeks and the totals, and a failed run no longer paints
// its day in the attention color.
func TestLogsCalendarColorsDaysByTheirLogs(t *testing.T) {
	a := testApp(t)
	a.palette = newPalette(a.themeNamed("blood"), true)
	today := dateOf(time.Now())
	busy := time.Date(today.Year(), today.Month(), 1, 10, 0, 0, 0, time.Local)
	for i := range 9 {
		writeTestLog(t, busy.Add(time.Duration(i)*time.Minute), "deploy", 10)
	}
	pressDrawn(a, prefixPress, letter('v'))
	p := a.currentTab().focusedView().(*logsPanel)

	buf := uv.NewScreenBuffer(a.size.X, a.size.Y)
	a.draw(buf, image.Rect(0, 0, a.size.X, a.size.Y))
	frame, _ := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	grid := contentRect(frame).Min.Add(p.grid)
	offset := (int(busy.Weekday()) + 6) % 7
	cellOf := func(day, column int) *uv.Cell {
		cell := offset + day - 1
		return buf.CellAt(grid.X+cell%7*dayCellWidth+column, grid.Y+cell/7)
	}
	for _, column := range []int{0, dayCellWidth - 1} {
		if bg := cellOf(1, column).Style.Bg; !colorsEqual(bg, a.palette.heat[heatLevels-1]) {
			t.Errorf("column %d of the busiest day has background %v, want the last heat step", column, bg)
		}
	}
	if cellOf(1, 2).Style.Attrs&uv.AttrBold != 0 {
		t.Error("a day with logs is bold")
	}
	if today.Day() != 2 && cellOf(2, 2).Style.Bg != a.palette.background {
		t.Errorf("a day without logs got a background: %v", cellOf(2, 2).Style.Bg)
	}
	if !strings.Contains(screenText(a), fmt.Sprintf("[%2d]", today.Day())) {
		t.Error("today has no brackets around it")
	}
	selected := p.dayStyle(p.day, dayStats{}, 9)
	if selected.Underline != uv.UnderlineSingle || selected.Bg != a.palette.background {
		t.Errorf("the selected day's style is %+v, want only an underline", selected)
	}

	lines := strings.Split(screenText(a), "\n")
	legend, totals := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "activity less") {
			legend = i
		}
		if strings.Contains(line, "this month") {
			totals = i
		}
	}
	if legend < 0 || legend != grid.Y+6 || totals != legend+2 {
		t.Errorf("legend on line %d and totals on %d, want the legend right under the six weeks (%d)", legend, totals, grid.Y+6)
	}

	failed := p.dayStyle(today.AddDate(0, 0, 1), dayStats{logs: 1, runs: 1, failed: 1}, 9)
	if colorsEqual(failed.Fg, a.palette.status.Fg) {
		t.Error("a day with a failed run is still in the attention color")
	}
}

func colorsEqual(a, b interface{ RGBA() (r, g, b, a uint32) }) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// A calendar too short for the totals, over a narrow list, shows none of them.
func TestShortLogsCalendarLeavesTheTotalsOut(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(60, 30) // too narrow for the calendar beside the list, too short for 18 rows
	pressDrawn(a, prefixPress, letter('v'))
	if screen := screenText(a); strings.Contains(screen, "this month") || !strings.Contains(screen, "activity less") {
		t.Errorf("the short calendar shows the totals or lacks its legend:\n%s", screen)
	}
}
