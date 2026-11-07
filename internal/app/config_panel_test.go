package app

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/theme"
)

func openConfigAt(t *testing.T, row configRow) *App {
	t.Helper()
	a := testApp(t, sideBySideTab(1, 2))
	a.openConfig()
	a.configPanel.selected = row
	return a
}

// capturePrefixWith opens the prefix's capture and presses keys in it.
func capturePrefixWith(t *testing.T, keys ...uv.KeyPressEvent) *App {
	t.Helper()
	a := openConfigAt(t, prefixRow)
	press(a, enterPress)
	press(a, keys...)
	return a
}

func TestPrefixMustKeepCtrlOrAlt(t *testing.T) {
	a := capturePrefixWith(t, letter('a'), enterPress)
	if a.config.Prefix != config.Default().Prefix || a.configPanel.capture.problem == "" {
		t.Errorf("a plain a: prefix %+v, problem %q", a.config.Prefix, a.configPanel.capture.problem)
	}
}

func TestChangingThePrefixAppliesAndSaves(t *testing.T) {
	a := capturePrefixWith(t, uv.KeyPressEvent{Code: uv.KeyF2, Mod: uv.ModAlt})
	if a.config.Prefix != config.Default().Prefix {
		t.Fatal("the combination was used before enter")
	}
	press(a, enterPress)
	if !a.prefix.matches(uv.KeyPressEvent{Code: uv.KeyF2, Mod: uv.ModAlt}) || a.configPanel.capture.active {
		t.Fatalf("prefix is %s, want alt+f2", a.prefix)
	}
	saved, err := config.Load(a.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Prefix != (config.Prefix{Alt: true, Key: "f2"}) {
		t.Errorf("saved prefix %+v", saved.Prefix)
	}
}

func TestCapturingEscKeepsThePrefix(t *testing.T) {
	a := capturePrefixWith(t, uv.KeyPressEvent{Code: 'x', Mod: uv.ModAlt}, escPress)
	if a.config.Prefix != config.Default().Prefix || a.mode != configMode || a.configPanel.capture.active {
		t.Errorf("prefix %+v mode %v after esc", a.config.Prefix, a.mode)
	}
}

// Enter is pressed as the combination first, and only then confirms: it cannot be a prefix, so
// it stays a problem instead of closing the capture.
func TestPrefixRefusesShiftTerminalsCannotSend(t *testing.T) {
	a := capturePrefixWith(t, uv.KeyPressEvent{Code: uv.KeySpace, Mod: uv.ModCtrl | uv.ModShift}, enterPress)
	if a.config.Prefix.Shift || a.configPanel.capture.problem == "" {
		t.Errorf("ctrl+shift+space was accepted (shift=%v, problem=%q)", a.config.Prefix.Shift, a.configPanel.capture.problem)
	}
}

func TestPrefixCaptureWarnsWhatItTakes(t *testing.T) {
	notes := func(a *App) string {
		var all []string
		for _, line := range a.configLines() {
			if line.warning {
				all = append(all, line.label)
			}
		}
		return strings.Join(all, "\n")
	}
	a := capturePrefixWith(t, uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl})
	if !strings.Contains(notes(a), "ctrl+c interrupts") {
		t.Errorf("ctrl+c warned %q", notes(a))
	}
	press(a, uv.KeyPressEvent{Code: 'p', Mod: uv.ModAlt}) // alt is the default direct shortcut
	if !strings.Contains(notes(a), "direct shortcut to new panel") {
		t.Errorf("alt+p warned %q", notes(a))
	}
	press(a, enterPress)
	if a.prefix.String() != "alt+p" {
		t.Errorf("a warned combination can still be used: prefix %s", a.prefix)
	}
}

func TestResetAllToDefaultsAsksFirst(t *testing.T) {
	a := openConfigAt(t, themeRow)
	press(a, rightPress)
	a.configPanel.selected = resetRow
	press(a, enterPress)
	if a.config.Appearance.Theme == config.Default().Appearance.Theme {
		t.Fatal("one enter reset the settings")
	}
	press(a, upPress, downPress, enterPress)
	if a.config.Appearance.Theme == config.Default().Appearance.Theme || !a.resetArmed() {
		t.Fatal("moving away kept the reset armed")
	}
	press(a, enterPress)
	if a.config.Appearance.Theme != config.Default().Appearance.Theme || a.mode != configMode {
		t.Errorf("theme %s mode %v after confirming the reset", a.config.Appearance.Theme, a.mode)
	}
}

