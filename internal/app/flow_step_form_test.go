package app

import (
	"image"
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
)

// setCell types text over the focused cell's: enter, ctrl+u, the text, and esc.
func setCell(a *App, text string) {
	pressDrawn(a, enterPress, clearLine)
	typeDrawn(a, text)
	pressDrawn(a, escPress)
}

// e on an if changes its condition in its form, keeping the steps of its branches, and esc
// leaves it as it was when the config says to.
func TestEditingAStepKeepsWhatItsFormDoesNotAsk(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "branch", `steps:
  - {id: test, run: 'true', capture: {state: '(\w+)'}}
  - if: ok
    then: [{run: 'echo yes'}]
    else: [{run: 'echo no'}]
`)
	pressDrawn(a, prefixPress, letter('i'), tabPress, downPress, letter('e'))
	form := openStepForm(t, a)
	if !strings.Contains(screenText(a), `state == "value"`) {
		t.Errorf("the if's examples do not use the flow's variables:\n%s", screenText(a))
	}
	a.config.Flows.LeaveForm = config.LeaveDiscard
	pressDrawn(a, enterPress, clearLine, escPress, escPress)
	if steps := loadFlowSteps(t, "branch"); steps[1].If != "ok" {
		t.Fatalf("esc changed the if to %q", steps[1].If)
	}
	if !strings.Contains(form.hints(), "save") || form.grid.buttons[len(form.grid.buttons)-1] != "save" {
		t.Errorf("editing hints %q, buttons %v", form.hints(), form.grid.buttons)
	}
	pressDrawn(a, letter('e'))
	setCell(a, `state == "ready"`)
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "branch")
	if steps[1].If != `state == "ready"` || len(steps[1].Then) != 1 || len(steps[1].Else) != 1 {
		t.Errorf("the if became %+v", steps[1])
	}
}

// A prompt's secret is a toggle in its form: enter on it makes the prompt secret.
func TestAPromptFormMakesItSecret(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "login", "steps:\n  - {prompt: 'password:', var: pw}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('e'))
	openStepForm(t, a)
	pressDrawn(a, letter('j'), letter('j'), letter('j'), letter('j'), enterPress, ctrlS)
	if steps := loadFlowSteps(t, "login"); !steps[0].Secret || steps[0].Prompt != "password:" || steps[0].Var != "pw" {
		t.Errorf("the prompt became %+v", steps[0])
	}
}

// A switch's cases are a list in its form; the cases kept keep their steps, and ctrl+s takes
// the list as it is, even open.
func TestEditingASwitchKeepsTheStepsOfItsCases(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "sort", `steps:
  - switch: answer
    cases:
      - {is: bug, steps: [{run: 'echo bug'}]}
      - {is: [flaky, env], steps: [{run: 'echo again'}]}
`)
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('e'), letter('j'), enterPress, ctrlDelete, ctrlDown, enterPress)
	typeDrawn(a, "docs")
	pressDrawn(a, ctrlS) // with the list open and docs still typed in
	steps := loadFlowSteps(t, "sort")
	cases := steps[0].Cases
	if len(cases) != 2 || strings.Join(cases[0].Is, "|") != "flaky|env" || len(cases[0].Steps) != 1 || len(cases[1].Steps) != 0 {
		t.Errorf("cases %+v", cases)
	}
}

// A value that does not fit keeps the form open, its row in the failure color with the reason.
func TestAStepFormRefusesWhatDoesNotFit(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "pause", "steps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "wait")
	setCell(a, "soon")
	pressDrawn(a, ctrlS)
	openStepForm(t, a)
	if !strings.Contains(screenText(a), "✗ a duration, as 30s, 5m or 1h30m") {
		t.Errorf("the form does not say why:\n%s", screenText(a))
	}
	if len(loadFlowSteps(t, "pause")) != 0 {
		t.Error("a wait of soon was added")
	}
}

