package flow

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestValidateRejectsBadTimeouts(t *testing.T) {
	cases := map[string]string{
		"flow, not a duration": "timeout: soon\nsteps: [{run: ls}]",
		"flow, zero":           "timeout: 0s\nsteps: [{run: ls}]",
		"step, negative":       "steps: [{run: ls, timeout: -5s}]",
		"step, no unit":        "steps: [{run: ls, timeout: '10'}]",
		"on a wait":            "steps: [{wait: 1s, timeout: 5s}]",
		"in the background":    "steps: [{run: ls, pane: new, background: true, timeout: 5s}]",
	}
	for name, text := range cases {
		if err := parse(t, text).Validate(nil); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	valid := "timeout: 1h30m\nsteps: [{run: ls, timeout: 30s}, {run: ls, pane: new, timeout: 10m}]"
	if err := parse(t, valid).Validate(nil); err != nil {
		t.Errorf("valid flow: %v", err)
	}
	if err := parse(t, "timeout: soon\nsteps: []").Validate(nil); err == nil || !strings.Contains(err.Error(), "30s") {
		t.Errorf("the error does not say what a duration looks like: %v", err)
	}
}

// A step that runs past its timeout is stopped and fails, and then does what its on_fail says.
func TestAStepThatTimesOutFollowsOnFail(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
steps:
  - {id: slow, run: 'sleep 30', timeout: 200ms, on_fail: continue}
  - {run: echo after}
`)
	start := time.Now()
	result := r.Run(f, nil)
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("the step ran for %s", took)
	}
	step := result.Steps["0"]
	if step.Status != StatusFailed || step.Message != "timed out after 200ms" || step.Exit != timeoutExit {
		t.Errorf("step %+v", step)
	}
	if !result.OK || !strings.Contains(out.String(), "after") {
		t.Errorf("the flow did not go on: %+v\n%s", result, out)
	}
}

// Stopping a step stops what it started too, not only its shell.
func TestAStepTimeoutStopsItsChildren(t *testing.T) {
	r, _ := testRunner(t, nil)
	pidFile := filepath.Join(t.TempDir(), "pid")
	f := parse(t, "steps: [{run: 'sleep 30 & echo $! > "+pidFile+"; wait', timeout: 300ms}]")
	if result := r.Run(f, nil); result.OK {
		t.Fatal("the step did not time out")
	}
	text, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(text)))
	for range 50 {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Errorf("the step's child %d is still running", pid)
}

// A flow's timeout ends the run, whatever the step's on_fail says.
func TestAFlowTimeoutStopsTheRun(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
timeout: 300ms
steps:
  - {run: 'sleep 30', on_fail: continue}
  - {run: echo never}
`)
	result := r.Run(f, nil)
	if result.OK || result.Exit != timeoutExit || result.Steps["0"].Message != "flow timed out after 300ms" {
		t.Errorf("result %+v", result)
	}
	if strings.Contains(out.String(), "never") {
		t.Error("a step ran after the flow timed out")
	}
}

// The flow's timeout covers waits too, which have no process to stop.
func TestAFlowTimeoutCoversWaits(t *testing.T) {
	r, out := testRunner(t, nil)
	r.Sleep = time.Sleep
	f := parse(t, "timeout: 200ms\nsteps: [{wait: 20s}, {run: echo never}]")
	start := time.Now()
	result := r.Run(f, nil)
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("the wait went on for %s", took)
	}
	if result.OK || strings.Contains(out.String(), "never") {
		t.Errorf("the flow went on past its timeout: %+v", result)
	}
}

// A step in a pane of its own asks the workspace to stop it at its timeout, and a pane that
// ends at that time timed out.
func TestAPaneStepTimeoutGoesToTheWorkspace(t *testing.T) {
	r, _ := testRunner(t, nil)
	var asked time.Duration
	r.OpenPane = func(req PaneRequest) (int, error) {
		asked = req.Timeout
		time.Sleep(req.Timeout) // the workspace stops the pane's program then
		return -1, nil
	}
	result := r.Run(parse(t, "steps: [{run: 'sleep 30', pane: new, timeout: 100ms}]"), nil)
	if asked <= 0 || asked > 100*time.Millisecond {
		t.Errorf("asked for a timeout of %s", asked)
	}
	if step := result.Steps["0"]; step.Message != "timed out after 100ms" {
		t.Errorf("step %+v", step)
	}
}

func TestTimeoutWarningForFlowsThatRunByThemselves(t *testing.T) {
	cases := map[string]bool{
		"triggers: [{cron: '0 9 * * *'}]\nsteps: [{run: ls}]":                                    true,
		"triggers: [{watch: {paths: [a]}}]\nsteps: [{run: ls}]":                                  true,
		"triggers: [{cron: '0 9 * * *'}]\ntimeout: 1h\nsteps: [{run: ls}]":                       false,
		"triggers: [{cron: '0 9 * * *'}]\nsteps: [{run: ls, timeout: 5m}]":                       false,
		"triggers: [{after: {flow: other}}]\nsteps: [{run: ls}]":                                 true,
		"triggers: [manual]\nsteps: [{run: ls}]":                                                 false,
		"steps: [{run: ls}]":                                                                     false,
		"triggers: [{cron: '0 9 * * *'}]\nsteps: [{if: 'true', then: [{run: ls, timeout: 5m}]}]": false,
	}
	for text, warns := range cases {
		if got := parse(t, text).TimeoutWarning(); (got != "") != warns {
			t.Errorf("%s\nwarning %q", text, got)
		}
	}
}
