package vt

import "testing"

// noct: in UTF-8, 0x9C is a continuation byte ("✳" is E2 9C B3), not the C1 string terminator.
// Ending the OSC there cut Claude Code's title to "\xe2" and printed the rest into the pane.
func TestTitleKeepsCharactersWithAC1ByteInTheirUTF8(t *testing.T) {
	term := newTestTerminal(t, 20, 2)
	var title string
	term.SetCallbacks(Callbacks{Title: func(s string) { title = s }})

	_, _ = term.Write([]byte("\x1b]0;✳ Claude ✅\x07"))

	if title != "✳ Claude ✅" {
		t.Errorf("title = %q, want %q", title, "✳ Claude ✅")
	}
	if cell := term.CellAt(0, 0); cell != nil && cell.Content != " " && cell.Content != "" {
		t.Errorf("the title leaked into the screen: cell (0,0) = %q", cell.Content)
	}
}

func TestHyperlinkKeepsCharactersWithAC1ByteInTheirUTF8(t *testing.T) {
	term := newTestTerminal(t, 20, 2)
	_, _ = term.Write([]byte("\x1b]8;;file:///tmp/✳\x1b\\X\x1b]8;;\x1b\\"))

	cell := term.CellAt(0, 0)
	if cell == nil || cell.Content != "X" || cell.Link.URL != "file:///tmp/✳" {
		t.Errorf("cell (0,0) = %+v, want X linking to file:///tmp/✳", cell)
	}
}
