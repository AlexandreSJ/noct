package flow

import (
	"path/filepath"
	"testing"
)

// failsOnce fails the first time it runs, leaving a mark, and passes after.
func failsOnce(t *testing.T) string {
	mark := filepath.Join(t.TempDir(), "ran")
	return "steps:\n  - {id: once, run: 'test -f " + mark + " || { touch " + mark + "; exit 4; }', on_fail: ask}\n  - {id: next, run: 'true'}\n"
}

func TestAskRetriesAFailedStep(t *testing.T) {
	r, _ := testRunner(t, nil)
	var asked []string
	r.Ask = func(question string) string { asked = append(asked, question); return "r" }
	result := r.Run(parse(t, failsOnce(t)), nil)
	if !result.OK || len(asked) != 1 || result.Steps["0"].Status != StatusOK {
		t.Errorf("retry: ok %v, asked %q, step %+v", result.OK, asked, result.Steps["0"])
	}
}

func TestAskGoesOnOrStops(t *testing.T) {
	cases := map[string]bool{"c": true, "s": false}
	for answer, goesOn := range cases {
		r, _ := testRunner(t, nil)
		r.Ask = func(string) string { return answer }
		result := r.Run(parse(t, failsOnce(t)), nil)
		_, ranNext := result.Steps["1"]
		if result.Steps["0"].Status != StatusFailed || ranNext != goesOn || result.OK != goesOn {
			t.Errorf("answer %s: ok %v, next ran %v", answer, result.OK, ranNext)
		}
	}
}

// Without anyone to ask, as in a run by cron, the flow stops.
func TestAskWithoutATerminalStops(t *testing.T) {
	r, _ := testRunner(t, nil)
	if result := r.Run(parse(t, failsOnce(t)), nil); result.OK || result.Exit != 4 {
		t.Errorf("ok %v, exit %d", result.OK, result.Exit)
	}
}
