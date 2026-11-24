package app

import (
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/config"
)

// openStepForm is the step form the focused inspect shows in place of its chart.
func openStepForm(t *testing.T, a *App) *stepForm {
	t.Helper()
	p, ok := a.currentTab().focusedView().(*inspectPanel)
	if !ok {
		t.Fatalf("the focused view is not the inspect")
	}
	form, ok := p.editor.(*stepForm)
	if !ok {
		t.Fatalf("the inspect shows no step form:\n%s", screenText(a))
	}
	return form
}

// pickOption presses space on the focused option cell until it holds value.
func pickOption(t *testing.T, a *App, value string) {
	t.Helper()
	form := openStepForm(t, a)
	for range form.grid.cell().options {
		if form.grid.cell().value() == value {
			return
		}
		pressDrawn(a, spacePress)
	}
	t.Fatalf("no option %q in %v", value, form.grid.cell().options)
}

// An agent step is set up in one form in place of the chart, what it asks, its answers, the
// agent and the timeout, and one enter adds it; {{. completes the variables in scope.
func TestAnAgentStepIsAddedFromOneForm(t *testing.T) {
	machineAgents(t)
	a := testApp(t)
	a.size = image.Pt(140, 50)
	a.config.Agents = map[string]string{"local": "ollama run qwen"}
	saveFlow(t, "review", "steps:\n  - {id: test, run: 'echo 3 failed'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('a'))
	form := openStepForm(t, a)
	if a.mode != normalMode {
		t.Errorf("the form took noct's mode %v; it belongs to the panel", a.mode)
	}
	typeDrawn(a, "Did {{.ou")
	if !strings.Contains(screenText(a), "variables here") {
		t.Errorf("the help does not list the variables:\n%s", screenText(a))
	}
	pressDrawn(a, tabPress)
	typeDrawn(a, " pass?")
	pressDrawn(a, tabPress)
	typeDrawn(a, "pass fail")
	pressDrawn(a, tabPress)
	options := form.grid.cell().options
	for _, want := range []string{"auto", "local", "ollama"} {
		if !slices.Contains(options, want) {
			t.Errorf("the agents offered %v lack %q", options, want)
		}
	}
	if slices.Contains(options, "codex") {
		t.Errorf("the agents offered %v hold codex, which this machine lacks", options)
	}
	pressDrawn(a, spacePress) // the agent after the default
	if !strings.Contains(screenText(a), "enter add") {
		t.Errorf("the form does not say enter adds:\n%s", screenText(a))
	}
	pressDrawn(a, enterPress)
	steps := loadFlowSteps(t, "review")
	if len(steps) != 2 || steps[1].Agent == nil || steps[1].ID != "agent" {
		t.Fatalf("steps %+v", steps)
	}
	agent := steps[1].Agent
	if agent.Ask != "Did {{.output}} pass?" || agent.Use != options[1] || strings.Join(agent.Choices, ",") != "pass,fail" || steps[1].Timeout != agentStepTimeout {
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
	typeDrawn(a, "Is it flaky?")
	pressDrawn(a, tabPress, tabPress)
	options := openStepForm(t, a).grid.cell().options
	if options[0] != "default" || options[1] != "auto" || options[len(options)-1] != customAgent {
		t.Fatalf("the agents offered: %v", options)
	}
	if !strings.Contains(screenText(a), "default: ") {
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

// custom agent shows rows for the agent's name and command in the same form; enter adds them to
// the config's [agents] and the step asks that agent, and a reserved name is refused in place.
func TestACustomAgentIsAddedToTheConfig(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 50)
	saveFlow(t, "review", "steps:\n  - {id: test, run: 'echo 3 failed'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('a'))
	typeDrawn(a, "Is it flaky?")
	pressDrawn(a, tabPress, tabPress)
	if strings.Contains(screenText(a), "command") {
		t.Errorf("the custom agent's rows show before it is picked:\n%s", screenText(a))
	}
	pickOption(t, a, customAgent)
	pressDrawn(a, tabPress, tabPress)
	typeDrawn(a, "auto")
	pressDrawn(a, enterPress)
	if !strings.Contains(screenText(a), "names another agent") || a.mode != normalMode {
		t.Errorf("auto was taken as a name:\n%s", screenText(a))
	}
	pressDrawn(a, clearLine)
	typeDrawn(a, "mymodel")
	pressDrawn(a, tabPress)
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
