package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func (s *session) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(s.binary, args...)
	cmd.Env = s.env
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestSessionsKeepTheirProgramsApart(t *testing.T) {
	s := newSession(t)
	dev, devExited := s.attach(t, "new", "dev")
	waitForScreen(t, dev, "the home screen", showsHome)
	openTerminal(t, dev)
	typeText(dev, "NOCT_MARK=in-dev")
	enter(dev)
	dev.SendKey(prefix)
	typeText(dev, "d")
	waitForExit(t, devExited, "detaching did not end the client")

	other, otherExited := s.attach(t, "new", "other")
	waitForScreen(t, other, "the new session's home", showsHome)
	other.SendKey(prefix)
	typeText(other, "d")
	waitForExit(t, otherExited, "detaching did not end the client")

	for _, command := range []string{"sessions", "attach"} {
		output, err := s.run(t, command)
		if err != nil || !strings.Contains(output, "dev") || !strings.Contains(output, "other") {
			t.Fatalf("noct %s: %v\n%s", command, err, output)
		}
	}

	back, _ := s.attach(t, "attach", "dev")
	waitForScreen(t, back, "dev's shell", func(screen string) bool { return strings.Contains(screen, "NOCT_MARK=in-dev") })
}

func TestAttachingAMissingSessionFails(t *testing.T) {
	noct, exited := newSession(t).attach(t, "attach", "nowhere")
	waitForExit(t, exited, "noct attach did not end for a missing session")
	if screen := screenText(noct); !strings.Contains(screen, "no session named nowhere") {
		t.Errorf("noct attach nowhere did not say why:\n%s", screen)
	}
}

func TestSessionOpensAgainAfterQuit(t *testing.T) {
	s := newSession(t)
	dir := t.TempDir()
	noct, exited := s.attach(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)
	typeText(noct, "cd "+dir)
	enter(noct)
	typeText(noct, "echo moved")
	enter(noct)
	waitForScreen(t, noct, "the shell in the new directory", func(screen string) bool { return strings.Count(screen, "moved") >= 2 })

	if output, err := s.run(t, "quit"); err != nil {
		t.Fatalf("noct quit: %v\n%s", err, output)
	}
	waitForExit(t, exited, "the attached client did not end when the daemon quit")

	again, _ := s.attach(t)
	waitForScreen(t, again, "a shell opened again", func(screen string) bool { return strings.Contains(screen, "$ ") })
	typeText(again, "pwd")
	enter(again)
	resolved, _ := filepath.EvalSymlinks(dir)
	waitForScreen(t, again, "the shell back in its directory", func(screen string) bool {
		return strings.Contains(screen, dir) || strings.Contains(screen, resolved)
	})
}

func TestKillForgetsASessionWithoutNoctRunning(t *testing.T) {
	s := newSession(t)
	noct, exited := s.attach(t, "new", "gone")
	waitForScreen(t, noct, "the home screen", showsHome)
	if output, err := s.run(t, "quit"); err != nil {
		t.Fatalf("noct quit: %v\n%s", err, output)
	}
	waitForExit(t, exited, "the attached client did not end when the daemon quit")

	if output, err := s.run(t, "kill", "gone"); err != nil {
		t.Fatalf("noct kill: %v\n%s", err, output)
	}
	content, _ := os.ReadFile(filepath.Join(stateDir(s), "sessions.json"))
	if strings.Contains(string(content), `"gone"`) {
		t.Errorf("the session is still saved:\n%s", content)
	}
}

func TestTerminalOffersToRunAgainAfterQuit(t *testing.T) {
	s := newSession(t)
	noct, exited := s.attach(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)
	typeText(noct, `sh -c 'echo started; sleep 300; :'`)
	enter(noct)
	waitForScreen(t, noct, "the program running", func(screen string) bool { return strings.Count(screen, "started") >= 2 })

	if output, err := s.run(t, "quit"); err != nil {
		t.Fatalf("noct quit: %v\n%s", err, output)
	}
	waitForExit(t, exited, "the attached client did not end when the daemon quit")

	again, _ := s.attach(t)
	waitForScreen(t, again, "the offer to run it again", func(screen string) bool { return strings.Contains(screen, "run again?") })
	enter(again)
	waitForScreen(t, again, "the program running again", func(screen string) bool {
		return strings.Contains(screen, "started") && !strings.Contains(screen, "run again?")
	})
}

func TestDuplicateOffersWhatTheTerminalRuns(t *testing.T) {
	noct, _ := startNoct(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)
	typeText(noct, `sh -c 'echo started; sleep 300; :'`)
	enter(noct)
	waitForScreen(t, noct, "the program running", func(screen string) bool { return strings.Count(screen, "started") >= 2 })

	noct.SendKey(prefix)
	typeText(noct, "D")
	waitForScreen(t, noct, "the offer to run it in the new terminal", func(screen string) bool {
		return strings.Contains(screen, "run again?") && strings.Contains(screen, "sleep 300")
	})
}

func TestANewSessionOpensATemplate(t *testing.T) {
	s := newSession(t)
	var home string
	for _, entry := range s.env {
		if dir, ok := strings.CutPrefix(entry, "HOME="); ok {
			home = dir
		}
	}
	dir := filepath.Join(home, ".config", "noct", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	template := "layout: columns\npanes:\n  - terminal: { name: from-template }\n  - terminal: { name: beside-it }\n"
	if err := os.WriteFile(filepath.Join(dir, "pair.yaml"), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	noct, _ := s.attach(t, "new", "work", "--template", "pair")
	waitForScreen(t, noct, "the template's panes", func(screen string) bool {
		return strings.Contains(screen, "from-template") && strings.Contains(screen, "beside-it")
	})
	missing, exited := s.attach(t, "new", "other", "--template", "nope")
	waitForExit(t, exited, "a missing template did not end noct new")
	if screen := screenText(missing); !strings.Contains(screen, "no template nope") {
		t.Errorf("noct new --template nope did not say why:\n%s", screen)
	}
}
