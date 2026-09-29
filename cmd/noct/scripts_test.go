package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// saveScript writes a personal script into the session's home.
func (s *session) saveScript(t *testing.T, name, content string) {
	t.Helper()
	var home string
	for _, entry := range s.env {
		if dir, ok := strings.CutPrefix(entry, "HOME="); ok {
			home = dir
		}
	}
	dir := filepath.Join(home, ".local", "share", "noct", "scripts", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "script.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// command runs noct with args, without a terminal, and returns its output and exit code.
func (s *session) command(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(s.binary, args...)
	cmd.Env, cmd.Dir = s.env, t.TempDir()
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(output), exit.ExitCode()
	}
	if err != nil {
		t.Fatalf("running noct %v: %v", args, err)
	}
	return string(output), 0
}

func TestScriptCommands(t *testing.T) {
	s := newSession(t)
	s.saveScript(t, "greet", "description: Says hello\nrun: echo {{.greeting}} {{.who}}\nparams:\n"+
		"  - {name: greeting, choices: [hello, bye]}\n  - {name: who, default: world, remember: true}\n")
	s.saveScript(t, "fail", "run: exit 4\n")

	if output, code := s.command(t, "ls"); code != 0 || !strings.Contains(output, "Says hello") {
		t.Errorf("ls exited %d:\n%s", code, output)
	}
	if output, code := s.command(t, "check"); code != 0 || !strings.Contains(output, "2 scripts and 0 flows ok") {
		t.Errorf("check exited %d:\n%s", code, output)
	}
	if output, code := s.command(t, "run", "greet", "who=ana"); code != 0 || output != "hello ana\n" {
		t.Errorf("run exited %d: %q", code, output)
	}
	if output, _ := s.command(t, "run", "greet", "greeting=bye"); output != "bye ana\n" {
		t.Errorf("who was not remembered: %q", output)
	}
	if output, code := s.command(t, "-s", "greet", "who=bia"); code != 0 || output != "hello bia\n" {
		t.Errorf("-s exited %d: %q", code, output)
	}
	if output, code := s.command(t, "--script", "missing"); code == 0 || !strings.Contains(output, "Script not found at") || !strings.Contains(output, "noct/scripts") {
		t.Errorf("a missing script (%d): %s", code, output)
	}
	if _, code := s.command(t, "run", "fail"); code != 4 {
		t.Errorf("the script's exit code became %d", code)
	}
	if output, code := s.command(t, "run", "greet", "greeting=hi"); code == 0 || !strings.Contains(output, "one of") {
		t.Errorf("a value outside the choices ran (%d): %s", code, output)
	}

	s.saveScript(t, "broken", "run: echo {{.x\n")
	if output, code := s.command(t, "check"); code == 0 || !strings.Contains(output, "broken") {
		t.Errorf("check passed a broken script (%d):\n%s", code, output)
	}
}

func TestScriptRunsFromTheScriptsPanel(t *testing.T) {
	s := newSession(t)
	s.saveScript(t, "hello", "run: printf 'from the script\\n'\n")
	noct, _ := s.attach(t)
	waitForScreen(t, noct, "the raven", showsHome)

	noct.SendKey(prefix)
	typeText(noct, "s")
	waitForScreen(t, noct, "the scripts panel", func(screen string) bool { return strings.Contains(screen, "hello") })
	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitForScreen(t, noct, "the finished run", func(screen string) bool {
		return strings.Contains(screen, "from the script") && strings.Contains(screen, "✓ done")
	})

	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitForScreen(t, noct, "the run closed", func(screen string) bool { return frameCount(screen) == 1 })
}

func (s *session) home() string {
	for _, entry := range s.env {
		if dir, ok := strings.CutPrefix(entry, "HOME="); ok {
			return dir
		}
	}
	return ""
}

func TestFlowCommands(t *testing.T) {
	s := newSession(t)
	s.saveScript(t, "shout", "run: echo {{.word}}!\nparams:\n  - {name: word}\n")
	flows := filepath.Join(s.home(), ".local", "share", "noct", "flows")
	if err := os.MkdirAll(flows, 0o755); err != nil {
		t.Fatal(err)
	}
	flowText := "steps:\n  - {id: pick, run: echo color=red, capture: {color: 'color=(\\w+)'}}\n" +
		"  - if: '{{eq .steps.pick.color \"red\"}}'\n    then: [{script: shout, with: {word: '{{.steps.pick.color}}'}}]\n" +
		"  - run: exit 5\n"
	if err := os.WriteFile(filepath.Join(flows, "colors.yaml"), []byte(flowText), 0o644); err != nil {
		t.Fatal(err)
	}

	output, code := s.command(t, "flow", "run", "colors")
	if code != 5 || !strings.Contains(output, "red!") || !strings.Contains(output, "stopped: exit 5") {
		t.Errorf("flow run exited %d:\n%s", code, output)
	}
	if output, code := s.command(t, "flow", "ls"); code != 0 || !strings.Contains(output, "colors") {
		t.Errorf("flow ls exited %d:\n%s", code, output)
	}
	if output, code := s.command(t, "check"); code != 0 || !strings.Contains(output, "1 flows ok") {
		t.Errorf("check exited %d:\n%s", code, output)
	}
	state := ""
	for _, entry := range s.env {
		if dir, ok := strings.CutPrefix(entry, "XDG_STATE_HOME="); ok {
			state = filepath.Join(dir, "noct")
		}
	}
	if state == "" {
		state = filepath.Join(s.home(), ".local", "state", "noct")
	}
	if _, err := os.Stat(filepath.Join(state, "flows", "colors.json")); err != nil {
		t.Errorf("no result kept: %v", err)
	}
	runs, _ := os.ReadFile(filepath.Join(state, "runs.json"))
	if !strings.Contains(string(runs), `"flow":"colors"`) || !strings.Contains(string(runs), `"exit":5`) {
		t.Errorf("runs = %s", runs)
	}
}
