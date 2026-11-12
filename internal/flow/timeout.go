package flow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
)

// timeoutExit is the exit code of a step that ran out of time, as timeout(1) gives.
const timeoutExit = 124

// StopGrace is how long a step that ran out of time has to end after SIGTERM, before SIGKILL.
const StopGrace = 2 * time.Second

// parseTimeout reads a timeout: a positive duration, or "" for no limit.
func parseTimeout(text string) (time.Duration, error) {
	if text == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(text)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("timeout %q is not a positive duration like 30s, 10m or 1h30m", text)
	}
	return d, nil
}

// validateTimeout checks a step's timeout: only script and run steps that the flow waits for
// have one.
func (s Step) validateTimeout(kind Kind) error {
	switch {
	case s.Timeout == "":
		return nil
	case kind != ScriptStep && kind != CommandStep && kind != AgentStep:
		return errors.New("timeout belongs to a script, run or agent step")
	case s.Background:
		return errors.New("a step in the background is not waited for, so it has no timeout")
	}
	_, err := parseTimeout(s.Timeout)
	return err
}

// TimeoutWarning tells when a flow that triggers start has no timeout, neither its own nor on a
// step: a run that hangs keeps its triggers from starting it again until the workspace restarts.
func (f Flow) TimeoutWarning() string {
	if f.Timeout != "" || len(f.TriggerLabels()) == 0 || anyStepTimeout(f.Steps) {
		return ""
	}
	return "no timeout: a run that hangs keeps its triggers from starting it again; add one like timeout: 1h to the flow or to its slow steps"
}

func anyStepTimeout(steps []Step) bool {
	for _, s := range steps {
		if s.Timeout != "" {
			return true
		}
		for _, inner := range s.Branches() {
			if anyStepTimeout(inner.Steps) {
				return true
			}
		}
	}
	return false
}

// startDeadline sets when the flow's timeout ends the run, if it has one.
func (x *running) startDeadline(f Flow) {
	if d, _ := parseTimeout(f.Timeout); d > 0 { // checked by Validate
		x.deadline, x.timeout = time.Now().Add(d), f.Timeout
	}
}

// runTimedOut tells whether the flow's timeout has passed.
func (x *running) runTimedOut() bool {
	return passed(x.deadline)
}

// stopTimedOut ends the run at the step at path because the flow's timeout passed.
func (x *running) stopTimedOut(path string) bool {
	return x.stop(path, "flow timed out after "+x.timeout, timeoutExit)
}

// sleep waits d, or until the flow's timeout, whichever comes first.
func (x *running) sleep(d time.Duration) {
	if !x.deadline.IsZero() {
		d = min(d, time.Until(x.deadline))
	}
	if d > 0 {
		x.Sleep(d)
	}
}

// stepDeadline is when a step must end: at its timeout or the flow's, whichever comes first;
// zero when neither has one.
func (x *running) stepDeadline(s Step) time.Time {
	d, _ := parseTimeout(s.Timeout) // checked by Validate
	if d == 0 {
		return x.deadline
	}
	end := time.Now().Add(d)
	if !x.deadline.IsZero() && x.deadline.Before(end) {
		return x.deadline
	}
	return end
}

// passed tells whether a deadline, when there is one, has come.
func passed(deadline time.Time) bool {
	return !deadline.IsZero() && !time.Now().Before(deadline)
}

// commandUntil is a shell command line that is stopped at the deadline, when there is one.
func (x *running) commandUntil(deadline time.Time, command string) (*exec.Cmd, context.CancelFunc) {
	if deadline.IsZero() {
		return exec.Command(x.Shell, "-c", command), func() {}
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	cmd := exec.CommandContext(ctx, x.Shell, "-c", command)
	stopGently(cmd, x.Stdin)
	return cmd, cancel
}

// stopGently makes the end of the command's context stop it with SIGTERM, then SIGKILL after
// StopGrace. A step whose input is not a terminal, as in a run by a trigger, gets a process group
// of its own so its children stop too; one that reads the terminal must stay in the terminal's
// group to read it, so only its shell is signalled.
func stopGently(cmd *exec.Cmd, stdin io.Reader) {
	cmd.WaitDelay = StopGrace
	if file, ok := stdin.(*os.File); ok && term.IsTerminal(file.Fd()) {
		cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		group := cmd.Process.Pid
		// The group is gone by then unless a child ignored SIGTERM; ESRCH is not worth reporting.
		time.AfterFunc(StopGrace, func() { _ = syscall.Kill(-group, syscall.SIGKILL) })
		return syscall.Kill(-group, syscall.SIGTERM)
	}
}
