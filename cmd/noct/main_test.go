package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/pane"
	"github.com/AlexandreSJ/noct/internal/pty"
)

var outerSize = pty.Size{Cols: 100, Rows: 40}

var prefix = uv.KeyPressEvent{Code: uv.KeySpace, Mod: uv.ModCtrl}

// session is one isolated noct: its own daemon socket, state and home, stopped after the test.
type session struct {
	binary string
	env    []string
}

// builtBinary is the noct binary under test, built once by TestMain.
var builtBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "noct-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	builtBinary = filepath.Join(dir, "noct")
	if output, err := exec.Command("go", "build", "-o", builtBinary, ".").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building noct: %v\n%s", err, output)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newSession(t *testing.T) *session {
	t.Helper()
	// HISTFILE: where /bin/sh is bash, a shell killed by noct quit writes its history to HOME
	// after quit returns, racing the removal of the test's HOME.
	s := &session{binary: builtBinary, env: []string{
		"TERM=xterm-256color", "PATH=/usr/bin:/bin", "SHELL=/bin/sh", "HISTFILE=/dev/null",
		"HOME=" + t.TempDir(), "XDG_RUNTIME_DIR=" + t.TempDir(), "XDG_STATE_HOME=" + t.TempDir(),
	}}
	t.Cleanup(func() {
		quit := exec.Command(s.binary, "quit")
		quit.Env = s.env
		_ = quit.Run() // stops the daemon; with none running there is nothing to do
	})
	return s
}

// attach runs noct inside a pane, which plays the role of the user's terminal; args choose the
// session, as on the command line.
func (s *session) attach(t *testing.T, args ...string) (*pane.Pane, <-chan struct{}) {
	t.Helper()
	exited := make(chan struct{})
	outer, err := pane.Start(pane.Options{
		Argv:   append([]string{s.binary}, args...),
		Env:    s.env,
		Size:   outerSize,
		OnExit: func(int) { close(exited) },
	})
	if err != nil {
		t.Fatalf("starting noct: %v", err)
	}
	t.Cleanup(outer.Close)
	return outer, exited
}

func startNoct(t *testing.T) (*pane.Pane, <-chan struct{}) {
	t.Helper()
	return newSession(t).attach(t)
}

func stateDir(s *session) string {
	for _, entry := range s.env {
		if dir, ok := strings.CutPrefix(entry, "XDG_STATE_HOME="); ok {
			return filepath.Join(dir, "noct")
		}
	}
	return ""
}

func screenText(p *pane.Pane) string {
	buffer := uv.NewScreenBuffer(outerSize.Cols, outerSize.Rows)
	p.Draw(buffer, buffer.Bounds())
	return buffer.Render()
}

func waitForScreen(t *testing.T, p *pane.Pane, describe string, ready func(screen string) bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ready(screenText(p)) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; screen:\n%s", describe, screenText(p))
}

// openTerminal opens a terminal panel, which on the home screen also opens a tab, and waits
// for its prompt.
func openTerminal(t *testing.T, noct *pane.Pane) {
	t.Helper()
	noct.SendKey(prefix)
	typeText(noct, "p")
	waitForScreen(t, noct, "a shell prompt", func(s string) bool { return strings.Contains(s, "$ ") })
}

func showsHome(screen string) bool {
	return strings.Contains(screen, "█▀▄ █▀█ █▀▀ ▀█▀") && frameCount(screen) == 0
}

func waitForExit(t *testing.T, exited <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatal(what)
	}
}

func frameCount(screen string) int {
	return strings.Count(screen, "┌")
}

func typeText(p *pane.Pane, text string) {
	for _, r := range text {
		p.SendKey(uv.KeyPressEvent{Code: r, Text: string(r)})
	}
}

func TestStartsOnTheHomeScreen(t *testing.T) {
	noct, _ := startNoct(t)
	waitForScreen(t, noct, "the raven", showsHome)
}

func TestWorkspaceSplitsTypesAndQuits(t *testing.T) {
	noct, exited := startNoct(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)

	noct.SendKey(prefix)
	typeText(noct, "p")
	waitForScreen(t, noct, "a second pane with its prompt", func(s string) bool {
		return frameCount(s) == 2 && strings.Count(s, "$ ") == 2
	})

	typeText(noct, "echo noct-works")
	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitForScreen(t, noct, "the command output", func(s string) bool {
		return strings.Count(s, "noct-works") >= 2 // the typed command and its output
	})

	noct.SendKey(prefix)
	typeText(noct, "q")
	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitForExit(t, exited, "noct did not quit")
}

func TestClosingTheLastShellReturnsHome(t *testing.T) {
	noct, exited := startNoct(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)

	typeText(noct, "exit")
	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitForScreen(t, noct, "the home screen again", showsHome)

	noct.SendKey(prefix)
	typeText(noct, "q")
	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitForExit(t, exited, "noct did not quit from the home screen")
}

func TestCtrlCInterruptsAndCtrlDClosesTheShell(t *testing.T) {
	noct, _ := startNoct(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)

	typeText(noct, "sleep 30")
	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitForScreen(t, noct, "sleep to start", func(s string) bool { return strings.Contains(s, "sleep 30") })
	noct.SendKey(uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl})
	waitForScreen(t, noct, "the prompt back after ctrl+c", func(s string) bool { return strings.Count(s, "$ ") >= 2 })
	typeText(noct, "echo interrupted")
	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitForScreen(t, noct, "a new prompt after ctrl+c", func(s string) bool {
		return strings.Count(s, "interrupted") >= 2
	})

	noct.SendKey(uv.KeyPressEvent{Code: 'd', Mod: uv.ModCtrl})
	waitForScreen(t, noct, "the home screen after ctrl+d", showsHome)
}

