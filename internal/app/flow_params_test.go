package app

import (
	"image"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/script"
	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

const calcFlow = `variables:
  a: {type: number, required: true, prompt: First number}
  b: {type: number, default: "2"}
  text: hi
steps:
  - {run: 'echo $a'}
`

// r on a flow with a required parameter empty opens its form; the form refuses what the types
// refuse, runs with what it has, and r then runs again with the values it remembers.
func TestFlowParamsFormRunsAndRemembers(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveFlow(t, "calc", calcFlow)
	pressDrawn(a, prefixPress, letter('i'))
	inspect := a.currentTab().focused
	launched := len(l.programs)

	pressDrawn(a, letter('r'))
	if a.mode != formMode || len(l.programs) != launched {
		t.Fatalf("r with a required parameter empty: mode %v, %d launches", a.mode, len(l.programs)-launched)
	}
	if screen := screenText(a); !strings.Contains(screen, "First number") || !strings.Contains(screen, "run calc") {
		t.Errorf("form does not show the flow's parameters:\n%s", screen)
	}
	typeDrawn(a, "x")
	pressDrawn(a, enterPress)
	if a.mode != formMode || !strings.Contains(a.statusMessage, "a must be a number") {
		t.Fatalf("a non-number ran: mode %v, status %q", a.mode, a.statusMessage)
	}
	pressDrawn(a, clearLine)
	typeDrawn(a, "5")
	pressDrawn(a, tabPress, tabPress, clearLine)
	typeDrawn(a, "hello")
	pressDrawn(a, enterPress)
	want := []string{"flow", "run", "calc", "a=5", "text=hello"} // b stays at its default, 2
	if got := l.last(t).argv[1:]; a.mode == formMode || !slices.Equal(got, want) {
		t.Fatalf("the form ran %v, want %v", got, want)
	}

	a.focusPane(a.currentTab(), inspect)
	pressDrawn(a, letter('r'))
	if got := l.last(t).argv[1:]; a.mode == formMode || !slices.Equal(got, want) {
		t.Errorf("r ran %v, want the remembered %v", got, want)
	}

	a.focusPane(a.currentTab(), inspect)
	pressDrawn(a, letter('p'))
	if a.mode != formMode || a.form.values["a"] != "5" || a.form.values["text"] != "hello" {
		t.Errorf("p: mode %v, values %v", a.mode, a.form.values)
	}
	pressDrawn(a, escPress)
}

func TestFlowParamsFormNeedsParameters(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "plain", "steps:\n  - {run: 'true'}\n")
	pressDrawn(a, prefixPress, letter('i'), letter('p'))
	if a.mode == formMode || !strings.Contains(a.statusMessage, "no variables") {
		t.Errorf("mode %v, status %q", a.mode, a.statusMessage)
	}
}

// The add menu puts prompt, set and unset steps in, each from its form in place of the chart.
func TestInspectAddsVariableSteps(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "vars", "variables: {b: \"1\"}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "prompt")
	setCell(a, "New b?")
	pressDrawn(a, letter('j'))
	setCell(a, "b")
	pressDrawn(a, ctrlS)
	addStep(a, "set")
	pressDrawn(a, enterPress)
	typeDrawn(a, "sum")
	pressDrawn(a, tabPress)
	setCell(a, "$b")
	pressDrawn(a, ctrlS)
	addStep(a, "unset")
	addItems(a, "sum", "steps.x", "b")
	pressDrawn(a, ctrlS)
	if a.statusMessage != "" {
		t.Fatalf("status: %s", a.statusMessage)
	}
	f, err := flow.Load(filepath.Join(flow.Dir(), "vars"+flow.Ext))
	if err != nil {
		t.Fatal(err)
	}
	prompt, _ := f.Step("0")
	set, _ := f.Step("1")
	unset, _ := f.Step("2")
	switch {
	case prompt.Prompt != "New b?" || prompt.Var != "b":
		t.Errorf("prompt = %+v", prompt)
	case set.Set["sum"] != "$b":
		t.Errorf("set = %+v", set)
	case !slices.Equal(unset.Unset, []string{"sum", "steps.x", "b"}):
		t.Errorf("unset = %+v", unset)
	}
	chart := chartText(a.currentTab().focusedView().(*inspectPanel).chart)
	for _, want := range []string{"? prompt", "asks you", "= set", "∅ unset", "sum, steps.x, b"} {
		if !strings.Contains(chart, want) {
			t.Errorf("chart lacks %q:\n%s", want, chart)
		}
	}
}

