package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
)

// fakePrograms puts executables with these names on a PATH of their own.
func fakePrograms(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestFirstRunIsWithoutConfigAndSessions(t *testing.T) {
	dir := t.TempDir()
	configPath, sessionsPath := filepath.Join(dir, "config.toml"), filepath.Join(dir, "sessions.json")
	if !isFirstRun(configPath, sessionsPath) {
		t.Error("no config and no sessions is not a first run")
	}
	if err := os.WriteFile(sessionsPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isFirstRun(configPath, sessionsPath) {
		t.Error("sessions without a config is a first run; it is an upgrade")
	}
}

func TestTheSetupOpensOnTheFirstRunOrWhenAsked(t *testing.T) {
	a := testApp(t)
	a.openSetupOnAttach(false)
	if a.mode == setupMode {
		t.Fatal("the setup opened unasked on a later run")
	}
	a.firstRun = true
	a.openSetupOnAttach(false)
	if a.mode != setupMode || a.firstRun {
		t.Fatalf("mode %v firstRun %v, want the setup once", a.mode, a.firstRun)
	}
	a.mode = normalMode
	a.openSetupOnAttach(true)
	if a.mode != setupMode {
		t.Error("noct setup did not open it")
	}
}

func TestKeepingEverythingWritesNothing(t *testing.T) {
	a := testApp(t)
	content := []byte("# mine\n[appearance]\ntheme = \"blood\"\n")
	if err := os.WriteFile(a.configPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	a.openSetup()
	press(a, enterPress)
	if got, _ := os.ReadFile(a.configPath); string(got) != string(content) || a.mode != normalMode {
		t.Errorf("mode %v, file now %q", a.mode, got)
	}
}

func TestTheDefaultsOnTheFirstRun(t *testing.T) {
	a := testApp(t)
	a.firstRun = true
	a.openSetupOnAttach(false)
	press(a, enterPress)
	saved, err := config.Load(a.configPath)
	if _, statErr := os.Stat(a.configPath); statErr != nil || err != nil || saved.Appearance.Theme != config.Default().Appearance.Theme {
		t.Errorf("saved %+v, %v, %v", saved.Appearance, err, statErr)
	}
}

// Step by step: the keys, one theme, few animations, no demo (skipped), an AI client and an
// example bind, saved together at the end.
func TestTheSetupStepByStep(t *testing.T) {
	fakePrograms(t, "claude", "less")
	a := testApp(t)
	l := recordLaunches(a)
	a.openSetup()
	press(a, downPress, downPress, enterPress) // choose step by step
	if a.setup.step != setupKeys {
		t.Fatalf("step %v, want the keys", a.setup.step)
	}
	press(a, downPress, rightPress, rightPress, enterPress) // shortcut alt → ctrl → ctrl+alt
	press(a, letter('a'))                                   // none of the themes
	for a.setup.themeNames()[a.setup.selected] != "blood" {
		press(a, downPress)
	}
	press(a, enterPress)
	press(a, downPress, enterPress) // few
	press(a, letter('s'))           // no demo
	press(a, spacePress, enterPress)
	press(a, enterPress) // less's bind, marked already
	if a.setup.step != setupSummary || !strings.Contains(screenText(a), "blood") {
		t.Fatalf("step %v, want the summary naming blood:\n%s", a.setup.step, screenText(a))
	}
	press(a, enterPress)

	saved, err := config.Load(a.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Shortcut.Ctrl || !saved.Shortcut.Alt || saved.Appearance.Theme != "blood" || !slices.Equal(saved.Appearance.Themes, []string{"blood"}) {
		t.Errorf("saved shortcut %+v, theme %q among %q", saved.Shortcut, saved.Appearance.Theme, saved.Appearance.Themes)
	}
	if saved.Animations.Panels != "none" || saved.Animations.Hover != "fade" {
		t.Errorf("few gave %+v", saved.Animations)
	}
	if len(saved.CustomBinds) != 1 || saved.CustomBinds[0].Command != "less {{.file}}" {
		t.Errorf("binds %+v, want less's", saved.CustomBinds)
	}
	if _, err := os.Stat(filepath.Join(flow.Dir(), "demo-deploy.yaml")); err == nil {
		t.Error("the demo was installed after skipping it")
	}
	if run := l.last(t); !strings.Contains(run.argv[2], "claude mcp add") {
		t.Errorf("ran %q, want noct added to Claude Code", run.argv)
	}
	if a.mode != normalMode {
		t.Errorf("mode %v after saving", a.mode)
	}
}

// The demo step installs the demo; esc goes back a step.
func TestTheSetupInstallsTheDemo(t *testing.T) {
	fakePrograms(t)
	a := testApp(t)
	a.openSetup()
	press(a, downPress, downPress, enterPress, enterPress, enterPress, enterPress)
	if a.setup.step != setupDemo {
		t.Fatalf("step %v, want the demo", a.setup.step)
	}
	press(a, escPress)
	if a.setup.step != setupAnimations {
		t.Errorf("esc went to %v, want the animations", a.setup.step)
	}
	press(a, enterPress, enterPress, enterPress, enterPress, enterPress)
	if _, err := os.Stat(filepath.Join(flow.Dir(), "demo-deploy.yaml")); err != nil {
		t.Errorf("the demo was not installed: %v", err)
	}
}

// s on a step puts back what the step had before the setup, even after changing it.
func TestSkippingAStepKeepsItAsItWas(t *testing.T) {
	fakePrograms(t)
	a := testApp(t)
	before := a.config.Shortcut
	a.openSetup()
	press(a, downPress, downPress, enterPress) // choose step by step
	press(a, downPress, rightPress)            // the shortcut changes in the draft
	if a.setup.draft.Shortcut == before {
		t.Fatal("the shortcut did not change")
	}
	press(a, letter('s'))
	if a.setup.step != setupThemes || a.setup.draft.Shortcut != before {
		t.Errorf("step %v, shortcut %+v, want the themes with %+v", a.setup.step, a.setup.draft.Shortcut, before)
	}
}

// Someone who had noct before the setup is told about it once, and not after opening it.
func TestTheSetupIsToldOnceToWhoHadNoctBefore(t *testing.T) {
	a := testApp(t)
	a.openSetupOnAttach(false)
	if a.mode == setupMode || a.notice != setupNotice {
		t.Fatalf("mode %v, notice %q, want the notice only", a.mode, a.notice)
	}
	a.notice = ""
	a.openSetupOnAttach(false)
	if a.notice != "" {
		t.Errorf("told again: %q", a.notice)
	}

	b := testApp(t)
	b.openSetupOnAttach(true) // noct setup
	b.mode, b.notice = normalMode, ""
	b.openSetupOnAttach(false)
	if b.notice != "" {
		t.Errorf("told after noct setup: %q", b.notice)
	}
}
