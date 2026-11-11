package app

import (
	"image"
	"strings"
	"testing"
)

func TestAnAgentStepIsAddedFromWhatItAsks(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 50)
	a.config.Agents = map[string]string{"local": "ollama run qwen"}
	saveFlow(t, "review", "steps:\n  - {id: test, run: 'echo 3 failed'}\n")
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	pressDrawn(a, letter('a'), letter('g'))
	typeDrawn(a, "Did {{.output}} pass?")
	pressDrawn(a, enterPress)
	typeDrawn(a, "pass fail")
	pressDrawn(a, enterPress)
	screen := screenText(a)
	for _, want := range []string{"claude (default)", "local", "ollama", "opencode"} {
		if !strings.Contains(screen, want) {
			t.Errorf("the agents offered lack %q:\n%s", want, screen)
		}
	}
	pressDrawn(a, enterPress) // the default
	steps := loadFlowSteps(t, "review")
	if len(steps) != 2 || steps[1].Agent == nil || steps[1].Agent.Use != "" || strings.Join(steps[1].Agent.Choices, ",") != "pass,fail" || steps[1].ID != "agent" {
		t.Fatalf("steps %+v", steps)
	}
	if !strings.Contains(screenText(a), "→ pass | fail") {
		t.Errorf("the agent's card does not tell its answers:\n%s", screenText(a))
	}
}