func enter(p *pane.Pane) {
	p.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
}

func TestDetachKeepsProgramsRunningUntilReattached(t *testing.T) {
	s := newSession(t)
	first, firstExited := s.attach(t)
	waitForScreen(t, first, "the home screen", showsHome)
	openTerminal(t, first)
	typeText(first, "NOCT_MARK=still-here")
	enter(first)

	first.SendKey(prefix)
	typeText(first, "d")
	waitForExit(t, firstExited, "detaching did not end the client")

	second, _ := s.attach(t)
	waitForScreen(t, second, "the same shell", func(screen string) bool { return strings.Contains(screen, "NOCT_MARK=still-here") })
	typeText(second, "echo mark:$NOCT_MARK")
	enter(second)
	waitForScreen(t, second, "the variable set before detaching", func(screen string) bool {
		return strings.Contains(screen, "mark:still-here")
	})
}

func TestQuitCommandStopsTheDaemon(t *testing.T) {
	s := newSession(t)
	noct, exited := s.attach(t)
	waitForScreen(t, noct, "the home screen", showsHome)

	quit := exec.Command(s.binary, "quit")
	quit.Env = s.env
	if output, err := quit.CombinedOutput(); err != nil {
		t.Fatalf("noct quit: %v\n%s", err, output)
	}
	waitForExit(t, exited, "the attached client did not end when the daemon quit")
}

// writeConfig gives the session's noct a config file before it starts.
func writeConfig(t *testing.T, s *session, content string) {
	t.Helper()
	for _, entry := range s.env {
		if home, ok := strings.CutPrefix(entry, "HOME="); ok {
			dir := filepath.Join(home, ".config", "noct")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("the session has no HOME")
}

// echoInATerminal prints logged-42 in a new terminal pane and waits for it on screen.
func echoInATerminal(t *testing.T, s *session) {
	t.Helper()
	noct, _ := s.attach(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)
	typeText(noct, "echo logged-$((6*7))")
	enter(noct)
	waitForScreen(t, noct, "the output", func(screen string) bool { return strings.Contains(screen, "logged-42") })
}

func logsContain(s *session, text string) bool {
	logs, _ := filepath.Glob(filepath.Join(stateDir(s), "logs", "*", "*.log"))
	for _, log := range logs {
		if content, _ := os.ReadFile(log); strings.Contains(string(content), text) {
			return true
		}
	}
	return false
}

func TestTerminalOutputIsLoggedWhenTheConfigSaysSo(t *testing.T) {
	s := newSession(t)
	writeConfig(t, s, "[logs]\nterminals = true\n")
	echoInATerminal(t, s)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if logsContain(s, "logged-42") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the pane output never reached a log file")
}

func TestTerminalOutputIsNotLoggedByDefault(t *testing.T) {
	s := newSession(t)
	echoInATerminal(t, s)
	time.Sleep(500 * time.Millisecond) // a log, were there one, gets the output within this
	if logsContain(s, "logged-42") {
		t.Error("a terminal's output was logged with terminals off")
	}
}

func TestScrollModeShowsEarlierOutput(t *testing.T) {
	noct, _ := startNoct(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)
	typeText(noct, "i=1; while [ $i -le 60 ]; do echo row$i; i=$((i+1)); done")
	enter(noct)
	waitForScreen(t, noct, "the last row", func(screen string) bool { return strings.Contains(screen, "row60") })

	noct.SendKey(prefix)
	typeText(noct, "ug")
	waitForScreen(t, noct, "the first rows", func(screen string) bool {
		return strings.Contains(screen, "row1\x1b") || strings.Contains(screen, "row1 ")
	})
	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyEscape})
	waitForScreen(t, noct, "the live screen", func(screen string) bool { return strings.Contains(screen, "row60") })
}

func TestDraggingTheDividerResizesTheTerminal(t *testing.T) {
	noct, _ := startNoct(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)
	noct.SendKey(prefix)
	typeText(noct, "p")
	waitForScreen(t, noct, "two shells", func(s string) bool { return strings.Count(s, "$ ") >= 2 })
	for deadline := time.Now().Add(5 * time.Second); !noct.WantsMouse() && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}

	noct.SendMouse(uv.MouseClickEvent{X: 49, Y: 10, Button: uv.MouseLeft})
	noct.SendMouse(uv.MouseMotionEvent{X: 30, Y: 10, Button: uv.MouseLeft})
	noct.SendMouse(uv.MouseReleaseEvent{X: 30, Y: 10, Button: uv.MouseLeft})
	typeText(noct, "stty size")
	enter(noct)
	waitForScreen(t, noct, "the right shell 68 columns wide", func(s string) bool { return strings.Contains(s, " 68") })
}

// The cursor must reach the outer terminal in the same frame as the change that moved it, not
// one frame later.
func TestCursorFollowsTheProgramRightAway(t *testing.T) {
	noct, _ := startNoct(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	openTerminal(t, noct)
	typeText(noct, "abc")
	waitForScreen(t, noct, "the typed text", func(s string) bool { return strings.Contains(s, "$ abc") })
	settled, _ := noct.Cursor()

	noct.SendKey(uv.KeyPressEvent{Code: uv.KeyLeft})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cursor, _ := noct.Cursor(); cursor.X == settled.X-1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	cursor, _ := noct.Cursor()
	t.Fatalf("cursor at column %d after left, want %d", cursor.X, settled.X-1)
}
