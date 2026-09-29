package flow

import (
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/script"
)

// The result keeps each command step's output as templates read it, or only its start when
// the runner keeps less.
func TestResultKeepsTheStepOutput(t *testing.T) {
	f := parse(t, "steps:\n  - {id: say, run: printf 'abcdef\\n'}\n  - wait: 0s\n")
	r, _ := testRunner(t, nil)
	if got := r.Run(f, nil).Steps["0"].Output; got != "abcdef" {
		t.Errorf("output kept = %q, want all of it", got)
	}
	r.KeepOutput = 3
	if got := r.Run(f, nil).Steps["0"].Output; got != "abc" {
		t.Errorf("output kept with a limit of 3 = %q, want abc", got)
	}
	if got := r.Run(f, nil).Steps["1"].Output; got != "" {
		t.Errorf("a wait kept output %q", got)
	}
}

func TestTruncateKeepsWholeCharacters(t *testing.T) {
	if got := truncate("aé"+strings.Repeat("x", 3), 2); got != "a" {
		t.Errorf("truncate cut a character in half: %q", got)
	}
}

// A script's outputs are captured without a regex; the step's own capture of the same name wins.
func TestScriptOutputsAreCaptured(t *testing.T) {
	scripts := map[string]script.Script{"pick": {Name: "pick", Run: "printf 'color=red\\nfile=a b\\n'", Outputs: []string{"color", "file"}}}
	r, _ := testRunner(t, scripts)
	f := parse(t, "steps:\n  - {id: p, script: pick, capture: {color: 'color=(r)'}}\n")
	got := r.Run(f, nil).Steps["0"].Captures
	if got["color"] != "r" || got["file"] != "a b" {
		t.Errorf("captures = %v, want color from the step's regex and file from the outputs", got)
	}
}

// Testing a range of steps reads the last result's values, replaces only those steps in it,
// marked as tried, and keeps at most TriedOutput of their output.
func TestRunStepsTestsARangeOverTheLastResult(t *testing.T) {
	f := parse(t, `
steps:
  - {id: a, run: printf fresh}
  - {id: b, run: 'echo {{.steps.a.output}}-x'}
  - {id: c, run: 'head -c 5000 /dev/zero | tr "\\0" y'}
`)
	r, out := testRunner(t, nil)
	previous := Result{Flow: "test", OK: true, Steps: map[string]StepResult{"0": {Status: StatusOK, Output: "seeded"}}}
	got, err := r.RunSteps(f, nil, "1", "2", previous)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "seeded-x") {
		t.Errorf("step b did not read a's last output:\n%s", out)
	}
	if !got.Steps["1"].Tried || !got.Steps["2"].Tried || got.Steps["0"].Output != "seeded" || got.Steps["0"].Tried {
		t.Errorf("steps = %+v", got.Steps)
	}
	if n := len(got.Steps["2"].Output); n != TriedOutput {
		t.Errorf("tried output kept %d bytes, want %d", n, TriedOutput)
	}
}

func TestRunStepsWantsSiblings(t *testing.T) {
	f := parse(t, explore)
	r, _ := testRunner(t, diceScript)
	if _, err := r.RunSteps(f, nil, "0", "1/steps/0", Result{}); err == nil {
		t.Error("a range across lists ran")
	}
	if _, err := r.RunSteps(f, nil, "2", "0", Result{}); err == nil {
		t.Error("a range backwards ran")
	}
}
