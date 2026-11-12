package app

import (
	"errors"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/script"
)

// An agent step is added in the inspect from one form: what it asks, the answers it may give,
// which of the user's agents it asks (the ones noct knows and the config's [agents]) and how long
// it may take. enter adds it from any field.

// agentStepTimeout is the timeout a new agent step starts with, so a slow model cannot hold the
// flow forever.
const agentStepTimeout = "5m"

// agentChoices are the agents a step can ask, the default first, each telling whether this
// machine has it.
func (a *App) agentChoices() []flow.AgentStatus {
	agents := flow.Agents(a.config.Agents)
	i := slices.IndexFunc(agents, func(s flow.AgentStatus) bool { return s.Name == a.config.Agent })
	if i < 0 {
		return append([]flow.AgentStatus{{Name: a.config.Agent}}, agents...)
	}
	return append([]flow.AgentStatus{agents[i]}, slices.Delete(agents, i, i+1)...)
}

// openAgentForm asks for an agent step's settings in one form, and adds the step.
func (a *App) openAgentForm(insert func(flow.Step), pickVariable func()) {
	agents := a.agentChoices()
	auto, _, err := flow.ResolveAgent(a.config.Agents, flow.AutoAgent)
	if err != nil {
		auto = ""
	}
	var names []string
	for _, agent := range agents {
		names = append(names, agent.Name)
	}
	a.form = paramsForm{
		title: "ask an AI agent",
		fields: []script.Param{
			{Name: "ask", Prompt: "ask"},
			{Name: "answers", Prompt: "answers"},
			{Name: "agent", Prompt: "agent", Choices: names},
			{Name: "timeout", Prompt: "timeout"},
		},
		values: map[string]string{"agent": a.config.Agent, "timeout": agentStepTimeout},
		verb:   "add",
		pick:   pickVariable,
		preview: func(values map[string]string) string {
			return agentFormNote(a.form.field().Name, values["agent"], auto, agents)
		},
		submit: func(values map[string]string) error {
			ask := strings.TrimSpace(values["ask"])
			if ask == "" {
				return errors.New("ask: what the agent is asked")
			}
			step := flow.Step{Agent: &flow.Agent{Ask: ask, Choices: strings.Fields(values["answers"])}, Timeout: strings.TrimSpace(values["timeout"])}
			if values["agent"] != a.config.Agent {
				step.Agent.Use = values["agent"]
			}
			insert(step)
			return nil
		},
	}
	a.mode = formMode
}

// agentFormNote is the line under the form: what the field selected takes, or, on the agent,
// whether this machine has it; auto is the agent auto asks now, or "" with none ready.
func agentFormNote(field, name, auto string, agents []flow.AgentStatus) string {
	switch field {
	case "ask":
		return "a template; ctrl+t types a variable, as a step's output"
	case "answers":
		return "a space between, as: flaky bug env; empty takes any"
	case "timeout":
		return "like 30s or 5m; empty waits for as long as it takes"
	}
	if name == flow.AutoAgent {
		if auto == "" {
			return "✗ no agent is ready here: noct agents says how to get one"
		}
		return "✓ auto: asks " + auto + " now, the first agent ready here"
	}
	i := slices.IndexFunc(agents, func(s flow.AgentStatus) bool { return s.Name == name })
	switch {
	case i < 0:
		return name
	case agents[i].Problem != nil:
		return "✗ " + agents[i].Problem.Error()
	case agents[i].Command != "":
		return "✓ " + name + ": " + agents[i].Command
	}
	return name
}
