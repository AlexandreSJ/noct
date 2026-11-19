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
	"unicode"
)

// agentPreset is how noct asks an agent it knows: the command, with {model} where the model goes,
// so the config only names the model and noct keeps the program's flags current.
type agentPreset struct {
	command string
	// flag goes before the model, as --model; "" when the model is a word of its own.
	flag string
	// model is the one it runs when the config names none; "" leaves the program its own.
	model string
}

// agentPresets are the agents noct knows how to ask, by name; the config picks their models, or
// gives others, or its own command for one of these. None may act: claude runs with no tools,
// codex in its read-only sandbox, opencode with its read-only plan agent, and llm and ollama only
// answer, so a question carrying someone else's text cannot make them change anything.
var agentPresets = map[string]agentPreset{
	"claude":   {`claude -p {model} --tools "" --strict-mcp-config < {prompt}`, "--model", "haiku"},
	"codex":    {"codex exec --ephemeral --skip-git-repo-check --sandbox read-only {model} - < {prompt}", "--model", ""},
	"llm":      {"llm {model} < {prompt}", "-m", ""},
	"ollama":   {"ollama run --hidethinking {model} < {prompt}", "", OllamaModel},
	"opencode": {"opencode run --agent plan {model} {prompt}", "--model", ""},
}

// AgentConfig is what the config says of the agents: Own are the commands of agents by name, its
// own or over a preset's; Models are the model each preset runs, by name, over the preset's own.
type AgentConfig struct {
	Own    map[string]string
	Models map[string]string
}

// IsPreset tells whether noct knows how to ask the agent called name.
func IsPreset(name string) bool {
	_, ok := agentPresets[name]
	return ok
}

// PresetModel is the model a preset runs when the config names none; "" for the program's own.
func PresetModel(name string) string { return agentPresets[name].model }

// PresetCommand is the command of a preset running model, its own one for "".
func PresetCommand(name, model string) (string, bool) {
	p, ok := agentPresets[name]
	if !ok {
		return "", false
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = p.model
	}
	words := model
	if model != "" && p.flag != "" {
		words = p.flag + " " + model
	}
	return strings.Join(strings.Fields(strings.Replace(p.command, "{model}", words, 1)), " "), true
}

// ValidModel tells why model cannot go in a command as the model's one word, or nil: a word
// with a space, a quote or a shell sign would change the command, and one starting with - would
// pass a flag, such as one that gives an agent tools.
func ValidModel(model string) error {
	if model == "" || strings.HasPrefix(model, "-") || strings.ContainsAny(model, "\"'`\\{}$;|&<>") ||
		strings.ContainsFunc(model, unicode.IsSpace) {
		return fmt.Errorf("a model is one word, as qwen3.5:4b or sonnet")
	}
	return nil
}

// presetOrder is the order auto looks for the presets in. ollama comes last: it runs the model
// on this machine, which is slower than the others.
var presetOrder = []string{"claude", "codex", "opencode", "llm", "ollama"}

// AutoAgent, as the default agent, asks the first agent this machine has: the config's own, then
// the presets in presetOrder.
const AutoAgent = "auto"

// DefaultAgent, as a step's use, asks the agent the config names as the default; a step with no
// use asks it too.
const DefaultAgent = "default"

// ReservedAgent tells whether name stands for another agent, so no agent of the config may have it.
func ReservedAgent(name string) bool {
	return name == AutoAgent || name == DefaultAgent
}

// StepAgent is the agent a step's use, rendered, names: the default one, configured, for "" or
// default, and auto when the config names none.
func StepAgent(use, configured string) string {
	use = strings.TrimSpace(use)
	if use == "" || use == DefaultAgent {
		use = strings.TrimSpace(configured)
	}
	if use == "" || use == DefaultAgent {
		return AutoAgent
	}
	return use
}

// OllamaModel is the model the ollama preset runs, and the one to pull to start from nothing.
const OllamaModel = "llama3.2"

// NoAgent is what a run tells when it finds no agent to ask, with one to start from.
const NoAgent = "no AI agent found here. noct knows claude, codex, opencode, llm and ollama, " +
	"and [agents] in the config adds any other. To start without an account, ollama runs " +
	"models on this machine for free: install it from https://ollama.com, then run " +
	"ollama pull " + OllamaModel + ". noct agents shows what noct finds"