// A function's call asks a value for each of its params in one form, checked against their types.
func TestACallStepAsksItsValuesInOneForm(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "ship", "steps: []\n")
	saveFunction(t, "deploy", "variables:\n  env: {type: option, choices: [dev, prod]}\n  tries: {type: number, default: '1'}\nsteps: [{run: 'true'}]\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "function call")
	typeDrawn(a, "deploy")
	pressDrawn(a, enterPress)
	setCell(a, "staging")
	pressDrawn(a, ctrlS)
	if !strings.Contains(screenText(a), "must be one of dev, prod") {
		t.Errorf("the call's form took staging:\n%s", screenText(a))
	}
	setCell(a, "$target")
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "ship")
	if len(steps) != 1 || steps[0].Call != "deploy" || steps[0].With["env"] != "$target" || len(steps[0].With) != 1 {
		t.Errorf("steps %+v", steps)
	}
}

// Set takes any number of variables, a row each, added with + add variable.
func TestASetStepTakesARowPerVariable(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "vars", "steps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "set")
	typeDrawn(a, "a")
	pressDrawn(a, tabPress)
	setCell(a, "1")
	pressDrawn(a, letter('j'), enterPress)
	typeDrawn(a, "2b")
	if !strings.Contains(screenText(a), "a variable's name is letters") {
		t.Errorf("2b is not told apart:\n%s", screenText(a))
	}
	pressDrawn(a, clearLine)
	typeDrawn(a, "b")
	pressDrawn(a, tabPress)
	setCell(a, "$a")
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "vars")
	if len(steps) != 1 || steps[0].Set["a"] != "1" || steps[0].Set["b"] != "$a" || len(steps[0].Set) != 2 {
		t.Errorf("steps %+v", steps)
	}
	if steps[0].Kind() != flow.SetStep {
		t.Errorf("kind %q", steps[0].Kind())
	}
}

// A case whose value changes keeps its steps; a set row with a value and no name says so.
func TestFormsKeepStepsAndAskForNames(t *testing.T) {
	if cases := switchCases([]flow.Case{{Is: []string{"bug"}, Steps: []flow.Step{{Run: "fix"}}}, {Is: []string{"env"}}}, []string{"bugs", "env", "new"}); len(cases) != 3 || len(cases[0].Steps) != 1 || len(cases[2].Steps) != 0 {
		t.Errorf("cases %+v", cases)
	}
	a := testApp(t)
	saveFlow(t, "vars", "steps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "set")
	pressDrawn(a, tabPress)
	setCell(a, "1")
	pressDrawn(a, ctrlS)
	if !strings.Contains(screenText(a), "a name is needed for this value") || len(loadFlowSteps(t, "vars")) != 0 {
		t.Errorf("a value without a name was taken:\n%s", screenText(a))
	}
}

// A script with params is added with its with in one form, checked as a step runs it: without
// asking, so a param without a default needs a value. e changes the values later.
func TestAScriptStepAsksItsParamsInOneForm(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveScript(t, "deploy", "run: echo {{.env}} {{.tag}}\nparams:\n  - {name: env, choices: [dev, prod]}\n  - {name: tag, default: latest}\n")
	saveFlow(t, "ship", "steps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "script")
	typeDrawn(a, "deploy")
	pressDrawn(a, enterPress, ctrlS)
	if !strings.Contains(screenText(a), "the script needs it") {
		t.Errorf("env without a value was taken:\n%s", screenText(a))
	}
	setCell(a, "prod")
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "ship")
	if len(steps) != 1 || steps[0].Script != "deploy" || steps[0].With["env"] != "prod" || len(steps[0].With) != 1 {
		t.Fatalf("steps %+v", steps)
	}
	pressDrawn(a, letter('e'), letter('j'))
	setCell(a, "v2")
	pressDrawn(a, ctrlS)
	if steps := loadFlowSteps(t, "ship"); steps[0].With["tag"] != "v2" || steps[0].ID != "deploy" {
		t.Errorf("e made %+v", steps[0])
	}
}

// A new set is typed in at once; tab completes a variable's name, shown dim before it is put
// in, and a variable the flow has already inherits its type.
func TestASetCompletesNamesAndTakesTheirTypes(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "typed", "variables:\n  counter: {type: number, default: '1'}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "set")
	typeDrawn(a, "cou")
	form := openStepForm(t, a)
	if ghost := form.grid.ghost(); ghost != "nter" || !strings.Contains(screenText(a), "counter") {
		t.Errorf("after cou the ghost is %q:\n%s", ghost, screenText(a))
	}
	pressDrawn(a, tabPress, tabPress) // the name, then the value
	setCell(a, "${counter + 1}")
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "typed")
	if len(steps) != 1 || steps[0].Set["counter"] != "${counter + 1}" || len(steps[0].Types) != 0 {
		t.Fatalf("set %+v, want counter inheriting its type", steps)
	}
}

