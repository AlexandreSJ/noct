package flow

import (
	"errors"
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
	r.Agents = map[string]string{"tester": command}
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
	r.Agents = map[string]string{"fake": command} // auto asks the config's own agents first
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

// machineWith makes this machine look as if it has only the programs given, and ollama the models.
func machineWith(t *testing.T, programs []string, models ...string) {
	t.Helper()
	look, list := lookPath, ollamaModels
	t.Cleanup(func() { lookPath, ollamaModels = look, list })
	lookPath = func(file string) (string, error) {
		if slices.Contains(programs, file) {
			return "/usr/bin/" + file, nil
		}
		return "", errors.New("not found")
	}
	ollamaModels = func() []string { return models }
}

func TestAutoAsksTheFirstAgentThisMachineHas(t *testing.T) {
	machineWith(t, []string{"opencode", "ollama"}, "qwen3.5:4b")
	name, command, err := ResolveAgent(nil, AutoAgent)
	if err != nil || name != "opencode" || command != AgentPresets["opencode"] {
		t.Errorf("auto gave %s %q %v, want opencode before ollama", name, command, err)
	}
	if _, _, err := ResolveAgent(map[string]string{"mine": "ollama run qwen3.5:4b"}, AutoAgent); err != nil {
		t.Error(err)
	}
	if name, _, _ := ResolveAgent(map[string]string{"mine": "ollama run qwen3.5:4b"}, AutoAgent); name != "mine" {
		t.Errorf("auto gave %s, want the config's own agent first", name)
	}
}

// The ollama preset runs a model the machine has instead of downloading llama3.2; a command the
// config wrote is run as written.
func TestOllamaRunsAModelItHas(t *testing.T) {
	machineWith(t, []string{"ollama"}, "qwen3.5:4b", "gpt-oss:20b")
	if _, command, err := ResolveAgent(nil, "ollama"); err != nil || command != "ollama run --hidethinking qwen3.5:4b" {
		t.Errorf("ollama gave %q %v", command, err)
	}
	machineWith(t, []string{"ollama"}, "llama3.2:latest")
	if _, command, _ := ResolveAgent(nil, "ollama"); command != AgentPresets["ollama"] {
		t.Errorf("ollama gave %q, want the preset's model, which it has", command)
	}
	_, _, err := ResolveAgent(map[string]string{"ollama": "ollama run mistral"}, "ollama")
	if err == nil || !strings.Contains(err.Error(), "ollama pull mistral") {
		t.Errorf("err = %v, want how to get the model the config names", err)
	}
	machineWith(t, []string{"ollama"})
	if _, _, err := ResolveAgent(nil, AutoAgent); err == nil {
		t.Error("auto took ollama with no model")
	}
}

func TestWithNoAgentTheRunSaysHowToGetOne(t *testing.T) {
	machineWith(t, nil)
	r, _ := testRunner(t, nil)
	f := parse(t, "steps:\n  - agent: {ask: hi}\n")
	result := r.Run(f, nil)
	if result.OK || !strings.Contains(result.Steps["0"].Message, "https://ollama.com") {
		t.Errorf("result %+v", result.Steps["0"])
	}
	for _, a := range Agents(nil) {
		if a.Problem == nil || !strings.Contains(a.Problem.Error(), "is not installed") {
			t.Errorf("%s: %v", a.Name, a.Problem)
		}
	}
}

// use is a template, so a param can pick the agent; empty is the default one.
func TestUseCanComeFromAParam(t *testing.T) {
	command, _ := fakeAgent(t, "ok")
	r, out := testRunner(t, nil)
	r.Agents = map[string]string{"picked": command}
	r.DefaultAgent = "nobody"
	f := parse(t, "params: {agent: picked}\nsteps:\n  - agent: {ask: hi, use: '{{.agent}}'}\n")
	if result := r.Run(f, nil); !result.OK {
		t.Fatalf("result %+v:\n%s", result.Steps["0"], out)
	}
	if result := r.Run(f, map[string]string{"agent": ""}); result.OK || !strings.Contains(result.Steps["0"].Message, "no agent nobody") {
		t.Errorf("an empty use did not ask the default: %+v", result.Steps["0"])
	}
}

// A model that reasons aloud names choices it then turns down; the answer is what comes after.
func TestTheAnswerLeavesOutTheThinking(t *testing.T) {
	for _, said := range []string{
		"Thinking...\nnot env, not flaky: a wrong value\n...done thinking.\n\nbug",
		"<think>could be env or flaky</think>\nbug",
	} {
		if got := pickChoice(withoutThinking(said), []string{"flaky", "bug", "env"}); got != "bug" {
			t.Errorf("from %q picked %q, want bug", said, got)
		}
	}
}
