package vt

import (
	"strings"
	"testing"
)

// noct: scroll margins may lie past the screen, as when a program read the size before a
// resize and set a bottom margin below the last row. Scrolling inside them moves only the rows
// the screen has, instead of indexing past the buffer and stopping the daemon.
func TestMarginsPastTheScreenDoNotPanic(t *testing.T) {
	cases := []struct{ name, input string }{
		{"reverse index", "\x1b[1;19r\x1bM"},
		{"scroll down", "\x1b[1;19r\x1b[15T"},
		{"scroll up", "\x1b[1;19r\x1b[15S"},
		{"insert line", "\x1b[1;19r\x1b[5L"},
		{"delete line", "\x1b[1;19r\x1b[5M"},
		{"newline at the bottom", "\x1b[1;19r\x1b[16H\n\n\n"},
		{"insert and delete characters", "\x1b[?69h\x1b[1;30s\x1b[5@\x1b[5P"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			term := newTestTerminal(t, 20, 16)
			_, _ = term.Write([]byte("top\x1b[16;1Hbottom\x1b[H"))
			_, _ = term.Write([]byte(c.input))
		})
	}
}

// noct: a reverse index at the top of margins past the screen pushes the rows down and drops
// the last one, as with margins inside it.
func TestReverseIndexWithMarginsPastTheScreen(t *testing.T) {
	term := newTestTerminal(t, 10, 3)
	_, _ = term.Write([]byte("a\r\nb\r\nc\x1b[1;5r\x1b[H\x1bM"))
	want := []string{"", "a", "b"}
	for y, line := range want {
		if got := term.scr.buf.Line(y).String(); strings.TrimRight(got, " ") != line {
			t.Errorf("row %d = %q, want %q", y, got, line)
		}
	}
}
