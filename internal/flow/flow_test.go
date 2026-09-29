package flow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/script"
)

func testRunner(t *testing.T, scripts map[string]script.Script) (*Runner, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	return &Runner{
		Lookup: func(id string) (script.Script, bool) { s, ok := scripts[id]; return s, ok },
		Shell:  "/bin/sh",
		Dir:    t.TempDir(),
		Getenv: func(string) string { return "" },
		Stdin:  strings.NewReader(""),
		Out:    out,
		Sleep:  func(time.Duration) {},
	}, out
}

func parse(t *testing.T, text string) Flow {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "test"+Ext)
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCapturesFeedTheNextSteps(t *testing.T) {
	scripts := map[string]script.Script{
		"shout": {Name: "shout", Run: `echo "{{.word}}!"`, Params: []script.Param{{Name: "word"}}},
	}
	r, out := testRunner(t, scripts)
	f := parse(t, `
steps:
  - id: first
    run: printf 'noise\nid=42\n'
    capture: { id: 'id=(\d+)' }
  - id: second
    script: shout
    with: { word: 'got {{.steps.first.id}}' }
`)
	if err := f.Validate(r.Lookup); err != nil {
		t.Fatal(err)
	}
	result := r.Run(f, nil)
	if !result.OK {
		t.Fatalf("run failed: %s", out)
	}
	if got := result.Steps["0"].Captures["id"]; got != "42" {
		t.Errorf("capture = %q, want 42", got)
	}
	if !strings.Contains(out.String(), "got 42!") {
		t.Errorf("output lacks the captured value:\n%s", out)
	}
}

func TestIfTakesTheBranchTheConditionGives(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
params: { color: red }
steps:
  - if: '{{eq .color "red"}}'
    then: [ { run: echo took-then } ]
    else: [ { run: echo took-else } ]
`)
	result := r.Run(f, map[string]string{"color": "blue"})
	if !result.OK || !strings.Contains(out.String(), "took-else") || strings.Contains(out.String(), "took-then") {
		t.Fatalf("with blue, want only the else branch:\n%s", out)
	}
	if _, ran := result.Steps["0/else/0"]; !ran {
		t.Errorf("else step has no result: %v", result.Steps)
	}
}

// A repeat until starts each pass without the results of the pass before: a step that did not
// run this pass reads as empty, so the loop ends.
func TestRepeatForgetsStepsThatDidNotRunThisPass(t *testing.T) {
	r, out := testRunner(t, nil)
	counter := filepath.Join(r.Dir, "count")
	f := parse(t, `
steps:
  - repeat: { until: '{{ne .steps.six.value "6"}}', max: 5 }
    steps:
      - id: roll
        run: 'n=$(($(cat count 2>/dev/null || echo 0) + 1)); echo $n > count; echo roll=$n'
        capture: { n: 'roll=(\d+)' }
      - if: '{{eq .steps.roll.n "1"}}'
        then:
          - id: six
            run: echo value=6
            capture: { value: 'value=(\d)' }
`)
	result := r.Run(f, nil)
	if !result.OK {
		t.Fatalf("run failed:\n%s", out)
	}
	count, _ := os.ReadFile(counter)
	if strings.TrimSpace(string(count)) != "2" {
		t.Errorf("passes = %s, want 2: the six of pass 1 must not end pass 2", count)
	}
}

func TestRepeatStopsAtMax(t *testing.T) {
	r, _ := testRunner(t, nil)
	f := parse(t, `
steps:
  - repeat: { until: 'false', max: 3 }
    steps: [ { run: 'true' } ]
`)
	result := r.Run(f, nil)
	if result.OK || !strings.Contains(result.Steps["0"].Message, "3 passes") {
		t.Fatalf("want a failure after 3 passes, got %+v", result)
	}
}

func TestAFailingStepStopsTheFlowUnlessToldToGoOn(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
steps:
  - { run: 'exit 3', on_fail: continue }
  - { run: 'exit 4' }
  - { run: echo never }
`)
	result := r.Run(f, nil)
	if result.OK || result.Exit != 4 || strings.Contains(out.String(), "never") {
		t.Fatalf("want a stop with exit 4 at the second step, got %+v\n%s", result, out)
	}
	if result.Steps["0"].Status != StatusFailed {
		t.Errorf("the first step failed too, even going on")
	}
}

