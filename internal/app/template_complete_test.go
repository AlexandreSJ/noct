package app

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// A step form lists the variables in scope by their short names, naming by its step only an
// earlier step's field a later one set again: after a $ in a command, and alone in a condition,
// with the functions.
func TestAStepFormCompletesTheShortNames(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "done", "variables: {env: dev}\nsteps:\n  - {id: build, run: 'echo v=1', capture: {v: 'v=(\\d)'}}\n  - {id: test, run: 'true'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress, downPress)
	addStep(a, "command")
	pressDrawn(a, enterPress)
	typeDrawn(a, "echo $")
	screen := screenText(a)
	for _, want := range []string{"$env", "$output", "$v", "$build.output"} {
		if !strings.Contains(screen, want) {
			t.Errorf("$ does not offer %s:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "$test.output") || strings.Contains(screen, "$build.v") {
		t.Errorf("$ offers a long name where the short one reads the same:\n%s", screen)
	}
	typeDrawn(a, "ou")
	pressDrawn(a, tabPress)
	form := openStepForm(t, a)
	if got := form.grid.cell().value(); got != "echo $output" {
		t.Errorf("tab made %q, want the variable put in", got)
	}
	typeDrawn(a, " $build.")
	pressDrawn(a, downPress, tabPress)
	if got := form.grid.cell().value(); got != "echo $output $build.exit" {
		t.Errorf("down and tab made %q", got)
	}
	typeDrawn(a, " $jo")
	pressDrawn(a, tabPress)
	typeDrawn(a, "bu")
	pressDrawn(a, tabPress)
	if got := form.grid.cell().value(); got != "echo $output $build.exit $join(build.output" {
		t.Errorf("a function and a name inside it made %q", got)
	}
}

func TestAConditionCompletesNamesAlone(t *testing.T) {
	field := newTextField("")
	vars := []flow.Variable{{Field: "env"}, {Step: "build", Field: "exit"}}
	for _, c := range []struct {
		typed string
		want  []string
	}{
		{"en", []string{"env", "endsWith("}},
		{"ok and bu", []string{"build.exit"}},
		{"le", []string{"len("}},
		{"len(e", []string{"env", "endsWith("}},
		{"3", nil},
	} {
		field.text, field.cursor = []rune(c.typed), len([]rune(c.typed))
		got, _ := templateSuggestions(&field, vars, true)
		if !slices.Equal(got, c.want) {
			t.Errorf("%q offers %q, want %q", c.typed, got, c.want)
		}
	}
}

func TestXExportsAVariableFromABlock(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "loop", "steps:\n  - repeat: {times: 2}\n    steps:\n      - {set: {count: '1'}}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('x'), enterPress)
	typeDrawn(a, "count")
	pressDrawn(a, tabPress)
	setCell(a, "passes")
	pressDrawn(a, ctrlS)
	steps := loadFlowSteps(t, "loop")
	if steps[0].Export["count"] != "passes" {
		t.Fatalf("export %v, want count as passes", steps[0].Export)
	}
	if !strings.Contains(screenText(a), "export count → passes") {
		t.Errorf("the repeat does not tell its export:\n%s", screenText(a))
	}
}