func TestResetTakesOnlyEnter(t *testing.T) {
	for _, row := range []configRow{resetRow, resetBindsRow} {
		a := openConfigAt(t, themeRow)
		press(a, rightPress)
		if row == resetBindsRow {
			a.openBinds()
		}
		a.configPanel.selected = row
		press(a, rightPress, rightPress, leftPress, letter('l'), letter(' '))
		if a.resetArmed() || a.config.Appearance.Theme == config.Default().Appearance.Theme {
			t.Errorf("row %d: arrows and space armed or ran the reset", row)
		}
		press(a, enterPress, rightPress)
		if !a.resetArmed() {
			t.Errorf("row %d: enter did not arm the reset", row)
		}
	}
}

func TestResetLetsGoAfterItsWindow(t *testing.T) {
	a := openConfigAt(t, themeRow)
	press(a, rightPress)
	a.configPanel.selected = resetRow
	press(a, enterPress)
	if a.resetValue() == "" || a.resetExpired == nil {
		t.Fatal("the first enter did not ask again")
	}
	a.configPanel.resetArmedAt = time.Now().Add(-resetWindow)
	if a.resetValue() != "" {
		t.Error("the prompt stayed after the window")
	}
	press(a, enterPress)
	if a.config.Appearance.Theme == config.Default().Appearance.Theme || !a.resetArmed() {
		t.Error("an enter after the window reset instead of asking again")
	}
}

func TestThemeAndBackground(t *testing.T) {
	a := openConfigAt(t, themeRow)
	press(a, rightPress)
	if a.config.Appearance.Theme != theme.Builtins()[1].Name {
		t.Errorf("theme %q after right, want %q", a.config.Appearance.Theme, theme.Builtins()[1].Name)
	}
	press(a, leftPress, leftPress)
	if a.config.Appearance.Theme != theme.Builtins()[len(theme.Builtins())-1].Name {
		t.Errorf("theme %q did not wrap to the last one", a.config.Appearance.Theme)
	}
	press(a, downPress, downPress, downPress, downPress, enterPress) // past new theme, edit theme and mode
	if a.palette.background == nil {
		t.Error("background did not switch to the theme's")
	}
	press(a, enterPress)
	if a.palette.background != nil {
		t.Error("background did not switch back to the terminal's")
	}
}

func TestConfigClosesWithEsc(t *testing.T) {
	a := openConfigAt(t, themeRow)
	press(a, escPress)
	if a.mode != normalMode {
		t.Error("esc did not close the config")
	}
}

// The flows' output setting goes from keeping all of it to keeping less, and back.
func TestFlowOutputKeptSteps(t *testing.T) {
	a := openConfigAt(t, flowsOutputRow)
	press(a, rightPress)
	if a.config.Flows.OutputKB != 256 {
		t.Fatalf("right from all: %d KB", a.config.Flows.OutputKB)
	}
	press(a, rightPress, rightPress, rightPress, rightPress)
	if a.config.Flows.OutputKB != 4 {
		t.Errorf("right to the end: %d KB, want 4", a.config.Flows.OutputKB)
	}
	press(a, leftPress, leftPress, leftPress, leftPress)
	if a.config.Flows.OutputKB != 0 {
		t.Errorf("left back to the start: %d KB, want 0 (all)", a.config.Flows.OutputKB)
	}
}

// configPoint is where a row of the config is on screen: its label, or the first or last cell of
// its value.
func configPoint(t *testing.T, a *App, row configRow, part string) image.Point {
	t.Helper()
	l := a.configLayout()
	inner := l.inner()
	for y := inner.Min.Y; y < inner.Max.Y; y++ {
		line, ok := l.lineAt(y)
		if !ok || !line.selectable() || line.row != row {
			continue
		}
		valueX := inner.Max.X - 1 - ansi.StringWidth(line.value)
		switch part {
		case "label":
			return image.Pt(inner.Min.X+3, y)
		case "value start":
			return image.Pt(valueX, y)
		}
		return image.Pt(inner.Max.X-2, y)
	}
	t.Fatalf("row %d is not on screen", row)
	return image.Point{}
}

func tallConfig(t *testing.T) *App {
	t.Helper()
	a := openConfigAt(t, prefixRow)
	a.size = image.Pt(100, 60)
	return a
}

func TestClickingAConfigLabelSelectsIt(t *testing.T) {
	a := tallConfig(t)
	p := configPoint(t, a, mouseRow, "label")
	click(a, p.X, p.Y)
	if a.configPanel.selected != mouseRow || !a.config.Mouse {
		t.Errorf("selected %d, mouse %v after clicking the label: want it selected, unchanged", a.configPanel.selected, a.config.Mouse)
	}
}

