package app

import (
	"image"
	"image/color"
	"strings"
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
	f := newTextField(`a\nb`)
	f.paint = escapePaint(pal)
	drawTextCut(buffer, 0, 0, 20, `a\nb`, pal.panelText)
	f.paintLine(buffer, image.Pt(0, 0), 20, `a\nb`, 0)
	want := shadeStyle(pal, conditionShades/2).Fg
	for x, marked := range []bool{false, true, true, false} {
		if got := buffer.CellAt(x, 0).Style.Fg; (got == want) != marked {
			t.Errorf("cell %d: %v, marked %v", x, got, marked)
		}
	}
}

func TestAFieldScrollsToTheCursorWithAnEllipsisBefore(t *testing.T) {
	buffer := uv.NewScreenBuffer(10, 1)
	f := newTextField("0123456789abcdef")
	f.draw(buffer, 0, 0, 10, false, uv.Style{}, uv.Style{})
	if got := buffer.Render(); !strings.HasPrefix(got, "…") || !strings.Contains(got, "ef") {
		t.Errorf("the end of the text with the cursor there: %q", got)
	}
	f.multiline = true
	f.insert("\nshort")
	f.cursor = f.posOfLine(0, 16)
	lines := uv.NewScreenBuffer(10, 2)
	f.drawLines(lines, image.Rect(0, 0, 10, 2), uv.Style{}, uv.Style{})
	if got := lines.Render(); !strings.HasPrefix(got, "…") || !strings.Contains(got, "ef") {
		t.Errorf("a long line with the cursor at its end: %q", got)
	}
}

func TestAConditionsLogicIsColoredAsItIsTyped(t *testing.T) {
	pal := &palette{selectedRow: uv.Style{Bg: color.RGBA{R: 0xd0, A: 0xff}}}
	colors := logicPaint(pal)("a and (b)")
	for i, marked := range []bool{false, false, true, true, true, false, true, false, true} {
		if _, ok := colors[i]; ok != marked {
			t.Errorf("rune %d colored %v", i, ok)
		}
	}
}

// Moving the cursor back near the end of a long text keeps the text where it was, with what
// follows the cursor still in sight.
func TestAFieldKeepsItsScrollAsTheCursorMovesBack(t *testing.T) {
	buffer := uv.NewScreenBuffer(10, 1)
	f := newTextField("0123456789abcdef")
	f.draw(buffer, 0, 0, 10, false, uv.Style{}, uv.Style{})
	shown := f.shown
	f.cursor -= 3
	f.draw(buffer, 0, 0, 10, false, uv.Style{}, uv.Style{})
	if f.shown != shown || !strings.Contains(buffer.Render(), "def") {
		t.Errorf("shown %d, was %d: %q", f.shown, shown, buffer.Render())
	}
	f.cursor = 2
	f.draw(buffer, 0, 0, 10, false, uv.Style{}, uv.Style{})
	if f.shown != 0 {
		t.Errorf("near the start it shows from %d", f.shown)
	}
}
