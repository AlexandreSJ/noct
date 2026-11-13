package app

import (
	"image"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// A switch is added with its cases in two prompts, drawn with a branch for each case and the
// else, and the head of a case adds a step into it.
func TestASwitchIsAddedAndDrawnWithABranchPerCase(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(160, 60)
	saveFlow(t, "sort", "steps:\n  - {id: ask, run: 'echo bug'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	p := a.currentTab().focusedView().(*inspectPanel)
	p.selected = stepKey("0")
	pressDrawn(a, letter('a'), letter('o'), clearLine)
	typeDrawn(a, "{{.output}}")
	pressDrawn(a, enterPress)
	typeDrawn(a, "flaky bug|regression")
	pressDrawn(a, enterPress)
	if a.statusMessage != "" {
		t.Fatalf("status: %s", a.statusMessage)
	}
	steps := loadFlowSteps(t, "sort")
	if len(steps) != 2 || steps[1].Switch != "{{.output}}" || len(steps[1].Cases) != 2 || strings.Join(steps[1].Cases[1].Is, " ") != "bug regression" {
		t.Fatalf("steps = %+v", steps)
	}
	text := chartText(p.chart)
	for _, want := range []string{"◈ switch", "flaky", "bug|regression", "else"} {
		if !strings.Contains(text, want) {
			t.Errorf("chart lacks %q:\n%s", want, text)
		}
	}
	if _, ok := p.chart.block(headKey(chartSlot{parent: "1", branch: flow.CaseBranch(1)})); !ok {
		t.Fatalf("no head for the second case:\n%s", text)
	}
	p.selected = headKey(chartSlot{parent: "1", branch: flow.CaseBranch(1)})
	pressDrawn(a, letter('a'), letter('c'))
	typeDrawn(a, "echo fix")
	pressDrawn(a, enterPress)
	f, _ := flow.Find(a.flows, "sort")
	if s, ok := f.Step("1/case1/0"); !ok || s.Run != "echo fix" {
		t.Fatalf("the step added at the case's head = %+v, %v", s, ok)
	}
}

func TestAForEachIsAddedAndTitledByItsLines(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(160, 60)
	saveFlow(t, "hosts", "steps:\n  - {id: list, run: 'printf \"a\\nb\\n\"'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	p := a.currentTab().focusedView().(*inspectPanel)
	p.selected = stepKey("0")
	pressDrawn(a, letter('a'), letter('e'), clearLine)
	typeDrawn(a, "{{.steps.list.output}}")
	pressDrawn(a, enterPress, clearLine)
	typeDrawn(a, "host")
	pressDrawn(a, enterPress)
	steps := loadFlowSteps(t, "hosts")
	if len(steps) != 2 || steps[1].Repeat == nil || steps[1].Repeat.Each != "{{.steps.list.output}}" || steps[1].Repeat.As != "host" {
		t.Fatalf("steps = %+v", steps)
	}
	if text := chartText(p.chart); !strings.Contains(text, "↻ for each host in .steps.list.output") {
		t.Errorf("chart lacks the for each title:\n%s", text)
	}
}

func TestRShowsAndSetsAStepsRetry(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(160, 60)
	saveFlow(t, "flaky", "steps:\n  - {id: test, run: 'true'}\n  - {wait: 1s}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	p := a.currentTab().focusedView().(*inspectPanel)
	p.selected = stepKey("0")
	pressDrawn(a, letter('R'), clearLine)
	typeDrawn(a, "2 every 5s")
	pressDrawn(a, enterPress)
	steps := loadFlowSteps(t, "flaky")
	if r := steps[0].Retry; r == nil || r.Max != 2 || r.Every != "5s" {
		t.Fatalf("retry = %+v", r)
	}
	if text := chartText(p.chart); !strings.Contains(text, "retry 2× every 5s") {
		t.Errorf("the card lacks the retry:\n%s", text)
	}
	pressDrawn(a, letter('R'), clearLine, enterPress)
	if steps := loadFlowSteps(t, "flaky"); steps[0].Retry != nil {
		t.Errorf("an empty answer takes the retry away: %+v", steps[0].Retry)
	}
	p.selected = stepKey("1")
	pressDrawn(a, letter('R'))
	if !strings.Contains(a.statusMessage, "only a script") {
		t.Errorf("a wait does not retry: %q", a.statusMessage)
	}
}
