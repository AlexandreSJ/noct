package main

import (
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/charmbracelet/x/term"
)

// Keys a masked answer reads as it is typed.
const (
	keyInterrupt = 0x03 // ctrl+c
	keyEOF       = 0x04 // ctrl+d
	keyBackspace = 0x08 // ctrl+h
	keyKill      = 0x15 // ctrl+u
	keyEscape    = 0x1b
	keyDelete    = 0x7f
)

// mask stands for each character of a secret answer, as the forms show secrets.
const mask = "•"

// terminalAskSecret asks on the terminal and reads the answer with each character shown as •,
// for a secret prompt; without a terminal there is nobody to ask.
func terminalAskSecret(in *os.File, out io.Writer) func(string) string {
	if !term.IsTerminal(in.Fd()) {
		return nil
	}
	return func(question string) string {
		fmt.Fprint(out, question)
		state, err := term.MakeRaw(in.Fd())
		if err != nil {
			return ""
		}
		answer, interrupted := readMasked(in, out)
		_ = term.Restore(in.Fd(), state)
		fmt.Fprintln(out)
		if interrupted {
			// Raw mode takes ctrl+c as a key: it stops the run as it does anywhere else.
			if self, err := os.FindProcess(os.Getpid()); err == nil {
				_ = self.Signal(os.Interrupt)
			}
		}
		return answer
	}
}

// readMasked reads a line from a terminal in raw mode, writing a mask for each character typed:
// backspace takes the last one back, ctrl+u all of them, and a key's escape sequence is dropped.
// It tells whether ctrl+c ended it.
func readMasked(in io.Reader, out io.Writer) (string, bool) {
	line := maskedLine{out: out}
	b := make([]byte, 1)
	for {
		if n, err := in.Read(b); n == 0 || err != nil {
			return string(line.text), false
		}
		if done, interrupted := line.take(b[0]); done {
			return string(line.text), interrupted
		}
	}
}

// maskedLine is a secret answer as it is typed, shown as a mask a character.
type maskedLine struct {
	text []byte
	out  io.Writer
	// escaped is true inside a key's escape sequence, as an arrow's.
	escaped bool
}

// take is one byte typed: it tells whether the line is done, and whether by ctrl+c, which
// leaves it empty.
func (l *maskedLine) take(c byte) (done, interrupted bool) {
	switch {
	case l.escaped:
		// A sequence ends at its final byte, a letter or ~; ESC [ and its parameters come first.
		l.escaped = c == '[' || c == 'O' || c >= '0' && c <= '9' || c == ';'
	case c == '\r' || c == '\n' || c == keyEOF:
		return true, false
	case c == keyInterrupt:
		l.text = nil
		return true, true
	case c == keyEscape:
		l.escaped = true
	case c == keyDelete || c == keyBackspace:
		l.erase(1)
	case c == keyKill:
		l.erase(utf8.RuneCount(l.text))
	case c >= 0x20:
		l.text = append(l.text, c)
		if utf8.RuneStart(c) {
			fmt.Fprint(l.out, mask)
		}
	}
	return false, false
}

// erase takes back the last n characters and their masks.
func (l *maskedLine) erase(n int) {
	for ; n > 0 && len(l.text) > 0; n-- {
		_, size := utf8.DecodeLastRune(l.text)
		l.text = l.text[:len(l.text)-size]
		fmt.Fprint(l.out, "\b \b")
	}
}
