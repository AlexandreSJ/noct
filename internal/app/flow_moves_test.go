package app

import (
	"image"
	"path/filepath"
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

func TestDraggingACardDropsItBelowAnother(t *testing.T) {
	a, p := openMovesFlow(t)
	frame, _ := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	origin := contentRect(frame).Min.Add(p.chartArea.Min).Sub(p.pan)
	at := func(key string) image.Point {
		b, ok := p.chart.block(key)
		if !ok {
			t.Fatalf("no block %s", key)
		}
		return origin.Add(b.rect.Min).Add(image.Pt(2, 1))
	}
	from, to := at(stepKey("0")), at(stepKey("2"))
	a.handleMouse(uv.MouseClickEvent{X: from.X, Y: from.Y, Button: uv.MouseLeft})
	a.handleMouse(uv.MouseMotionEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft})
	drawnApp(a)
	a.handleMouse(uv.MouseReleaseEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft})
	steps := loadFlowSteps(t, "moves")
	if len(steps) != 3 || steps[0].ID != "when" || steps[1].ID != "c" || steps[2].ID != "a" {
		t.Errorf("dropped a on c: %+v, want a below c", steps)
	}
}
