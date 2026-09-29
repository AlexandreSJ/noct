package flow

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestParamsTakeAPlainDefaultOrASpec(t *testing.T) {
	f := parse(t, `
params:
  env: dev
  a: {default: "3", type: number, prompt: First number}
  b: {type: number, required: true}
steps: [{run: 'true'}]
`)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	want := map[string]Param{
		"env": {Default: "dev"},
		"a":   {Default: "3", Type: NumberParam, Prompt: "First number"},
		"b":   {Type: NumberParam, Required: true},
	}
	for name, p := range want {
		if f.Params[name] != p {
			t.Errorf("param %s = %+v, want %+v", name, f.Params[name], p)
		}
	}
	if d := f.Defaults(); d["env"] != "dev" || d["a"] != "3" || d["b"] != "" {
		t.Errorf("defaults = %v", d)
	}
}

// A plain default saves back as plain text, so flows that never used a spec stay the same.
func TestParamsSaveBackPlainWhenTheyOnlyHaveADefault(t *testing.T) {
	f := parse(t, "params:\n  env: dev\n  a: {default: \"1\", type: number}\nsteps: [{run: 'true'}]\n")
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	again := parse(t, readFile(t, f.File))
	if again.Params["env"] != (Param{Default: "dev"}) || again.Params["a"] != (Param{Default: "1", Type: NumberParam}) {
		t.Errorf("reloaded params = %+v", again.Params)
	}
	if !strings.Contains(readFile(t, f.File), "env: dev") {
		t.Errorf("env did not stay plain:\n%s", readFile(t, f.File))
	}
}

func TestCheckParamsRefusesNonNumbersAndMissingRequired(t *testing.T) {
	f := parse(t, "params:\n  a: {type: number}\n  b: {required: true}\nsteps: [{run: 'true'}]\n")
	cases := []struct {
		values  map[string]string
		problem string
	}{
		{map[string]string{"a": "2.5", "b": "x"}, ""},
		{map[string]string{"a": "-4", "b": "x"}, ""},
		{map[string]string{"a": "two", "b": "x"}, "a must be a number"},
		{map[string]string{"a": "1", "b": " "}, "b is required"},
		{map[string]string{"a": "1", "b": "x", "c": "1"}, "has no parameter c"},
	}
	for _, c := range cases {
		err := f.CheckParams(c.values)
		if (c.problem == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), c.problem) {
			t.Errorf("%v: err %v, want %q", c.values, err, c.problem)
		}
	}
}

func TestInvalidParamTypeIsAProblem(t *testing.T) {
	f := parse(t, "params:\n  a: {type: date}\nsteps: [{run: 'true'}]\n")
	if err := f.Validate(nil); err == nil || !strings.Contains(err.Error(), "type") {
		t.Errorf("err = %v", err)
	}
}

