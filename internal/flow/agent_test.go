package flow

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeAgent is an agent's command that answers what answer says, writing the question it got
// beside it.
func fakeAgent(t *testing.T, answer string) (command string, asked func() string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "agent")
	question := filepath.Join(dir, "question")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat > "+question+"\nprintf '%s\\n' \""+answer+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, func() string { content, _ := os.ReadFile(question); return string(content) }
}

func TestAnAgentStepKeepsTheAnswerOfOneOfItsChoices(t *testing.T) {
	command, asked := fakeAgent(t, "I would say: Rollback, the health check failed.")
	f := parse(t, `steps:
  - {id: health, run: 'echo 503'}
  - id: review
    agent: {ask: 'Health said {{.output}}. Ship or roll back?', use: tester, choices: [ship, rollback]}
  - if: '{{eq .answer "rollback"}}'
    then: [{run: 'echo rolling back'}]
`)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	r, out := testRunner(t, nil)
	r.Agents = func(name string) (string, bool) { return command, name == "tester" }
	if result := r.Run(f, nil); !result.OK || result.Steps["1"].Captures["answer"] != "rollback" {
		t.Fatalf("result %+v:\n%s", result.Steps["1"], out)
	}
	if q := asked(); !strings.Contains(q, "Health said 503") || !strings.Contains(q, "exactly one of: ship, rollback") {
		t.Errorf("the agent was asked %q", q)
	}
	if !strings.Contains(out.String(), "rolling back") {
		t.Errorf("the if did not branch on the answer:\n%s", out)
	}
}

func TestAnAnswerOutsideTheChoicesFails(t *testing.T) {
	command, _ := fakeAgent(t, "maybe")
	r, out := testRunner(t, nil)
	r.Agents = func(string) (string, bool) { return command, true }
	f := parse(t, "steps:\n  - agent: {ask: 'yes or no?', choices: [yes, no]}\n")
	if result := r.Run(f, nil); result.OK || !strings.Contains(result.Steps["0"].Message, "did not answer one of yes, no") {
		t.Errorf("result %+v:\n%s", result.Steps["0"], out)
	}
}

func TestAnAgentsCommandTakesTheQuestionAsOneArgument(t *testing.T) {
	argv, input, err := AgentArgv(`opencode run --model "a b" {prompt}`, "hi; rm -rf /")
	if err != nil || !slices.Equal(argv, []string{"opencode", "run", "--model", "a b", "hi; rm -rf /"}) || input != "" {
		t.Errorf("argv %q input %q err %v", argv, input, err)
	}
	argv, input, _ = AgentArgv(AgentPresets["claude"], "hi")
	if !slices.Equal(argv, []string{"claude", "-p", "--model", "haiku", "--tools", ""}) || input != "hi" {
		t.Errorf("claude: argv %q input %q, want no tools and the question as input", argv, input)
	}
	if _, _, err := AgentArgv(`x "open`, "q"); err == nil {
		t.Error("took a command with an open quote")
	}
}

func TestAnAgentNoctDoesNotKnowStops(t *testing.T) {
	r, _ := testRunner(t, nil)
	f := parse(t, "steps:\n  - agent: {ask: hi, use: nobody}\n")
	if result := r.Run(f, nil); result.OK || !strings.Contains(result.Steps["0"].Message, "there is no agent nobody") {
		t.Errorf("result %+v", result.Steps["0"])
	}
}
