package flow

import (
	"slices"
	"testing"

	"github.com/AlexandreSJ/noct/internal/script"
)

// explore is like the example flow: a step, a loop with a capture and an if, and a step after.
const explore = `
params: {env: dev}
steps:
  - {id: where, run: pwd}
  - id: loop
    repeat: {until: '{{ne .steps.dice.number "6"}}', max: 5}
    steps:
      - {id: list, run: ls, capture: {count: '(\d+)'}}
      - if: '{{eq .steps.list.count "3"}}'
        then:
          - {id: dice, script: dice}
  - {id: after, run: 'echo {{.steps.dice.number}}'}
`

var diceScript = map[string]script.Script{"dice": {Name: "dice", Run: "echo number=4", Outputs: []string{"number"}}}

func lookupIn(scripts map[string]script.Script) func(string) (script.Script, bool) {
	return func(id string) (script.Script, bool) { s, ok := scripts[id]; return s, ok }
}

// names lists a scope as template text with its certainty, for comparing.
func names(vars []Variable) []string {
	var out []string
	for _, v := range vars {
		out = append(out, v.Template()+map[Certainty]string{Set: "", ThisPass: " ↻", Maybe: " ?"}[v.Certainty])
	}
	return out
}

func TestScopeAfterALoopHoldsItsStepsAndMaybeTheBranch(t *testing.T) {
	f := parse(t, explore)
	got := names(f.Scope("", "", 2, lookupIn(diceScript)))
	want := []string{
		"{{.env}}",
		"{{.steps.where.output}}", "{{.steps.where.exit}}", "{{.steps.where.ok}}",
		"{{.steps.list.output}}", "{{.steps.list.exit}}", "{{.steps.list.ok}}", "{{.steps.list.count}}",
		"{{.steps.dice.output}} ?", "{{.steps.dice.exit}} ?", "{{.steps.dice.ok}} ?", "{{.steps.dice.number}} ?",
	}
	if !slices.Equal(got, want) {
		t.Errorf("scope after the loop:\n got %q\nwant %q", got, want)
	}
}

func TestScopeInsideALoopIsThisPass(t *testing.T) {
	f := parse(t, explore)
	got := names(f.Scope("1", "steps", 1, lookupIn(diceScript)))
	if !slices.Contains(got, "{{.steps.list.count}} ↻") || !slices.Contains(got, "{{.steps.where.ok}}") {
		t.Errorf("before the if in the loop: %q", got)
	}
	if slices.ContainsFunc(got, func(n string) bool { return n == "{{.steps.dice.number}} ?" }) {
		t.Error("the loop's if is in scope before it runs")
	}
	until := names(f.Scope("1", "steps", 2, lookupIn(diceScript)))
	if !slices.Contains(until, "{{.steps.dice.number}} ?") {
		t.Errorf("the until does not see the if's step: %q", until)
	}
}

func TestWarningsPointAtTheStepThatReads(t *testing.T) {
	f := parse(t, `
steps:
  - {id: first, run: 'echo {{.steps.later.output}} {{.steps.nope.output}}'}
  - {id: later, run: 'echo {{.steps.first.bogus}}'}
  - if: 'true'
    then:
      - {id: branch, run: 'true'}
  - {run: 'echo {{.steps.branch.output}}'}
`)
	var got []string
	for _, w := range f.Warnings(nil) {
		got = append(got, w.Path+": "+w.Text)
	}
	want := []string{
		"0: step later has not run yet here",
		"0: there is no step nope",
		"1: step first has no bogus",
		"3: step branch may not have run here",
	}
	if !slices.Equal(got, want) {
		t.Errorf("warnings:\n got %q\nwant %q", got, want)
	}
}

func TestExploreWarnsOnlyAboutTheBranch(t *testing.T) {
	f := parse(t, explore)
	for _, w := range f.Warnings(lookupIn(diceScript)) {
		if w.Text != "step dice may not have run here" {
			t.Errorf("%s: %s", w.Path, w.Text)
		}
	}
}

func TestReferencesInsideActionsAndBranches(t *testing.T) {
	got := references(`{{if eq .steps.a.b "x"}}{{.steps.c.output}}{{else}}{{.env}}{{end}}`)
	want := []reference{{step: "a", field: "b"}, {step: "c", field: "output"}, {field: "env"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
