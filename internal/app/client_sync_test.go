package app

import (
	"bytes"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestAClientAsksWhetherItsTerminalDrawsFramesAtOnce(t *testing.T) {
	var out bytes.Buffer
	c := &client{screen: uv.NewTerminalScreen(&out, uv.Environ{"TERM=xterm-256color"})}
	if err := c.start(); err != nil {
		t.Fatal(err)
	}
	_ = c.screen.Flush()
	if !strings.Contains(out.String(), ansi.RequestModeSynchronizedOutput) {
		t.Errorf("the client did not ask for mode 2026: %q", out.String())
	}
	if c.screen.SynchronizedUpdates() {
		t.Error("frames go as synchronized updates before the terminal said it has them")
	}
}

// A terminal without mode 2026 shows the cursor moving to every cell a frame changes, so the
// cursor is hidden while drawing until the terminal says it has the mode.
func TestTheTerminalsAnswerChoosesHowFramesAreSent(t *testing.T) {
	for value, synchronized := range map[ansi.ModeSetting]bool{
		ansi.ModeReset: true, ansi.ModeSet: true, ansi.ModePermanentlySet: true,
		ansi.ModeNotRecognized: false, ansi.ModePermanentlyReset: false,
	} {
		a := testApp(t)
		a.screen = uv.NewTerminalScreen(&bytes.Buffer{}, uv.Environ{"TERM=xterm-256color"})
		if err := a.handleEvent(uv.ModeReportEvent{Mode: ansi.ModeSynchronizedOutput, Value: value}); err != nil {
			t.Fatal(err)
		}
		if got := a.screen.SynchronizedUpdates(); got != synchronized {
			t.Errorf("answer %v: synchronized updates %v, want %v", value, got, synchronized)
		}
	}
}
