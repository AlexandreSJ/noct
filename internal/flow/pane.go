package flow

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// PaneNew is the one value of a step's pane: a new pane beside the flow's run.
const PaneNew = "new"

// PaneRequest asks noct's workspace for a pane that runs a step's command line: waiting for its
// exit code, or, for a step in the background, only for the pane to open.
type PaneRequest struct {
	Command string
	Dir     string
	Title   string
	Wait    bool
}

// validatePane checks pane and background: only script and run steps have them, and a step in
// a pane of its own has no output to capture or check.
func (s Step) validatePane(kind Kind) error {
	switch {
	case s.Pane == "" && !s.Background:
		return nil
	case kind != ScriptStep && kind != CommandStep:
		return errors.New("pane and background belong to a script or run step")
	case s.Pane != PaneNew:
		return fmt.Errorf("pane is %q, or left out", PaneNew)
	case len(s.Capture) > 0 || s.Expect != nil:
		return errors.New("a step in its own pane has no output to capture or expect")
	}
	return nil
}

// runInPane runs a step's command line in a pane of its own and gives its exit code; a step in
// the background gives 0 once started. Outside noct's workspace there is no pane to open: the
// step runs in the flow's terminal, and in the background the flow does not wait for it.
func (x *running) runInPane(s Step, command, dir string) (string, int) {
	if x.OpenPane == nil {
		if !s.Background {
			return x.execute(command, dir)
		}
		return "", x.startDetached(command, dir)
	}
	exit, err := x.OpenPane(PaneRequest{Command: command, Dir: dir, Title: s.Label(), Wait: !s.Background})
	if err != nil {
		fmt.Fprintln(x.Out, "opening a pane:", err)
		return "", 1
	}
	return "", exit
}

// startDetached starts a command line without waiting for it. It writes to the flow's terminal
// directly, never through the flow's own output, which the flow keeps writing meanwhile; with
// no terminal, what it prints is dropped.
func (x *running) startDetached(command, dir string) int {
	cmd := exec.Command(x.Shell, "-c", command)
	cmd.Dir = dir
	if terminal, ok := x.Out.(*os.File); ok {
		cmd.Stdout, cmd.Stderr = terminal, terminal
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(x.Out, err)
		return 127
	}
	go func() { _ = cmd.Wait() }() // reaps it when it ends; the flow has moved on
	return 0
}
