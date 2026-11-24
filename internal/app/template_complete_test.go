package app

import (
	"image"
	"strings"
	"testing"
)

// A step form lists the variables in scope by their short names, naming by its step only an
// earlier step's field a later one set again, and {{. completes them.
func TestAStepFormCompletesTheShortNames(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "done", "params: {env: dev}\nsteps:\n  - {id: build, run: 'echo v=1', capture: {v: 'v=(\\d)'}}\n  - {id: test, run: 'true'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, downPress, letter('a'), letter('c'))
	typeDrawn(a, "echo {{.")
	screen := screenText(a)
	for _, want := range []string{".env", ".output", ".v", ".steps.build.output"} {
		if !strings.Contains(screen, want) {
			t.Errorf("{{. does not offer %s:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, ".steps.test.output") || strings.Contains(screen, ".steps.build.v") {
		t.Errorf("{{. offers a long name where the short one reads the same:\n%s", screen)
	}
	typeDrawn(a, "ou")
	pressDrawn(a, tabPress)
	form := openStepForm(t, a)
	if got := form.grid.cell().value(); got != "echo {{.output}}" {
		t.Errorf("tab made %q, want the variable put in with its braces closed", got)
	}
	typeDrawn(a, " {{.steps.")
	pressDrawn(a, downPress, tabPress)
	if got := form.grid.cell().value(); got != "echo {{.output}} {{.steps.build.exit}}" {
		t.Errorf("down and tab made %q", got)
	}
}

func TestXExportsAVariableFromABlock(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "loop", "steps:\n  - repeat: {times: 2}\n    steps:\n      - {set: {count: '1'}}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('x'))
	typeDrawn(a, "count as passes")
	pressDrawn(a, enterPress)
	steps := loadFlowSteps(t, "loop")
	if steps[0].Export["count"] != "passes" {
		t.Fatalf("export %v, want count as passes", steps[0].Export)
	}
	if !strings.Contains(screenText(a), "export count → passes") {
		t.Errorf("the repeat does not tell its export:\n%s", screenText(a))
	}
}
