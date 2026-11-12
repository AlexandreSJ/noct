package flow

import (
	"os"
	"testing"
)

func TestARunTellsTheStepItIsOn(t *testing.T) {
	dir := t.TempDir()
	if _, running := ReadProgress(dir, "ship"); running {
		t.Fatal("a flow never run is running")
	}
	if err := SaveProgress(dir, "ship", "1/then/0"); err != nil {
		t.Fatal(err)
	}
	if p, running := ReadProgress(dir, "ship"); !running || p.Path != "1/then/0" || p.PID != os.Getpid() {
		t.Errorf("progress %+v, running %v", p, running)
	}
	ClearProgress(dir, "ship")
	if _, running := ReadProgress(dir, "ship"); running {
		t.Error("still running after it ended")
	}
}

func TestTheRunnerTellsEachStepAsItStarts(t *testing.T) {
	f := parse(t, "steps:\n  - {id: a, run: 'true'}\n  - if: 'true'\n    then: [{id: b, run: 'true'}]\n")
	var seen []string
	r, _ := testRunner(t, nil)
	r.Progress = func(path string) { seen = append(seen, path) }
	r.Run(f, nil)
	if len(seen) != 3 || seen[0] != "0" || seen[1] != "1" || seen[2] != "1/then/0" {
		t.Errorf("progress %v, want each step in order", seen)
	}
}
