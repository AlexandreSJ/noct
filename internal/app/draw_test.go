package app

import (
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// The status sits at the right of a frame's line, and a title too long for what is left is cut
// with "…".
func TestTitleLineKeepsTheStatusAtTheRight(t *testing.T) {
	cases := []struct{ title, status, want string }{
		{"shell", "", "─ shell ─────────────────────"},
		{"shell", "↑ 0/18", "─ shell ──────────── ↑ 0/18 ─"},
		{"a very long panel name", "⛶ fullscreen", "─ a very lo… ─ ⛶ fullscreen ─"},
	}
	for _, c := range cases {
		buffer := uv.NewScreenBuffer(29, 1)
		drawTitleLine(buffer, 0, 29, 0, c.title, c.status, uv.Style{}, uv.Style{})
		if got := strings.TrimRight(buffer.Render(), " "); got != strings.TrimRight(c.want, " ") {
			t.Errorf("title %q status %q drew\n%q, want\n%q", c.title, c.status, got, c.want)
		}
	}
}

// In fullscreen the middle of each side with a pane beyond it is dashed, a cell of line and a
// cell of gap in turn, and only that side. The top line is left out: its title has spaces.
func TestFullscreenDashesTheSidesWithPanes(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	a.currentTab().focus(2)
	press(a, prefixPress, letter('f'))
	area := a.workspaceArea()
	right, bottom := area.Max.X-1, area.Max.Y-1
	gaps := func() (left, rightSide, across int) {
		buf := drawnApp(a)
		gap := func(x, y int) bool {
			if buf.CellAt(x, y).Content != " " {
				return false
			}
			if x == area.Min.X || x == right { // a gap sits between two cells of line
				return buf.CellAt(x, y-1).Content == "│" && buf.CellAt(x, y+1).Content == "│"
			}
			return buf.CellAt(x-1, y).Content == "─" && buf.CellAt(x+1, y).Content == "─"
		}
		for y := area.Min.Y + 1; y < bottom; y++ {
			if gap(area.Min.X, y) {
				left++
			}
			if gap(right, y) {
				rightSide++
			}
		}
		for x := area.Min.X + 1; x < right; x++ {
			if gap(x, bottom) {
				across++
			}
		}
		return left, rightSide, across
	}
	if left, rightSide, across := gaps(); left == 0 || rightSide != 0 || across != 0 {
		t.Errorf("focused on the right: %d gaps left, %d right, %d below", left, rightSide, across)
	}
	press(a, prefixPress, leftPress, escPress)
	if left, rightSide, across := gaps(); left != 0 || rightSide == 0 || across != 0 {
		t.Errorf("focused on the left: %d gaps left, %d right, %d below", left, rightSide, across)
	}
	press(a, prefixPress, letter('f'))
	if left, rightSide, across := gaps(); left+rightSide+across != 0 {
		t.Errorf("out of fullscreen: %d gaps", left+rightSide+across)
	}
}

func TestExitSpanIsTheMiddleThirdOddLong(t *testing.T) {
	cases := []struct{ start, end, from, to int }{
		{1, 31, 11, 20}, // 30 cells: 9 in the middle, odd to end with line
		{1, 6, 2, 5},    // 5 cells: never under 3
		{1, 3, 1, 2},    // 2 cells: one, as two cannot end with line
	}
	for _, c := range cases {
		if from, to := exitSpan(c.start, c.end); from != c.from || to != c.to {
			t.Errorf("exitSpan(%d, %d) = %d, %d, want %d, %d", c.start, c.end, from, to, c.from, c.to)
		}
	}
}
