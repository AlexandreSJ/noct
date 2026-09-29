package app

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

func drawApp(a *App) {
	a.draw(uv.NewScreenBuffer(a.size.X, a.size.Y), image.Rect(0, 0, a.size.X, a.size.Y))
}

// pressDrawn presses keys, drawing after each as the screen would: the chart finds its blocks
// when drawn.
func pressDrawn(a *App, keys ...uv.KeyPressEvent) {
	for _, key := range keys {
		press(a, key)
		drawApp(a)
	}
}

func typeDrawn(a *App, text string) {
	for _, r := range text {
		pressDrawn(a, letter(r))
	}
}

var clearLine = uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl}

// The example flow is built with the inspect panel's keys alone: a repeat until the dice is not
// 6, with a script, an if, and a script in its then whose capture is edited in the editor.
func TestInspectBuildsAFlowStepByStep(t *testing.T) {
	a := testApp(t)
	a.config.Editor = "true" // any installed program: the test writes what the editor would
	l := recordLaunches(a)
	saveScript(t, "where", "run: pwd\n")
	saveScript(t, "dice", "run: echo number=6\n")
	pressDrawn(a, prefixPress, letter('i'))
	if _, ok := a.currentTab().focusedView().(*inspectPanel); !ok {
		t.Fatal("prefix+i did not open the inspect panel")
	}
	pressDrawn(a, letter('a'))
	typeDrawn(a, "explore")
	pressDrawn(a, enterPress, letter('a'), letter('l'), clearLine)
	typeDrawn(a, `{{ne .steps.dice.number "6"}}`)
	pressDrawn(a, enterPress, downPress, letter('a'), letter('s'))
	typeDrawn(a, "where")
	pressDrawn(a, enterPress, letter('a'), letter('i'), clearLine)
	typeDrawn(a, `{{eq .steps.where.exit "0"}}`)
	pressDrawn(a, enterPress, downPress, letter('a'), letter('s'))
	typeDrawn(a, "dice")
	pressDrawn(a, enterPress, letter('e'))
	editPart(t, l, "id: dice\nscript: dice\ncapture: {number: 'number=(\\d)'}\n")

	if a.statusMessage != "" {
		t.Fatalf("status: %s", a.statusMessage)
	}
	f, err := flow.Load(filepath.Join(flow.Dir(), "explore"+flow.Ext))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Validate(a.scripts.Find); err != nil {
		t.Fatalf("built flow is invalid: %v", err)
	}
	repeat, _ := f.Step("0")
	inner, _ := f.Step("0/steps/0")
	branch, _ := f.Step("0/steps/1")
	dice, _ := f.Step("0/steps/1/then/0")
	switch {
	case repeat.Repeat == nil || repeat.Repeat.Until != `{{ne .steps.dice.number "6"}}` || repeat.Repeat.Max != 10:
		t.Errorf("repeat = %+v", repeat.Repeat)
	case inner.Script != "where" || inner.ID != "where":
		t.Errorf("first step inside = %+v", inner)
	case branch.If != `{{eq .steps.where.exit "0"}}`:
		t.Errorf("if = %+v", branch)
	case dice.Script != "dice" || dice.Capture["number"] != `number=(\d)`:
		t.Errorf("then step = %+v", dice)
	}

	pressDrawn(a, letter('r'))
	if run := l.last(t); !slices.Equal(run.argv[1:], []string{"flow", "run", "explore"}) {
		t.Errorf("r ran %v", run.argv)
	}
}

func TestInspectMovesAndRemovesSteps(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "order", "steps:\n  - {id: one, run: echo 1}\n  - {id: two, run: echo 2}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('J'))
	f, _ := flow.Load(filepath.Join(flow.Dir(), "order"+flow.Ext))
	if f.Steps[0].ID != "two" {
		t.Fatalf("J did not move the step down: %+v", f.Steps)
	}
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyDelete}, enterPress)
	f, _ = flow.Load(filepath.Join(flow.Dir(), "order"+flow.Ext))
	if len(f.Steps) != 1 || f.Steps[0].ID != "two" {
		t.Errorf("del removed the wrong step: %+v", f.Steps)
	}
}

// A flow that is one loop, like the example explore, gets steps before and after the loop by
// selecting the gaps around it; tab hides the list of flows and gives it back.
func TestInspectAddsAtTheSelectedGap(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "loop", "steps:\n  - repeat: {times: 2}\n    steps:\n      - {id: inside, run: echo in}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	p := a.currentTab().focusedView().(*inspectPanel)
	if !p.focusChart || !p.list.Empty() {
		t.Fatalf("tab did not hide the list: chart focused %v, list %v", p.focusChart, p.list)
	}
	pressDrawn(a, upPress)
	if p.selected != gapKey(chartSlot{}, 0) {
		t.Fatalf("up from the loop selected %q, want the gap before it", p.selected)
	}
	addCommand(a, "echo before")
	for range 6 { // gap, loop, gap, inside, gap, and the gap after the loop
		pressDrawn(a, downPress)
	}
	if p.selected != gapKey(chartSlot{}, 2) {
		t.Fatalf("selected %q, want the gap after the loop", p.selected)
	}
	addCommand(a, "echo after")
	f, _ := flow.Load(filepath.Join(flow.Dir(), "loop"+flow.Ext))
	if len(f.Steps) != 3 || f.Steps[0].Run != "echo before" || f.Steps[1].Repeat == nil || f.Steps[2].Run != "echo after" {
		t.Errorf("steps = %+v", f.Steps)
	}
	pressDrawn(a, tabPress)
	if p.focusChart || p.list.Empty() {
		t.Errorf("tab did not bring the list back: chart focused %v, list %v", p.focusChart, p.list)
	}
}

