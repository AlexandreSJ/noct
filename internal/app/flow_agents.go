package app

import (
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// An agent step is added in the inspect from what it asks, the answers it may give, and which of
// the user's agents it asks: the ones noct knows and the config's [agents].

// agentNames are the agents a step can ask, the default first.
func (a *App) agentNames() []string {
	names := slices.Sorted(maps.Keys(flow.AgentPresets))
	for name := range a.config.Agents {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	if i := slices.Index(names, a.config.Agent); i > 0 {
		names = append([]string{a.config.Agent}, slices.Delete(names, i, i+1)...)
	}
	return names
}

// askAgentStep asks the answers an agent step may give and the agent it asks, then adds it.
func (a *App) askAgentStep(ask string, insert func(flow.Step)) {
	a.openPrompt("answers", "", "{enter} next  esc cancel  the answers it may give, a space between; empty for any", func(text string) {
		agent := &flow.Agent{Ask: ask, Choices: strings.Fields(text)}
		var options []choiceOption
		for i, name := range a.agentNames() {
			label := name
			if name == a.config.Agent {
				label += " (default)"
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
