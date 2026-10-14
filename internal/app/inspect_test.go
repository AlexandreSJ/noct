package app

import (
	"image"
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
)

var (
	shiftDownPress = uv.KeyPressEvent{Code: uv.KeyDown, Mod: uv.ModShift}
	ctrlRightPress = uv.KeyPressEvent{Code: uv.KeyRight, Mod: uv.ModCtrl}
)

// screenText draws the app and gives the screen as text, a line per row.
func screenText(a *App) string {
	buf := uv.NewScreenBuffer(a.size.X, a.size.Y)
	a.draw(buf, image.Rect(0, 0, a.size.X, a.size.Y))
	var lines []string
	for y := range a.size.Y {
		var line strings.Builder
		for x := range a.size.X {
			if c := buf.CellAt(x, y); c != nil && c.Width > 0 {
				line.WriteString(c.Content)
			} else if c == nil || c.Content == "" {
				line.WriteString(" ")
			}
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

const threeSteps = "steps:\n  - {id: one, run: printf 1}\n  - {id: two, run: 'echo {{.steps.one.output}}'}\n  - {id: three, run: printf 3}\n"

// Shift+down selects the next step too, t tests the range, and esc leaves just the step;
// ctrl+arrows pan the chart.
func TestInspectTestsARangeOfSteps(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveFlow(t, "trio", threeSteps)
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	inspect := a.currentTab().focused
	pressDrawn(a, shiftDownPress, letter('t'))
	if got := l.last(t).argv[1:]; !slices.Equal(got, []string{"flow", "run", "trio", "--steps", "0..1"}) {
		t.Fatalf("t on a range ran %v", got)
	}
	a.focusPane(a.currentTab(), inspect) // the test's run took the focus
	pressDrawn(a, escPress, letter('t'))
	a.focusPane(a.currentTab(), inspect)
	if got := l.last(t).argv[1:]; !slices.Equal(got, []string{"flow", "run", "trio", "--steps", "1"}) {
		t.Errorf("t after esc ran %v", got)
	}
	p := a.currentTab().views[inspect].(*inspectPanel)
	pan := p.pan
	pressDrawn(a, ctrlRightPress)
	if p.pan.X <= pan.X || !p.focusChart {
		t.Errorf("ctrl+right: pan %v → %v, chart focused %v", pan, p.pan, p.focusChart)
	}
}

// s shows what the selected step can read, and a step that reads one not run yet is marked.
func TestScopePaneAndWarnings(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "trio", "steps:\n  - {id: one, run: printf 1, capture: {n: '(\\d)'}}\n  - {id: two, run: 'echo {{.steps.three.output}}'}\n  - {id: three, run: printf 3}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, downPress, downPress, letter('s'))
	screen := screenText(a)
	for _, want := range []string{"scope", ".steps.one.n", "⚠ step three has not run yet here"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen lacks %q:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, ".steps.three.output") {
		t.Error("the scope of step two lists step three")
	}
}

// ctrl+t in a template prompt picks a variable in scope and types it.
func TestPromptInsertsAVariableInScope(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "trio", threeSteps)
	pressDrawn(a, prefixPress, letter('i'), tabPress, downPress, letter('a'), letter('c'))
	typeDrawn(a, "echo ")
	pressDrawn(a, uv.KeyPressEvent{Code: 't', Mod: uv.ModCtrl})
	if a.mode != finderMode {
		t.Fatalf("ctrl+t: mode %v, want the finder", a.mode)
	}
	typeDrawn(a, "one.output")
	pressDrawn(a, enterPress)
	if a.mode != promptMode || a.prompt.input.String() != "echo {{.steps.one.output}}" {
		t.Errorf("mode %v, prompt %q", a.mode, a.prompt.input.String())
	}
	pressDrawn(a, uv.KeyPressEvent{Code: 't', Mod: uv.ModCtrl}, escPress)
	if a.mode != promptMode {
		t.Errorf("esc in the finder left the prompt: mode %v", a.mode)
	}
}

// A step's about tells its kept output and whether a test ran it last.
func TestAboutStepShowsTheKeptOutput(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "trio", threeSteps)
	result := flow.Result{Flow: "trio", OK: true, Steps: map[string]flow.StepResult{"0": {Status: flow.StatusOK, Output: "hello\nworld", Tried: true}}}
	if err := flow.SaveResult(flow.ResultsDir(), result); err != nil {
		t.Fatal(err)
	}
	pressDrawn(a, prefixPress, letter('i'), tabPress, enterPress)
	var texts []string
	for _, line := range a.info.lines {
		texts = append(texts, line.text)
	}
	joined := strings.Join(texts, "\n")
	if !strings.Contains(joined, "tried") || !strings.Contains(joined, "hello\nworld") {
		t.Errorf("about lacks the test mark or the output:\n%s", joined)
	}
}

// A range belongs to its flow: choosing another flow in the list starts without one.
func TestRangeEndsWithItsFlow(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveFlow(t, "a-trio", threeSteps)
	saveFlow(t, "b-trio", threeSteps)
	pressDrawn(a, prefixPress, letter('i'), enterPress, shiftDownPress)
	p := a.currentTab().focusedView().(*inspectPanel)
	p.focusChart = false // back to the list without esc, as a click on it does
	pressDrawn(a, downPress, enterPress)
	p.selected = stepKey("1") // as a click on the step does
	pressDrawn(a, letter('t'))
	if got := l.last(t).argv[1:]; !slices.Equal(got, []string{"flow", "run", "b-trio", "--steps", "1"}) {
		t.Errorf("t in the other flow ran %v", got)
	}
}

// Moving a step ends the range, whose steps moved.
func TestMovingAStepEndsTheRange(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "trio", threeSteps)
	pressDrawn(a, prefixPress, letter('i'), tabPress, shiftDownPress, letter('J'))
	if p := a.currentTab().focusedView().(*inspectPanel); p.anchor != "" {
		t.Errorf("the range still starts at %q after J", p.anchor)
	}
}