const tokenFlow = `variables:
  token: {from: 'env:DEPLOY_TOKEN', secret: true, prompt: Deploy token}
  env: dev
steps:
  - {run: 'echo $env'}
`

// A param whose variable is set is not asked; unset, the form asks it masked, and the value
// typed goes to the run as that variable, never on its command line nor remembered.
func TestAFlowParamFromTheEnvironment(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveFlow(t, "ship", tokenFlow)
	pressDrawn(a, prefixPress, letter('i'))
	inspect := a.currentTab().focused

	pressDrawn(a, letter('p'))
	if a.mode != formMode || len(a.form.fields) != 2 || !a.form.fields[slices.IndexFunc(a.form.fields, func(p script.Param) bool { return p.Name == "token" })].Secret {
		t.Fatalf("p with DEPLOY_TOKEN unset: mode %v, fields %+v; want token asked, masked", a.mode, a.form.fields)
	}
	pressDrawn(a, clearLine) // token first, as the flow writes it
	typeDrawn(a, "typed")
	if strings.Contains(screenText(a), "typed") {
		t.Errorf("the token shows as typed:\n%s", screenText(a))
	}
	pressDrawn(a, enterPress)
	run := l.last(t)
	if slices.ContainsFunc(run.argv, func(arg string) bool { return strings.Contains(arg, "typed") }) || !slices.Contains(run.env, "DEPLOY_TOKEN=typed") {
		t.Fatalf("ran %v with env %v; want the token only in the environment", run.argv, run.env)
	}
	if remembered := a.memory.FlowValues("ship"); remembered["token"] != "" {
		t.Errorf("remembered the token: %v", remembered)
	}

	a.shellEnv = []string{"DEPLOY_TOKEN=from-env"}
	a.focusPane(a.currentTab(), inspect)
	pressDrawn(a, letter('p'))
	if len(a.form.fields) != 1 || a.form.fields[0].Name != "env" {
		t.Errorf("p with DEPLOY_TOKEN set asks %+v, want only env", a.form.fields)
	}
}