func addCommand(a *App, command string) {
	pressDrawn(a, letter('a'), letter('c'))
	typeDrawn(a, command)
	pressDrawn(a, enterPress)
}

func saveFlow(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(flow.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flow.Dir(), name+flow.Ext), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A double click on a step tells what it does; dragging the chart moves it.
func TestChartDoubleClickAndDrag(t *testing.T) {
	a := testApp(t)
	saveScript(t, "where", "description: Prints the folder\nrun: pwd\n")
	saveFlow(t, "one", "steps:\n  - {id: here, script: where}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	p := a.currentTab().focusedView().(*inspectPanel)
	block, _ := p.chart.block(stepKey("0"))
	frame, _ := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	at := contentRect(frame).Min.Add(p.chartArea.Min).Add(block.rect.Min.Sub(p.pan)).Add(image.Pt(2, 1))
	for range 2 {
		a.handleEvent(uv.MouseClickEvent{X: at.X, Y: at.Y, Button: uv.MouseLeft})
		a.handleEvent(uv.MouseReleaseEvent{X: at.X, Y: at.Y, Button: uv.MouseLeft})
	}
	if a.mode != infoMode || !slices.ContainsFunc(a.info.lines, func(l styledLine) bool { return l.text == "Prints the folder" }) {
		t.Fatalf("double click: mode %v, lines %+v", a.mode, a.info.lines)
	}
	press(a, escPress)
	pan := p.pan
	a.handleEvent(uv.MouseClickEvent{X: at.X, Y: at.Y, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseMotionEvent{X: at.X - 5, Y: at.Y - 2, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseReleaseEvent{X: at.X - 5, Y: at.Y - 2, Button: uv.MouseLeft})
	if p.pan != pan.Add(image.Pt(5, 2)) || a.mode == infoMode {
		t.Errorf("drag moved the chart from %v to %v, want 5 right and 2 down", pan, p.pan)
	}
}

func TestZoomChangesTheBoxes(t *testing.T) {
	f := flow.Flow{Steps: []flow.Step{{ID: "a", Run: "echo a"}}}
	pal := newPalette(themes[0], false)
	near := buildChart(f, chartStyle{pal: &pal, zoom: 2, describe: func(string) string { return "" }})
	far := buildChart(f, chartStyle{pal: &pal, zoom: 0, describe: func(string) string { return "" }})
	if near.size.X <= far.size.X {
		t.Errorf("zooming in did not widen the chart: %v vs %v", near.size, far.size)
	}
}

func writeTestLog(t *testing.T, at time.Time, name string, size int) logstore.Entry {
	t.Helper()
	path := logstore.Path(name, at)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x\n", size/2)), 0o600); err != nil {
		t.Fatal(err)
	}
	return logstore.Entry{Path: path, Name: name, At: at.Truncate(time.Second), Size: int64(size)}
}

func TestLogsPanelShowsTheDayAndClearsIt(t *testing.T) {
	a := testApp(t)
	now := time.Now()
	today := dateOf(now).Add(10 * time.Hour)
	failed := writeTestLog(t, today, "greet", 400)
	writeTestLog(t, today.Add(time.Minute), "pane3", 100)
	writeTestLog(t, today.AddDate(0, 0, -1), "older", 50)
	exit := 2
	if err := script.AddRun(runsPath(), script.Run{Script: "greet", At: today, Log: failed.Path, Exit: &exit}); err != nil {
		t.Fatal(err)
	}
	pressDrawn(a, prefixPress, letter('v'))
	p, ok := a.currentTab().focusedView().(*logsPanel)
	if !ok {
		t.Fatal("prefix+h did not open the logs panel")
	}
	p.day = dateOf(today)
	rows := p.rows()
	if len(rows) != 2 || rows[1].run == nil || runMark(rows[1].run) != "✗" {
		t.Fatalf("rows = %+v", rows)
	}
	if stats := p.statsByDay()[dateOf(today)]; stats.logs != 2 || stats.size != 500 || stats.runs != 1 || stats.failed != 1 {
		t.Errorf("stats = %+v", stats)
	}
	pressDrawn(a, letter('s'), letter('s'))
	if p.order != logsBySize || p.rows()[0].entry.Name != "greet" {
		t.Errorf("sort by size: %s, first %s", p.order, p.rows()[0].entry.Name)
	}
	pressDrawn(a, leftPress)
	if !p.day.Equal(dateOf(today).AddDate(0, 0, -1)) {
		t.Errorf("left went to %v", p.day)
	}
	pressDrawn(a, rightPress, uv.KeyPressEvent{Code: uv.KeyDelete}, letter('d'), enterPress)
	left, _ := logstore.List(logstore.LogsDir())
	if len(left) != 1 || left[0].Name != "older" {
		t.Errorf("after clearing the day, left %+v", left)
	}
	runs, _ := script.Runs(runsPath())
	if runs[0].Log != "" {
		t.Errorf("the run still points at its deleted log: %+v", runs[0])
	}
}

func TestLogBarsGoFromLightestToHeaviest(t *testing.T) {
	if fraction(10, 10, 110) != 0 || fraction(110, 10, 110) != 1 || fraction(60, 10, 110) != 0.5 || fraction(5, 5, 5) != 1 {
		t.Error("fractions are not placed between the lightest and the heaviest")
	}
}
