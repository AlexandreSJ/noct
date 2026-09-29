package app

import (
	"image"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/script"
)

// runLoopTask runs the next task a background goroutine sends to the loop.
func runLoopTask(t *testing.T, a *App) {
	t.Helper()
	select {
	case task := <-a.loopTasks:
		task()
	case <-time.After(5 * time.Second):
		t.Fatal("nothing came back to the loop")
	}
}

// A flow started by a trigger runs without a pane: its output goes to a log, and the run is
// recorded with what started it. While it runs, the trigger cannot start it again.
func TestTriggeredRunIsLoggedAndRecorded(t *testing.T) {
	a := testApp(t)
	a.loopTasks, a.stopped = make(chan func()), make(chan struct{})
	fake := filepath.Join(t.TempDir(), "noct")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"ran $*\"\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.binary.path = fake
	a.startTriggered("nightly", "cron 0 2 * * *")
	a.startTriggered("nightly", "cron 0 2 * * *")
	runLoopTask(t, a)
	runs, err := script.Runs(runsPath())
	if err != nil || len(runs) != 1 {
		t.Fatalf("runs = %+v, %v", runs, err)
	}
	run := runs[0]
	if run.Flow != "nightly" || run.Trigger != "cron 0 2 * * *" || run.Exit == nil || *run.Exit != 3 {
		t.Errorf("run = %+v", run)
	}
	if log, _ := os.ReadFile(run.Log); !strings.Contains(string(log), "ran flow run nightly") {
		t.Errorf("log = %q", log)
	}
	if a.triggered["nightly"] {
		t.Error("the flow still counts as running")
	}
}

// o in the inspect's list turns the flow's triggers on and off, and the chart tells them.
func TestInspectTurnsTriggersOnAndOff(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(120, 30)
	saveFlow(t, "nightly", "triggers: [{cron: '0 2 * * *'}]\nsteps: [{run: 'true'}]\n")
	pressDrawn(a, prefixPress, letter('i'), letter('o'))
	if !flow.Enabled(flow.TriggersFile())["nightly"] {
		t.Fatal("o did not enable the triggers")
	}
	screen := screenText(a)
	// The clock takes two cells, so the screen text has a space more after it.
	if !regexp.MustCompile(`⏰ +nightly`).MatchString(screen) || !strings.Contains(screen, "triggers on: cron 0 2 * * *") {
		t.Errorf("screen does not show the triggers on:\n%s", screen)
	}
	pressDrawn(a, letter('o'))
	if flow.Enabled(flow.TriggersFile())["nightly"] || !strings.Contains(screenText(a), "triggers off: cron 0 2 * * *") {
		t.Error("o did not turn the triggers off")
	}
}

// T adds a trigger without editing the file, and turns the flow's triggers on.
func TestInspectAddsTriggers(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "nightly", "steps: [{run: 'true'}]\n")
	saveFlow(t, "deploy", "steps: [{run: 'true'}]\n")
	pressDrawn(a, prefixPress, letter('i'), downPress, letter('T'), letter('c'), clearLine)
	typeDrawn(a, "0 2 * * *")
	pressDrawn(a, enterPress, letter('T'), letter('a'))
	typeDrawn(a, "deploy")
	pressDrawn(a, enterPress, letter('T'), letter('w'), clearLine)
	typeDrawn(a, "src/**/*.go docs/*.md")
	pressDrawn(a, enterPress)
	if a.statusMessage != "" {
		t.Fatalf("status: %s", a.statusMessage)
	}
	f, _ := flow.Load(filepath.Join(flow.Dir(), "nightly"+flow.Ext))
	if got := f.TriggerLabels(); !slices.Equal(got, []string{"cron 0 2 * * *", "after deploy", "watch src/**/*.go docs/*.md"}) {
		t.Errorf("triggers = %q", got)
	}
	if !flow.Enabled(flow.TriggersFile())["nightly"] {
		t.Error("adding a trigger did not turn them on")
	}
}
