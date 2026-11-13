package app

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/script"
)

// An agent step is added in the inspect from one form: what it asks, the answers it may give,
// which of the user's agents it asks (default, the config's; auto; the ones noct knows and the
// config's [agents]; or a custom one it adds) and how long it may take. enter adds it from any
// field. The step always says its agent in use: a use deleted by hand asks the default one.

// agentStepTimeout is the timeout a new agent step starts with, so a slow model cannot hold the
// flow forever.
const agentStepTimeout = "5m"

// customAgent, among the agents the form offers, adds an agent of the user's own to the config.
const customAgent = "custom agent"

// agentChoices are the agents a step can ask, those this machine has first, each telling whether
// it has it.
func (a *App) agentChoices() []flow.AgentStatus {
	agents := flow.Agents(a.config.Agents)
	ready := slices.DeleteFunc(slices.Clone(agents), func(s flow.AgentStatus) bool { return s.Problem != nil })
	return append(ready, slices.DeleteFunc(agents, func(s flow.AgentStatus) bool { return s.Problem == nil })...)
}

// openAgentForm asks for an agent step's settings in one form, and adds the step. Its agent is
// default (the config's), auto, one of the agents, or a custom agent the form then asks for.
func (a *App) openAgentForm(insert func(flow.Step), pickVariable func()) {
	agents := a.agentChoices()
	names := []string{flow.DefaultAgent, flow.AutoAgent}
	for _, agent := range agents {
		names = append(names, agent.Name)
	}
	names = append(names, customAgent)
	notes := agentNotes(a.config, agents)
	a.form = paramsForm{
		title: "ask an AI agent",
		fields: []script.Param{
			{Name: "ask", Prompt: "ask"},
			{Name: "answers", Prompt: "answers"},
			{Name: "agent", Prompt: "agent", Choices: names},
			{Name: "timeout", Prompt: "timeout"},
		},
		values: map[string]string{"agent": flow.DefaultAgent, "timeout": agentStepTimeout},
		verb:   "add",
		pick:   pickVariable,
		preview: func(values map[string]string) string {
			return agentFormNote(a.form.field().Name, values["agent"], notes)
		},
		submit: func(values map[string]string) error {
			ask := strings.TrimSpace(values["ask"])
			if ask == "" {
				return errors.New("ask: what the agent is asked")
			}
			step := flow.Step{Agent: &flow.Agent{Ask: ask, Use: values["agent"], Choices: strings.Fields(values["answers"])}, Timeout: strings.TrimSpace(values["timeout"])}
			if step.Agent.Use != customAgent {
				insert(step)
				return nil
			}
			a.askCustomAgent(func(name string) {
				step.Agent.Use = name
				insert(step)
			})
			return nil
		},
	}
	a.mode = formMode
}

// askCustomAgent asks for an agent of the user's own, its name and then its command, adds it to
// the config's [agents], and hands its name on.
func (a *App) askCustomAgent(added func(name string)) {
	a.openPrompt("name of the agent", "", "{enter} next  esc cancel  letters, digits and _, as mymodel", func(name string) {
		name = strings.TrimSpace(name)
		switch {
		case !naming.IsValid(name):
			a.statusMessage = "an agent's name is letters, digits and _, as mymodel"
			return
		case flow.ReservedAgent(name):
			a.statusMessage = name + " names another agent: pick another name"
			return
		}
		initial, _ := flow.AgentCommand(a.config.Agents, name)
		a.openPrompt("command of "+name, initial, "{enter} add  esc cancel  the question goes in place of {prompt}, or in as its input; give it no tool that acts", func(command string) {
			command = strings.TrimSpace(command)
			if _, _, err := flow.AgentArgv(command, "question"); err != nil {
				a.statusMessage = "command of " + name + ": " + err.Error()
				return
			}
			next := a.config
			next.Agents = maps.Clone(a.config.Agents)
			if next.Agents == nil {
				next.Agents = map[string]string{}
			}
			next.Agents[name] = command
			a.applyConfig(next)
			added(name)
		})
	})
}

// agentFormNote is the line under the form: what the field selected takes, or, on the agent,
// what agentNotes says of it.
func agentFormNote(field, name string, notes map[string]string) string {
	switch field {
	case "ask":
		return "a template; ctrl+t types a variable, as a step's output"
	case "answers":
		return "a space between, as: flaky bug env; empty takes any"
	case "timeout":
		return "like 30s or 5m; empty waits for as long as it takes"
	}
	return notes[name]
}

// agentNotes tell, for each agent the form offers, whether this machine has it, worked out once
// as the form opens: finding an agent may run a program, as ollama list.
func agentNotes(cfg config.Config, agents []flow.AgentStatus) map[string]string {
	notes := map[string]string{customAgent: "your own agent: enter asks its name and its command"}
	for _, agent := range agents {
		notes[agent.Name] = "✓ " + agent.Name + ": " + agent.Command
		if agent.Problem != nil {
			notes[agent.Name] = "✗ " + agent.Problem.Error()
		}
	}
	auto, _, err := flow.ResolveAgent(cfg.Agents, flow.AutoAgent)
	notes[flow.AutoAgent] = "✓ auto: asks " + auto + " now, the first agent ready here"
	if err != nil {
		notes[flow.AutoAgent] = "✗ no agent is ready here: noct agents says how to get one"
	}
	resolved, _, err := flow.ResolveAgent(cfg.Agents, cfg.Agent)
	switch {
	case err != nil:
		notes[flow.DefaultAgent] = "✗ default, from the config: " + err.Error()
	case cfg.Agent == flow.AutoAgent:
		notes[flow.DefaultAgent] = "✓ default, from the config: auto, " + resolved + " now"
	default:
		notes[flow.DefaultAgent] = "✓ default, from the config: " + resolved + " now"
	}
	return notes
}
