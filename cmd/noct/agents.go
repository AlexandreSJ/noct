package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
)

// agentsCommand lists the agents, or, with default, tells or changes the one steps ask by default.
func agentsCommand(out io.Writer, args []string) error {
	switch {
	case len(args) == 0:
		return listAgents(out, loadConfig())
	case args[0] == "default" && len(args) <= 2:
		path, err := config.Path()
		if err != nil {
			return err
		}
		if len(args) == 1 {
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "agent = %q: steps with use: default, or no use, ask it.\n", cfg.Agent)
			return nil
		}
		return setDefaultAgent(out, path, args[1])
	}
	return errors.New("usage: noct agents [default [<name>]]")
}

// listAgents prints each agent a step can ask and whether this machine has it, then the one a
// step with use: default asks; with none installed, it says how to get one.
func listAgents(out io.Writer, cfg config.Config) error {
	for _, a := range flow.Agents(cfg.Agents) {
		if a.Problem != nil {
			fmt.Fprintf(out, "  · %-9s %s\n", a.Name, a.Problem)
		} else {
			fmt.Fprintf(out, "  ✓ %-9s %s\n", a.Name, a.Command)
		}
	}
	for name := range cfg.Agents {
		if flow.ReservedAgent(name) {
			fmt.Fprintf(out, "  ✗ %-9s is not asked: %s names another agent, so [agents] cannot use it\n", name, name)
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
		fmt.Fprintf(out, "\nuse: default asks %s: agent = \"auto\" takes the first one ready.\n", name)
	default:
		fmt.Fprintf(out, "\nuse: default asks %s.\n", name)
	}
	if ready := readyAgents(cfg); len(ready) > 1 || err != nil {
		fmt.Fprintf(out, "noct agents default <name> picks another, as noct agents default %s.\n", ready[len(ready)-1])
	}
	fmt.Fprint(out, "[agents] in the config (prefix+,) changes a command or adds another, such as a\n"+
		"model of your own:\n  [agents]\n  ollama = \"ollama run --hidethinking qwen3.5:4b\"\n")
	return nil
}

// readyAgents are the names of the agents this machine has, with auto first.
func readyAgents(cfg config.Config) []string {
	ready := []string{flow.AutoAgent}
	for _, a := range flow.Agents(cfg.Agents) {
		if a.Problem == nil {
			ready = append(ready, a.Name)
		}
	}
	return ready
}

// setDefaultAgent makes name the agent of agent = in the config at path. It must be an agent noct
// knows or the config adds, or auto; one this machine lacks is saved, with what it lacks.
func setDefaultAgent(out io.Writer, path, name string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if name == flow.DefaultAgent {
		return errors.New("default is the agent this command sets: give an agent's name, or auto")
	}
	if _, known := flow.AgentCommand(cfg.Agents, name); !known && name != flow.AutoAgent {
		return fmt.Errorf("there is no agent %s; noct agents lists them, and [agents] in the config adds one", name)
	}
	cfg.Agent = name
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	if _, _, err := flow.ResolveAgent(cfg.Agents, name); err != nil {
		fmt.Fprintf(out, "Saved agent = %q, but it cannot be asked yet: %v\n", name, err)
		return nil
	}
	fmt.Fprintf(out, "Saved agent = %q: steps with use: default, or no use, ask it from the next run.\n", name)
	return nil
}
