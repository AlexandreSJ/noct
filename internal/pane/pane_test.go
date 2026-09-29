package pane

import (
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/pty"
)

const testTimeout = 5 * time.Second

func startTestPane(t *testing.T, script string) (*Pane, <-chan struct{}) {
	t.Helper()
	exited := make(chan struct{})
	p, err := Start(Options{
		Argv:   []string{"sh", "-c", script},
		Env:    []string{"TERM=xterm-256color", "PATH=/usr/bin:/bin"},
		Size:   pty.Size{Cols: 20, Rows: 5},
		OnExit: func(int) { close(exited) },
	})
	if err != nil {
		t.Fatalf("starting pane: %v", err)
	}
	t.Cleanup(p.Close)
	return p, exited
}

func waitFor(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the pane")
	}
}

func screenText(p *Pane) string {
	buffer := uv.NewScreenBuffer(p.size.Cols, p.size.Rows)
	p.Draw(buffer, buffer.Bounds())
	return buffer.Render()
}

func TestPaneDrawsProgramOutput(t *testing.T) {
	p, exited := startTestPane(t, `printf 'hello\r\n\033[31mred\033[0m'`)
	waitFor(t, exited)

	text := screenText(p)
	if !strings.Contains(text, "hello") || !strings.Contains(text, "red") {
		t.Errorf("screen does not show the output:\n%s", text)
	}
}

// Invalid UTF-8 in a title would reach the real terminal as is and throw its cursor off.
func TestPaneTitleIsValidUTF8(t *testing.T) {
	p, exited := startTestPane(t, `printf '\033]0;a\377b\007'`)
	waitFor(t, exited)

	if got, want := p.Title(), "a�b"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestPaneForwardsKeysToProgram(t *testing.T) {
	p, exited := startTestPane(t, `read line; printf 'got:%s' "$line"`)

	for _, r := range "ok" {
		p.SendKey(uv.KeyPressEvent{Code: r, Text: string(r)})
	}
	p.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitFor(t, exited)

	if text := screenText(p); !strings.Contains(text, "got:ok") {
		t.Errorf("program did not receive the keys:\n%s", text)
	}
}

func TestPaneResizeReachesProgram(t *testing.T) {
	p, exited := startTestPane(t, `read _; stty size`)

	p.Resize(pty.Size{Cols: 30, Rows: 7})
	p.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitFor(t, exited)

	if text := screenText(p); !strings.Contains(text, "7 30") {
		t.Errorf("program did not see the new size:\n%s", text)
	}
}

func TestPaneTracksTitle(t *testing.T) {
	p, exited := startTestPane(t, `printf '\033]2;my title\007'`)
	waitFor(t, exited)

	if got := p.Title(); got != "my title" {
		t.Errorf("title = %q, want %q", got, "my title")
	}
}

func TestPaneCloseStopsRunningProgram(t *testing.T) {
	p, exited := startTestPane(t, `sleep 30`)

	p.SendKey(uv.KeyPressEvent{Code: 'x', Text: "x"})
	p.Close()
	waitFor(t, exited)

	p.SendKey(uv.KeyPressEvent{Code: 'y', Text: "y"}) // must not block after closing
}

// Terminals with keyboard enhancements report NumLock and CapsLock as modifiers; with either on,
// Ctrl+D must still reach the program as EOF.
func TestPaneIgnoresLockModifiers(t *testing.T) {
	for _, lock := range []uv.KeyMod{uv.ModNumLock, uv.ModCapsLock} {
		p, exited := startTestPane(t, `stty raw -echo; dd bs=1 count=1 2>/dev/null | od -An -tx1`)
		time.Sleep(100 * time.Millisecond) // let stty switch the terminal to raw mode first

		p.SendKey(uv.KeyPressEvent{Code: 'd', Mod: uv.ModCtrl | lock})
		waitFor(t, exited)

		if text := screenText(p); !strings.Contains(text, "04") {
			t.Errorf("with lock modifier %d the program did not get ctrl+d:\n%s", lock, text)
		}
	}
}

func TestDrawScrolledShowsHistory(t *testing.T) {
	p, exited := startTestPane(t, `i=1; while [ $i -le 12 ]; do printf 'line%d\r\n' $i; i=$((i+1)); done`)
	waitFor(t, exited)

	if history := p.ScrollbackLen(); history < 7 {
		t.Fatalf("history of %d lines, want the lines that scrolled off a 5-row screen", history)
	}
	buffer := uv.NewScreenBuffer(20, 5)
	p.DrawScrolled(buffer, buffer.Bounds(), p.ScrollbackLen())
	if top := strings.Split(buffer.Render(), "\n")[0]; !strings.Contains(top, "line1") {
		t.Errorf("scrolled to the top, first line is %q", top)
	}
	if live := screenText(p); strings.Contains(live, "line1\r") || !strings.Contains(live, "line12") {
		t.Errorf("live screen:\n%s", live)
	}
}

func TestPaneReportsTheExitCode(t *testing.T) {
	codes := make(chan int, 1)
	_, err := Start(Options{
		Argv:   []string{"sh", "-c", "exit 3"},
		Env:    []string{"PATH=/usr/bin:/bin"},
		Size:   pty.Size{Cols: 20, Rows: 5},
		OnExit: func(code int) { codes <- code },
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-codes:
		if code != 3 {
			t.Errorf("exit code %d, want 3", code)
		}
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the exit")
	}
}

func TestForegroundProgramIsTheJobTheShellRuns(t *testing.T) {
	p, _ := startTestPane(t, `set -m; sleep 30; true`)
	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		if name, ok := p.ForegroundProgram(); ok {
			if name != "sleep" {
				t.Fatalf("foreground program %q, want sleep", name)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("never saw sleep in the foreground")
}

func TestMouseReachesProgramsThatAskForIt(t *testing.T) {
	// The program turns on SGR mouse reporting, then prints the first report it reads.
	p, exited := startTestPane(t, `printf '\033[?1000h\033[?1006h'; stty raw -echo; dd bs=1 count=9 2>/dev/null | od -An -c`)
	deadline := time.Now().Add(testTimeout)
	for !p.WantsMouse() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !p.WantsMouse() {
		t.Fatal("the program's mouse mode was not noticed")
	}
	time.Sleep(100 * time.Millisecond) // let stty switch the terminal to raw mode first
	p.SendMouse(uv.MouseClickEvent{X: 2, Y: 1, Button: uv.MouseLeft})
	waitFor(t, exited)
	if text := screenText(p); !strings.Contains(text, "3") || !strings.Contains(text, "M") {
		t.Errorf("the program did not get the click at column 3, row 2:\n%s", text)
	}
}

// A full history drops its oldest lines and keeps the newest (the scrollback fix in third_party).
func TestFullHistoryKeepsTheNewestLines(t *testing.T) {
	p, exited := startTestPane(t, `seq 1 10050`)
	waitFor(t, exited)

	history := p.ScrollbackLen()
	if history < 9990 || history > 10000 {
		t.Fatalf("history of %d lines, want it full at about 10000", history)
	}
	buffer := uv.NewScreenBuffer(20, 5)
	p.DrawScrolled(buffer, buffer.Bounds(), history)
	oldest := strings.TrimSpace(strings.Split(buffer.Render(), "\n")[0])
	if oldest == "1" || oldest == "" {
		t.Errorf("oldest line kept is %q, want the first lines dropped", oldest)
	}
	if live := screenText(p); !strings.Contains(live, "10050") {
		t.Errorf("live screen:\n%s", live)
	}
}
