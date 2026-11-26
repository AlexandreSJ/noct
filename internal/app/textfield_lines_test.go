package app

import "testing"

func TestAMultilineFieldMovesThroughItsLines(t *testing.T) {
	f := newTextField("")
	f.multiline = true
	f.insert("ok and\n  ready\nx")
	if f.String() != "ok and\n  ready\nx" || f.cursorLine() != 2 || f.lineCount() != 3 {
		t.Fatalf("text %q, line %d of %d", f.String(), f.cursorLine(), f.lineCount())
	}
	f.cursor = f.posOfLine(1, 4) // in ready
	if !f.moveLine(-1) || f.cursor != 4 {
		t.Errorf("up from ready: cursor %d", f.cursor)
	}
	if f.moveLine(-1) {
		t.Error("up past the first line moved")
	}
	f.cursor = f.posOfLine(1, 9)
	if f.cursor != len("ok and\n  ready") || f.lineStart() != 7 || f.lineEnd() != 14 {
		t.Errorf("end of ready: cursor %d, line %d-%d", f.cursor, f.lineStart(), f.lineEnd())
	}
	single := newTextField("")
	single.insert("a\nb")
	if single.String() != "a b" {
		t.Errorf("a field of one line took %q", single.String())
	}
}
