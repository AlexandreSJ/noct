package vt

import "testing"

// noct: the main screen can be read while a full-screen program shows the alternate one.
func TestMainScreenCellAtReadsUnderTheAlternateScreen(t *testing.T) {
	term := NewEmulator(10, 2)
	_, _ = term.Write([]byte("shell\x1b[?1049h\x1b[Hfull"))
	if !term.IsAltScreen() {
		t.Fatal("not on the alternate screen")
	}
	if got := term.CellAt(0, 0).Content; got != "f" {
		t.Errorf("the alternate screen starts with %q, want f", got)
	}
	if got := term.MainScreenCellAt(0, 0).Content; got != "s" {
		t.Errorf("the main screen starts with %q, want s", got)
	}
}
