// Command noct attaches the terminal to noct's workspace, starting its daemon when needed. The
// commands and flags are in help, which noct -h prints.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/AlexandreSJ/noct/internal/client"
	"github.com/AlexandreSJ/noct/internal/daemon"
	"github.com/AlexandreSJ/noct/internal/ipc"
)

const usage = "usage: noct [-h] [ls | check | -s <script> [name=value ...] | flow run <flow> [--steps <first>..<last>] [name=value ...] | flow ls | enable <flow> | disable <flow> | promote <flow or script> | mcp | quit]"

const help = `noct: a terminal workspace for people who test.

Usage:
  noct                              attach to the workspace, starting it if needed
  noct -s, --script <script> [name=value ...]
                                    run a script in this terminal; parameters not given
                                    are asked, or keep their defaults without a terminal.
                                    <script> is its name, or folder/name inside a folder
  noct ls                           list the scripts, their program version and folder
  noct flow run <flow> [--steps <first>..<last>] [name=value ...]
                                    run a flow in this terminal, step by step; --steps
                                    tests only those steps (paths like 2 or 0/steps/1),
                                    reading the values of the flow's last run
  noct flow ls                      list the flows, with their triggers on or off
  noct enable <flow>                let the flow's triggers (cron, watch, after) start it;
                                    the running workspace starts it, even detached
  noct disable <flow>               turn the flow's triggers off
  noct promote <flow or script>     let a draft an AI wrote run, once reviewed
  noct mcp                          serve noct to an AI client (Claude Code and others) as an
                                    MCP server on stdin and stdout; see docs/mcp.md
  noct check                        report the scripts and flows that cannot be loaded,
                                    and warn about steps read where they may not have run
  noct quit                         close every pane and stop the workspace
  noct -h, --help                   show this help

Inside noct, press ctrl+space (the prefix) to see the keys.
Settings: ~/.config/noct/config.toml (prefix+c inside noct). Scripts: prefix+s.
Flows: prefix+i. Logs and the calendar of runs: prefix+v.
`

func main() {
	err := run(os.Args[1:])
	var code exitCode
	switch {
	case errors.As(err, &code):
		os.Exit(int(code))
	case err != nil:
		fmt.Fprintln(os.Stderr, "noct:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	socket := ipc.SocketPath()
	if len(args) == 0 {
		return attach(socket)
	}
	switch args[0] {
	case "quit":
		return client.Quit(socket)
	case "daemon":
		return daemon.Run(socket)
	case "ls":
		return listScripts(os.Stdout)
	case "check":
		return checkScripts(os.Stdout)
	case "run", "-s", "--script":
		return runScript(args[1:])
	case "flow":
		return flowCommand(args[1:])
	case "mcp":
		return serveMCP()
	case "promote":
		if len(args) != 2 {
			return errors.New("usage: noct promote <flow or script>")
		}
		return promote(args[1])
	case "enable", "disable":
		if len(args) != 2 {
			return fmt.Errorf("usage: noct %s <flow>", args[0])
		}
		return enableTriggers(args[1], args[0] == "enable")
	case "-h", "--help", "help":
		fmt.Print(help)
		return nil
	}
	return fmt.Errorf("unknown command %q; %s", args[0], usage)
}

func attach(socket string) error {
	if err := daemon.EnsureRunning(socket); err != nil {
		return err
	}
	reason, err := client.Attach(socket)
	if err != nil {
		return err
	}
	if reason != "quit" {
		fmt.Printf("[noct %s; run noct to attach again]\n", reason)
	}
	return nil
}
