package app

import (
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

// One line of text filters by name, date, time, status, size and lines, every word at once.
func TestLogFilterMatchesEveryWord(t *testing.T) {
	failed := 1
	row := logRow{
		entry: logstore.Entry{Name: "flow-deploy", At: time.Date(2026, 9, 27, 14, 30, 5, 0, time.Local), Size: 3 << 20},
		run:   &script.Run{Flow: "deploy", Exit: &failed},
	}
	lines := func() int { return 250 }
	cases := map[string]bool{
		"":                 true,
		"deploy":           true,
		"DEPLOY":           true,
		"2026-09-27":       true,
		"09-27":            true,
		"sep 27":           true,
		"14:30":            true,
		"failed":           true,
		"ok":               false,
		"deploy 09-28":     false,
		">1MB":             true,
		"<1MB":             false,
		">=3MB <4mb":       true,
		"size<2k":          false,
		">100":             true,
		"lines<100":        false,
		"lines>=250 fail":  true,
		"deploy >1GB":      false,
		"script":           false,
		"flow deploy >2MB": true,
	}
	for query, want := range cases {
		if got := parseLogFilter(query).matches(row, lines); got != want {
			t.Errorf("%q: got %v, want %v", query, got, want)
		}
	}
}

// Lines are counted only when the filter asks for them.
func TestLogFilterCountsLinesOnlyWhenAsked(t *testing.T) {
	counted := false
	lines := func() int { counted = true; return 1 }
	parseLogFilter("deploy >1MB").matches(logRow{entry: logstore.Entry{Name: "deploy", Size: 2 << 20}}, lines)
	if counted {
		t.Error("a filter without lines counted them")
	}
}

// f filters the list over every day, and an empty filter gives the day back.
func TestLogsPanelFiltersAcrossDays(t *testing.T) {
	a := testApp(t)
	today := dateOf(time.Now()).Add(10 * time.Hour)
	writeTestLog(t, today, "deploy", 400)
	writeTestLog(t, today, "pane3", 100)
	writeTestLog(t, today.AddDate(0, 0, -3), "deploy", 50)
	pressDrawn(a, prefixPress, letter('v'), letter('f'))
	typeDrawn(a, "deploy")
	pressDrawn(a, enterPress)
	p := a.currentTab().focusedView().(*logsPanel)
	if rows := p.rows(); len(rows) != 2 || rows[0].entry.Name != "deploy" || rows[1].entry.Name != "deploy" {
		t.Fatalf("filtered rows = %+v", rows)
	}
	if !strings.Contains(screenText(a), "filter: deploy") {
		t.Error("the filter is not shown")
	}
	pressDrawn(a, letter('f'), clearLine, enterPress)
	if rows := p.rows(); len(rows) != 2 || p.filter != "" {
		t.Errorf("after clearing: filter %q, rows %+v", p.filter, rows)
	}
}

// i on a flow's run in the logs opens its chart with how each block went in that run, not in
// the last one.
func TestLogsOpenAFlowRunInTheInspect(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "trio", threeSteps)
	at := dateOf(time.Now()).Add(9 * time.Hour)
	entry := writeTestLog(t, at, "flow-trio", 100)
	old := flow.Result{Flow: "trio", Start: at, Exit: 3, Steps: map[string]flow.StepResult{"0": {Status: flow.StatusFailed, Exit: 3}}}
	if err := flow.SaveResultTo(logstore.ResultPath(entry.Path), old); err != nil {
		t.Fatal(err)
	}
	exit := 3
	if err := script.AddRun(runsPath(), script.Run{Flow: "trio", At: at, Log: entry.Path, Exit: &exit}); err != nil {
		t.Fatal(err)
	}
	pressDrawn(a, prefixPress, letter('v'))
	a.currentTab().focusedView().(*logsPanel).day = dateOf(at)
	pressDrawn(a, tabPress, letter('i'))
	p, ok := a.currentTab().focusedView().(*inspectPanel)
	if !ok || p.run == nil || p.flowName != "trio" {
		t.Fatalf("i opened %T", a.currentTab().focusedView())
	}
	screen := screenText(a)
	if !strings.Contains(screen, "run of "+at.Format("Jan 2 15:04")) || !strings.Contains(screen, "✗ one") {
		t.Errorf("the chart does not show that run:\n%s", screen)
	}
}

// A run a trigger started tells so in the list, and the filter finds it by that.
func TestLogRowsTellWhatStartedTheRun(t *testing.T) {
	exit := 0
	row := logRow{entry: logstore.Entry{Name: "flow-nightly"}, run: &script.Run{Flow: "nightly", Exit: &exit, Trigger: "cron 0 2 * * *"}}
	if row.name() != "flow nightly (cron 0 2 * * *)" || !parseLogFilter("cron").matches(row, nil) {
		t.Errorf("name %q", row.name())
	}
}
