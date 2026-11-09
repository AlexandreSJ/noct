package app

import (
	"image"
	"image/color"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

var (
	closedColor = color.RGBA{R: 200, A: 0xff}
	belowColor  = color.RGBA{B: 200, A: 0xff}
)

// closingOver is a closing animation of effect e over area, whose cells showed "x" on red, drawn
// at progress p over a screen that shows "y" on blue there.
func closingOver(t *testing.T, e effect, area image.Rectangle, p float64) uv.ScreenBuffer {
	t.Helper()
	a, _ := animatedApp(t)
	paint := func(content string, bg color.Color) uv.ScreenBuffer {
		buf := uv.NewScreenBuffer(a.size.X, a.size.Y)
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				buf.SetCell(x, y, &uv.Cell{Content: content, Width: 1, Style: uv.Style{Fg: color.White, Bg: bg}})
			}
		}
		return buf
	}
	an := &animation{moment: momentPanels, effect: e, closing: true, area: area, cells: paint("x", closedColor), seed: 3}
	screen := paint("y", belowColor)
	a.drawClosing(&screen, an, p, time.Now())
	return screen
}

func countContent(buf uv.ScreenBuffer, area image.Rectangle, content string) int {
	n := 0
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if c := buf.CellAt(x, y); c != nil && c.Content == content {
				n++
			}
		}
	}
	return n
}

func TestAClosingFadeCrossesIntoWhatIsBelow(t *testing.T) {
	area := image.Rect(2, 2, 12, 6)
	early := closingOver(t, effectFade, area, 0.1).CellAt(4, 3)
	if early.Content != "x" || colorsEqual(early.Style.Bg, belowColor) {
		t.Errorf("early in the closing the cell is %q on %v, want x near red", early.Content, early.Style.Bg)
	}
	middle := closingOver(t, effectFade, area, 0.5).CellAt(4, 3)
	if colorsEqual(middle.Style.Bg, closedColor) || colorsEqual(middle.Style.Bg, belowColor) {
		t.Errorf("halfway the background is %v, want a mix of red and blue", middle.Style.Bg)
	}
	late := closingOver(t, effectFade, area, 0.9).CellAt(4, 3)
	if late.Content != "y" {
		t.Errorf("late in the closing the cell shows %q, want what is below", late.Content)
	}
}

func TestAClosingEffectRemovesCellsBeforeItEnds(t *testing.T) {
	area := image.Rect(0, 1, 40, 9)
	for _, e := range []effect{effect01, effectJitter} {
		if below := countContent(closingOver(t, e, area, 0.05), area, "y"); below > area.Dx()*area.Dy()/4 {
			t.Errorf("effect %v: %d cells already gone at the start", e, below)
		}
		if below := countContent(closingOver(t, e, area, 0.9), area, "y"); below < area.Dx()*area.Dy()/2 {
			t.Errorf("effect %v: only %d cells gone near the end", e, below)
		}
	}
}

func TestFadingInMixesTheBackgroundToo(t *testing.T) {
	a, _ := animatedApp(t)
	area := image.Rect(1, 1, 5, 3)
	screen := uv.NewScreenBuffer(a.size.X, a.size.Y)
	screen.SetCell(2, 2, &uv.Cell{Content: "z", Width: 1, Style: uv.Style{Fg: color.White, Bg: closedColor}})
	an := &animation{moment: momentPanels, effect: effectFade}
	a.applyEffect(&screen, an, area, 0.3, time.Now(), nil)
	if bg := screen.CellAt(2, 2).Style.Bg; colorsEqual(bg, closedColor) {
		t.Errorf("a cell fading in has its background %v already", bg)
	}
}
