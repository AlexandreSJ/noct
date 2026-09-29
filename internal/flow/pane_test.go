package flow

import (
	"slices"
	"strings"
	"testing"
)

// A pane step runs in a pane of its own, beside the run; in the background the flow goes on
// without waiting for it.
func TestPaneStepsOpenPanes(t *testing.T) {
	r, _ := testRunner(t, nil)
	var opened []PaneRequest
	r.OpenPane = func(req PaneRequest) (int, error) {
		opened = append(opened, req)
		if req.Wait {
			return 2, nil
		}
		return 0, nil
	}
	f := parse(t, `
steps:
  - {id: tail, run: 'tail -f log', pane: new, background: true}
  - {id: shell, run: 'echo hi', pane: new, on_fail: continue}
  - {id: after, run: 'true'}
`)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	result := r.Run(f, nil)
	if len(opened) != 2 || opened[0].Wait || !opened[1].Wait || opened[0].Command != "tail -f log" || opened[1].Title != "shell" {
		t.Fatalf("opened %+v", opened)
	}
	if result.Steps["0"].Status != StatusOK || result.Steps["1"].Exit != 2 || result.Steps["1"].Status != StatusFailed || !result.OK {
		t.Errorf("steps %+v", result.Steps)
	}
}

// Outside noct there is no pane to open: the step runs in the flow's terminal, and in the
// background the flow does not wait for it.
func TestPaneStepsWithoutNoctRunInline(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, "steps:\n  - {run: 'sleep 5', pane: new, background: true}\n  - {run: 'echo inline; exit 3', pane: new}\n")
	result := r.Run(f, nil)
	if result.Exit != 3 || !strings.Contains(out.String(), "inline") {
		t.Errorf("exit %d, output %q", result.Exit, out)
	}
}

func TestValidateRejectsBadPaneSteps(t *testing.T) {
	cases := []string{
		"steps: [{run: x, pane: old}]",
		"steps: [{run: x, background: true}]",
		"steps: [{wait: 1s, pane: new}]",
		"steps: [{run: x, pane: new, capture: {a: b}}]",
	}
	for _, text := range cases {
		if err := parse(t, text).Validate(nil); err == nil {
			t.Errorf("accepted %s", text)
		}
	}
}

func TestPaneRequestCarriesTheStepsCommand(t *testing.T) {
	r, _ := testRunner(t, nil)
	var got []string
	r.OpenPane = func(req PaneRequest) (int, error) { got = append(got, req.Command, req.Dir); return 0, nil }
	r.Run(parse(t, "params: {who: you}\nsteps: [{run: 'echo {{.who}}', pane: new}]\n"), nil)
	if !slices.Equal(got, []string{"echo you", r.Dir}) {
		t.Errorf("request %q", got)
	}
}