// a on the params card opens the params table on a new row, its name typed in at once; the type
// is picked from a menu, an option's options are a list with its default marked, and each row is
// saved as it changes.
func TestTheParamsTableAddsEveryType(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 50)
	saveFlow(t, "typed", "steps:\n  - {id: one, run: 'true'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, upPress)
	p := a.currentTab().focusedView().(*inspectPanel)
	if p.selected != paramsKey || !strings.Contains(screenText(a), "$ variables") {
		t.Fatalf("up twice from the first step selected %q:\n%s", p.selected, screenText(a))
	}
	pressDrawn(a, letter('a'))
	typeDrawn(a, "env")
	pressDrawn(a, enterPress, letter('h'))
	screen := screenText(a)
	for _, kind := range flow.ParamTypes {
		if !strings.Contains(screen, kind+"  "+paramTypeNotes[kind]) {
			t.Errorf("the help lacks the type %s:\n%s", kind, screen)
		}
	}
	pressDrawn(a, enterPress, letter('j'), letter('j'), letter('j'), enterPress, letter('l'), letter('l'))
	if !strings.Contains(screenText(a), "no options") {
		t.Errorf("an option without options does not say so:\n%s", screenText(a))
	}
	pressDrawn(a, enterPress)
	for _, option := range []string{"dev", "prod"} {
		pressDrawn(a, ctrlDown, enterPress)
		typeDrawn(a, option)
		pressDrawn(a, enterPress)
	}
	pressDrawn(a, spacePress, escPress)
	if !strings.Contains(screenText(a), `"prod" · 2 options`) {
		t.Errorf("the value cell does not show prod and the count:\n%s", screenText(a))
	}
	pressDrawn(a, escPress)
	got := loadFlowParams(t, "typed")["env"]
	if got.Type != flow.OptionParam || !slices.Equal(got.Choices, []string{"dev", "prod"}) || got.Default != "prod" {
		t.Fatalf("saved %+v, want an option of dev and prod, prod by default", got)
	}
	if !strings.Contains(screenText(a), "env  option dev|prod = prod") {
		t.Errorf("the params card does not list env:\n%s", screenText(a))
	}

	pressDrawn(a, letter('a'))
	typeDrawn(a, "token")
	pressDrawn(a, enterPress, letter('l'), letter('l'))
	setCell(a, "TOKEN")
	pressDrawn(a, escPress)
	if got := loadFlowParams(t, "typed")["token"]; got.From != "env:TOKEN" {
		t.Errorf("saved %+v, want it read from $TOKEN", got)
	}

	pressDrawn(a, letter('p'))
	field := a.form.fields[slices.IndexFunc(a.form.fields, func(p script.Param) bool { return p.Name == "env" })]
	if !slices.Equal(field.Choices, []string{"dev", "prod"}) {
		t.Errorf("the form's env is %+v, want its choices", field)
	}
}

// A list param's default is a list of items, each text or JSON, saved as a JSON list.
func TestAListParamTakesItsItemsInAList(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "hosts", "variables: {hosts: {type: list, default: '[\"a\"]'}}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, enterPress, letter('l'), letter('l'))
	addItems(a, "b c", `{"port": 1}`)
	if !strings.Contains(screenText(a), `"a" · 3 items`) {
		t.Errorf("the value cell does not show a and the count:\n%s", screenText(a))
	}
	if got := loadFlowParams(t, "hosts")["hosts"]; got.Default != `["a","b c",{"port":1}]` {
		t.Errorf("saved %q", got.Default)
	}
}

func loadFlowParams(t *testing.T, name string) map[string]flow.Param {
	t.Helper()
	f, err := flow.Load(filepath.Join(flow.Dir(), name+flow.Ext))
	if err != nil {
		t.Fatal(err)
	}
	return f.Params
}

// openParamsTable is the params table the focused inspect shows in place of its chart.
func openParamsTable(t *testing.T, a *App) *paramsTable {
	t.Helper()
	table, ok := a.currentTab().focusedView().(*inspectPanel).editor.(*paramsTable)
	if !ok {
		t.Fatalf("the inspect shows no params table:\n%s", screenText(a))
	}
	return table
}

// A row that is not valid is not saved: its cell turns the failure color and says why, the table
// closes with a notice, and opening it again brings the row back as it was left.
func TestAParamRowThatIsNotValidWaitsToBeFixed(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "ports", "variables: {env: dev}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, letter('a'))
	typeDrawn(a, "port")
	pressDrawn(a, enterPress, letter('h'), enterPress, letter('j'), enterPress, letter('l'), letter('l'))
	setCell(a, "x")
	if !strings.Contains(screenText(a), `✗ value: the value must be a number, not "x"`) {
		t.Errorf("the table does not say why the row waits:\n%s", screenText(a))
	}
	pressDrawn(a, escPress)
	if got := loadFlowParams(t, "ports")["port"]; got.Default != "" || got.Type != flow.NumberParam {
		t.Fatalf("saved %+v, want the number as it was before x", got)
	}
	if !strings.Contains(a.notice, "1 variable not saved") {
		t.Errorf("notice %q", a.notice)
	}
	pressDrawn(a, enterPress)
	table := openParamsTable(t, a)
	if name, p := table.param(table.grid.row); name != "port" || p.Default != "x" {
		t.Fatalf("the table opened again on %q %+v, want the row left", name, p)
	}
	setCell(a, "8080")
	pressDrawn(a, escPress)
	if got := loadFlowParams(t, "ports")["port"]; got.Type != flow.NumberParam || got.Default != "8080" {
		t.Errorf("saved %+v", got)
	}
}

