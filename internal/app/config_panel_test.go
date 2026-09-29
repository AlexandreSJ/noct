package app

import (
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
)

func openConfigAt(t *testing.T, row configRow) *App {
	t.Helper()
	a := testApp(t, sideBySideTab(1, 2))
	a.openConfig()
	a.configPanel.selected = row
	return a
}

func TestPrefixMustKeepCtrlOrAlt(t *testing.T) {
	a := openConfigAt(t, prefixCtrlRow)
	press(a, enterPress)
	if !a.config.Prefix.Ctrl || a.statusMessage == "" {
		t.Errorf("turning off the only modifier: ctrl=%v message=%q", a.config.Prefix.Ctrl, a.statusMessage)
	}
}

func TestChangingThePrefixAppliesAndSaves(t *testing.T) {
	a := openConfigAt(t, prefixAltRow)
	press(a, enterPress) // alt on
	a.configPanel.selected = prefixCtrlRow
	press(a, enterPress) // ctrl off
	a.configPanel.selected = prefixKeyRow
	press(a, enterPress, uv.KeyPressEvent{Code: uv.KeyF2})

	if !a.prefix.matches(uv.KeyPressEvent{Code: uv.KeyF2, Mod: uv.ModAlt}) {
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

func TestCapturingEscKeepsThePrefixKey(t *testing.T) {
	a := openConfigAt(t, prefixKeyRow)
	press(a, enterPress, escPress)
	if a.config.Prefix.Key != "space" || a.mode != configMode {
		t.Errorf("key=%q mode=%v", a.config.Prefix.Key, a.mode)
	}
}

func TestPrefixRefusesShiftTerminalsCannotSend(t *testing.T) {
	a := openConfigAt(t, prefixShiftRow)
	press(a, enterPress) // ctrl+shift+space arrives as ctrl+space
	if a.config.Prefix.Shift || a.statusMessage == "" {
		t.Errorf("ctrl+shift+space was accepted (shift=%v, message=%q)", a.config.Prefix.Shift, a.statusMessage)
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
