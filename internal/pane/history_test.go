package pane

import (
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/pty"
)

func TestHistoryShowsAboveTheNewProgram(t *testing.T) {
	exited := make(chan struct{})
	// The old output asks for the cursor position and ends in a full-screen program: neither the
	// answer nor the alternate screen may reach the new program.
	history := "old output\r\n\x1b[6n\x1b[?1049hfull screen"
	p, err := Start(Options{
		Argv:    []string{"sh", "-c", `read -r line; printf 'got:[%s]' "$line"`},
		Env:     []string{"TERM=xterm-256color", "PATH=/usr/bin:/bin"},
		Size:    pty.Size{Cols: 40, Rows: 8},
		OnExit:  func(int) { close(exited) },
		History: []byte(history),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	p.SendKey(uv.KeyPressEvent{Code: 'x', Text: "x"})
	p.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	waitFor(t, exited)

	text := screenText(p)
	if !strings.Contains(text, "old output") || !strings.Contains(text, "restored from the log") {
		t.Errorf("the history is not on screen:\n%s", text)
	}
	if strings.Contains(text, "full screen") {
		t.Errorf("the alternate screen stayed on:\n%s", text)
	}
	if !strings.Contains(text, "got:[x]") {
		t.Errorf("the program read more than what was typed:\n%s", text)
	}
}

func TestEmptyHistoryShowsNothing(t *testing.T) {
	if got := renderHistory([]byte("\r\n\r\n"), 20); len(got) != 0 {
		t.Errorf("blank history rendered as %q", got)
	}
}
