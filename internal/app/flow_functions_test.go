package app

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

func saveFunction(t *testing.T, name, content string) {
	t.Helper()
	file := flow.FilePath(flow.FunctionsDir(), name) // a name with / goes in folders
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAFunctionShowsWhoCallsItAndIsCalledFromAStep(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 50)
	saveFunction(t, "greet", "variables: {who: {default: world}}\nsteps:\n  - {id: say, run: 'echo hi $who'}\nreturns: {greeting: '$output'}\n")
	saveFlow(t, "morning", "steps:\n  - {id: wake, run: 'true'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	addStep(a, "function call")
	typeDrawn(a, "greet")
	pressDrawn(a, enterPress)
	setCell(a, "$output")
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "morning")
	if len(steps) != 2 || steps[1].Call != "greet" || steps[1].With["who"] != "world$output" && steps[1].With["who"] != "$output" {
		t.Fatalf("steps %+v, want a call to greet after wake", steps)
	}
	if !strings.Contains(screenText(a), "ƒ greet") {
		t.Errorf("the call's card does not tell the function:\n%s", screenText(a))
	}
	pressDrawn(a, tabPress, letter('j'), letter('j')) // past the ƒ functions folder, open
	p := a.currentTab().focusedView().(*inspectPanel)
	if f, _, _ := p.current(); !f.Function || f.Name != "greet" {
		t.Fatalf("the list's next entry is %+v, want the function", f)
	}
	pressDrawn(a, enterPress)
	screen := screenText(a)
	for _, want := range []string{"used by morning", "↩ greeting = $output", "who  text = world"} {
		if !strings.Contains(screen, want) {
			t.Errorf("the function's chart lacks %q:\n%s", want, screen)
		}
	}
}

// r on a function's params card edits its returns in a form, a name and its template a row;
// ctrl+delete takes a return away.
func TestAFunctionsReturnsAreEditedInAForm(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFunction(t, "greet", "steps:\n  - {id: say, run: 'echo hi'}\nreturns: {old: x}\n")
	pressDrawn(a, prefixPress, letter('i'))
	p := a.currentTab().focusedView().(*inspectPanel)
	p.flowName = "ƒ greet"
	pressDrawn(a, tabPress, upPress, upPress, letter('r'), ctrlDelete, enterPress)
	typeDrawn(a, "greeting")
	pressDrawn(a, tabPress)
	setCell(a, "{{.output}}")
	pressDrawn(a, ctrlS)
	fn, ok := flow.Find(a.functions, "greet")
	if !ok || len(fn.Returns) != 1 || fn.Returns["greeting"] != "{{.output}}" {
		t.Errorf("returns %+v", fn.Returns)
	}
}