func TestClickingAConfigValueChangesIt(t *testing.T) {
	a := tallConfig(t)
	p := configPoint(t, a, closeEmptyTabsRow, "value end")
	click(a, p.X, p.Y)
	if a.configPanel.selected != closeEmptyTabsRow || a.config.CloseEmptyTabs {
		t.Fatalf("close when empty %v after clicking its on", a.config.CloseEmptyTabs)
	}
	theme := a.config.Appearance.Theme
	p = configPoint(t, a, themeRow, "value end")
	click(a, p.X, p.Y)
	if a.config.Appearance.Theme != a.nextThemeName(theme, 1) {
		t.Fatalf("theme %s after clicking ▸, want the next one", a.config.Appearance.Theme)
	}
	p = configPoint(t, a, themeRow, "value start")
	click(a, p.X, p.Y)
	if a.config.Appearance.Theme != theme {
		t.Errorf("theme %s after clicking ◂, want %s back", a.config.Appearance.Theme, theme)
	}
}

func TestConfigFollowsTheMouse(t *testing.T) {
	a := tallConfig(t)
	a.applyMouseMode()
	p := configPoint(t, a, themeRow, "label")
	a.handleEvent(uv.MouseMotionEvent{X: p.X, Y: p.Y})
	if !a.configPanel.hovering || a.configPanel.hover != themeRow {
		t.Errorf("hover %d (%v) over the theme row", a.configPanel.hover, a.configPanel.hovering)
	}
	a.handleEvent(uv.MouseWheelEvent{X: p.X, Y: p.Y, Button: uv.MouseWheelDown})
	if a.configPanel.selected != bindsRow {
		t.Errorf("selected %d after the wheel went down one row", a.configPanel.selected)
	}
	click(a, 0, 0)
	if a.mode != normalMode {
		t.Errorf("mode %v after clicking outside the config", a.mode)
	}
}

// A note too wide for the panel scrolls, after a pause, only while the row above it is selected.
func TestWideNoteScrollsWhileItsRowIsSelected(t *testing.T) {
	const note = "logs not written to for longer and, past the size, the least recent are deleted"
	now := time.Unix(0, 0)
	a := testApp(t, sideBySideTab(1, 2))
	a.clock = func() time.Time { return now }
	a.openConfig()
	a.selectConfigRow(logsSizeRow)
	if !strings.Contains(screenText(a), note[:40]) {
		t.Fatalf("before the pause, the note shows its start:\n%s", screenText(a))
	}
	now = now.Add(noteHold + 2*noteStep)
	if text := screenText(a); !strings.Contains(text, note[2:42]) || strings.Contains(text, note[:10]) {
		t.Errorf("after the pause and two steps, the note moved two cells:\n%s", text)
	}
	if a.noteTick == nil {
		t.Error("a scrolling note asks for its next frame")
	}
	a.selectConfigRow(logsTerminalsRow)
	now = now.Add(noteHold + 2*noteStep)
	if !strings.Contains(screenText(a), note[:40]) {
		t.Errorf("with another row selected, the note stays at its start:\n%s", screenText(a))
	}
}

func TestTheConfigsShortcutClosesItToo(t *testing.T) {
	a := openConfigAt(t, prefixRow)
	a.shortcut = shortcutFromConfig(config.Shortcut{Alt: true})
	press(a, uv.KeyPressEvent{Code: ',', Mod: uv.ModAlt})
	if a.mode != normalMode {
		t.Errorf("shortcut + , left mode %v, want the config closed", a.mode)
	}
}

// The logs list's columns are typed in order; a word that is no column shows as left out, and
// enter saves the columns known. The log's side and size change with the arrows.
func TestConfigSetsTheLogsView(t *testing.T) {
	a := openConfigAt(t, logsColumnsRow)
	press(a, enterPress, clearLine)
	for _, r := range "name weight size" {
		press(a, letter(r))
	}
	if text := screenText(a); !strings.Contains(text, "not a column, left out: weight") {
		t.Errorf("the unknown word is not pointed out:\n%s", text)
	}
	press(a, enterPress)
	if got := a.config.Logs.Columns; strings.Join(got, " ") != "name size" || a.configPanel.logColumns.String() != "name size" {
		t.Errorf("columns %v, typed %q; want name size", got, a.configPanel.logColumns.String())
	}
	a.selectConfigRow(logsPreviewRow)
	press(a, rightPress)
	a.selectConfigRow(logsPreviewPercentRow)
	press(a, leftPress)
	if a.config.Logs.Preview != "right" || a.config.Logs.PreviewPercent != 40 {
		t.Errorf("logs %+v, want the log on the right at 40%%", a.config.Logs)
	}
}