// ctrl+delete removes a param and K moves it, both saved at once; undo and
// redo walk them back and forth, and a name another row has is told.
func TestTheParamsTableRemovesMovesAndRenames(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "pair", "variables: {a: '1', b: '2', c: '3'}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, enterPress, letter('l'))
	setCell(a, "b")
	if !strings.Contains(screenText(a), "another variable is b") {
		t.Errorf("two rows named b are not told:\n%s", screenText(a))
	}
	pressDrawn(a, ctrlZ, letter('j'), letter('j'), ctrlDelete)
	if params := loadFlowParams(t, "pair"); len(params) != 2 || params["a"].Default != "1" {
		t.Fatalf("params %+v, want c removed and a as it was", params)
	}
	pressDrawn(a, ctrlZ)
	if _, back := loadFlowParams(t, "pair")["c"]; !back {
		t.Fatal("undo did not bring c back")
	}
	pressDrawn(a, ctrlY, letter('k'), moveUp, escPress)
	f, err := flow.Load(filepath.Join(flow.Dir(), "pair"+flow.Ext))
	if err != nil || !slices.Equal(f.ParamNames(), []string{"b", "a"}) {
		t.Errorf("params %v, %v; want b moved above a, c removed again", f.ParamNames(), err)
	}
}

// While the params are edited, the prefix still reaches every panel and tab, and the table is
// there when the inspect has the focus again.
func TestTheParamsTableLeavesNoctFree(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "free", "variables: {a: '1'}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, enterPress)
	tab := a.currentTab()
	pressDrawn(a, prefixPress, letter('t'))
	if a.currentTab() == tab || a.mode != normalMode {
		t.Fatalf("prefix+t did not open a tab while editing: mode %v", a.mode)
	}
	pressDrawn(a, prefixPress, backTab)
	if a.currentTab() != tab {
		t.Fatal("prefix+shift+tab did not come back")
	}
	openParamsTable(t, a)
	if hints := a.currentTab().focusedView().(*inspectPanel).hints(); !strings.Contains(hints, "esc close") || strings.Contains(hints, "\n") {
		t.Errorf("hints %q", hints)
	}
}

// A new row with values but no name says so, and is kept for when the table opens again.
func TestANewParamRowNeedsAName(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "named", "steps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, letter('a'), escPress, letter('l'))
	setCell(a, "dev")
	if !strings.Contains(screenText(a), "✗ name: a variable needs a name") {
		t.Errorf("the row does not ask for a name:\n%s", screenText(a))
	}
	pressDrawn(a, escPress)
	if !strings.Contains(a.notice, "1 variable not saved") {
		t.Errorf("notice %q", a.notice)
	}
}

// A param whose default its type refuses can still be removed.
func TestAParamWithABadDefaultCanBeRemoved(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "bad", "variables: {n: {type: number, default: '1'}, keep: x}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, enterPress, letter('l'), letter('l'))
	setCell(a, "z")
	pressDrawn(a, ctrlDelete, escPress)
	if params := loadFlowParams(t, "bad"); len(params) != 1 || params["keep"].Default != "x" {
		t.Errorf("params %+v, want n removed", params)
	}
}