// esc saves what changed in a step's form; ask, the default, asks first, where n leaves the step
// as it was; a save esc cannot make keeps the form, and esc again leaves it.
func TestEscSavesAStepsFormOrAsksFirst(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "leave", "steps:\n  - {wait: 5s}\n")
	a.config.Flows.LeaveForm = config.LeaveSaves
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('e'))
	setCell(a, "7s")
	pressDrawn(a, escPress)
	if steps := loadFlowSteps(t, "leave"); steps[0].Wait != "7s" {
		t.Fatalf("esc did not save: %+v", steps[0])
	}

	a.config.Flows.LeaveForm = config.LeaveAsks
	pressDrawn(a, letter('e'))
	setCell(a, "9s")
	pressDrawn(a, escPress)
	if form := openStepForm(t, a); !form.asking || !strings.Contains(screenText(a), "save what changed?") {
		t.Fatalf("esc did not ask:\n%s", screenText(a))
	}
	pressDrawn(a, letter('n'))
	if steps := loadFlowSteps(t, "leave"); steps[0].Wait != "7s" {
		t.Errorf("n saved: %+v", steps[0])
	}

	a.config.Flows.LeaveForm = config.LeaveSaves
	pressDrawn(a, letter('e'))
	setCell(a, "soon")
	pressDrawn(a, escPress)
	form := openStepForm(t, a)
	if !strings.Contains(form.message, "esc again") {
		t.Fatalf("a refused save on leaving: %q", form.message)
	}
	pressDrawn(a, escPress)
	if _, open := a.currentTab().focusedView().(*inspectPanel).editor.(*stepForm); open {
		t.Error("esc again kept the form")
	}
	if steps := loadFlowSteps(t, "leave"); steps[0].Wait != "7s" {
		t.Errorf("the refused value was saved: %+v", steps[0])
	}
}

// A form's text takes several lines: enter starts a new one, esc leaves the cell, and a
// condition not typed in shows laid out by its logic, a part a line.
func TestAFormsTextTakesLinesAndAConditionShowsByItsLogic(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "lines", "steps: [{run: 'true'}]\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "if")
	pressDrawn(a, enterPress)
	typeDrawn(a, "(ready and up)")
	pressDrawn(a, enterPress)
	typeDrawn(a, "or forced")
	pressDrawn(a, escPress)
	screen := screenText(a)
	for _, want := range []string{"  (\n", "      ready\n", "    and\n", "  )\n", "or\n", "  forced"} {
		if !strings.Contains(screen, want[:len(want)-1]) {
			t.Errorf("the condition does not show %q:\n%s", want, screen)
		}
	}
	pressDrawn(a, ctrlS)
	if steps := loadFlowSteps(t, "lines"); len(steps) != 2 || steps[1].If != "(ready and up)\nor forced" {
		t.Fatalf("steps %+v", steps)
	}
}

// An object's fields complete after its name, the next one shown dim, and tab puts it in.
func TestAnObjectsFieldsComplete(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "objects", "variables:\n  obj: '{\"isOnline\": true, \"user\": {\"name\": \"$login\"}}'\nsteps: [{run: 'true'}]\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "if")
	pressDrawn(a, enterPress)
	typeDrawn(a, "obj")
	form := openStepForm(t, a)
	if ghost := form.grid.ghost(); ghost != ".isOnline" {
		t.Errorf("after obj the ghost is %q", ghost)
	}
	typeDrawn(a, ".user.")
	if ghost := form.grid.ghost(); ghost != "name" {
		t.Errorf("after obj.user. the ghost is %q", ghost)
	}
	pressDrawn(a, tabPress)
	if got := form.grid.cell().field.String(); got != "obj.user.name" {
		t.Errorf("tab put in %q", got)
	}
}

