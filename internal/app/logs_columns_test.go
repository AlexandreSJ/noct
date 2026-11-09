package app

import (
	"image"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

func TestCompactCountFitsItsColumn(t *testing.T) {
	for n, want := range map[int]string{0: "0", 12345: "12,345", 99999: "99,999", 300000: "300k", 1234567: "1.2M", 45000000: "45M"} {
		if got := compactCount(n); got != want || len(got+" ln") > linesColumnWidth {
			t.Errorf("compactCount(%d) = %q, want %q within the column", n, got, want)
		}
	}
}

// A terminal left open since yesterday shows on both days, by when it was last written to; the
// month counts it once.
func TestLogsShowOnTheDayTheyStartedAndWereLastWritten(t *testing.T) {
	a := testApp(t)
	today := dateOf(time.Now()).Add(10 * time.Hour)
	open := writeTestLog(t, today.AddDate(0, 0, -1), "pane1", 10)
	if err := os.Chtimes(open.Path, today, today); err != nil {
		t.Fatal(err)
	}
	writeTestLog(t, today.Add(-time.Hour), "pane2", 10)
	p := newLogsPanel(a, func() {})
	if rows := p.rows(); len(rows) != 2 || rows[0].entry.Name != "pane1" {
		t.Fatalf("today lists %+v, want pane1 first, written last", rows)
	}
	p.moveDay(-1, 0)
	if rows := p.rows(); len(rows) != 1 || rows[0].entry.Name != "pane1" {
		t.Errorf("yesterday lists %+v, want the day pane1 started", rows)
	}
	stats := p.statsByDay()
	if stats[dateOf(today)].logs != 2 || stats[dateOf(today).AddDate(0, 0, -1)].logs != 1 {
		t.Errorf("stats %+v", stats)
	}
	p.day = dateOf(today)
	if month := p.monthStats(); today.Day() > 1 && month.logs != 2 {
		t.Errorf("the month counts %d logs, want each once", month.logs)
	}
}

// The session column names a terminal's session, open or closed, and a run's.
func TestLogsNameTheirSession(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	at := dateOf(time.Now()).Add(10 * time.Hour)
	terminal := writeTestLog(t, at, "pane1", 10)
	run := writeTestLog(t, at.Add(time.Minute), "greet", 10)
	a.shells[1] = &shell{log: terminal.Path}
	closed := writeTestLog(t, at.Add(2*time.Minute), "pane9", 10)
	a.sessions = append(a.sessions, &session{name: "old", closed: &saved.Session{Tabs: []saved.Tab{{Panes: []saved.Pane{{Kind: saved.Terminal, Log: closed.Path}}}}}})
	if err := script.AddRun(runsPath(), script.Run{Script: "greet", At: at, Log: run.Path, Session: "work"}); err != nil {
		t.Fatal(err)
	}
	p := newLogsPanel(a, func() {})
	sessions := map[string]string{}
	for _, row := range p.rows() {
		sessions[row.entry.Name] = p.sessionOf(row)
	}
	if sessions["pane1"] != "1" || sessions["greet"] != "work" || sessions["pane9"] != "old" {
		t.Errorf("sessions %v, want pane1 in 1, greet in work and pane9 in old", sessions)
	}
}

// On a narrow list the columns go, session first, until the name has room; the rest keep their
// order and widths, whatever rows show.
func TestLogColumnsMakeRoomForTheName(t *testing.T) {
	a := testApp(t)
	writeTestLog(t, dateOf(time.Now()).Add(10*time.Hour), "one", 10)
	p := newLogsPanel(a, func() {})
	rows := p.rows()
	ids := func(width int) []string {
		var ids []string
		for _, c := range p.logColumns(rectOfWidth(width), rows) {
			ids = append(ids, c.id)
		}
		return ids
	}
	if got := ids(200); !slices.Equal(got, config.DefaultLogColumns) {
		t.Errorf("wide: %v", got)
	}
	got := ids(60)
	if slices.Contains(got, config.LogColumnSession) || !slices.Contains(got, config.LogColumnName) {
		t.Errorf("narrow: %v, want the session gone and the name kept", got)
	}
	for _, c := range p.logColumns(rectOfWidth(60), rows) {
		if c.id == config.LogColumnName && c.width < minNameWidth {
			t.Errorf("the name has %d cells", c.width)
		}
	}
	a.config.Logs.Columns = []string{"size", "name"}
	if got := ids(200); strings.Join(got, " ") != "size name" {
		t.Errorf("configured: %v", got)
	}
}

func rectOfWidth(width int) image.Rectangle {
	return image.Rect(0, 0, width, 10)
}

// ? lists the panel's keys as the binds have them, and its key changes like any other; space
// goes from the calendar to the list as enter does.
func TestLogsKeysPopupFollowsTheBinds(t *testing.T) {
	a := testApp(t)
	a.config.Keys = map[string]map[string][]string{"logs": {"keys": {"f1"}, "filter": {"ctrl+f"}}}
	a.keymap = newKeymap(a.config.Keys)
	pressDrawn(a, prefixPress, letter('v'), letter(' '))
	p := a.currentTab().focusedView().(*logsPanel)
	if !p.focusList {
		t.Error("space on the calendar did not go to the list")
	}
	pressDrawn(a, letter('?'))
	if a.mode == infoMode {
		t.Fatal("? opened the keys after the binds moved them to f1")
	}
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyF1})
	if a.mode != infoMode {
		t.Fatalf("f1 left mode %v, want the keys", a.mode)
	}
	text := screenText(a)
	if !strings.Contains(text, "ctrl+f         filter") || strings.Contains(text, "│  f ") {
		t.Errorf("the popup does not show the filter's key as bound:\n%s", text)
	}
}

// A log shows on each day it had output between the first and the last, and not on the days
// it was quiet.
func TestLogsShowOnEveryDayTheyHadOutput(t *testing.T) {
	a := testApp(t)
	today := dateOf(time.Now()).Add(10 * time.Hour)
	open := writeTestLog(t, today.AddDate(0, 0, -3), "pane1", 10)
	if err := os.WriteFile(logstore.DaysPath(open.Path), []byte(today.AddDate(0, 0, -2).Format(logstore.DayLayout)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(open.Path, today, today); err != nil {
		t.Fatal(err)
	}
	p := newLogsPanel(a, func() {})
	for back, want := range map[int]int{0: 1, 1: 0, 2: 1, 3: 1} {
		p.day = dateOf(today.AddDate(0, 0, -back))
		if rows := p.rows(); len(rows) != want {
			t.Errorf("%d days ago lists %d logs, want %d", back, len(rows), want)
		}
	}
	p.filter = today.AddDate(0, 0, -2).Format(logstore.DayLayout)
	if rows := p.rows(); len(rows) != 1 {
		t.Errorf("filtering by a day it had output on lists %d logs, want it", len(rows))
	}
}
