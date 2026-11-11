package flow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/AlexandreSJ/noct/internal/atomicfile"
)

// While a flow runs, the step it is on is kept beside its last result, for the inspect to show the
// run as it goes. The file goes when the run ends; one left by a run that died is ignored.

// Progress is where a run is: the path of the step it runs, since when, and the run's process.
type Progress struct {
	Path  string    `json:"path"`
	Since time.Time `json:"since"`
	PID   int       `json:"pid"`
}

// ProgressPath is the file of a flow's run in progress.
func ProgressPath(dir, name string) string {
	return filepath.Join(dir, name+".running.json")
}

// SaveProgress notes that this process runs the step at path of the flow.
func SaveProgress(dir, name, path string) error {
	content, err := json.Marshal(Progress{Path: path, Since: time.Now(), PID: os.Getpid()})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return atomicfile.Write(ProgressPath(dir, name), content, 0o600)
}

// ReadProgress is where a run of the flow is now, false when none runs.
func ReadProgress(dir, name string) (Progress, bool) {
	content, err := os.ReadFile(ProgressPath(dir, name))
	if err != nil {
		return Progress{}, false
	}
	var p Progress
	if json.Unmarshal(content, &p) != nil || !alive(p.PID) {
		return Progress{}, false
	}
	return p, true
}

// ClearProgress forgets the flow's run in progress, as it ends.
func ClearProgress(dir, name string) {
	_ = os.Remove(ProgressPath(dir, name)) // a run that left none has nothing to clear
}

func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