// A set's JSON closes its braces and quotes as it is typed, enter adds the comma, it shows a
// member a line, and an object that reads itself is refused.
func TestASetsJSONIsTypedAndShownAsJSON(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "json", "variables: {login: ana}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "set")
	typeDrawn(a, "user")
	pressDrawn(a, tabPress, enterPress)
	typeDrawn(a, `{"name": "$login"`)
	pressDrawn(a, enterPress)
	typeDrawn(a, `"online": true`)
	form := openStepForm(t, a)
	if got := form.grid.cell().field.String(); got != "{\"name\": \"$login\",\n  \"online\": true}" {
		t.Fatalf("typed %q", got)
	}
	pressDrawn(a, escPress)
	for _, want := range []string{"{", `  "name": "$login",`, `  "online": true`, "}"} {
		if !strings.Contains(screenText(a), want) {
			t.Errorf("the JSON does not show %q:\n%s", want, screenText(a))
		}
	}
	pressDrawn(a, enterPress, clearLine)
	typeDrawn(a, `{"me": "$user`)
	pressDrawn(a, escPress, ctrlS)
	if !strings.Contains(form.message+form.problem(0), "reads itself") {
		t.Errorf("an object reading itself: %q", form.message)
	}
}

// Letters work on a form's rows while nothing is typed: shift+arrows mark rows, d doubles, c and
// v copy and paste, x cuts, z undoes, esc lets the marks go, and ctrl+click marks one.
func TestLettersWorkOnAFormsRows(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "rows", "steps:\n  - set: {a: '1', b: '2', c: '3'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('e'))
	form := openStepForm(t, a)
	names := func() string {
		var out []string
		for _, v := range form.formValues() {
			out = append(out, v.name)
		}
		return strings.Join(out, " ")
	}
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyDown, Mod: uv.ModShift}, letter('d'))
	if got := names(); got != "a b a b c" {
		t.Fatalf("d on a and b marked: %q", got)
	}
	pressDrawn(a, letter('z'))
	if got := names(); got != "a b c" {
		t.Fatalf("z: %q", got)
	}
	pressDrawn(a, letter('x'))
	if got := names(); got != "a c" {
		t.Fatalf("x on b: %q", got)
	}
	pressDrawn(a, letter('v'))
	if got := names(); got != "a c b" {
		t.Fatalf("v after c: %q", got)
	}
	row, _ := form.grid.focusRect()
	pressDrawn(a, letter('k'))
	a.handleMouse(uv.MouseClickEvent{X: row.Min.X, Y: row.Min.Y, Button: uv.MouseLeft, Mod: uv.ModCtrl})
	if !form.grid.marked[2] {
		t.Errorf("ctrl+click did not mark b: %v", form.grid.marked)
	}
	pressDrawn(a, escPress)
	if len(form.grid.marked) != 0 || p(a).editor == nil {
		t.Errorf("esc: marks %v, editor %v", form.grid.marked, p(a).editor)
	}
}

func p(a *App) *inspectPanel { return a.currentTab().focusedView().(*inspectPanel) }

// A set of an object's field inherits the field's type: a boolean takes true, false or none,
// and giving a variable another type than its own is told.
func TestASetOfAFieldInheritsItsTypeAndRetypingIsTold(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "fields", "variables:\n  obj: '{\"isOnline\": false}'\n  count: {type: number, default: '1'}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "set")
	typeDrawn(a, "obj.isOnline")
	pressDrawn(a, escPress)
	form := openStepForm(t, a)
	value := form.grid.at(0, stepFormValueColumn)
	if kind := form.grid.at(0, stepFormTypeColumn).value(); kind != "inherit" || value.kind != optionCell || !slices.Equal(value.options, []string{"true", "false", "none"}) {
		t.Fatalf("type %q, value %+v", kind, value)
	}
	if problem := form.problem(0); problem != "" {
		t.Errorf("obj.isOnline is refused: %q", problem)
	}
	pressDrawn(a, letter('j'))
	form.grid.addRow(form.row(form.spec.rowField))
	form.grid.at(1, stepFormLabelColumn).field = newTextField("count")
	form.grid.at(1, stepFormTypeColumn).field = newTextField("text")
	if problem := form.problem(1); !strings.Contains(problem, "count is a number already") {
		t.Errorf("count as text: %q", problem)
	}
}
