package schedule

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// newTestScheduler records what it starts.
func newTestScheduler(t *testing.T) (*Scheduler, *[]string) {
	t.Helper()
	var started []string
	s := &Scheduler{
		Dir:        t.TempDir(),
		ResultsDir: t.TempDir(),
		Start:      func(name, reason string) { started = append(started, name+": "+reason) },
	}
	return s, &started
}

func at(clock string) time.Time {
	v, _ := time.ParseInLocation("2006-01-02 15:04:05", "2026-09-28 "+clock, time.Local)
	return v
}

func TestCronStartsEnabledFlowsOncePerMinute(t *testing.T) {
	s, started := newTestScheduler(t)
	flows := []flow.Flow{
		{Name: "morning", Triggers: []flow.Trigger{{Cron: "0 9 * * *"}}},
		{Name: "off", Triggers: []flow.Trigger{{Cron: "0 9 * * *"}}},
	}
	enabled := map[string]bool{"morning": true}
	for _, clock := range []string{"08:59:58", "09:00:00", "09:00:30", "09:01:00"} {
		s.Tick(at(clock), flows, enabled)
	}
	if !slices.Equal(*started, []string{"morning: cron 0 9 * * *"}) {
		t.Errorf("started %q", *started)
	}
}

// The minute the scheduler starts in is not a reason to run: a restart would run it twice.
func TestCronDoesNotRunOnTheFirstTick(t *testing.T) {
	s, started := newTestScheduler(t)
	flows := []flow.Flow{{Name: "each", Triggers: []flow.Trigger{{Cron: "* * * * *"}}}}
	s.Tick(at("09:00:10"), flows, map[string]bool{"each": true})
	if len(*started) != 0 {
		t.Errorf("started %q on the first tick", *started)
	}
}

func TestAfterStartsWhenTheOtherFlowEnds(t *testing.T) {
	s, started := newTestScheduler(t)
	flows := []flow.Flow{
		{Name: "deploy"},
		{Name: "smoke", Triggers: []flow.Trigger{{After: &flow.After{Flow: "deploy", Status: flow.AfterOK}}}},
		{Name: "alert", Triggers: []flow.Trigger{{After: &flow.After{Flow: "deploy", Status: flow.AfterFailed}}}},
		{Name: "report", Triggers: []flow.Trigger{{After: &flow.After{Flow: "deploy"}}}},
	}
	enabled := map[string]bool{"smoke": true, "alert": true, "report": true}
	saveResult(t, s.ResultsDir, flow.Result{Flow: "deploy", End: at("08:00:00"), OK: true})
	s.Tick(at("09:00:00"), flows, enabled) // an end seen for the first time is old news
	saveResult(t, s.ResultsDir, flow.Result{Flow: "deploy", End: at("09:00:01"), OK: true})
	s.Tick(at("09:00:02"), flows, enabled)
	if !slices.Equal(*started, []string{"smoke: after deploy", "report: after deploy"}) {
		t.Errorf("started %q", *started)
	}
}

func TestWatchStartsOnceFilesSettle(t *testing.T) {
	s, started := newTestScheduler(t)
	source := filepath.Join(s.Dir, "src", "a", "main.go")
	writeFile(t, source, "package a")
	writeFile(t, filepath.Join(s.Dir, "notes.txt"), "x")
	flows := []flow.Flow{{Name: "build", Triggers: []flow.Trigger{{Watch: &flow.Watch{Paths: []string{"src/**/*.go"}, Debounce: "2s"}}}}}
	enabled := map[string]bool{"build": true}
	s.Tick(at("09:00:00"), flows, enabled)
	writeFile(t, filepath.Join(s.Dir, "notes.txt"), "changed")
	s.Tick(at("09:00:01"), flows, enabled)
	if len(*started) != 0 {
		t.Fatalf("a file not watched started %q", *started)
	}
	writeFile(t, source, "package a // changed")
	s.Tick(at("09:00:02"), flows, enabled)
	s.Tick(at("09:00:03"), flows, enabled)
	if len(*started) != 0 {
		t.Fatalf("started %q before the files settled", *started)
	}
	s.Tick(at("09:00:05"), flows, enabled)
	s.Tick(at("09:00:07"), flows, enabled)
	if !slices.Equal(*started, []string{"build: watch src/**/*.go"}) {
		t.Errorf("started %q", *started)
	}
}

func saveResult(t *testing.T, dir string, r flow.Result) {
	t.Helper()
	if err := flow.SaveResult(dir, r); err != nil {
		t.Fatal(err)
	}
	// Results written in the same instant must still read as changed.
	later := time.Now().Add(time.Duration(len(r.Flow)+r.End.Second()) * time.Second)
	if err := os.Chtimes(filepath.Join(dir, r.Flow+".json"), later, later); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A flow's first run ever ends a run too: its result shows up where there was none.
func TestAfterStartsOnTheFirstRunEver(t *testing.T) {
	s, started := newTestScheduler(t)
	flows := []flow.Flow{
		{Name: "first"},
		{Name: "second", Triggers: []flow.Trigger{{After: &flow.After{Flow: "first"}}}},
	}
	enabled := map[string]bool{"second": true}
	s.Tick(at("09:00:00"), flows, enabled)
	saveResult(t, s.ResultsDir, flow.Result{Flow: "first", End: at("09:00:01"), OK: true})
	s.Tick(at("09:00:02"), flows, enabled)
	if !slices.Equal(*started, []string{"second: after first"}) {
		t.Errorf("started %q", *started)
	}
}
