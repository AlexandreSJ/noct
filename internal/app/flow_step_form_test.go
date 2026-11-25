package app

import (
	"image"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// setCell types text over the focused cell's: enter, ctrl+u, the text, and enter.
func setCell(a *App, text string) {
	pressDrawn(a, enterPress, clearLine)
	typeDrawn(a, text)
	pressDrawn(a, enterPress)
}

// e on an if changes its condition in its form, keeping the steps of its branches, and esc
// leaves it as it was.
func TestEditingAStepKeepsWhatItsFormDoesNotAsk(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "branch", `steps:
  - {id: test, run: 'true', capture: {state: '(\w+)'}}
  - if: '{{.ok}}'
    then: [{run: 'echo yes'}]
    else: [{run: 'echo no'}]
`)
	pressDrawn(a, prefixPress, letter('i'), tabPress, downPress, letter('e'))
	form := openStepForm(t, a)
	if !strings.Contains(screenText(a), `{{eq .state "value"}}`) {
		t.Errorf("the if's examples do not use the flow's variables:\n%s", screenText(a))
	}
	pressDrawn(a, enterPress, clearLine, escPress, escPress)
	if steps := loadFlowSteps(t, "branch"); steps[1].If != "{{.ok}}" {
		t.Fatalf("esc changed the if to %q", steps[1].If)
	}
	if !strings.Contains(form.hints(), "save") || form.grid.buttons[len(form.grid.buttons)-1] != "save" {
		t.Errorf("editing hints %q, buttons %v", form.hints(), form.grid.buttons)
	}
	pressDrawn(a, letter('e'))
	setCell(a, `{{eq .state "ready"}}`)
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "branch")
	if steps[1].If != `{{eq .state "ready"}}` || len(steps[1].Then) != 1 || len(steps[1].Else) != 1 {
		t.Errorf("the if became %+v", steps[1])
	}
}

// A switch's cases are a list in its form; the cases kept keep their steps, and ctrl+s takes
// the list as it is, even open.
func TestEditingASwitchKeepsTheStepsOfItsCases(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "sort", `steps:
  - switch: '{{.answer}}'
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
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('a'), letter('w'))
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
	saveFunction(t, "deploy", "params:\n  env: {type: option, choices: [dev, prod]}\n  tries: {type: number, default: '1'}\nsteps: [{run: 'true'}]\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('a'), letter('f'))
	typeDrawn(a, "deploy")
	pressDrawn(a, enterPress)
	setCell(a, "staging")
	pressDrawn(a, ctrlS)
	if !strings.Contains(screenText(a), "must be one of dev, prod") {
		t.Errorf("the call's form took staging:\n%s", screenText(a))
	}
	setCell(a, "{{.target}}")
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "ship")
	if len(steps) != 1 || steps[0].Call != "deploy" || steps[0].With["env"] != "{{.target}}" || len(steps[0].With) != 1 {
		t.Errorf("steps %+v", steps)
	}
}

// Set takes any number of variables, a row each, added with + add variable.
func TestASetStepTakesARowPerVariable(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "vars", "steps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('a'), letter('v'), enterPress)
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
	setCell(a, "{{.a}}")
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "vars")
	if len(steps) != 1 || steps[0].Set["a"] != "1" || steps[0].Set["b"] != "{{.a}}" || len(steps[0].Set) != 2 {
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
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('a'), letter('v'), enterPress, tabPress)
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
	pressDrawn(a, prefixPress, letter('i'), tabPress, letter('a'), letter('s'))
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
