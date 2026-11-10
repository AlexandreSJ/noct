package app

import (
	"fmt"
	"image"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// listView lists rows a line each, selected in the selection's style.
type listView struct {
	fakeView
	pal      *palette
	rows     int
	selected int
	list     image.Rectangle
}

func (v *listView) Draw(screen uv.Screen, area uv.Rectangle) {
	for i := range v.rows {
		style := v.pal.panelText
		if i == v.selected {
			style = v.pal.selectedRow
		}
		drawRow(screen, area, area.Min.Y+i, fmt.Sprintf("row %d", i), "", style)
	}
	v.list = image.Rect(0, 0, area.Dx(), v.rows)
}

func (v *listView) RowAt(point image.Point) (image.Rectangle, bool, bool) {
	return listRowAt(v.list, point, 0, v.rows, v.selected)
}

// listApp shows a listView of 4 rows, the first selected, with hover fading; row is where a row
// shows on screen.
func listApp(t *testing.T) (a *App, v *listView, row func(int) image.Point) {
	t.Helper()
	a, tb := animatedApp(t)
	a.config.Animations.Hover = "fade"
	v = &listView{pal: &a.palette, rows: 4}
	tb.views[tb.focused] = v
	drawnApp(a)
	frame, _ := tb.frameOf(tb.focused, a.workspaceArea())
	content := contentRect(frame)
	if a.palette.selectedRow.Bg == nil {
		t.Fatal("the test palette has no selection color")
	}
	return a, v, func(i int) image.Point { return image.Pt(content.Min.X+2, content.Min.Y+i) }
}

func settle(a *App) {
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
}

func TestHoveringAListRowLightsOnlyItsText(t *testing.T) {
	a, _, row := listApp(t)
	_, before := cellStyleAt(a, row(2))
	a.handleMouse(motion(row(2).X, row(2).Y))
	if fg, _ := cellStyleAt(a, row(2)); !colorsEqual(fg, a.palette.panelText.Fg) {
		t.Errorf("as the hover starts the row is %v, want its own %v", fg, a.palette.panelText.Fg)
	}
	settle(a)
	fg, bg := cellStyleAt(a, row(2))
	if !colorsEqual(fg, a.palette.selectedRow.Bg) {
		t.Errorf("hovered, the row is %v, want the selection's color %v", fg, a.palette.selectedRow.Bg)
	}
	if bg != before && (bg == nil || before == nil || !colorsEqual(bg, before)) {
		t.Errorf("hovered, the row is on %v, want its own background %v", bg, before)
	}
	if fg, _ := cellStyleAt(a, row(3)); !colorsEqual(fg, a.palette.panelText.Fg) {
		t.Errorf("the row below is %v, want it untouched %v", fg, a.palette.panelText.Fg)
	}
}

func TestLeavingAListRowFadesItBack(t *testing.T) {
	a, _, row := listApp(t)
	a.handleMouse(motion(row(2).X, row(2).Y))
	settle(a)
	a.handleMouse(motion(row(3).X, row(3).Y))
	if fg, _ := cellStyleAt(a, row(2)); !colorsEqual(fg, a.palette.selectedRow.Bg) {
		t.Errorf("as the mouse leaves the row is %v, want it still lit %v", fg, a.palette.selectedRow.Bg)
	}
	settle(a)
	if fg, _ := cellStyleAt(a, row(2)); !colorsEqual(fg, a.palette.panelText.Fg) {
		t.Errorf("after leaving the row is %v, want its own %v", fg, a.palette.panelText.Fg)
	}
	if fg, _ := cellStyleAt(a, row(3)); !colorsEqual(fg, a.palette.selectedRow.Bg) {
		t.Errorf("the row the mouse went to is %v, want it lit %v", fg, a.palette.selectedRow.Bg)
	}
}

func TestHoveringTheSelectedRowLeavesIt(t *testing.T) {
	a, _, row := listApp(t)
	want := drawnApp(a).CellAt(row(0).X, row(0).Y).Style
	a.handleMouse(motion(row(0).X, row(0).Y))
	settle(a)
	if got := drawnApp(a).CellAt(row(0).X, row(0).Y).Style; !got.Equal(&want) {
		t.Errorf("hovered, the selected row is %+v, want it as it was %+v", got, want)
	}
}

func TestAHoveredRowSelectedShowsAsSelected(t *testing.T) {
	a, v, row := listApp(t)
	a.handleMouse(motion(row(2).X, row(2).Y))
	v.selected = 2
	fg, _ := cellStyleAt(a, row(2))
	if !colorsEqual(fg, a.palette.selectedRow.Fg) {
		t.Errorf("selected under the mouse, the row is %v, want the selection's text %v", fg, a.palette.selectedRow.Fg)
	}
}

func TestARowSelectedAsItFadesBackShowsAsSelected(t *testing.T) {
	a, v, row := listApp(t)
	a.handleMouse(motion(row(2).X, row(2).Y))
	settle(a)
	a.handleMouse(motion(row(3).X, row(3).Y))
	v.selected = 2
	if fg, _ := cellStyleAt(a, row(2)); !colorsEqual(fg, a.palette.selectedRow.Fg) {
		t.Errorf("selected as it fades back, the row is %v, want the selection's text %v", fg, a.palette.selectedRow.Fg)
	}
}

func TestAListRowLightsWithoutAnimations(t *testing.T) {
	a, _, row := listApp(t)
	a.config.Animations.Hover = "none"
	a.handleMouse(motion(row(1).X, row(1).Y))
	if fg, _ := cellStyleAt(a, row(1)); !colorsEqual(fg, a.palette.selectedRow.Bg) {
		t.Errorf("hovered with no effect, the row is %v, want it lit at once %v", fg, a.palette.selectedRow.Bg)
	}
}

func TestListRowAt(t *testing.T) {
	list := image.Rect(1, 2, 11, 6)
	cases := []struct {
		point          image.Point
		first, rows    int
		row            image.Rectangle
		selected, isAt bool
	}{
		{image.Pt(3, 3), 0, 4, image.Rect(1, 3, 11, 4), false, true},
		{image.Pt(3, 2), 0, 4, image.Rect(1, 2, 11, 3), true, true},
		{image.Pt(3, 5), 0, 3, image.Rectangle{}, false, false},
		{image.Pt(3, 4), 2, 5, image.Rect(1, 4, 11, 5), false, true},
		{image.Pt(0, 3), 0, 4, image.Rectangle{}, false, false},
	}
	for _, c := range cases {
		row, selected, ok := listRowAt(list, c.point, c.first, c.rows, 0)
		if row != c.row || selected != c.selected || ok != c.isAt {
			t.Errorf("listRowAt(%v, first %d, %d rows) = %v %v %v, want %v %v %v", c.point, c.first, c.rows, row, selected, ok, c.row, c.selected, c.isAt)
		}
	}
}

var (
	_ rowLister = (*scriptsPanel)(nil)
	_ rowLister = (*logsPanel)(nil)
	_ rowLister = (*inspectPanel)(nil)
)

func TestHoveringAScriptLightsItsRow(t *testing.T) {
	a := testApp(t)
	saveScript(t, "greet", greetScript)
	saveScript(t, "wave", greetScript)
	panel := openScriptsPanel(t, a)
	drawnApp(a)
	frame, _ := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	content := contentRect(frame)
	var row image.Point
	for y := panel.list.Min.Y; y < panel.list.Max.Y; y++ {
		if _, selected, ok := panel.RowAt(image.Pt(panel.list.Min.X, y)); ok && !selected {
			row = content.Min.Add(image.Pt(panel.list.Min.X+3, y))
			break
		}
	}
	if row == (image.Point{}) {
		t.Fatal("the scripts list has no row besides the selected one")
	}
	a.handleMouse(motion(row.X, row.Y))
	if fg, _ := cellStyleAt(a, row); !colorsEqual(fg, a.palette.selectedRow.Bg) {
		t.Errorf("a hovered script's row is %v, want the selection's color %v", fg, a.palette.selectedRow.Bg)
	}
}
