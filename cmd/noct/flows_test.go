package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// runNoct runs the binary with the session's environment and returns what it printed.
func runNoct(t *testing.T, s *session, args ...string) (string, error) {
	t.Helper()
	return runNoctWith(t, s, nil, args...)
}

// runNoctWith runs the binary with more variables in its environment.
func runNoctWith(t *testing.T, s *session, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(s.binary, args...)
	cmd.Env = append(append([]string{}, s.env...), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func homeOf(s *session) string {
	for _, entry := range s.env {
		if dir, ok := strings.CutPrefix(entry, "HOME="); ok {
			return dir
		}
	}
	return ""
}

func writeFlow(t *testing.T, s *session, name, content string) {
	t.Helper()
	dir := filepath.Join(homeOf(s), ".local", "share", "noct", "flows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+flow.Ext), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lastResult(t *testing.T, s *session, name string) flow.Result {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(stateDir(s), "flows", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var r flow.Result
	if err := json.Unmarshal(content, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// noct check warns about a step read before it runs; a full run keeps the outputs, and a test
// of one step reads them and replaces only that step.
func TestFlowWarningsAndTestingAStep(t *testing.T) {
	s := newSession(t)
	writeFlow(t, s, "pair", `steps:
  - {id: a, run: printf one}
  - {id: b, run: 'echo {{.steps.a.output}}-{{.steps.c.output}}'}
  - {id: c, run: printf three}
`)
	out, err := runNoct(t, s, "check")
	if err != nil || !strings.Contains(out, "⚠ flow pair, step b (1): step c has not run yet here") {
		t.Fatalf("noct check: %v\n%s", err, out)
	}
	if out, err := runNoct(t, s, "flow", "run", "pair"); err != nil {
		t.Fatalf("flow run: %v\n%s", err, out)
	}
	if got := lastResult(t, s, "pair").Steps["0"].Output; got != "one" {
		t.Errorf("kept output of a = %q", got)
	}
	out, err = runNoct(t, s, "flow", "run", "pair", "--steps", "1..2")
	if err != nil || !strings.Contains(out, "one-three") {
		t.Fatalf("testing steps 1..2: %v\n%s", err, out)
	}
	r := lastResult(t, s, "pair")
	if r.Steps["0"].Tried || !r.Steps["1"].Tried || !r.Steps["2"].Tried {
		t.Errorf("tried marks: %+v", r.Steps)
	}
	if _, err := runNoct(t, s, "flow", "run", "pair", "--steps", "9"); err == nil {
		t.Error("testing a missing step did not fail")
	}
}

// A run noct keeps a record of also keeps its result where noct asks, for the calendar.
func TestFlowRunKeepsItsResultWhereAsked(t *testing.T) {
	s := newSession(t)
	writeFlow(t, s, "one", "steps:\n  - {id: a, run: printf hi}\n")
	path := filepath.Join(t.TempDir(), "run.result.json")
	if out, err := runNoctWith(t, s, []string{flow.ResultEnv + "=" + path}, "flow", "run", "one"); err != nil {
		t.Fatalf("flow run: %v\n%s", err, out)
	}
	content, err := os.ReadFile(path)
	var r flow.Result
	if err == nil {
		err = json.Unmarshal(content, &r)
	}
	if err != nil || r.Flow != "one" || r.Steps["0"].Output != "hi" {
		t.Errorf("result kept for the run: %+v, %v", r, err)
	}
}

// The answer to on_fail: ask takes its line and leaves the rest for the next steps.
func TestReadLineLeavesTheRest(t *testing.T) {
	in := strings.NewReader("r\nnext step input")
	if got := readLine(in); got != "r" {
		t.Fatalf("answer = %q", got)
	}
	if rest, _ := io.ReadAll(in); string(rest) != "next step input" {
		t.Errorf("left %q", rest)
	}
}

// noct enable turns a flow's triggers on, flow ls shows it, and noct disable turns them off.
func TestEnableAndDisableTriggers(t *testing.T) {
	s := newSession(t)
	writeFlow(t, s, "nightly", "triggers: [{cron: '0 2 * * *'}]\nsteps: [{run: 'true'}]\n")
	if out, err := runNoct(t, s, "enable", "nightly"); err != nil {
		t.Fatalf("enable: %v\n%s", err, out)
	}
	out, _ := runNoct(t, s, "flow", "ls")
	if !strings.Contains(out, "nightly\ton: cron 0 2 * * *") {
		t.Errorf("flow ls after enable:\n%s", out)
	}
	if _, err := runNoct(t, s, "disable", "nightly"); err != nil {
		t.Fatal(err)
	}
	if out, _ := runNoct(t, s, "flow", "ls"); !strings.Contains(out, "nightly\toff: cron 0 2 * * *") {
		t.Errorf("flow ls after disable:\n%s", out)
	}
	if _, err := runNoct(t, s, "enable", "missing"); err == nil {
		t.Error("enabled a flow that does not exist")
	}
}

// A step with pane: new runs in a pane the workspace opens, and the flow gets its exit code.
func TestPaneStepRunsInAWorkspacePane(t *testing.T) {
	s := newSession(t)
	noct, _ := s.attach(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)
	writeFlow(t, s, "paned", "steps:\n  - {id: build, run: 'echo in-its-pane; exit 4', pane: new}\n")
	out, err := runNoctWith(t, s, []string{"NOCT_PANE=1"}, "flow", "run", "paned")
	var code *exec.ExitError
	if !errors.As(err, &code) || code.ExitCode() != 4 {
		t.Fatalf("flow run: %v\n%s", err, out)
	}
	waitForScreen(t, noct, "the step's pane", func(screen string) bool {
		return frameCount(screen) == 2 && strings.Contains(screen, "in-its-pane") && strings.Contains(screen, "build")
	})
}

// With the workspace running, an enabled flow waiting for another starts by itself when that
// one ends, even when it ran from a shell; its run is recorded with what started it.
func TestAfterTriggerRunsInTheWorkspace(t *testing.T) {
	s := newSession(t)
	noct, _ := s.attach(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	writeFlow(t, s, "first", "steps: [{run: 'true'}]\n")
	writeFlow(t, s, "second", "triggers: [{after: {flow: first, status: ok}}]\nsteps: [{id: s, run: printf second-ran}]\n")
	if out, err := runNoct(t, s, "enable", "second"); err != nil {
		t.Fatalf("enable: %v\n%s", err, out)
	}
	if out, err := runNoct(t, s, "flow", "run", "first"); err != nil {
		t.Fatalf("flow run: %v\n%s", err, out)
	}
	resultOf := filepath.Join(stateDir(s), "flows", "second.json")
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(resultOf); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if got := lastResult(t, s, "second").Steps["0"].Output; got != "second-ran" {
		t.Fatalf("second's output = %q", got)
	}
	runs, _ := os.ReadFile(filepath.Join(stateDir(s), "runs.json"))
	if !strings.Contains(string(runs), `"trigger":"after first"`) {
		t.Errorf("runs.json lacks the trigger:\n%s", runs)
	}
}

// A draft does not run until promoted; noct promote clears the mark.
func TestDraftsRunOnlyOncePromoted(t *testing.T) {
	s := newSession(t)
	writeFlow(t, s, "drafted", "draft: true\nsteps: [{run: 'echo ran'}]\n")
	out, err := runNoct(t, s, "flow", "run", "drafted")
	if err == nil || !strings.Contains(out, "draft") {
		t.Fatalf("a draft ran: %v\n%s", err, out)
	}
	if out, err := runNoct(t, s, "promote", "drafted"); err != nil {
		t.Fatalf("promote: %v\n%s", err, out)
	}
	if out, err := runNoct(t, s, "flow", "run", "drafted"); err != nil || !strings.Contains(out, "ran") {
		t.Errorf("after promoting: %v\n%s", err, out)
	}
}
