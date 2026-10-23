package config

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("got %+v", cfg)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "noct", "config.toml")
	want := Config{
		Prefix:         Prefix{Alt: true, Shift: true, Key: "f2"},
		Shortcut:       Shortcut{Alt: true},
		Appearance:     Appearance{Theme: "ember", Background: BackgroundTheme, AnimateIntro: IntroAlways, Animations: AnimationsOff},
		Animations:     Animations{Hover: "none", Panels: "fade", Tabs: "jitter", Focus: "01", Move: "fade", Fullscreen: "01", Popup: "jitter", Activity: "none"},
		AnimationTimes: AnimationTimes{Hover: 100, Panels: 200, Tabs: 300, Focus: 400, Move: 500, Fullscreen: 600, Popup: 700, Activity: 800},
		NewTab:         "scripts",
		// CloseEmptyTabs off: a bool that is on by default must survive being saved off
		Mouse:         false,
		Editor:        "code -w",
		FileManager:   "yazi",
		ScriptsDir:    "~/tests/scripts",
		ScriptsSort:   SortByRun,
		ScriptFolders: []string{"~/work/scripts", "/srv/shared"},
		Keys:          map[string]map[string][]string{"scripts": {"edit": {"x", "ctrl+e"}, "delete": {}}},
		Logs:          Logs{KeepDays: 30, MaxSizeKB: 20000, Terminals: true},
		Flows:         Flows{OutputKB: 64},
		MCP:           MCP{Policy: map[string]string{"run_command": PolicyDeny}, ApprovalSeconds: 60},
	}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadKeepsDefaultsForMissingSettings(t *testing.T) {
	path := writeConfig(t, "[appearance]\ntheme = \"frost\"\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Prefix != Default().Prefix || cfg.Shortcut != Default().Shortcut || !cfg.Mouse || cfg.Editor != DefaultEditor || cfg.NewTab != NewTabTerminal || !cfg.CloseEmptyTabs || cfg.Appearance.Theme != "frost" {
		t.Errorf("got %+v", cfg)
	}
}

func TestLoadRejectsPrefixWithoutCtrlOrAlt(t *testing.T) {
	path := writeConfig(t, "[prefix]\nshift = true\nctrl = false\nkey = \"a\"\n")
	cfg, err := Load(path)
	if err == nil {
		t.Fatal("a shift-only prefix was accepted")
	}
	if cfg.Prefix != Default().Prefix {
		t.Errorf("invalid prefix not replaced by the default: %+v", cfg.Prefix)
	}
}

