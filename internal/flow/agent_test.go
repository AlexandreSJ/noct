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
	r.Agents = AgentConfig{Own: map[string]string{"tester": command}}
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
	r.Agents = AgentConfig{Own: map[string]string{"fake": command}} // auto asks the config's own agents first
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
	argv, input, _ = AgentArgv(presetCommand(t, "claude", ""), "hi")
	if !slices.Equal(argv, []string{"claude", "-p", "--model", "haiku", "--tools", "", "--strict-mcp-config"}) || input != "hi" {
		t.Errorf("claude: argv %q input %q, want no tools, no MCP servers and the question as input", argv, input)
	}
	if argv, input, _ := AgentArgv("llm -m mini", "hi"); !slices.Equal(argv, []string{"llm", "-m", "mini"}) || input != "hi" {
		t.Errorf("a command with no {prompt}: argv %q input %q, want the question as input", argv, input)
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
	name, command, err := ResolveAgent(AgentConfig{}, AutoAgent)
	if err != nil || name != "opencode" || command != presetCommand(t, "opencode", "") {
		t.Errorf("auto gave %s %q %v, want opencode before ollama", name, command, err)
	}
	if _, _, err := ResolveAgent(AgentConfig{Own: map[string]string{"mine": "ollama run qwen3.5:4b"}}, AutoAgent); err != nil {
		t.Error(err)
	}
	if name, _, _ := ResolveAgent(AgentConfig{Own: map[string]string{"mine": "ollama run qwen3.5:4b"}}, AutoAgent); name != "mine" {
		t.Errorf("auto gave %s, want the config's own agent first", name)
	}
}