func TestSetAndUnsetValidation(t *testing.T) {
	cases := map[string]string{
		"steps: [{set: {sum: '1'}}]":                         "",
		"steps: [{unset: [sum, steps.x]}]":                   "",
		"steps: [{prompt: 'Name?', var: name}]":              "",
		"steps: [{prompt: 'Name?'}]":                         "var",
		"steps: [{set: {steps: '1'}}]":                       "steps",
		"steps: [{set: {'a-b': '1'}}]":                       "a-b",
		"steps: [{unset: ['steps.']}]":                       "steps.",
		"steps: [{unset: []}]":                               "exactly one",
		"steps: [{run: 'true', var: x}]":                     "belong to a prompt",
		"steps: [{prompt: 'N?', var: n, type: date}]":        "type",
		"steps: [{prompt: '{{.x', var: n}]":                  "unclosed",
		"steps: [{set: {a: '1'}, run: 'true'}]":              "exactly one",
		"steps: [{prompt: 'N?', var: n, capture: {x: '.'}}]": "capture",
	}
	for text, problem := range cases {
		err := parse(t, text).Validate(nil)
		if problem == "" {
			if err != nil {
				t.Errorf("%s: %v", text, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), problem) {
			t.Errorf("%s: err %v, want %q", text, err, problem)
		}
	}
}

func TestSetCreatesAndOverwritesVariables(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
params: {a: "2"}
steps:
  - {id: sum, run: 'echo $(( {{.a}} + 3 ))'}
  - set: {sum: '{{.steps.sum.output}}', a: 'ten'}
  - run: 'echo sum={{.sum}} a={{.a}}'
`)
	result := r.Run(f, nil)
	if !result.OK || !strings.Contains(out.String(), "sum=5 a=ten") {
		t.Fatalf("ok %v:\n%s", result.OK, out)
	}
	if got := result.Steps["1"].Captures; got["sum"] != "5" || got["a"] != "ten" {
		t.Errorf("set's result keeps %v", got)
	}
}

func TestUnsetClearsVariablesAndStepResults(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
params: {a: "2", text: hi}
steps:
  - {id: sum, run: 'echo 5'}
  - set: {sum: '{{.steps.sum.output}}'}
  - unset: [a, sum, steps.sum]
  - run: 'echo "a=[{{.a}}] sum=[{{.sum}}] out=[{{.steps.sum.output}}] text=[{{.text}}]"'
`)
	result := r.Run(f, nil)
	if !result.OK || !strings.Contains(out.String(), "a=[] sum=[] out=[] text=[hi]") {
		t.Fatalf("ok %v:\n%s", result.OK, out)
	}
	if msg := result.Steps["2"].Message; !strings.Contains(msg, "a, sum, steps.sum") {
		t.Errorf("unset message = %q", msg)
	}
}

func TestPromptSetsAVariableFromTheAnswer(t *testing.T) {
	r, out := testRunner(t, nil)
	var asked []string
	r.Ask = func(q string) string { asked = append(asked, q); return "7" }
	attention := 0
	r.Attention = func() { attention++ }
	f := parse(t, `
params: {b: "3"}
steps:
  - {id: adjust, prompt: 'New b (was {{.b}})', var: b, default: '{{.b}}', type: number}
  - run: 'echo b={{.b}}'
`)
	result := r.Run(f, nil)
	if !result.OK || !strings.Contains(out.String(), "b=7") {
		t.Fatalf("ok %v:\n%s", result.OK, out)
	}
	if len(asked) != 1 || !strings.Contains(asked[0], "New b (was 3)") || !strings.Contains(asked[0], "[3]") {
		t.Errorf("asked %q", asked)
	}
	if attention != 1 {
		t.Errorf("attention asked %d times", attention)
	}
	if result.Steps["0"].Captures["b"] != "7" {
		t.Errorf("prompt's result keeps %v", result.Steps["0"].Captures)
	}
}

func TestPromptEmptyAnswerKeepsTheDefault(t *testing.T) {
	r, out := testRunner(t, nil)
	r.Ask = func(string) string { return "  " }
	f := parse(t, "params: {b: \"3\"}\nsteps:\n  - {prompt: New b, var: b, default: '{{.b}}'}\n  - run: 'echo b={{.b}}'\n")
	if result := r.Run(f, nil); !result.OK || !strings.Contains(out.String(), "b=3") {
		t.Errorf("ok %v:\n%s", result.OK, out)
	}
}

func TestPromptAsksAgainForANumberThenGivesUp(t *testing.T) {
	r, _ := testRunner(t, nil)
	answers := []string{"x", "4"}
	r.Ask = func(string) string { a := answers[0]; answers = answers[1:]; return a }
	f := parse(t, "steps:\n  - {prompt: N, var: n, type: number}\n")
	if result := r.Run(f, nil); !result.OK || result.Steps["0"].Captures["n"] != "4" {
		t.Errorf("second answer: %+v", result)
	}

	asks := 0
	r.Ask = func(string) string { asks++; return "" }
	result := r.Run(f, nil)
	if result.OK || asks != maxPromptTries || !strings.Contains(result.Steps["0"].Message, "number") {
		t.Errorf("no number: ok %v, asked %d, %+v", result.OK, asks, result.Steps["0"])
	}
}

