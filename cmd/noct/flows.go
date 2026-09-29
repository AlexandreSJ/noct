package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/AlexandreSJ/noct/internal/client"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/script"
)

func loadFlows(lib script.Library) ([]flow.Flow, error) {
	return flow.LoadAll(flow.Dir(), lib.Find)
}

const flowUsage = "usage: noct flow run <flow> [--steps <first>..<last>] [name=value ...] | noct flow ls"

// flowCommand runs the flow subcommands: noct flow run <flow> [--steps a..b] [name=value ...],
// noct flow ls.
func flowCommand(args []string) error {
	if len(args) > 0 && args[0] == "ls" {
		return listFlows(os.Stdout)
	}
	if len(args) < 2 || args[0] != "run" {
		return errors.New(flowUsage)
	}
	name, args := args[1], args[2:]
	steps := ""
	if len(args) > 0 && args[0] == "--steps" {
		if len(args) < 2 {
			return errors.New(flowUsage)
		}
		steps, args = args[1], args[2:]
	}
	return runFlow(name, steps, args)
}

func listFlows(out io.Writer) error {
	flows, err := loadFlows(library())
	enabled := flow.Enabled(flow.TriggersFile())
	for _, f := range flows {
		if f.Problem != nil {
			fmt.Fprintf(out, "✗ %s\tbroken\t%s\n", f.Name, f.Problem)
			continue
		}
		fmt.Fprintf(out, "%s\t%s%s\n", f.Name, triggersText(f, enabled[f.Name]), f.Description)
	}
	return err
}

// triggersText says whether a flow's triggers are on and what they are, before its description.
func triggersText(f flow.Flow, on bool) string {
	labels := f.TriggerLabels()
	if len(labels) == 0 {
		return ""
	}
	state := "off"
	if on {
		state = "on"
	}
	return state + ": " + strings.Join(labels, ", ") + "\t"
}

// promote clears the draft mark of a flow or, when no flow has the name, of a script.
func promote(name string) error {
	lib := library()
	flows, err := loadFlows(lib)
	if err != nil {
		return err
	}
	if f, ok := flow.Find(flows, name); ok {
		fresh, err := flow.Load(f.File)
		if err != nil {
			return err
		}
		fresh.Draft = false
		return fresh.Save()
	}
	if s, ok := lib.Find(name); ok {
		return script.Promote(s)
	}
	return fmt.Errorf("no flow or script called %s", name)
}

// enableTriggers turns a flow's triggers on or off on this machine.
func enableTriggers(name string, on bool) error {
	flows, err := loadFlows(library())
	if err != nil {
		return err
	}
	if _, ok := flow.Find(flows, name); !ok {
		return fmt.Errorf("flow not found at %s: %s", flow.Dir(), name)
	}
	return flow.SetEnabled(flow.TriggersFile(), name, on)
}

// runFlow runs a flow in this terminal, or tests some of its steps, keeping its result for
// noct's inspect view and, unless noct's workspace runs it and keeps the record itself, the
// record of the run.
func runFlow(name, steps string, args []string) error {
	lib := library()
	flows, err := loadFlows(lib)
	if err != nil {
		return err
	}
	f, ok := flow.Find(flows, name)
	if !ok {
		return fmt.Errorf("flow not found at %s: %s", flow.Dir(), name)
	}
	if f.Problem != nil {
		return fmt.Errorf("flow %s is broken: %w", name, f.Problem)
	}
	if err := f.DraftError(); err != nil {
		return err
	}
	params := map[string]string{}
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if _, known := f.Params[key]; !ok || !known {
			return fmt.Errorf("%q is not name=value for a parameter of %s", arg, name)
		}
		params[key] = value
	}
	if err := f.CheckParams(params); err != nil {
		return fmt.Errorf("flow %s: %w", name, err)
	}
	record := script.Run{Flow: name, At: time.Now()}
	tracked := os.Getenv(flow.TrackedEnv) != ""
	if !tracked {
		if err := script.AddRun(runsPath(), record); err != nil {
			fmt.Fprintln(os.Stderr, "noct:", err)
		}
	}
	runner := &flow.Runner{
		Lookup: lib.Find,
		Shell:  shell(),
		Dir:    mustGetwd(),
		Getenv: os.Getenv,
		Stdin:  os.Stdin,
		Out:    os.Stdout,
		Sleep:  time.Sleep,
		// The config keeps kilobytes; 0, all of the output, stays 0.
		KeepOutput: loadConfig().Flows.OutputKB << 10,
		Ask:        terminalAsk(os.Stdin, os.Stdout),
		OpenPane:   workspacePanes(),
		Attention:  workspaceAttention(),
	}
	result, err := runSteps(runner, f, params, steps)
	if err != nil {
		result.Exit = 1 // the record ends like a stop that was not a program's
	}
	if err == nil {
		saveResults(result)
	}
	if !tracked {
		if err := script.FinishRun(runsPath(), record, result.Exit); err != nil {
			fmt.Fprintln(os.Stderr, "noct:", err)
		}
	}
	if err != nil {
		return err
	}
	if !result.OK {
		return exitCode(result.Exit)
	}
	return nil
}

