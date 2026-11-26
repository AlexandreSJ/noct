package flow

import (
	"slices"
	"testing"

	"github.com/AlexandreSJ/noct/internal/script"
)

// explore is like the example flow: a step, a loop with a capture and an if, and a step after.
const explore = `
variables: {env: dev}
steps:
  - {id: where, run: pwd}
  - id: loop
    repeat: {until: dice.number != "6", max: 5}
    steps:
      - {id: list, run: ls, capture: {count: '(\d+)'}}
      - if: list.count == "3"
        then:
          - {id: dice, script: dice}
  - {id: after, run: 'echo $dice.number'}
`

var diceScript = map[string]script.Script{"dice": {Name: "dice", Run: "echo number=4", Outputs: []string{"number"}}}

func lookupIn(scripts map[string]script.Script) func(string) (script.Script, bool) {
	return func(id string) (script.Script, bool) { s, ok := scripts[id]; return s, ok }
}

// names lists a scope as template text with its certainty, for comparing.
func names(vars []Variable) []string {
	var out []string
	for _, v := range vars {
		out = append(out, v.Name()+map[Certainty]string{Set: "", ThisPass: " ↻", Maybe: " ?"}[v.Certainty])
	}
	return out
}

func TestScopeAfterALoopHoldsItsStepsAndMaybeTheBranch(t *testing.T) {
	f := parse(t, explore)
	got := names(f.Scope("", "", 2, lookupIn(diceScript)))
	want := []string{
		"env",
		"where.output", "where.exit", "where.ok",
		"output", "exit", "ok", // the last step's, which the loop changes and keeps
		"list.output", "list.exit", "list.ok", "list.count",
		"dice.output ?", "dice.exit ?", "dice.ok ?", "dice.number ?",
	}
	if !slices.Equal(got, want) {
		t.Errorf("scope after the loop:\n got %q\nwant %q", got, want)
	}
}

func TestScopeInsideALoopIsThisPass(t *testing.T) {
	f := parse(t, explore)
	got := names(f.Scope("1", "steps", 1, lookupIn(diceScript)))
	if !slices.Contains(got, "list.count ↻") || !slices.Contains(got, "where.ok") {
		t.Errorf("before the if in the loop: %q", got)
	}
	if slices.ContainsFunc(got, func(n string) bool { return n == "dice.number ?" }) {
		t.Error("the loop's if is in scope before it runs")
	}
	until := names(f.Scope("1", "steps", 2, lookupIn(diceScript)))
	if !slices.Contains(until, "dice.number ?") {
		t.Errorf("the until does not see the if's step: %q", until)
	}
}

func TestWarningsPointAtTheStepThatReads(t *testing.T) {
	f := parse(t, `
steps:
  - {id: first, run: 'echo $later.output $nope.output'}
  - {id: later, run: 'echo $first.bogus'}
  - if: true
    then:
      - {id: branch, run: 'true'}
  - {run: 'echo $branch.output'}
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
	f := parse(t, `variables: {env: dev}
steps:
  - {id: a, run: 'true', capture: {b: x}}
  - {id: c, run: 'true'}
`)
	got := f.references(read{text: `if(a.b == "x", any([c], s => s.output), steps.c.exit) ?? env`, expression: true}, nil)
	want := []reference{{step: "a", field: "b"}, {step: "c", field: "exit"}, {field: "env"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	got = f.references(read{text: `at $a.b: $join(map([c.output], s => s), " ") $env`}, nil)
	want = []reference{{step: "a", field: "b"}, {step: "c", field: "output"}, {field: "env"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Readable leaves out a step's field that its short name reads, and keeps the ones a later step
// set again, which only steps.<id> reaches.
func TestReadableKeepsOnlyTheNamesThatNeedTheirStep(t *testing.T) {
	f := parse(t, `variables: {env: dev}
steps:
  - {id: build, run: make, capture: {version: 'v(\S+)'}}
  - {id: test, run: go test}
`)
	got := names(Readable(f.Scope("", "", 2, nil)))
	want := []string{"env", "output", "exit", "ok", "version", "build.output", "build.exit", "build.ok"}
	if !slices.Equal(got, want) {
		t.Errorf("readable\n got %v\nwant %v", got, want)
	}
}

// A short name two steps set warns which one it reads; output and the like never do.
func TestScopeWarningForAShortNameTwoStepsSet(t *testing.T) {
	f := parse(t, `steps:
  - {id: build, run: make, capture: {version: 'v(\S+)'}}
  - {id: test, run: go test, capture: {version: 'v(\S+)'}}
  - {run: 'echo $version $output $build.version'}
`)
	var texts []string
	for _, w := range f.Warnings(nil) {
		texts = append(texts, w.Text)
	}
	want := []string{"scope: version is set by build and test; version reads test's, build.version reads build's"}
	if !slices.Equal(texts, want) {
		t.Errorf("warnings %q, want %q", texts, want)
	}
}
