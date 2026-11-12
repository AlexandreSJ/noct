package app

import (
	"image"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// An agent step is set up in one form, what it asks, its answers, the agent and the timeout,
// and one enter adds it.
func TestAnAgentStepIsAddedFromOneForm(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 50)
	a.config.Agents = map[string]string{"local": "ollama run qwen"}
	saveFlow(t, "review", "steps:\n  - {id: test, run: 'echo 3 failed'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('a'))
	if a.mode != formMode {
		t.Fatalf("a, a opened mode %v, want the agent form", a.mode)
	}
	typeDrawn(a, "Did ")
	pressDrawn(a, uv.KeyPressEvent{Code: 't', Mod: uv.ModCtrl})
	typeDrawn(a, "steps.test.output")
	pressDrawn(a, enterPress)
	typeDrawn(a, " pass?")
	pressDrawn(a, tabPress)
	typeDrawn(a, "pass fail")
	pressDrawn(a, tabPress)
	for _, want := range []string{"auto", "local", "claude", "codex", "ollama", "opencode"} {
		if !strings.Contains(strings.Join(a.form.fields[2].Choices, " "), want) {
			t.Errorf("the agents offered %v lack %q", a.form.fields[2].Choices, want)
		}
	}
	pressDrawn(a, rightPress) // the agent after the default
	if !strings.Contains(screenText(a), "enter add") {
		t.Errorf("the form does not say enter adds:\n%s", screenText(a))
	}
	pressDrawn(a, enterPress)
	steps := loadFlowSteps(t, "review")
	if len(steps) != 2 || steps[1].Agent == nil || steps[1].ID != "agent" {
		t.Fatalf("steps %+v", steps)
	}
	agent := steps[1].Agent
	if agent.Ask != "Did {{.steps.test.output}} pass?" || agent.Use != a.form.fields[2].Choices[1] || strings.Join(agent.Choices, ",") != "pass,fail" || steps[1].Timeout != agentStepTimeout {
		t.Errorf("agent %+v, timeout %q", agent, steps[1].Timeout)
	}
	if !strings.Contains(chartText(a.currentTab().focusedView().(*inspectPanel).chart), "→ pass | fail") {
		t.Errorf("the agent's card does not tell its answers:\n%s", screenText(a))
	}
}
