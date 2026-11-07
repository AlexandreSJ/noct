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

// newYork is a zone with daylight saving time: on 2026-11-01 its clocks go back from 2:00 to
// 1:00, and on 2026-03-08 forward from 2:00 to 3:00.
func newYork(t *testing.T) *time.Location {
	t.Helper()
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no time zone data:", err)
	}
	return ny
}

// tickUTC ticks at each instant, given in UTC and seen on the clock of loc.
func tickUTC(s *Scheduler, loc *time.Location, flows []flow.Flow, enabled map[string]bool, instants ...string) {
	for _, instant := range instants {
		v, _ := time.Parse("2006-01-02 15:04", instant)
		s.Tick(v.In(loc), flows, enabled)
	}
}

// When clocks go back, the hour from 1:00 to 2:00 happens twice; a flow runs once in it.
func TestCronRunsOnceWhenClocksGoBack(t *testing.T) {
	ny := newYork(t)
	s, started := newTestScheduler(t)
	flows := []flow.Flow{
		{Name: "nightly", Triggers: []flow.Trigger{{Cron: "30 1 * * *"}}},
		{Name: "quarter", Triggers: []flow.Trigger{{Cron: "*/15 * * * *"}}},
	}
	enabled := map[string]bool{"nightly": true, "quarter": true}
	// 1:29, 1:30 and 1:45 EDT, then 1:30 and 1:45 EST again, then 2:00 EST.
	tickUTC(s, ny, flows, enabled, "2026-11-01 05:29", "2026-11-01 05:30", "2026-11-01 05:45", "2026-11-01 06:30", "2026-11-01 06:45", "2026-11-01 07:00")
	quarter := "quarter: cron */15 * * * *"
	want := []string{"nightly: cron 30 1 * * *", quarter, quarter, quarter}
	if !slices.Equal(*started, want) {
		t.Errorf("started %q, want %q", *started, want)
	}
}

// When clocks go forward, the hour from 2:00 to 3:00 does not happen: a flow scheduled in it is
// skipped that day, as cron does, and runs again the next.
func TestCronSkipsTheMinutesClocksJumpOver(t *testing.T) {
	ny := newYork(t)
	s, started := newTestScheduler(t)
	flows := []flow.Flow{{Name: "nightly", Triggers: []flow.Trigger{{Cron: "30 2 * * *"}}}}
	enabled := map[string]bool{"nightly": true}
	// 1:59 EST is followed by 3:00 EDT; a day later 2:30 EDT happens.
	tickUTC(s, ny, flows, enabled, "2026-03-08 06:59", "2026-03-08 07:00", "2026-03-08 07:30", "2026-03-09 06:30")
	if !slices.Equal(*started, []string{"nightly: cron 30 2 * * *"}) {
		t.Errorf("started %q", *started)
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

// A watch on one file reads that file alone, and starts when it changes.
func TestWatchOnOneFileStartsWhenItChanges(t *testing.T) {
	s, started := newTestScheduler(t)
	notes := filepath.Join(s.Dir, "notes.md")
	writeFile(t, notes, "x")
	flows := []flow.Flow{{Name: "publish", Triggers: []flow.Trigger{{Watch: &flow.Watch{Paths: []string{"notes.md"}, Debounce: "1s"}}}}}
	enabled := map[string]bool{"publish": true}
	s.Tick(at("09:00:00"), flows, enabled)
	writeFile(t, filepath.Join(s.Dir, "other.md"), "y")
	s.Tick(at("09:00:01"), flows, enabled)
	writeFile(t, notes, "changed")
	s.Tick(at("09:00:02"), flows, enabled)
	s.Tick(at("09:00:04"), flows, enabled)
	if !slices.Equal(*started, []string{"publish: watch notes.md"}) {
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