// AgentNames are the agents a step can ask: the config's, then the presets, in the order auto
// looks for them.
func AgentNames(c AgentConfig) []string {
	names := slices.DeleteFunc(slices.Sorted(maps.Keys(c.Own)), ReservedAgent)
	for _, name := range presetOrder {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// AgentCommand is the command of the agent called name: the config's, else noct's preset running
// the model the config picks for it.
func AgentCommand(c AgentConfig, name string) (string, bool) {
	if ReservedAgent(name) {
		return "", false
	}
	if command, ok := c.Own[name]; ok {
		return command, true
	}
	return PresetCommand(name, c.Models[name])
}

// ResolveAgent is the agent name stands for, auto being the first one ready, and the command to
// ask it with; the error says why it cannot be asked.
func ResolveAgent(c AgentConfig, name string) (resolved, command string, err error) {
	if name != AutoAgent {
		command, err = readyCommand(c, name)
		if _, known := AgentCommand(c, name); err != nil && known {
			err = fmt.Errorf("agent %s: %w", name, err)
		}
		return name, command, err
	}
	for _, name := range AgentNames(c) {
		if command, err := readyCommand(c, name); err == nil {
			return name, command, nil
		}
	}
	return "", "", errors.New(NoAgent)
}

// AgentStatus is, for each agent a step can ask, the command noct would run, or why it cannot.
// Own tells the config gives its command; Installed, that this machine has its program, even when
// it cannot be asked yet, as ollama with no model. Model is the model a preset runs, "" for the
// program's own or for an agent of the config's.
type AgentStatus struct {
	Name, Command, Model string
	Problem              error
	Own, Installed       bool
}

// Agents tells which agents a step can ask and which of them this machine has.
func Agents(c AgentConfig) []AgentStatus {
	var statuses []AgentStatus
	for _, name := range AgentNames(c) {
		command, err := readyCommand(c, name)
		if err != nil {
			command, _ = AgentCommand(c, name)
		}
		_, own := c.Own[name]
		status := AgentStatus{Name: name, Command: command, Problem: err, Own: own, Installed: installed(command)}
		if !own {
			status.Model = orModel(c.Models[name], PresetModel(name))
			if argv, _ := splitCommand(command); ollamaModelAt(argv) > 0 {
				status.Model = argv[ollamaModelAt(argv)] // the one it falls back to
			}
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func orModel(model, fallback string) string {
	if model = strings.TrimSpace(model); model != "" {
		return model
	}
	return fallback
}

// installed tells whether this machine has the program a command runs.
func installed(command string) bool {
	argv, err := splitCommand(command)
	if err != nil || len(argv) == 0 {
		return false
	}
	_, err = lookPath(argv[0])
	return err == nil
}

// readyCommand is the agent's command when its program is installed. The ollama preset runs a
// model this machine already has when it lacks OllamaModel and the config picks none, rather than
// downloading one.
func readyCommand(c AgentConfig, name string) (string, error) {
	_, own := c.Own[name]
	if model := strings.TrimSpace(c.Models[name]); !own && IsPreset(name) && model != "" {
		if err := ValidModel(model); err != nil {
			return "", fmt.Errorf("[agent_models] %s = %q: %w", name, model, err)
		}
	}
	command, ok := AgentCommand(c, name)
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
	if own || name != "ollama" || strings.TrimSpace(c.Models[name]) != "" || len(models) == 0 {
		return "", fmt.Errorf("ollama has no model %s; ollama pull %s gets it", argv[at], argv[at])
	}
	command, _ = PresetCommand(name, models[0])
	return command, nil
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

// AgentModels are the models the preset called name can run, as its program tells them: claude's
// names for its latest models, the ones ollama has, and the ones opencode and llm list. Listing may
// take seconds, as opencode looks its models up, so it is no work for the event loop. nil when the
// agent lists none, such as codex, whose model is typed.
func AgentModels(name string) []string {
	switch name {
	case "claude":
		return []string{"haiku", "sonnet", "opus", "fable"}
	case "ollama":
		return ollamaModels()
	case "opencode":
		// one model a line, as provider/model
		models := slices.DeleteFunc(listModels("opencode", "models"), func(line string) bool { return ValidModel(line) != nil })
		return slices.Compact(slices.Sorted(slices.Values(models)))
	case "llm":
		var models []string
		for _, line := range listModels("llm", "models") {
			// each line names the model after its plugin, as OpenAI Chat: gpt-4o (aliases: 4o)
			if _, after, ok := strings.Cut(line, ": "); ok && !strings.HasPrefix(line, "Default:") {
				if fields := strings.Fields(after); len(fields) > 0 {
					models = append(models, fields[0])
				}
			}
		}
		return models
	}
	return nil
}

// listModels is the lines a program prints when it lists its models, or nil when it fails. It is
// a variable so tests can tell what a program lists.
var listModels = func(argv ...string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, argv[0], argv[1:]...).Output()
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
