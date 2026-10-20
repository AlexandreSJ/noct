package vt

import (
	"strings"
	"testing"
)

func row(term *Emulator, y int) string {
	return strings.TrimRight(term.scr.buf.Line(y).String(), " ")
}

// noct: a narrower terminal hides the end of each line instead of losing it, so making it wide
// again shows what was printed: logs and values the user still has to read.
func TestNarrowingKeepsTheEndOfEachLine(t *testing.T) {
	term := newTestTerminal(t, 30, 4)
	_, _ = term.Write([]byte("value: 1234567890 tail\r\nsecond line here"))
	term.Resize(8, 4)
	if got := row(term, 0); got != "value: 1" {
		t.Fatalf("narrow row 0 = %q", got)
	}
	term.Resize(30, 4)
	if got := row(term, 0); got != "value: 1234567890 tail" {
		t.Errorf("row 0 after widening = %q", got)
	}
	if got := row(term, 1); got != "second line here" {
		t.Errorf("row 1 after widening = %q", got)
	}
}

// noct: what a program clears while the terminal is narrow stays cleared when it widens.
func TestClearingWhileNarrowForgetsTheHiddenEnd(t *testing.T) {
	for name, input := range map[string]string{
		"erase screen": "\x1b[2J",
		"erase line":   "\x1b[1;1H\x1b[K",
	} {
		t.Run(name, func(t *testing.T) {
			term := newTestTerminal(t, 30, 4)
			_, _ = term.Write([]byte("value: 1234567890 tail"))
			term.Resize(8, 4)
			_, _ = term.Write([]byte(input))
			term.Resize(30, 4)
			if got := row(term, 0); got != "" {
				t.Errorf("row 0 after clearing and widening = %q", got)
			}
		})
	}
}

// noct: widening past where a line ever reached shows blank cells, not empty ones.
func TestWideningPastTheOldWidthIsBlank(t *testing.T) {
	term := newTestTerminal(t, 10, 2)
	_, _ = term.Write([]byte("abc"))
	term.Resize(5, 2)
	term.Resize(40, 2)
	line := term.scr.buf.Line(0)
	for x := 3; x < 40; x++ {
		if c := line.At(x); c == nil || c.Content != " " || c.Width != 1 {
			t.Fatalf("cell %d after widening is %+v, want a blank", x, c)
		}
	}
}
