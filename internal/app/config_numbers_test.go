package app

import (
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/logstore"
)

func TestParseLogSize(t *testing.T) {
	tests := []struct {
		text string
		kb   int
		ok   bool
	}{
		{"222", 222 << 10, true},
		{"222 MB", 222 << 10, true},
		{"222mb", 222 << 10, true},
		{"20000 KB", 20000, true},
		{"20000k", 20000, true},
		{"2 GB", 2 << 20, true},
		{"1.5 GB", 3 << 19, true},
		{"1,5 gb", 3 << 19, true},
		{"0", 0, false},
		{"lots", 0, false},
		{"5 TB", 0, false},
		{"2000 GB", 0, false},
	}
	for _, tt := range tests {
		kb, err := parseLogSize(tt.text)
		if (err == nil) != tt.ok || kb != tt.kb {
			t.Errorf("%q: %d, %v; want %d, ok %v", tt.text, kb, err, tt.kb, tt.ok)
		}
	}
}

func TestParseKeepDays(t *testing.T) {
	for text, want := range map[string]int{"99": 99, "99d": 99, "99 days": 99, " 1 day ": 1, "0": 0, "-3": 0, "a week": 0, "99999": 0} {
		days, err := parseKeepDays(text)
		if days != want || (err == nil) != (want > 0) {
			t.Errorf("%q: %d, %v; want %d", text, days, err, want)
		}
	}
}

func TestTypingADigitOnALogLimitStartsItOver(t *testing.T) {
	a := openConfigAt(t, logsKeepRow)
	typeText(a, "99")
	if a.config.Logs.KeepDays != config.DefaultKeepDays {
		t.Fatal("the limit changed before enter")
	}
	press(a, enterPress)
	if a.config.Logs.KeepDays != 99 || a.configPanel.number != nil || a.mode != configMode {
		t.Errorf("keep %d, editing %v, mode %v", a.config.Logs.KeepDays, a.configPanel.number != nil, a.mode)
	}
}

func TestEnterOnALogLimitEditsItsValue(t *testing.T) {
	a := openConfigAt(t, logsSizeRow)
	press(a, enterPress)
	if a.configPanel.number == nil || a.configPanel.number.String() != "200 MB" {
		t.Fatalf("editing starts from %v", a.configPanel.number)
	}
	press(a, uv.KeyPressEvent{Code: 'w', Mod: uv.ModCtrl}, uv.KeyPressEvent{Code: 'w', Mod: uv.ModCtrl})
	typeText(a, "20000 KB")
	press(a, enterPress)
	if a.config.Logs.MaxSizeKB != 20000 {
		t.Errorf("max size %d KB, want 20000", a.config.Logs.MaxSizeKB)
	}
	if lines := a.numberLines("max size", logsSizeRow, "x"); lines[0].typed {
		t.Error("the row still shows the text being typed")
	}
}

func TestEscGivesUpATypedLogLimitAndAProblemKeepsItOpen(t *testing.T) {
	a := openConfigAt(t, logsKeepRow)
	typeText(a, "7x")
	press(a, enterPress)
	if a.configPanel.number == nil || a.configPanel.numberProblem == "" || a.config.Logs.KeepDays != config.DefaultKeepDays {
		t.Fatalf("a bad value: editing %v, problem %q, keep %d", a.configPanel.number != nil, a.configPanel.numberProblem, a.config.Logs.KeepDays)
	}
	press(a, escPress)
	if a.configPanel.number != nil || a.mode != configMode || a.config.Logs.KeepDays != config.DefaultKeepDays {
		t.Errorf("esc: editing %v, mode %v, keep %d", a.configPanel.number != nil, a.mode, a.config.Logs.KeepDays)
	}
	press(a, rightPress)
	if a.config.Logs.KeepDays != 180 {
		t.Errorf("the arrows stopped stepping: keep %d", a.config.Logs.KeepDays)
	}
}

func TestTerminalLogsAreOffUntilTurnedOn(t *testing.T) {
	a := openConfigAt(t, logsTerminalsRow)
	if a.terminalLogName(3) != "" {
		t.Fatal("a terminal is logged by default")
	}
	press(a, rightPress)
	if !a.config.Logs.Terminals || a.terminalLogName(3) != "pane3" {
		t.Errorf("terminals %v, log name %q", a.config.Logs.Terminals, a.terminalLogName(3))
	}
}

// oldLogs writes three logs, 40 days old, so a keep of 30 days would delete them.
func oldLogs(t *testing.T) {
	t.Helper()
	for i, name := range []string{"a", "b", "c"} {
		writeTestLog(t, time.Now().AddDate(0, 0, -40).Add(time.Duration(i)*time.Minute), name, 100)
	}
}

func TestLoweringALogLimitAsksBeforeDeleting(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	oldLogs(t)
	a.openConfig()
	a.configPanel.selected = logsKeepRow
	typeText(a, "30")
	press(a, enterPress)
	if a.mode != confirmMode || !strings.Contains(a.confirm.question, "delete 3 logs") {
		t.Fatalf("mode %v, question %q; want to be asked about 3 logs", a.mode, a.confirm.question)
	}
	press(a, escPress)
	left, _ := logstore.List(logstore.LogsDir())
	if a.mode != configMode || a.config.Logs.KeepDays != config.DefaultKeepDays || len(left) != 3 || a.configPanel.number == nil {
		t.Fatalf("no: mode %v, keep %d, %d logs, editing %v", a.mode, a.config.Logs.KeepDays, len(left), a.configPanel.number != nil)
	}
	press(a, enterPress, enterPress)
	left, _ = logstore.List(logstore.LogsDir())
	if a.mode != configMode || a.config.Logs.KeepDays != 30 || len(left) != 0 || a.configPanel.number != nil {
		t.Errorf("yes: mode %v, keep %d, %d logs, editing %v", a.mode, a.config.Logs.KeepDays, len(left), a.configPanel.number != nil)
	}
}

func TestALogLimitThatDeletesNothingAsksNothing(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	oldLogs(t)
	a.openConfig()
	a.configPanel.selected = logsKeepRow
	press(a, rightPress) // 90 to 180 days
	if a.mode != configMode || a.config.Logs.KeepDays != 180 {
		t.Errorf("mode %v, keep %d", a.mode, a.config.Logs.KeepDays)
	}
	a.configPanel.selected = logsSizeRow
	press(a, leftPress) // 200 MB to 100 MB, still far above 300 B
	if a.mode != configMode || a.config.Logs.MaxSizeKB != 100<<10 {
		t.Errorf("mode %v, max size %d KB", a.mode, a.config.Logs.MaxSizeKB)
	}
}

func TestConfigShowsHowMuchOfTheSizeTheLogsTake(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	writeTestLog(t, time.Now(), "big", 1<<20)
	a.config.Logs.MaxSizeKB = 4 << 10
	a.openConfig()
	lines := a.logUsageLine()
	if len(lines) != 1 || !lines[0].bar || lines[0].fraction != 0.25 || lines[0].label != "25% · 1 MB of 4 MB · 1 log" {
		t.Errorf("usage line %+v", lines)
	}
}
