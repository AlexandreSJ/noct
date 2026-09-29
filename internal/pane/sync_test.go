package pane

import (
	"strings"
	"testing"
	"time"
)

func TestSyncGateHoldsAFrameUntilItEnds(t *testing.T) {
	const begin, end = "\x1b[?2026h", "\x1b[?2026l"
	tests := []struct {
		name   string
		chunks []string
		want   []string // what each chunk lets through
	}{
		{"output outside a frame passes", []string{"hello"}, []string{"hello"}},
		{"a whole frame passes at once", []string{"a" + begin + "b" + end + "c"}, []string{"a" + begin + "b" + end + "c"}},
		{"a frame cut in two waits for its end", []string{"a" + begin + "b", "c" + end + "d"}, []string{"a", begin + "bc" + end + "d"}},
		{"an end marker cut in two", []string{begin + "b\x1b[?20", "26l"}, []string{"", begin + "b" + end}},
		{"a begin marker cut in two", []string{"a\x1b[?20", "26hb" + end}, []string{"a", begin + "b" + end}},
		{"only the frame that is open waits", []string{begin + "1" + end + begin + "2"}, []string{begin + "1" + end}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g syncGate
			for i, chunk := range tt.chunks {
				if got := string(g.feed([]byte(chunk))); got != tt.want[i] {
					t.Errorf("chunk %d let %q through, want %q", i, got, tt.want[i])
				}
			}
		})
	}
}

func TestSyncGateGivesUpOnAFrameTooLarge(t *testing.T) {
	var g syncGate
	g.feed([]byte("\x1b[?2026h"))
	if got := g.feed(make([]byte, maxHeldFrame+1)); len(got) != maxHeldFrame+1+len("\x1b[?2026h") {
		t.Errorf("let %d bytes through, want the whole frame", len(got))
	}
	if g.pending() {
		t.Error("still holding output")
	}
}

// A program that begins a frame and never ends it shows what it drew after syncTimeout.
func TestPaneShowsAnUnfinishedFrameAfterTheTimeout(t *testing.T) {
	p, _ := startTestPane(t, `printf '\033[?2026hdrawn'; sleep 5`)
	deadline := time.Now().Add(testTimeout)
	for !strings.Contains(screenText(p), "drawn") {
		if time.Now().After(deadline) {
			t.Fatal("the unfinished frame never showed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The cursor the program hides while drawing a frame is never seen hidden from outside it.
func TestPaneNeverShowsTheCursorHiddenInsideAFrame(t *testing.T) {
	p, exited := startTestPane(t, `i=0; while [ $i -lt 200 ]; do printf '\033[?2026h\033[?25l\033[1;1H%s' $i; printf '\033[2;1H\033[?25h\033[?2026l'; i=$((i+1)); done`)
	for {
		select {
		case <-exited:
			return
		default:
		}
		if _, visible := p.Cursor(); !visible {
			t.Fatal("cursor seen hidden in the middle of a frame")
		}
	}
}
