package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

const calcFlow = `params:
  a: {type: number, required: true, prompt: First number}
  b: {type: number, default: "2"}
  text: hi
steps:
  - {run: 'echo {{.a}}'}
`

// r on a flow with a required parameter empty opens its form; the form refuses what the types
// refuse, runs with what it has, and r then runs again with the values it remembers.
func TestFlowParamsFormRunsAndRemembers(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveFlow(t, "calc", calcFlow)
	pressDrawn(a, prefixPress, letter('i'))
	inspect := a.currentTab().focused
	launched := len(l.programs)

	pressDrawn(a, letter('r'))
	if a.mode != formMode || len(l.programs) != launched {
		t.Fatalf("r with a required parameter empty: mode %v, %d launches", a.mode, len(l.programs)-launched)
	}
	if screen := screenText(a); !strings.Contains(screen, "First number") || !strings.Contains(screen, "run calc") {
		t.Errorf("form does not show the flow's parameters:\n%s", screen)
	}
	typeDrawn(a, "x")
	pressDrawn(a, enterPress)
	if a.mode != formMode || !strings.Contains(a.statusMessage, "a must be a number") {
		t.Fatalf("a non-number ran: mode %v, status %q", a.mode, a.statusMessage)
	}
	pressDrawn(a, clearLine)
	typeDrawn(a, "5")
	pressDrawn(a, tabPress, tabPress, clearLine)
	typeDrawn(a, "hello")
	pressDrawn(a, enterPress)
	want := []string{"flow", "run", "calc", "a=5", "b=2", "text=hello"}
	if got := l.last(t).argv[1:]; a.mode == formMode || !slices.Equal(got, want) {
		t.Fatalf("the form ran %v, want %v", got, want)
	}

	a.focusPane(a.currentTab(), inspect)
	pressDrawn(a, letter('r'))
	if got := l.last(t).argv[1:]; a.mode == formMode || !slices.Equal(got, want) {
		t.Errorf("r ran %v, want the remembered %v", got, want)
	}

	a.focusPane(a.currentTab(), inspect)
	pressDrawn(a, letter('p'))
	if a.mode != formMode || a.form.values["a"] != "5" || a.form.values["text"] != "hello" {
		t.Errorf("p: mode %v, values %v", a.mode, a.form.values)
	}
	pressDrawn(a, escPress)
}

func TestFlowParamsFormNeedsParameters(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "plain", "steps:\n  - {run: 'true'}\n")
	pressDrawn(a, prefixPress, letter('i'), letter('p'))
	if a.mode == formMode || !strings.Contains(a.statusMessage, "no parameters") {
		t.Errorf("mode %v, status %q", a.mode, a.statusMessage)
	}
}

// The add menu puts prompt, set and unset steps in, each from a line typed in its prompt.
func TestInspectAddsVariableSteps(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "vars", "params: {b: \"1\"}\nsteps: []\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('p'), clearLine)
	typeDrawn(a, "New b?")
	pressDrawn(a, enterPress, clearLine)
	typeDrawn(a, "b")
	pressDrawn(a, enterPress, letter('a'), letter('v'), clearLine)
	typeDrawn(a, "sum = {{.b}}")
	pressDrawn(a, enterPress, letter('a'), letter('u'), clearLine)
	typeDrawn(a, "sum, steps.x b")
	pressDrawn(a, enterPress)
	if a.statusMessage != "" {
		t.Fatalf("status: %s", a.statusMessage)
	}
	f, err := flow.Load(filepath.Join(flow.Dir(), "vars"+flow.Ext))
	if err != nil {
		t.Fatal(err)
	}
	prompt, _ := f.Step("0")
	set, _ := f.Step("1")
	unset, _ := f.Step("2")
	switch {
	case prompt.Prompt != "New b?" || prompt.Var != "b":
		t.Errorf("prompt = %+v", prompt)
	case set.Set["sum"] != "{{.b}}":
		t.Errorf("set = %+v", set)
	case !slices.Equal(unset.Unset, []string{"sum", "steps.x", "b"}):
		t.Errorf("unset = %+v", unset)
	}
	screen := screenText(a)
	for _, want := range []string{"? prompt", "= set", "∅ unset", "clear sum, steps.x, b"} {
		if !strings.Contains(screen, want) {
			t.Errorf("chart lacks %q:\n%s", want, screen)
		}
	}
}
