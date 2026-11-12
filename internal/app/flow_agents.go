package app

import (
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// An agent step is added in the inspect from what it asks, the answers it may give, and which of
// the user's agents it asks: the ones noct knows and the config's [agents].

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

// askAgentStep asks the answers an agent step may give and the agent it asks, then adds it.
func (a *App) askAgentStep(ask string, insert func(flow.Step)) {
	a.openPrompt("answers", "", "{enter} next  esc cancel  the answers it may give, a space between; empty for any", func(text string) {
		agent := &flow.Agent{Ask: ask, Choices: strings.Fields(text)}
		var options []choiceOption
		for i, choice := range a.agentChoices() {
			name, label := choice.Name, choice.Name
			switch {
			case name == a.config.Agent:
				label += " (default)"
			case choice.Problem != nil:
				label += " · not ready"
			}
			options = append(options, choiceOption{key: choiceKey(i), label: label, pick: func() {
				if name != a.config.Agent {
					agent.Use = name
				}
				insert(flow.Step{Agent: agent})
			}})
		}
		a.openChoice("agent to ask", options...)
	})
}