// TestConfigLinesStayInPlace pins every line of the settings page, in order, as the panel shows it.
func TestConfigLinesStayInPlace(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	a.config.Editor = "noct-missing-editor -w" // custom: the command row and its note show
	a.openConfig()
	a.configPanel.scriptsDir = newTextField("/nonexistent/noct-scripts")
	a.configPanel.logUsage = &logUsage{count: 2, size: 1 << 20}
	a.configPanel.logColumns = newTextField("name size bogus")
	want := []string{
		"heading Keys = ",
		"row prefix = ctrl+space @0",
		"row binds of every screen = defaults ▸ @1",
		"heading Shortcut = alt+key",
		"row ctrl = off @2",
		"row alt = on @3",
		"heading Appearance = ",
		"row theme = ◂ noct ▸ @4",
		"row new theme = enter @5",
		"row edit theme = copy @6",
		"row mode = dark @7",
		"row background = terminal @8",
		"row animate intro = ◂ empty ▸ @9",
		"row mouse = on @11",
		"heading Animations = ",
		"row animations = off @10",
		"row hover = ◂ fade ▸ @3000",
		"row   time = ◂ 150 ms ▸ @3100",
		"row open and close panels = ◂ jitter ▸ @3001",
		"row   time = ◂ 300 ms ▸ @3101",
		"row open and close tabs = ◂ jitter ▸ @3002",
		"row   time = ◂ 300 ms ▸ @3102",
		"row focus = ◂ none ▸ @3003",
		"row   time = ◂ 100 ms ▸ @3103",
		"row move a panel = ◂ fade ▸ @3004",
		"row   time = ◂ 200 ms ▸ @3104",
		"row fullscreen = ◂ 01 ▸ @3005",
		"row   time = ◂ 200 ms ▸ @3105",
		"row popups = ◂ 01 ▸ @3006",
		"row   time = ◂ 200 ms ▸ @3106",
		"row activity = ◂ fade ▸ @3007",
		"row   time = ◂ 600 ms ▸ @3107",
		"heading Tabs = ",
		"row new tab opens = ◂ terminal (alt+p) ▸ @12",
		"row close when empty = on @13",
		"heading Programs = ",
		"row editor = ◂ custom ▸ @14",
		"typed command = noct-missing-editor -w @15",
		"note command not found = ",
		"row file manager = ◂ terminal ▸ @16",
		"heading Scripts = ",
		"typed location = /nonexistent/noct-scripts @18",
		"note enter edit · ctrl+c copy = ",
		"note folder not found; it is created with the first script = ",
		"row hints in new scripts = on @19",
		"row sort = ◂ alphabetical ▸ @20",
		"heading Logs = ",
		"row keep = ◂ 90 days ▸ @21",
		"row max size = ◂ 200 MB ▸ @22",
		"bar 0% · 1 MB of 200 MB · 2 logs =  0.0050",
		"note logs not written to for longer and, past the size, the least recent are deleted = ",
		"row terminals = off @23",
		"note log what terminal panes print; runs are always logged = ",
		"typed list columns = name size bogus @24",
		"note enter edit · in order, out of edited started status name session bar size lines = ",
		"warning not a column, left out: bogus = ",
		"row log = ◂ below ▸ @25",
		"row log size = ◂ 50% ▸ @26",
		"note where the logs view shows the selected log; there, p moves it and + - size it = ",
		"heading Flows = ",
		"row step output kept = ◂ all (up to 1 MB) ▸ @27",
		"note in each flow's last result; tests keep 4 KB = ",
		"heading Updates = ",
		"row check for new versions = on @28",
		"note only checks; install with ctrl+space+c = ",
		"heading Defaults = ",
		"row reset all to defaults =  @29",
	}
	var got []string
	for _, line := range a.configLines() {
		got = append(got, describeConfigLine(line))
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("config lines\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func describeConfigLine(line configLine) string {
	kind := "row"
	switch {
	case line.heading:
		kind = "heading"
	case line.bar:
		kind = "bar"
	case line.warning:
		kind = "warning"
	case line.note:
		kind = "note"
	case line.typed:
		kind = "typed"
	}
	text := kind + " " + line.label + " = " + line.value
	if line.bar {
		text += fmt.Sprintf(" %.4f", line.fraction)
	}
	if line.selectable() {
		text += fmt.Sprintf(" @%d", line.row)
	}
	return text
}