func TestExpectChecksTheOutput(t *testing.T) {
	r, _ := testRunner(t, nil)
	f := parse(t, `
steps:
  - run: echo status=PENDING
    expect: { output_matches: 'status=CREATED' }
`)
	if result := r.Run(f, nil); result.OK {
		t.Fatal("output without the match passed")
	}
}

func TestValidateRejects(t *testing.T) {
	lookup := func(id string) (script.Script, bool) { return script.Script{}, id == "known" }
	cases := map[string]string{
		"two kinds":      `steps: [ { run: ls, wait: 1s } ]`,
		"no kind":        `steps: [ { id: a } ]`,
		"endless repeat": `steps: [ { repeat: { until: 'false' }, steps: [ { run: ls } ] } ]`,
		"bad wait":       `steps: [ { wait: soon } ]`,
		"bad capture":    `steps: [ { run: ls, capture: { x: '(' } } ]`,
		"unknown script": `steps: [ { script: missing } ]`,
		"twice the id":   `steps: [ { id: a, run: ls }, { id: a, run: ls } ]`,
		"bad template":   `steps: [ { run: '{{.x' } ]`,
		"capture on if":  `steps: [ { if: 'true', capture: { x: y } } ]`,
	}
	for name, text := range cases {
		if err := parse(t, text).Validate(lookup); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if err := parse(t, `steps: [ { script: known }, { wait: 2s } ]`).Validate(lookup); err != nil {
		t.Errorf("valid flow: %v", err)
	}
}

func TestEditingByPath(t *testing.T) {
	f := parse(t, `
steps:
  - { id: a, run: echo a }
  - if: 'true'
    then: [ { id: b, run: echo b } ]
`)
	path, err := f.Insert("1", "else", 0, Step{ID: f.UniqueID("a"), Run: "echo c"})
	if err != nil || path != "1/else/0" {
		t.Fatalf("insert = %q, %v", path, err)
	}
	if s, _ := f.Step(path); s.ID != "a2" {
		t.Errorf("new id = %q, want a2", s.ID)
	}
	if moved := f.Move("0", 1); moved != "1" || f.Steps[0].If == "" {
		t.Errorf("move down = %q, steps %+v", moved, f.Steps)
	}
	if err := f.Remove("0/then/0"); err != nil || len(f.Steps[0].Then) != 0 {
		t.Errorf("remove: %v, %+v", err, f.Steps[0])
	}
	parent, branch, index := SplitPath("3/steps/2")
	if parent != "3" || branch != "steps" || index != 2 {
		t.Errorf("SplitPath = %q %q %d", parent, branch, index)
	}
}

func TestSaveAndLoadKeepTheFlow(t *testing.T) {
	dir := t.TempDir()
	f, err := Create(dir, "daily")
	if err != nil {
		t.Fatal(err)
	}
	f.Description = "every day"
	f.Steps = []Step{{Repeat: &Repeat{Times: 2}, Steps: []Step{{Wait: "1s"}}}}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	flows, err := LoadAll(dir, nil)
	if err != nil || len(flows) != 1 || flows[0].Problem != nil {
		t.Fatalf("LoadAll = %+v, %v", flows, err)
	}
	if got := flows[0]; got.Name != "daily" || got.Steps[0].Repeat.Times != 2 || got.Steps[0].Steps[0].Wait != "1s" {
		t.Errorf("loaded %+v", got)
	}
	if _, err := Create(dir, "daily"); err == nil {
		t.Error("created a flow twice")
	}
}