// A click on a cell edits it as enter does, a click on + add param adds one, and the wheel
// moves through the rows.
func TestTheParamsTableTakesTheMouse(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "clicks", "variables: {a: '1', b: '2'}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, enterPress)
	table := openParamsTable(t, a)
	click := func(row, col int) {
		t.Helper()
		for _, h := range table.grid.hits {
			if h.row == row && h.col == col {
				a.handleEvent(uv.MouseClickEvent{X: h.rect.Min.X, Y: h.rect.Min.Y, Button: uv.MouseLeft})
				drawApp(a)
				return
			}
		}
		t.Fatalf("no cell %d,%d drawn", row, col)
	}
	click(1, paramNameColumn)
	if table.grid.row != 1 || table.grid.col != paramNameColumn || !table.grid.editing {
		t.Fatalf("the click left the focus on %d,%d, editing %v", table.grid.row, table.grid.col, table.grid.editing)
	}
	click(2, 0)
	if len(table.grid.rows) != 3 || !table.grid.editing || table.grid.col != paramNameColumn {
		t.Fatalf("a click on + add param: %d rows, editing %v", len(table.grid.rows), table.grid.editing)
	}
	pressDrawn(a, escPress)
	at := table.grid.hits[0].rect.Min
	a.handleEvent(uv.MouseWheelEvent{X: at.X, Y: at.Y, Button: uv.MouseWheelUp})
	if table.grid.row != 1 {
		t.Errorf("the wheel up went to row %d", table.grid.row)
	}
}

// An option's default that is none of its options is told, and kept as the file has it.
func TestAnOptionDefaultOutsideItsOptionsIsKept(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "zones", "variables: {region: {type: option, choices: [a, b], default: c}}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, enterPress, ctrlRight, letter('h'), letter('h'))
	setCell(a, "Region")
	if !strings.Contains(screenText(a), `must be one of a, b, not "c"`) {
		t.Errorf("the default c is not told:\n%s", screenText(a))
	}
	pressDrawn(a, escPress)
	if got := loadFlowParams(t, "zones")["region"]; got.Default != "c" || got.Prompt != "" {
		t.Errorf("saved %+v, want the row kept as the file has it", got)
	}
}

// The cell under the mouse lights as a list's row does; the focused one is lit already.
func TestTheParamsTableLightsWhatTheMouseIsOver(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "hover", "variables: {a: '1', b: '2'}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, enterPress)
	table := openParamsTable(t, a)
	var name, focused gridHit
	for _, h := range table.grid.hits {
		switch {
		case h.row == 1 && h.col == paramNameColumn:
			name = h
		case h.row == 0 && h.col == 0:
			focused = h
		}
	}
	a.handleEvent(uv.MouseMotionEvent{X: name.rect.Min.X, Y: name.rect.Min.Y})
	if a.hoverTarget == "" || !strings.HasPrefix(a.hoverTarget, "row") {
		t.Fatalf("over b's name the hover is %q", a.hoverTarget)
	}
	a.handleEvent(uv.MouseMotionEvent{X: focused.rect.Min.X, Y: focused.rect.Min.Y})
	if a.hoverTarget != "" {
		t.Errorf("over the focused cell the hover is %q", a.hoverTarget)
	}
}

// Dragging over a cell's text marks it and copies it, as in a pane; typing replaces what is marked.
func TestDraggingOverAParamCellCopiesItsText(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "drag", "variables: {region: eu-central}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, upPress, enterPress)
	table := openParamsTable(t, a)
	var value gridHit
	for _, h := range table.grid.hits {
		if h.row == 0 && h.col == paramDefaultColumn {
			value = h
		}
	}
	at := value.rect.Min
	a.handleEvent(uv.MouseClickEvent{X: at.X + 3, Y: at.Y, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseMotionEvent{X: at.X + 10, Y: at.Y, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseReleaseEvent{X: at.X + 10, Y: at.Y, Button: uv.MouseLeft})
	field := &table.grid.at(0, paramDefaultColumn).field
	if got := field.selected(); got != "central" || a.notice != "copied to the clipboard" {
		t.Fatalf("the drag marked %q, notice %q; want central copied", got, a.notice)
	}
	typeDrawn(a, "west")
	if got := field.String(); got != "eu-west" {
		t.Errorf("typing over the marked text gave %q", got)
	}
}
