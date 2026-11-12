package flow

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// AgentPresets are the agents noct knows how to ask, by name; the config adds others or changes
// these. None may act: claude runs with no tools, codex in its read-only sandbox, opencode with
// its read-only plan agent, and llm and ollama only answer, so a question carrying someone
// else's text cannot make them change anything.
var AgentPresets = map[string]string{
	"claude":   `claude -p --model haiku --tools ""`,
	"codex":    "codex exec --ephemeral --skip-git-repo-check --sandbox read-only -",
	"llm":      "llm",
	"ollama":   "ollama run --hidethinking " + OllamaModel,
	"opencode": "opencode run --agent plan {prompt}",
}

// presetOrder is the order auto looks for the presets in. ollama comes last: it runs the model
// on this machine, which is slower than the others.
var presetOrder = []string{"claude", "codex", "opencode", "llm", "ollama"}

// AutoAgent, as the default agent, asks the first agent this machine has: the config's own, then
// the presets in presetOrder.
const AutoAgent = "auto"

// OllamaModel is the model the ollama preset runs, and the one to pull to start from nothing.
const OllamaModel = "llama3.2"

// NoAgent is what a run tells when it finds no agent to ask, with one to start from.
const NoAgent = "no AI agent found here. noct knows claude, codex, opencode, llm and ollama, " +
	"and [agents] in the config adds any other. To start without an account, ollama runs " +
	"models on this machine for free: install it from https://ollama.com, then run " +
	"ollama pull " + OllamaModel + ". noct agents shows what noct finds"

// AgentNames are the agents a step can ask: the config's, then the presets, in the order auto
// looks for them.
func AgentNames(own map[string]string) []string {
	names := slices.Sorted(maps.Keys(own))
	for _, name := range presetOrder {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// AgentCommand is the command of the agent called name: the config's, else noct's preset.
func AgentCommand(own map[string]string, name string) (string, bool) {
	if command, ok := own[name]; ok {
		return command, true
	}
	command, ok := AgentPresets[name]
	return command, ok
}

// ResolveAgent is the agent name stands for, auto being the first one ready, and the command to
// ask it with; the error says why it cannot be asked.
func ResolveAgent(own map[string]string, name string) (resolved, command string, err error) {
	if name != AutoAgent {
		command, err = readyCommand(own, name)
		if _, known := AgentCommand(own, name); err != nil && known {
			err = fmt.Errorf("agent %s: %w", name, err)
		}
		return name, command, err
	}
	for _, name := range AgentNames(own) {
		if command, err := readyCommand(own, name); err == nil {
			return name, command, nil
		}
	}
	return "", "", errors.New(NoAgent)
}

// AgentStatus is, for each agent a step can ask, the command noct would run, or why it cannot.
type AgentStatus struct {
	Name, Command string
	Problem       error
}

// Agents tells which agents a step can ask and which of them this machine has.
func Agents(own map[string]string) []AgentStatus {
	var statuses []AgentStatus
	for _, name := range AgentNames(own) {
		command, err := readyCommand(own, name)
		if err != nil {
			command, _ = AgentCommand(own, name)
		}
		statuses = append(statuses, AgentStatus{Name: name, Command: command, Problem: err})
	}
	return statuses
}

// readyCommand is the agent's command when its program is installed. The ollama preset runs a
// model this machine already has when it lacks OllamaModel, rather than downloading one.
func readyCommand(own map[string]string, name string) (string, error) {
	command, ok := AgentCommand(own, name)
	if !ok {
		return "", fmt.Errorf("there is no agent %s: add its command under [agents] in the config", name)
	}
	argv, err := splitCommand(command)
	if err != nil || len(argv) == 0 {
		return "", fmt.Errorf("its command %q does not split into words", command)
	}
	if _, err := lookPath(argv[0]); err != nil {
		return "", fmt.Errorf("%s is not installed", argv[0])
	}
	at := ollamaModelAt(argv)
	if at < 0 {
		return command, nil
	}
	models := ollamaModels()
	if hasModel(models, argv[at]) {
		return command, nil
	}
	if _, own := own[name]; own || name != "ollama" || len(models) == 0 {
		return "", fmt.Errorf("ollama has no model %s; ollama pull %s gets it", argv[at], argv[at])
	}
	return strings.Replace(command, OllamaModel, models[0], 1), nil
}

// ollamaModelAt is where the model is in an ollama run command, or -1 for another command.
func ollamaModelAt(argv []string) int {
	if len(argv) < 3 || argv[0] != "ollama" || argv[1] != "run" {
		return -1
	}
	for i, arg := range argv[2:] {
		if !strings.HasPrefix(arg, "-") {
			return i + 2
		}
	}
	return -1
}

func hasModel(models []string, model string) bool {
	return slices.ContainsFunc(models, func(m string) bool { return m == model || m == model+":latest" })
}

// lookPath and ollamaModels are variables so tests can tell what this machine has.
var lookPath = exec.LookPath

// ollamaModels are the models ollama has, leaving out those that only embed text.
var ollamaModels = func() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ollama", "list").Output()
	if err != nil {
		return nil
	}
	var models []string
	for _, line := range strings.Split(string(out), "\n")[1:] {
		if fields := strings.Fields(line); len(fields) > 0 && !strings.Contains(fields[0], "embed") {
			models = append(models, fields[0])
		}
	}
	return models
}