// The ollama preset runs a model the machine has instead of downloading llama3.2; a command the
// config wrote is run as written.
func TestOllamaRunsAModelItHas(t *testing.T) {
	machineWith(t, []string{"ollama"}, "qwen3.5:4b", "gpt-oss:20b")
	if _, command, err := ResolveAgent(AgentConfig{}, "ollama"); err != nil || command != "ollama run --hidethinking qwen3.5:4b < {prompt}" {
		t.Errorf("ollama gave %q %v", command, err)
	}
	machineWith(t, []string{"ollama"}, "llama3.2:latest")
	if _, command, _ := ResolveAgent(AgentConfig{}, "ollama"); command != presetCommand(t, "ollama", "") {
		t.Errorf("ollama gave %q, want the preset's model, which it has", command)
	}
	_, _, err := ResolveAgent(AgentConfig{Own: map[string]string{"ollama": "ollama run mistral"}}, "ollama")
	if err == nil || !strings.Contains(err.Error(), "ollama pull mistral") {
		t.Errorf("err = %v, want how to get the model the config names", err)
	}
	machineWith(t, []string{"ollama"})
	if _, _, err := ResolveAgent(AgentConfig{}, AutoAgent); err == nil {
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
	for _, a := range Agents(AgentConfig{}) {
		if a.Problem == nil || !strings.Contains(a.Problem.Error(), "is not installed") {
			t.Errorf("%s: %v", a.Name, a.Problem)
		}
	}
}

// use is a template, so a param can pick the agent; empty is the default one.
func TestUseCanComeFromAParam(t *testing.T) {
	command, _ := fakeAgent(t, "ok")
	r, out := testRunner(t, nil)
	r.Agents = AgentConfig{Own: map[string]string{"picked": command}}
	r.DefaultAgent = "nobody"
	f := parse(t, "params: {agent: picked}\nsteps:\n  - agent: {ask: hi, use: '{{.agent}}'}\n")
	if result := r.Run(f, nil); !result.OK {
		t.Fatalf("result %+v:\n%s", result.Steps["0"], out)
	}
	if result := r.Run(f, map[string]string{"agent": ""}); result.OK || !strings.Contains(result.Steps["0"].Message, "no agent nobody") {
		t.Errorf("an empty use did not ask the default: %+v", result.Steps["0"])
	}
}

// use: default asks the config's agent, as no use does; default and auto name no agent of the
// config's own.
func TestUseDefaultAsksTheConfigsAgent(t *testing.T) {
	command, _ := fakeAgent(t, "ok")
	r, out := testRunner(t, nil)
	r.Agents = AgentConfig{Own: map[string]string{"mine": command, "default": "nothing", "auto": "nothing"}}
	r.DefaultAgent = "mine"
	f := parse(t, "steps:\n  - agent: {ask: hi, use: default}\n  - agent: {ask: hi}\n")
	if result := r.Run(f, nil); !result.OK || strings.Count(out.String(), "ask mine") != 2 {
		t.Fatalf("result %+v:\n%s", result.Steps, out)
	}
	for use, want := range map[string]string{"": "mine", " default ": "mine", "auto": "auto", "ollama": "ollama"} {
		if got := StepAgent(use, "mine"); got != want {
			t.Errorf("StepAgent(%q) = %s, want %s", use, got, want)
		}
	}
	if got := StepAgent("default", ""); got != AutoAgent {
		t.Errorf("with no agent in the config, default is %s, want auto", got)
	}
	names := AgentNames(r.Agents)
	if slices.Contains(names, "default") || slices.Contains(names, "auto") {
		t.Errorf("a reserved name is listed as an agent: %v", names)
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

func presetCommand(t *testing.T, name, model string) string {
	t.Helper()
	command, ok := PresetCommand(name, model)
	if !ok {
		t.Fatalf("no preset %s", name)
	}
	return command
}

// The config picks a preset's model and noct writes the command around it, so a program's flags
// stay noct's to keep current; a model that is not one word is refused.
func TestTheConfigPicksAPresetsModel(t *testing.T) {
	machineWith(t, []string{"claude", "ollama", "opencode"}, "qwen3.5:4b", "gpt-oss:20b")
	c := AgentConfig{Models: map[string]string{"claude": "sonnet", "ollama": "gpt-oss:20b", "opencode": "ollama/qwen3.5:4b"}}
	for name, want := range map[string]string{
		"claude":   `claude -p --model sonnet --tools "" --strict-mcp-config < {prompt}`,
		"ollama":   "ollama run --hidethinking gpt-oss:20b < {prompt}",
		"opencode": "opencode run --agent plan --model ollama/qwen3.5:4b {prompt}",
	} {
		if _, command, err := ResolveAgent(c, name); err != nil || command != want {
			t.Errorf("%s gave %q %v, want %q", name, command, err, want)
		}
	}
	if got := presetCommand(t, "opencode", ""); got != "opencode run --agent plan {prompt}" {
		t.Errorf("opencode with no model gave %q, want the program's own", got)
	}
	c.Models["ollama"] = "mistral"
	if _, _, err := ResolveAgent(c, "ollama"); err == nil || !strings.Contains(err.Error(), "ollama pull mistral") {
		t.Errorf("err = %v, want how to get the model the config picks", err)
	}
	for _, model := range []string{"", "a b", "x;rm", `"q"`, "-v", "--dangerously-skip-permissions", "a\rb"} {
		if ValidModel(model) == nil {
			t.Errorf("took the model %q", model)
		}
	}
	// a model written by hand in the config is checked too, so it cannot give an agent a flag
	bad := AgentConfig{Models: map[string]string{"claude": "sonnet --allowedTools Bash"}}
	if _, _, err := ResolveAgent(bad, "claude"); err == nil || !strings.Contains(err.Error(), "[agent_models]") {
		t.Errorf("err = %v, want the model refused", err)
	}
	// a model of spaces is no model: ollama still runs one it has
	blank := AgentConfig{Models: map[string]string{"ollama": " "}}
	machineWith(t, []string{"ollama"}, "qwen3.5:4b")
	if _, command, err := ResolveAgent(blank, "ollama"); err != nil || command != "ollama run --hidethinking qwen3.5:4b < {prompt}" {
		t.Errorf("a blank model gave %q %v", command, err)
	}
}

// Agents tells which programs this machine has and the model each preset runs, so a list can
// leave out the ones it lacks.
func TestAgentsTellWhatIsInstalledAndItsModel(t *testing.T) {
	machineWith(t, []string{"ollama"}, "qwen3.5:4b")
	byName := map[string]AgentStatus{}
	for _, a := range Agents(AgentConfig{Own: map[string]string{"mine": "nothing-here"}}) {
		byName[a.Name] = a
	}
	if a := byName["ollama"]; !a.Installed || a.Model != "qwen3.5:4b" || a.Own {
		t.Errorf("ollama: %+v", a)
	}
	if a := byName["claude"]; a.Installed || a.Model != "haiku" {
		t.Errorf("claude: %+v", a)
	}
	if a := byName["mine"]; !a.Own || a.Installed || a.Model != "" {
		t.Errorf("mine: %+v", a)
	}
}

// The models an agent offers are the ones its program lists.
func TestAnAgentsModelsAreTheOnesItsProgramLists(t *testing.T) {
	machineWith(t, nil, "qwen3.5:4b")
	list := listModels
	t.Cleanup(func() { listModels = list })
	listModels = func(argv ...string) []string {
		switch argv[0] {
		case "opencode":
			return []string{"ollama/qwen3.5:4b", "anthropic/claude-sonnet", "warning: something odd"}
		case "llm":
			return []string{"OpenAI Chat: gpt-4o (aliases: 4o)", "Anthropic: claude-3 ", "Default: gpt-4o"}
		}
		return nil
	}
	for name, want := range map[string][]string{
		"opencode": {"anthropic/claude-sonnet", "ollama/qwen3.5:4b"},
		"llm":      {"gpt-4o", "claude-3"},
		"ollama":   {"qwen3.5:4b"},
		"codex":    nil,
	} {
		if got := AgentModels(name); !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}
