package app

import (
	"image"
	"path/filepath"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
)

const movesFlow = `steps:
  - {id: a, run: 'true'}
  - id: when
    if: 'true'
    then: [{id: b, run: 'true'}]
  - {id: c, run: 'true'}
`

func loadFlowSteps(t *testing.T, name string) []flow.Step {
	t.Helper()
	f, err := flow.Load(filepath.Join(flow.Dir(), name+flow.Ext))
	if err != nil {
		t.Fatal(err)
	}
	return f.Steps
}

func openMovesFlow(t *testing.T) (*App, *inspectPanel) {
	t.Helper()
	a := testApp(t)
	a.size = image.Pt(140, 60)
	saveFlow(t, "moves", movesFlow)
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	return a, a.currentTab().focusedView().(*inspectPanel)
}

func TestJMovesAStepIntoTheIfBelowAndZUndoesIt(t *testing.T) {
	a, p := openMovesFlow(t)
	pressDrawn(a, letter('J'))
	steps := loadFlowSteps(t, "moves")
	if len(steps) != 2 || len(steps[0].Then) != 2 || steps[0].Then[0].ID != "a" || p.selected != stepKey("0/then/0") {
		t.Fatalf("J: %+v, selected %q; want a first in the if's yes branch, still selected", steps, p.selected)
	}
	pressDrawn(a, letter('z'))
	if steps := loadFlowSteps(t, "moves"); len(steps) != 3 || steps[0].ID != "a" {
		t.Fatalf("z: %+v, want the flow as it was", steps)
	}
	pressDrawn(a, letter('y'))
	if steps := loadFlowSteps(t, "moves"); len(steps) != 2 {
		t.Errorf("y: %+v, want the move done again", steps)
	}
}

func TestMoveModeMovesWithTheArrows(t *testing.T) {
	a, p := openMovesFlow(t)
	pressDrawn(a, letter('m'), downPress, downPress, downPress, downPress) // into yes, past b, into no, out
	if !p.moving {
		t.Fatal("m did not start moving")
	}
	steps := loadFlowSteps(t, "moves")
	if len(steps) != 3 || steps[0].ID != "when" || steps[1].ID != "a" || steps[0].Else != nil && len(steps[0].Else) > 0 {
		t.Errorf("four downs: %+v, want a just after the if", steps)
	}
	pressDrawn(a, enterPress)
	if p.moving {
		t.Error("enter did not put the step down")
	}
}

// A card moves with alt (or ctrl) held: a dashed outline shows where it lands, between cards or
// into a block. Dragged without either, the chart pans and the flow stays as it was.
func TestAltDragShowsWhereACardLandsAndDropsItThere(t *testing.T) {
	a, p := openMovesFlow(t)
	frame, _ := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	origin := func() image.Point { return contentRect(frame).Min.Add(p.chartArea.Min).Sub(p.pan) }
	rect := func(key string) image.Rectangle {
		b, ok := p.chart.block(key)
		if !ok {
			t.Fatalf("no block %s", key)
		}
		return b.rect.Add(origin())
	}
	drag := func(from, to image.Point, mod uv.KeyMod) {
		a.handleMouse(uv.MouseClickEvent{X: from.X, Y: from.Y, Button: uv.MouseLeft, Mod: mod})
		a.handleMouse(uv.MouseMotionEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft, Mod: mod})
		drawnApp(a)
		a.handleMouse(uv.MouseReleaseEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft, Mod: mod})
		drawnApp(a)
	}
	a0, c := rect(stepKey("0")), rect(stepKey("2"))
	drag(a0.Min.Add(image.Pt(2, 1)), image.Pt(c.Min.X+2, c.Max.Y-1), 0)
	if steps := loadFlowSteps(t, "moves"); steps[0].ID != "a" {
		t.Fatalf("a plain drag moved a step: %+v", steps)
	}
	pressDrawn(a, letter('0')) // centered again, after the pan

	a0, c = rect(stepKey("0")), rect(stepKey("2"))
	from, to := a0.Min.Add(image.Pt(2, 1)), image.Pt(c.Min.X+2, c.Max.Y-1)
	a.handleMouse(uv.MouseClickEvent{X: from.X, Y: from.Y, Button: uv.MouseLeft, Mod: uv.ModAlt})
	a.handleMouse(uv.MouseMotionEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft, Mod: uv.ModAlt})
	drawnApp(a)
	if gap, ok := p.dropGap(); !ok || gap.slot != (chartSlot{}) || gap.index != 3 {
		t.Fatalf("over the lower half of c the place is %+v (%v), want after c", gap, ok)
	}
	if !strings.Contains(chartText(p.chart), "↳ a") {
		t.Errorf("no dashed outline with the card's name:\n%s", chartText(p.chart))
	}
	a.handleMouse(uv.MouseReleaseEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft, Mod: uv.ModAlt})
	drawnApp(a)
	if steps := loadFlowSteps(t, "moves"); len(steps) != 3 || steps[0].ID != "when" || steps[1].ID != "c" || steps[2].ID != "a" {
		t.Fatalf("dropped a after c: %+v, want when, c, a", steps)
	}

	// c, now at 1, goes into the if, above b: the upper half of b's card.
	c, b := rect(stepKey("1")), rect(stepKey("0/then/0"))
	drag(c.Min.Add(image.Pt(2, 1)), image.Pt(b.Min.X+2, b.Min.Y), uv.ModCtrl)
	steps := loadFlowSteps(t, "moves")
	if len(steps) != 2 || len(steps[0].Then) != 2 || steps[0].Then[0].ID != "c" || steps[0].Then[1].ID != "b" {
		t.Errorf("dropped c above b: %+v, want it first in the if's then", steps)
	}
}

