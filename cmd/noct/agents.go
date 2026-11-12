package main

import (
	"fmt"
	"io"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
)

// listAgents prints each agent a step can ask and whether this machine has it, then the one a
// step without use asks; with none installed, it says how to get one.
func listAgents(out io.Writer, cfg config.Config) error {
	for _, a := range flow.Agents(cfg.Agents) {
		if a.Problem != nil {
			fmt.Fprintf(out, "  · %-9s %s\n", a.Name, a.Problem)
		} else {
			fmt.Fprintf(out, "  ✓ %-9s %s\n", a.Name, a.Command)
		}
	}
	name, _, err := flow.ResolveAgent(cfg.Agents, cfg.Agent)
	switch {
	case err != nil && cfg.Agent == flow.AutoAgent:
		fmt.Fprintf(out, "\nno agent is ready here. To start without an account, ollama runs models on this\n"+
			"machine for free:\n  1. install it from https://ollama.com\n  2. ollama pull %s\n"+
			"then run noct agents again. You can also install one of the others or add yours under [agents].\n", flow.OllamaModel)
		return nil
	case err != nil:
		fmt.Fprintf(out, "\nagent = %q cannot be asked: %v\n", cfg.Agent, err)
	case cfg.Agent == flow.AutoAgent:
		fmt.Fprintf(out, "\nsteps without use ask %s: agent = \"auto\" takes the first one ready.\n", name)
	default:
		fmt.Fprintf(out, "\nsteps without use ask %s.\n", name)
	}
	fmt.Fprint(out, "pick one with agent = \"<name>\" in the config (prefix+,); [agents] changes a\n"+
		"command or adds another, such as a model of your own:\n  [agents]\n  ollama = \"ollama run --hidethinking qwen3.5:4b\"\n")
	return nil
}
