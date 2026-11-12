package app

import (
	"strings"
	"testing"
)

func TestATemplatePromptCompletesTheVariablesInScope(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "done", "params: {env: dev}\nsteps:\n  - {id: build, run: 'echo v=1', capture: {v: 'v=(\\d)'}}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('c'))
	typeDrawn(a, "echo {{.")
	for _, want := range []string{".env", ".output", ".v", ".steps.build.output"} {
		if !strings.Contains(screenText(a), want) {
			t.Errorf("{{. does not offer %s:\n%s", want, screenText(a))
		}
	}
	typeDrawn(a, "ou")
	pressDrawn(a, tabPress)
	if got := a.prompt.input.String(); got != "echo {{.output}}" {
		t.Errorf("tab made %q, want the variable put in with its braces closed", got)
	}
	typeDrawn(a, " {{.steps.")
	pressDrawn(a, downPress, tabPress)
	if got := a.prompt.input.String(); !strings.HasPrefix(got, "echo {{.output}} {{.steps.build.") || !strings.HasSuffix(got, "}}") {
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
