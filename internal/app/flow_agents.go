package app

import (
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/naming"
)

// An agent step is added in the inspect from its step form (agentForm): what it asks, the answers
// it may give, which of the user's agents it asks (default, the config's; auto; the ones noct knows
// and the config's [agents]; or a custom one it adds) and how long it may take. The step always
// says its agent in use: a use deleted by hand asks the default one.

// agentStepTimeout is the timeout a new agent step starts with, so a slow model cannot hold the
// flow forever.
const agentStepTimeout = "5m"

// customAgent, among the agents the form offers, adds an agent of the user's own to the config.
const customAgent = "custom agent"

// agentChoices are the agents a step can ask, of all of them, those ready first: the config's
// own and the ones this machine has, leaving out the ones noct knows that it lacks.
func agentChoices(all []flow.AgentStatus) []flow.AgentStatus {
	agents := slices.DeleteFunc(slices.Clone(all), func(s flow.AgentStatus) bool { return !s.Own && !s.Installed })
	ready := slices.DeleteFunc(slices.Clone(agents), func(s flow.AgentStatus) bool { return s.Problem != nil })
	return append(ready, slices.DeleteFunc(agents, func(s flow.AgentStatus) bool { return s.Problem == nil })...)
}

// askCustomAgent asks for an agent of the user's own, its name and then its command, adds it to
// the config's [agents], and hands its name on. back, when set, runs when either is cancelled or
// refused, such as to show the panel that asked again.
func (a *App) askCustomAgent(added func(name string), back func()) {
	if back == nil {
		back = func() {}
	}
	a.openPrompt("name of the agent", "", "{enter} next  esc cancel  letters, digits and _, as mymodel", func(name string) {
		name = strings.TrimSpace(name)
		switch {
		case !naming.IsValid(name):
			a.statusMessage = "an agent's name is letters, digits and _, as mymodel"
			back()
			return
		case flow.ReservedAgent(name):
			a.statusMessage = name + " names another agent: pick another name"
			back()
			return
		}
		initial := ""
		for _, s := range readAgents(agentConfigOf(a.config)) {
			if s.Name == name {
				initial = s.Command
			}
		}
		a.askAgentCommand(name, initial, added, back)
	})
	a.prompt.cancel = back
}

// askAgentCommand asks for the command of the agent called name, starting from initial, the one it
// runs now, and saves it in the config's [agents]; the same command changes nothing. Given back as
// noct writes it for one noct knows, it saves noct's again, so the program's flags stay noct's to
// keep current. back runs when it is cancelled or refused.
func (a *App) askAgentCommand(name, initial string, added func(name string), back func()) {
	a.openPrompt("command of "+name, initial, "{enter} save  esc cancel  {prompt} is the question", func(command string) {
		command = strings.TrimSpace(command)
		if _, _, err := flow.AgentArgv(command, "question"); err != nil {
			a.statusMessage = "command of " + name + ": " + err.Error()
			back()
			return
		}
		if command == initial {
			added(name)
			return
		}
		next := a.config
		next.Agents = maps.Clone(a.config.Agents)
		if next.Agents == nil {
			next.Agents = map[string]string{}
		}
		next.Agents[name] = command
		if preset, ok := flow.PresetCommand(name, a.config.AgentModels[name]); ok && preset == command {
			delete(next.Agents, name)
		}
		a.applyConfig(next)
		added(name)
	})
	a.prompt.cancel = back
}

// agentNotes tell, for each agent the form offers, whether it can be asked now, with its model.
func agentNotes(cfg config.Config, agents []flow.AgentStatus) map[string]string {
	notes := map[string]string{customAgent: "asks its name and command"}
	var auto flow.AgentStatus
	for _, agent := range agents {
		notes[agent.Name] = "✓ " + agentLabel(agent)
		if agent.Problem != nil {
			notes[agent.Name] = "✗ " + agent.Problem.Error()
		} else if auto.Name == "" {
			auto = agent
		}
	}
	notes[flow.AutoAgent] = "✓ auto: " + agentLabel(auto)
	if auto.Name == "" {
		notes[flow.AutoAgent] = "✗ no agent ready: see noct agents"
	}
	name := cfg.Agent
	if name == flow.AutoAgent {
		name = auto.Name
	}
	switch note, ok := notes[name]; {
	case cfg.Agent == flow.AutoAgent && auto.Name == "":
		notes[flow.DefaultAgent] = "✗ default: no agent ready"
	case !ok:
		notes[flow.DefaultAgent] = "✗ default: there is no agent " + cfg.Agent
	case strings.HasPrefix(note, "✗"):
		notes[flow.DefaultAgent] = "✗ default: " + strings.TrimPrefix(note, "✗ ")
	case cfg.Agent == flow.AutoAgent:
		notes[flow.DefaultAgent] = "✓ default: auto, " + strings.TrimPrefix(note, "✓ ")
	default:
		notes[flow.DefaultAgent] = "✓ default: " + strings.TrimPrefix(note, "✓ ")
	}
	return notes
}
