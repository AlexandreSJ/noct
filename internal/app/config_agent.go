package app

import (
	"slices"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// The config's default agent is the one an agent step asks with use: default, or with no use. Its
// row offers auto, the agents this machine has, the ones it lacks, then a custom agent: on that
// one, enter asks for the agent's name and command, adds it to [agents] and makes it the default.

// agentSetting is what the default agent's row offers, read once while the panel is open.
type agentSetting struct {
	// choices are auto, the agents ready here, the others, then customAgent.
	choices []string
	// notes tell about each choice, as the agent step's form does.
	notes map[string]string
	// custom is on while the row shows customAgent, which is no agent yet and is not saved.
	custom bool
}

// agentSetting reads the agents the row offers, once while the panel is open.
func (a *App) agentSetting() *agentSetting {
	if p := &a.configPanel; p.agents == nil {
		agents := a.agentChoices()
		choices := []string{flow.AutoAgent}
		for _, agent := range agents {
			choices = append(choices, agent.Name)
		}
		p.agents = &agentSetting{choices: append(choices, customAgent), notes: agentNotes(a.config, agents)}
	}
	return a.configPanel.agents
}

// shownAgent is the agent the row shows: the config's, or customAgent while it is chosen.
func (a *App) shownAgent() string {
	if p := a.configPanel.agents; p != nil && p.custom {
		return customAgent
	}
	return a.config.Agent
}

// agentNote is the note under the row: while it is selected, whether this machine has the agent
// shown; otherwise what the setting does.
func (a *App) agentNote() string {
	if a.configPanel.selected != agentRow {
		return "agent steps with use: default, or no use, ask it"
	}
	s := a.agentSetting()
	name := a.shownAgent()
	if note, ok := s.notes[name]; ok {
		return note
	}
	return "✗ there is no agent " + name + ": add its command under [agents] in the config"
}

// stepAgent shows the next agent the row offers, saving it as the default; customAgent is only
// shown, until enter adds it.
func (a *App) stepAgent(step int) {
	s := a.agentSetting()
	i := slices.Index(s.choices, a.shownAgent())
	if i < 0 {
		i = 0 // an agent the config names but no longer has steps from auto
	}
	next := s.choices[(i+step+len(s.choices))%len(s.choices)]
	s.custom = next == customAgent
	if s.custom {
		return
	}
	cfg := a.config
	cfg.Agent = next
	a.applyConfig(cfg)
}

// addAgentFromConfig asks for a custom agent's name and command, then makes it the default. The
// panel shows again either way, reading the agents anew.
func (a *App) addAgentFromConfig() {
	back := func() {
		a.configPanel.agents = nil // the agent added, or none, changes what the row offers
		a.mode = configMode
	}
	a.askCustomAgent(func(name string) {
		cfg := a.config
		cfg.Agent = name
		a.applyConfig(cfg)
		back()
	}, back)
}
