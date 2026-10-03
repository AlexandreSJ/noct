package flow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AlexandreSJ/noct/internal/logstore"
)

// Status is how a step went in a run.
type Status string

const (
	StatusOK     Status = "ok"
	StatusFailed Status = "failed"
)

// StepResult is how a step went, the last time the run passed through it.
type StepResult struct {
	Status   Status            `json:"status"`
	Exit     int               `json:"exit,omitempty"`
	Duration time.Duration     `json:"duration"`
	Captures map[string]string `json:"captures,omitempty"`
	// Output is the step's output as templates read it, cut to the runner's KeepOutput.
	Output string `json:"output,omitempty"`
	// Tried marks a step run alone, as a test, after the flow's last run.
	Tried bool `json:"tried,omitempty"`
	// Message says why the step failed, or what an if or repeat decided.
	Message string `json:"message,omitempty"`
}

// Result is how a run went, each step by its Path.
type Result struct {
	Flow  string                `json:"flow"`
	Start time.Time             `json:"start"`
	End   time.Time             `json:"end"`
	OK    bool                  `json:"ok"`
	Exit  int                   `json:"exit"`
	Steps map[string]StepResult `json:"steps"`
}

// ResultsDir keeps the last result of each flow.
func ResultsDir() string {
	return filepath.Join(logstore.Dir(), "flows")
}

// SaveResult keeps a run's result as its flow's last.
func SaveResult(dir string, r Result) error {
	return SaveResultTo(filepath.Join(dir, r.Flow+".json"), r)
}

// SaveResultTo writes a run's result to a file of its own.
func SaveResultTo(path string, r Result) error {
	content, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("saving the result: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("saving the result: %w", err)
	}
	if err := os.WriteFile(path+".tmp", content, 0o600); err != nil {
		return fmt.Errorf("saving the result: %w", err)
	}
	return os.Rename(path+".tmp", path)
}

func ReadResult(path string) (Result, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Result{}, false
	}
	var r Result
	if json.Unmarshal(content, &r) != nil {
		return Result{}, false
	}
	return r, true
}

// LastResult is the result of the flow's last run; ok is false when it never ran.
func LastResult(dir, name string) (Result, bool) {
	return ReadResult(filepath.Join(dir, name+".json"))
}
