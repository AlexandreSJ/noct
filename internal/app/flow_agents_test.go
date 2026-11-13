package app

import (
	"image"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/config"
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

// A new agent step says use: default unless another agent is picked, and its card tells it asks
// the default agent.
func TestAnAgentStepAsksTheDefaultAgentByName(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 50)
	saveFlow(t, "review", "steps:\n  - {id: test, run: 'echo 3 failed'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('a'))
	choices := a.form.fields[2].Choices
	if choices[0] != "default" || choices[1] != "auto" || choices[len(choices)-1] != customAgent {
		t.Fatalf("the agents offered: %v", choices)
	}
	typeDrawn(a, "Is it flaky?")
	pressDrawn(a, tabPress, tabPress)
	if !strings.Contains(screenText(a), "default, from the config") {
		t.Errorf("the form does not tell what default asks:\n%s", screenText(a))
	}
	pressDrawn(a, enterPress)
	steps := loadFlowSteps(t, "review")
	if len(steps) != 2 || steps[1].Agent == nil || steps[1].Agent.Use != "default" {
		t.Fatalf("steps %+v", steps)
	}
	if text := chartText(a.currentTab().focusedView().(*inspectPanel).chart); !strings.Contains(text, "asks the default agent") {
		t.Errorf("the card does not say the default agent:\n%s", text)
	}
}

// custom agent asks for a name and a command, adds them to the config's [agents], and the step
// asks that agent; a reserved name is refused.
func TestACustomAgentIsAddedToTheConfig(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 50)
	saveFlow(t, "review", "steps:\n  - {id: test, run: 'echo 3 failed'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('a'))
	typeDrawn(a, "Is it flaky?")
	pressDrawn(a, tabPress, tabPress, leftPress)
	if a.form.values["agent"] != customAgent {
		t.Fatalf("left from default picked %q", a.form.values["agent"])
	}
	pressDrawn(a, enterPress)
	typeDrawn(a, "auto")
	pressDrawn(a, enterPress)
	if !strings.Contains(a.statusMessage, "names another agent") {
		t.Errorf("auto was taken as a name: %q", a.statusMessage)
	}
	pressDrawn(a, letter('a'), letter('a'))
	typeDrawn(a, "Is it flaky?")
	pressDrawn(a, tabPress, tabPress, leftPress, enterPress)
	typeDrawn(a, "mymodel")
	pressDrawn(a, enterPress)
	typeDrawn(a, "ollama run qwen3.5:4b")
	pressDrawn(a, enterPress)
	saved, err := config.Load(a.configPath)
	if err != nil || saved.Agents["mymodel"] != "ollama run qwen3.5:4b" || a.config.Agents["mymodel"] == "" {
		t.Fatalf("config agents %v, %v", saved.Agents, err)
	}
	steps := loadFlowSteps(t, "review")
	if len(steps) != 2 || steps[1].Agent.Use != "mymodel" {
		t.Fatalf("steps %+v", steps)
	}
}
