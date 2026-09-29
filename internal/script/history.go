package script

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// RunsFile is the name of the list of recent runs in noct's state directory.
const RunsFile = "runs.json"

// runsKept bounds the list: older runs are forgotten.
const runsKept = 1000

// Run is one time a script or a flow was started, from the TUI or the CLI.
type Run struct {
	Script string `json:"script,omitempty"`
	// Flow is set instead of Script for a flow's run.
	Flow string    `json:"flow,omitempty"`
	At   time.Time `json:"at"`
	// Exit is the exit code, once the run ended; a run that never reported one was cut short.
	Exit *int `json:"exit,omitempty"`
	// Log is the file of what the run printed, when it ran in a pane.
	Log string `json:"log,omitempty"`
	// Trigger is what started a flow's run by itself, such as "cron 0 9 * * *"; "" when started
	// by hand.
	Trigger string `json:"trigger,omitempty"`
}

// Name is the script's or the flow's name, as its ID.
func (r Run) Name() string {
	if r.Flow != "" {
		return "flow " + r.Flow
	}
	return r.Script
}

// Failed tells whether the run ended with an exit code other than 0.
func (r Run) Failed() bool {
	return r.Exit != nil && *r.Exit != 0
}

// Runs reads the recent runs, oldest first; a missing file is no runs.
func Runs(path string) ([]Run, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the runs: %w", err)
	}
	var runs []Run
	if err := json.Unmarshal(content, &runs); err != nil {
		return nil, fmt.Errorf("reading the runs %s: %w", path, err)
	}
	return runs, nil
}

// RecordRun adds a script's run to the list, keeping only the latest runsKept.
func RecordRun(path, script string, at time.Time) error {
	return AddRun(path, Run{Script: script, At: at})
}

// AddRun adds a run to the list, keeping only the latest runsKept.
func AddRun(path string, run Run) error {
	return updateRuns(path, func(runs []Run) []Run { return append(runs, run) })
}

// FinishRun sets the exit code of the run started at the moment given, for the same script or flow.
func FinishRun(path string, run Run, exit int) error {
	return updateRuns(path, func(runs []Run) []Run {
		for i := len(runs) - 1; i >= 0; i-- {
			if runs[i].At.Equal(run.At) && runs[i].Script == run.Script && runs[i].Flow == run.Flow {
				runs[i].Exit = &exit
				break
			}
		}
		return runs
	})
}

// ForgetLogs clears the log of the runs whose log is one of those given, after they were deleted.
func ForgetLogs(path string, logs map[string]bool) error {
	return updateRuns(path, func(runs []Run) []Run {
		for i := range runs {
			if logs[runs[i].Log] {
				runs[i].Log = ""
			}
		}
		return runs
	})
}

func updateRuns(path string, change func([]Run) []Run) error {
	runs, err := Runs(path)
	if err != nil {
		runs = nil // a damaged list starts over rather than blocking runs
	}
	runs = change(runs)
	runs = runs[max(len(runs)-runsKept, 0):]
	content, err := json.Marshal(runs)
	if err != nil {
		return fmt.Errorf("saving the runs: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("saving the runs: %w", err)
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, content, 0o600); err != nil {
		return fmt.Errorf("saving the runs: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("saving the runs: %w", err)
	}
	return nil
}

// LastRuns is when each script last ran; flows are left out.
func LastRuns(runs []Run) map[string]time.Time {
	last := map[string]time.Time{}
	for _, run := range runs {
		if run.Script != "" && run.At.After(last[run.Script]) {
			last[run.Script] = run.At
		}
	}
	return last
}

// SortByName orders scripts alphabetically.
func SortByName(scripts []Script) {
	slices.SortFunc(scripts, func(a, b Script) int { return cmp.Compare(a.Name, b.Name) })
}

// SortByEdited puts the scripts edited most recently first.
func SortByEdited(scripts []Script) {
	slices.SortStableFunc(scripts, func(a, b Script) int { return b.Edited.Compare(a.Edited) })
}

// SortByLastRun puts the scripts run most recently first; those never run follow by name.
func SortByLastRun(scripts []Script, lastRun map[string]time.Time) {
	SortByName(scripts)
	slices.SortStableFunc(scripts, func(a, b Script) int { return lastRun[b.ID()].Compare(lastRun[a.ID()]) })
}
