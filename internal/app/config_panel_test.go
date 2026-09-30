package app

import (
	"image"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
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
	if a.config.Appearance.Theme == config.Default().Appearance.Theme || !a.configPanel.resetArmed {
		t.Fatal("moving away kept the reset armed")
	}
	press(a, enterPress)
	if a.config.Appearance.Theme != config.Default().Appearance.Theme || a.mode != configMode {
		t.Errorf("theme %s mode %v after confirming the reset", a.config.Appearance.Theme, a.mode)
	}
}

func TestThemeAndBackground(t *testing.T) {
	a := openConfigAt(t, themeRow)
	press(a, rightPress)
	if a.config.Appearance.Theme != themes[1].name {
		t.Errorf("theme %q after right, want %q", a.config.Appearance.Theme, themes[1].name)
	}
	press(a, leftPress, leftPress)
	if a.config.Appearance.Theme != themes[len(themes)-1].name {
		t.Errorf("theme %q did not wrap to the last one", a.config.Appearance.Theme)
	}
	press(a, downPress, enterPress)
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
	if a.config.Appearance.Theme != nextThemeName(theme, 1) {
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
