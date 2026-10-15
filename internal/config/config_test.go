package config

import (
	"os"
	"path/filepath"
	"reflect"
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
		Prefix:     Prefix{Alt: true, Shift: true, Key: "f2"},
		Shortcut:   Shortcut{Alt: true},
		Appearance: Appearance{Theme: "ember", Background: BackgroundTheme, AnimateIntro: IntroAlways},
		NewTab:     "scripts",
		// CloseEmptyTabs off: a bool that is on by default must survive being saved off
		Mouse:         false,
		Editor:        "code -w",
		FileManager:   "yazi",
		ScriptsDir:    "~/tests/scripts",
		ScriptsSort:   SortByRun,
		ScriptFolders: []string{"~/work/scripts", "/srv/shared"},
		Keys:          map[string]map[string][]string{"scripts": {"edit": {"x", "ctrl+e"}, "delete": {}}},
		Logs:          Logs{KeepDays: 30, MaxSizeMB: 1000},
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