// chartText is the chart's canvas as text, a line per row.
func chartText(c *flowChart) string {
	var out strings.Builder
	for y := range c.size.Y {
		for x := range c.size.X {
			if cell := c.canvas.CellAt(x, y); cell != nil && cell.Content != "" {
				out.WriteString(cell.Content)
			} else {
				out.WriteString(" ")
			}
		}
		out.WriteString("\n")
	}
	return out.String()
}

// A drag ends however the button goes up: released past the chart, as on the tab bar, the chart
// still gets it; the pointer leaving the window, or coming back with no button held, lets go
// where it last was inside, as the release out there never arrives.
func TestADragLetsGoWhereverTheButtonGoesUp(t *testing.T) {
	a, p := openMovesFlow(t)
	frame, _ := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	block, _ := p.chart.block(stepKey("0"))
	from := block.rect.Add(contentRect(frame).Min.Add(p.chartArea.Min).Sub(p.pan)).Min.Add(image.Pt(2, 1))
	press := func() {
		a.handleMouse(uv.MouseClickEvent{X: from.X, Y: from.Y, Button: uv.MouseLeft, Mod: uv.ModAlt})
		a.handleMouse(uv.MouseMotionEvent{X: from.X + 1, Y: from.Y + 1, Button: uv.MouseLeft, Mod: uv.ModAlt})
		drawnApp(a)
		if p.drag == nil {
			t.Fatal("the press did not start a drag")
		}
	}
	press()
	a.handleMouse(uv.MouseReleaseEvent{X: 3, Y: 0, Button: uv.MouseLeft})
	if p.drag != nil || a.mouseHeld != nil {
		t.Error("a release on the tab bar left the chart dragging")
	}
	press()
	a.handleMouse(uv.MouseMotionEvent{X: a.size.X + 4, Y: from.Y, Button: uv.MouseLeft})
	if p.drag != nil {
		t.Error("leaving the window left the chart dragging")
	}
	press()
	a.handleMouse(uv.MouseMotionEvent{X: from.X, Y: from.Y, Button: uv.MouseNone})
	if p.drag != nil {
		t.Error("coming back with no button left the chart dragging")
	}
	if steps := loadFlowSteps(t, "moves"); steps[0].ID != "a" {
		t.Errorf("letting go near where it was picked moved the card: %+v", steps)
	}
}

// The details under the chart keep their height whatever block is selected, so the chart,
// centered on the selection, does not jump up and down as it moves.
func TestTheChartKeepsItsPlaceAsTheSelectionMoves(t *testing.T) {
	a, p := openMovesFlow(t)
	area, pan := p.chartArea, p.pan
	for range 3 {
		pressDrawn(a, letter('j'))
		if p.chartArea != area {
			t.Fatalf("at %s the chart is %v, it was %v", p.selected, p.chartArea, area)
		}
		if p.pan.X != pan.X {
			t.Errorf("at %s the chart moved sideways: %v, was %v", p.selected, p.pan, pan)
		}
	}
}

// v gives the details' rows to the chart, and v again brings them back.
func TestVHidesTheDetails(t *testing.T) {
	a, p := openMovesFlow(t)
	shown := p.chartArea
	if p.details.Empty() {
		t.Fatal("no details under the chart")
	}
	pressDrawn(a, letter('v'))
	if !p.details.Empty() || p.chartArea.Dy() <= shown.Dy() {
		t.Errorf("v: details %v, chart %v, was %v", p.details, p.chartArea, shown)
	}
	pressDrawn(a, letter('j'), letter('v'))
	if p.details.Empty() || p.chartArea != shown {
		t.Errorf("v again: details %v, chart %v, want %v", p.details, p.chartArea, shown)
	}
}

// g selects the flow's start and G its last step, as they go to the top and end of a log.
func TestGHomeAndEndGoToTheFlowsStartAndEnd(t *testing.T) {
	a, p := openMovesFlow(t)
	pressDrawn(a, letter('G'))
	if p.selected != stepKey("2") {
		t.Errorf("G selected %q, want the last step", p.selected)
	}
	pressDrawn(a, letter('g'))
	if p.selected != headKey(chartSlot{}) {
		t.Errorf("g selected %q, want the start", p.selected)
	}
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyEnd})
	if p.selected != stepKey("2") {
		t.Errorf("end selected %q, want the last step", p.selected)
	}
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyHome})
	if p.selected != headKey(chartSlot{}) {
		t.Errorf("home selected %q, want the start", p.selected)
	}
}
