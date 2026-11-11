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
	if err := os.MkdirAll(flow.FunctionsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flow.FunctionsDir(), name+flow.Ext), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAFunctionShowsWhoCallsItAndIsCalledFromAStep(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 50)
	saveFunction(t, "greet", "params: {who: {default: world}}\nsteps:\n  - {id: say, run: 'echo hi {{.who}}'}\nreturns: {greeting: '{{.output}}'}\n")
	saveFlow(t, "morning", "steps:\n  - {id: wake, run: 'true'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('f'))
	typeDrawn(a, "greet")
	pressDrawn(a, enterPress)
	typeDrawn(a, "{{.output}}")
	pressDrawn(a, enterPress)
	steps := loadFlowSteps(t, "morning")
	if len(steps) != 2 || steps[1].Call != "greet" || steps[1].With["who"] != "world{{.output}}" && steps[1].With["who"] != "{{.output}}" {
		t.Fatalf("steps %+v, want a call to greet after wake", steps)
	}
	if !strings.Contains(screenText(a), "ƒ greet") {
		t.Errorf("the call's card does not tell the function:\n%s", screenText(a))
	}
	pressDrawn(a, tabPress, letter('j'))
	p := a.currentTab().focusedView().(*inspectPanel)
	if f, _, _ := p.current(); !f.Function || f.Name != "greet" {
		t.Fatalf("the list's next entry is %+v, want the function", f)
	}
	pressDrawn(a, enterPress)
	screen := screenText(a)
	for _, want := range []string{"used by morning", "↩ greeting = .output", "who  text = world"} {
		if !strings.Contains(screen, want) {
			t.Errorf("the function's chart lacks %q:\n%s", want, screen)
		}
	}
}
