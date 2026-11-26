package app

import (
	"image"
	"image/color"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

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

func TestAnEscapeShowsInTheDimmedPrimaryColor(t *testing.T) {
	pal := &palette{
		selectedRow: uv.Style{Bg: color.RGBA{R: 0xd0, G: 0x80, B: 0x20, A: 0xff}},
		panelMuted:  uv.Style{Fg: color.RGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xff}},
	}
	buffer := uv.NewScreenBuffer(20, 1)
	rect := image.Rect(0, 0, 20, 1)
	drawTextCut(buffer, 0, 0, 20, `a\nb`, pal.panelText)
	markEscapes(buffer, image.Pt(0, 0), rect, `a\nb`, pal)
	want := shadeStyle(pal, conditionShades/2).Fg
	for x, marked := range []bool{false, true, true, false} {
		if got := buffer.CellAt(x, 0).Style.Fg; (got == want) != marked {
			t.Errorf("cell %d: %v, marked %v", x, got, marked)
		}
	}
}