func TestLoadReportsMalformedFile(t *testing.T) {
	cfg, err := Load(writeConfig(t, "this is not toml ="))
	if err == nil {
		t.Fatal("malformed file was accepted")
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("got %+v", cfg)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadNormalizesUnknownBackground(t *testing.T) {
	cfg, err := Load(writeConfig(t, "[appearance]\nbackground = \"purple\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Appearance.Background != BackgroundTerminal {
		t.Errorf("got %q", cfg.Appearance.Background)
	}
}

func TestLoadPlaysTheIntroOnEmptyForAnUnknownValue(t *testing.T) {
	path := writeConfig(t, "[appearance]\nanimate_intro = \"sometimes\"\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Appearance.AnimateIntro != IntroEmpty {
		t.Errorf("animate intro %q, want %q", cfg.Appearance.AnimateIntro, IntroEmpty)
	}
}

func TestSaveLeavesOutTheOldLogSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Save(path, Default()); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "max_size_mb") {
		t.Errorf("saved the old setting:\n%s", content)
	}
}

func TestLoadMovesTheOldLogSizeInMBToKB(t *testing.T) {
	cfg, err := Load(writeConfig(t, "[logs]\nmax_size_mb = 300\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Logs.MaxSizeKB != 300<<10 || cfg.Logs.MaxSizeMB != 0 || cfg.Logs.Terminals {
		t.Errorf("got %+v", cfg.Logs)
	}
}

func TestLoadKeepsAllFlowOutputForANegativeLimit(t *testing.T) {
	cfg, err := Load(writeConfig(t, "[flows]\noutput_kb = -5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Flows.OutputKB != 0 {
		t.Errorf("output_kb = %d, want 0 (all)", cfg.Flows.OutputKB)
	}
}

// A policy word noct does not know asks, never allows.
func TestUnknownMCPPolicyAsks(t *testing.T) {
	cfg, err := Load(writeConfig(t, "[mcp.policy]\nrun_command = \"sure\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MCP.Policy["run_command"] != PolicyAsk || cfg.MCP.ApprovalSeconds != DefaultApprovalSeconds {
		t.Errorf("mcp = %+v", cfg.MCP)
	}
}

func TestConfigHasTheDefaultEffects(t *testing.T) {
	d := Default()
	if d.Appearance.Animations != AnimationsOn {
		t.Errorf("animations %q, want on", d.Appearance.Animations)
	}
	for moment, want := range map[string]string{"hover": "fade", "panels": "jitter", "tabs": "jitter", "focus": "none", "move": "fade", "fullscreen": "01", "popup": "01", "activity": "fade"} {
		if got := d.Animations.Effect(moment); got != want {
			t.Errorf("%s is %q, want %q", moment, got, want)
		}
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[appearance]\nanimations = \"sometimes\"\n[animations]\npanels = \"boing\"\nfocus = \"01\"\nmove = \"spring\"\npopup = \"flicker\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Appearance.Animations != AnimationsOn || c.Animations.Panels != "jitter" || c.Animations.Focus != "01" {
		t.Errorf("read animations %q, panels %q, focus %q", c.Appearance.Animations, c.Animations.Panels, c.Animations.Focus)
	}
	if c.Animations.Move != "fade" || c.Animations.Popup != "01" {
		t.Errorf("the effects noct no longer has came back as move %q and popup %q, want the defaults", c.Animations.Move, c.Animations.Popup)
	}
	c.Animations.SetEffect("popup", "none")
	if c.Animations.Popup != "none" {
		t.Errorf("SetEffect left popup %q", c.Animations.Popup)
	}
}

func TestEachMomentHasItsOwnTime(t *testing.T) {
	d := Default()
	for moment, want := range map[string]int{"hover": 150, "panels": 300, "tabs": 300, "focus": 100, "move": 200, "fullscreen": 200, "popup": 200, "activity": 600} {
		if got := d.AnimationTimes.Duration(moment); got != want {
			t.Errorf("%s lasts %d ms, want %d", moment, got, want)
		}
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[animation_ms]\nfocus = 400\npanels = 5\ntabs = 99999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.AnimationTimes.Focus != 400 || c.AnimationTimes.Panels != 300 || c.AnimationTimes.Tabs != 300 {
		t.Errorf("read focus %d, panels %d, tabs %d; want 400 and the defaults for the times out of range", c.AnimationTimes.Focus, c.AnimationTimes.Panels, c.AnimationTimes.Tabs)
	}
	c.AnimationTimes.SetDuration("popup", 450)
	if c.AnimationTimes.Popup != 450 {
		t.Errorf("SetDuration left popup at %d", c.AnimationTimes.Popup)
	}
}

func TestCustomBindTarget(t *testing.T) {
	cases := []struct {
		bind       CustomBind
		kind, what string
		fails      bool
	}{
		{CustomBind{Key: "ctrl+alt+g", Command: "less {{.file}}"}, CustomCommand, "less {{.file}}", false},
		{CustomBind{Key: "prefix m", Script: "tools/less"}, CustomScript, "tools/less", false},
		{CustomBind{Key: "prefix m", Flow: "demo-deploy"}, CustomFlow, "demo-deploy", false},
		{CustomBind{Key: "prefix m", Script: "a", Flow: "b"}, "", "", true},
		{CustomBind{Key: "prefix m"}, "", "", true},
		{CustomBind{Command: "less"}, "", "", true},
	}
	for _, c := range cases {
		kind, what, err := c.bind.Target()
		if (err != nil) != c.fails || kind != c.kind || what != c.what {
			t.Errorf("%+v: got %q %q %v", c.bind, kind, what, err)
		}
	}
}

func TestCustomBindsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.CustomBinds = []CustomBind{{Key: "ctrl+alt+g", Name: "less", Command: "less {{.file}}"}, {Key: "prefix m", Name: "deploy", Flow: "demo-deploy"}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !slices.Equal(got.CustomBinds, cfg.CustomBinds) {
		t.Errorf("loaded %+v, %v", got.CustomBinds, err)
	}
}