// Nobody answers a run by a trigger: the prompt fails at once instead of waiting.
func TestPromptWithoutATerminalFails(t *testing.T) {
	r, _ := testRunner(t, nil)
	result := r.Run(parse(t, "steps:\n  - {prompt: N, var: n}\n  - run: 'true'\n"), nil)
	if _, next := result.Steps["1"]; result.OK || next || !strings.Contains(result.Steps["0"].Message, "nobody") {
		t.Errorf("ok %v, next ran %v, %+v", result.OK, next, result.Steps["0"])
	}
}

func TestScopeHoldsSetVariablesAndDropsUnsetOnes(t *testing.T) {
	f := parse(t, `
params: {a: "1", text: hi}
steps:
  - {id: sum, run: 'echo 1'}
  - {id: keep, set: {sum: '{{.steps.sum.output}}'}}
  - {id: ask, prompt: 'Name?', var: name}
  - unset: [a, sum, steps.sum]
  - run: 'echo {{.text}}'
`)
	at3 := names(f.Scope("", "", 3, nil))
	for _, want := range []string{"{{.a}}", "{{.sum}}", "{{.name}}", "{{.steps.sum.output}}"} {
		if !slices.Contains(at3, want) {
			t.Errorf("before unset, scope lacks %s: %q", want, at3)
		}
	}
	at4 := names(f.Scope("", "", 4, nil))
	for _, gone := range []string{"{{.a}}", "{{.sum}}", "{{.steps.sum.output}}"} {
		if slices.Contains(at4, gone) {
			t.Errorf("after unset, scope keeps %s: %q", gone, at4)
		}
	}
	if !slices.Contains(at4, "{{.text}}") || !slices.Contains(at4, "{{.name}}") {
		t.Errorf("after unset, scope lost text or name: %q", at4)
	}
	// A variable set by a step points at it, so its last value comes from that step's result.
	for _, v := range f.Scope("", "", 3, nil) {
		if v.Step == "" && v.Field == "sum" && v.Path != "1" {
			t.Errorf("sum's path = %q, want 1", v.Path)
		}
	}
}

func TestWarningsForVariablesNotSet(t *testing.T) {
	f := parse(t, `
params: {a: "1"}
steps:
  - run: 'echo {{.sum}}'
  - set: {sum: '1'}
  - unset: [a]
  - run: 'echo {{.a}} {{.sum}}'
  - if: 'true'
    then: [{set: {maybe: '1'}}]
  - run: 'echo {{.maybe}}'
`)
	var got []string
	for _, w := range f.Warnings(nil) {
		got = append(got, w.Path+": "+w.Text)
	}
	want := []string{
		"0: variable sum is not set yet here",
		"3: variable a was unset before here",
		"5: variable maybe may not be set here",
	}
	if !slices.Equal(got, want) {
		t.Errorf("warnings:\n got %q\nwant %q", got, want)
	}
}

// Testing a step reads the variables the steps before it set in the last run, and not the ones
// they unset.
func TestTestingAStepReadsVariablesOfTheLastRun(t *testing.T) {
	r, out := testRunner(t, nil)
	r.Ask = func(string) string { return "typed" }
	f := parse(t, `
params: {a: "1", gone: "x"}
steps:
  - set: {sum: '{{.a}}0'}
  - {prompt: Name, var: name}
  - unset: [gone]
  - run: 'echo "sum={{.sum}} name={{.name}} gone=[{{.gone}}]"'
`)
	previous := r.Run(f, nil)
	out.Reset()
	if _, err := r.RunSteps(f, nil, "3", "3", previous); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "sum=10 name=typed gone=[]") {
		t.Errorf("tested step read:\n%s", out)
	}
}

func readFile(t *testing.T, file string) string {
	t.Helper()
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