// terminalAsk asks on the terminal and reads the answer's line, for on_fail: ask; without a
// terminal, as in a run by a trigger, there is nobody to ask.
func terminalAsk(in *os.File, out io.Writer) func(string) string {
	if !term.IsTerminal(in.Fd()) {
		return nil
	}
	return func(question string) string {
		fmt.Fprint(out, question)
		return readLine(in)
	}
}

// readLine reads up to the end of a line a byte at a time, so nothing after it is taken from the
// steps that read the terminal next.
func readLine(in io.Reader) string {
	var line []byte
	b := make([]byte, 1)
	for {
		n, err := in.Read(b)
		if n == 0 || err != nil || b[0] == '\n' {
			return string(line)
		}
		line = append(line, b[0])
	}
}

// workspacePanes opens the panes steps ask for, when the run is in a pane of noct's workspace;
// elsewhere those steps run in this terminal.
func workspacePanes() func(flow.PaneRequest) (int, error) {
	near, err := strconv.Atoi(os.Getenv("NOCT_PANE"))
	if err != nil {
		return nil
	}
	return func(r flow.PaneRequest) (int, error) {
		return client.OpenPane(ipc.SocketPath(), ipc.PaneRequest{Command: r.Command, Dir: r.Dir, Title: r.Title, Near: near, Wait: r.Wait})
	}
}

// workspaceAttention brings the run's pane to the screen before a prompt waits there, when the
// run is in a pane of noct's workspace.
func workspaceAttention() func() {
	near, err := strconv.Atoi(os.Getenv("NOCT_PANE"))
	if err != nil {
		return nil
	}
	return func() {
		// Best effort: the question is asked in this terminal either way.
		_ = client.Request(ipc.SocketPath(), "attention", map[string]int{"pane": near}, nil)
	}
}

// saveResults keeps the result as the flow's last and, when noct asks for it, beside the run's log.
func saveResults(result flow.Result) {
	if err := flow.SaveResult(flow.ResultsDir(), result); err != nil {
		fmt.Fprintln(os.Stderr, "noct:", err)
	}
	if path := os.Getenv(flow.ResultEnv); path != "" {
		if err := flow.SaveResultTo(path, result); err != nil {
			fmt.Fprintln(os.Stderr, "noct:", err)
		}
	}
}

// runSteps runs the whole flow, or tests the steps first..last (or one) over its last result.
func runSteps(runner *flow.Runner, f flow.Flow, params map[string]string, steps string) (flow.Result, error) {
	if steps == "" {
		return runner.Run(f, params), nil
	}
	first, last, isRange := strings.Cut(steps, "..")
	if !isRange {
		last = first
	}
	previous, _ := flow.LastResult(flow.ResultsDir(), f.Name) // never run: nothing to read
	return runner.RunSteps(f, params, first, last, previous)
}

// checkFlows reports every flow that cannot be run, and prints the warnings of the others.
func checkFlows(out io.Writer, lib script.Library) ([]error, int) {
	flows, err := loadFlows(lib)
	var problems []error
	if err != nil {
		problems = append(problems, err)
	}
	for _, f := range flows {
		if f.Problem != nil {
			problems = append(problems, f.Problem)
			continue
		}
		for _, w := range f.Warnings(lib.Find) {
			step, _ := f.Step(w.Path)
			fmt.Fprintf(out, "⚠ flow %s, step %s (%s): %s\n", f.Name, step.Label(), w.Path, w.Text)
		}
	}
	return problems, len(flows)
}
