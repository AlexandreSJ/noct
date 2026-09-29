package vt

import "testing"

// noct: Home and End follow application cursor keys mode (DECCKM), like the arrows.
func TestHomeAndEndFollowApplicationCursorKeys(t *testing.T) {
	cases := []struct {
		name, mode string
		key        rune
		want       string
	}{
		{"home, normal", "\x1b[?1l", KeyHome, "\x1b[H"},
		{"end, normal", "\x1b[?1l", KeyEnd, "\x1b[F"},
		{"home, application", "\x1b[?1h", KeyHome, "\x1bOH"},
		{"end, application", "\x1b[?1h", KeyEnd, "\x1bOF"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			term := newTestTerminal(t, 10, 2)
			_, _ = term.Write([]byte(c.mode))
			go term.SendKey(KeyPressEvent{Code: c.key})
			buf := make([]byte, 16)
			n, _ := term.Read(buf)
			if got := string(buf[:n]); got != c.want {
				t.Errorf("sent %q, want %q", got, c.want)
			}
		})
	}
}
