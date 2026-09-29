package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/schedule"
	"github.com/AlexandreSJ/noct/internal/script"
)

// triggerTick is how often the enabled flows' triggers are looked at.
const triggerTick = 2 * time.Second

// watchTriggers runs the scheduler until the workspace stops. It reads the flows on each tick,
// so edits and enabling take effect without a restart; watch paths start where the daemon did.
func (a *App) watchTriggers() {
	dir, _ := os.Getwd() // without it, relative watch paths start at the filesystem root
	s := &schedule.Scheduler{
		Dir:        dir,
		ResultsDir: flow.ResultsDir(),
		Start: func(name, reason string) {
			a.onLoop(func() { a.startTriggered(name, reason) })
		},
	}
	tick := func(now time.Time) {
		flows, _ := flow.LoadAll(flow.Dir(), nil) // unreadable flows start nothing
		s.Tick(now, runnable(flows), flow.Enabled(flow.TriggersFile()))
	}
	tick(time.Now()) // the first look right away, so a run ending soon after start is news
	ticker := time.NewTicker(triggerTick)
	defer ticker.Stop()
	for {
		select {
		case <-a.stopped:
			return
		case now := <-ticker.C:
			tick(now)
		}
	}
}

// runnable leaves out the flows with problems and the drafts, which triggers must not start.
func runnable(flows []flow.Flow) []flow.Flow {
	var ok []flow.Flow
	for _, f := range flows {
		if f.Problem == nil && !f.Draft {
			ok = append(ok, f)
		}
	}
	return ok
}

// startTriggered runs a flow a trigger started, with no pane: what it prints goes to its log,
// and the run is recorded with what started it, for the calendar. A flow still running from a
// trigger is not started again.
func (a *App) startTriggered(name, reason string) {
	if a.triggered[name] {
		return
	}
	run := script.Run{Flow: name, At: time.Now(), Trigger: reason}
	run.Log = logstore.Path(logNameOf(run), run.At)
	log, err := logstore.Create(run.Log, maxPaneLogSize)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	cmd := exec.Command(orDefault(a.binary.path, "noct"), "flow", "run", name)
	cmd.Dir, cmd.Stdout, cmd.Stderr = a.workDir, log, log
	cmd.Env = append(os.Environ(), flow.TrackedEnv+"=1", flow.ResultEnv+"="+logstore.ResultPath(run.Log))
	fmt.Fprintf(log, "\x1b[2m▶ flow %s started by %s\x1b[0m\n", name, reason)
	if err := cmd.Start(); err != nil {
		log.Close()
		a.statusMessage = "flow " + name + ": " + err.Error()
		return
	}
	a.recordRun(run)
	if a.triggered == nil {
		a.triggered = map[string]bool{}
	}
	a.triggered[name] = true
	a.notice = "flow " + name + " started by " + reason
	go func() {
		_ = cmd.Wait() // the exit code says how it went
		log.Close()
		code := cmd.ProcessState.ExitCode()
		a.onLoop(func() { a.finishTriggered(run, code) })
	}()
}

func (a *App) finishTriggered(run script.Run, code int) {
	delete(a.triggered, run.Flow)
	if err := script.FinishRun(runsPath(), run, code); err != nil {
		a.statusMessage = err.Error()
	}
	if code != 0 {
		a.statusMessage = fmt.Sprintf("flow %s (%s) failed with exit %d", run.Flow, run.Trigger, code)
	}
}

// addTrigger asks what should start the flow by itself, adds it to the flow's file and turns the
// flow's triggers on: a schedule, files to watch, or another flow to follow.
func (a *App) addTrigger(f flow.Flow) {
	add := func(t flow.Trigger) {
		if err := a.changeFlow(f, func(f *flow.Flow) error {
			f.Triggers = append(f.Triggers, t)
			return nil
		}); err != nil {
			a.statusMessage = err.Error()
			return
		}
		if err := flow.SetEnabled(flow.TriggersFile(), f.Name, true); err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.notice = "flow " + f.Name + " now starts on " + t.Label() + "; o turns its triggers off"
	}
	a.openChoice("add trigger",
		choiceOption{key: "c", label: "cron schedule", pick: func() {
			a.openPrompt("cron", "0 9 * * 1-5", "enter add  esc cancel  minute hour day month weekday", func(text string) {
				add(flow.Trigger{Cron: text})
			})
		}},
		choiceOption{key: "w", label: "files changing", pick: func() {
			a.openPrompt("watch", "", "enter add  esc cancel  paths or globs, ** any folders, space between", func(text string) {
				add(flow.Trigger{Watch: &flow.Watch{Paths: strings.Fields(text)}})
			})
		}},
		choiceOption{key: "a", label: "after another flow", pick: func() {
			var items []finderItem
			for _, other := range a.flows {
				if other.Name != f.Name {
					items = append(items, finderItem{label: other.Name, kind: other.Description, open: func() {
						add(flow.Trigger{After: &flow.After{Flow: other.Name}})
					}})
				}
			}
			a.openFinderWith("after which flow", items)
		}},
	)
}
