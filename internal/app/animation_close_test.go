package app

import (
	"image"
	"image/color"
	"math"
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
	late := closingOver(t, effectFade, area, 0.9).CellAt(4, 3)
	if late.Content != "y" || !colorsEqual(late.Style.Bg, belowColor) {
		t.Errorf("late in the closing the cell shows %q on %v, want what is below on its own background", late.Content, late.Style.Bg)
	}
	if colorsEqual(late.Style.Fg, color.White) {
		t.Error("late in the closing the text below is in its full color already")
	}
}

func TestAClosingFadeLeavesNoBackgroundOverATransparentOne(t *testing.T) {
	a, _ := animatedApp(t)
	area := image.Rect(2, 2, 12, 6)
	old := uv.NewScreenBuffer(a.size.X, a.size.Y)
	old.SetCell(4, 3, &uv.Cell{Content: "x", Width: 1, Style: uv.Style{Fg: color.White, Bg: closedColor}})
	screen := uv.NewScreenBuffer(a.size.X, a.size.Y)
	screen.SetCell(4, 3, &uv.Cell{Content: "y", Width: 1, Style: uv.Style{Fg: color.White}})
	an := &animation{moment: momentPanels, effect: effectFade, closing: true, area: area, cells: old}
	a.drawClosing(&screen, an, 0.9, time.Now())
	if bg := screen.CellAt(4, 3).Style.Bg; bg != nil {
		t.Errorf("over a transparent background the closing cell is on %v, want none", bg)
	}
}

func TestAClosingEffectRemovesCellsBeforeItEnds(t *testing.T) {
	area := image.Rect(0, 1, 40, 9)
	for _, e := range []effect{effect01, effectJitter, effectDissolve, effectWipe, effectGrow, effectShine} {
		if below := countContent(closingOver(t, e, area, 0.05), area, "y"); below > area.Dx()*area.Dy()/4 {
			t.Errorf("effect %v: %d cells already gone at the start", e, below)
		}
		if below := countContent(closingOver(t, e, area, 0.9), area, "y"); below < area.Dx()*area.Dy()/2 {
			t.Errorf("effect %v: only %d cells gone near the end", e, below)
		}
	}
}

func TestFadingNeverChangesTheBackground(t *testing.T) {
	a, _ := animatedApp(t)
	area := image.Rect(1, 1, 5, 3)
	screen := uv.NewScreenBuffer(a.size.X, a.size.Y)
	screen.SetCell(2, 2, &uv.Cell{Content: "z", Width: 1, Style: uv.Style{Fg: color.White, Bg: closedColor}})
	an := &animation{moment: momentPanels, effect: effectFade}
	a.applyEffect(&screen, an, area, 0.3, time.Now(), nil)
	if bg := screen.CellAt(2, 2).Style.Bg; !colorsEqual(bg, closedColor) {
		t.Errorf("a cell fading in is on %v, want its own background", bg)
	}
}

// openingOver is effect e opening over area, whose cells show "x" in red, drawn at progress p,
// with whether each cell already has its own color.
func openingOver(t *testing.T, e effect, area image.Rectangle, p float64) func(x, y int) bool {
	t.Helper()
	a, _ := animatedApp(t)
	screen := uv.NewScreenBuffer(a.size.X, a.size.Y)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			screen.SetCell(x, y, &uv.Cell{Content: "x", Width: 1, Style: uv.Style{Fg: closedColor}})
		}
	}
	a.applyEffect(&screen, &animation{moment: momentPanels, effect: e, seed: 5}, area, p, time.Now(), nil)
	return func(x, y int) bool { return colorsEqual(screen.CellAt(x, y).Style.Fg, closedColor) }
}

func TestDissolveShowsCellsInARandomOrder(t *testing.T) {
	area := image.Rect(0, 1, 40, 9)
	shown := openingOver(t, effectDissolve, area, 0.5)
	left, right := 0, 0
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			switch {
			case !shown(x, y):
			case x < 20:
				left++
			default:
				right++
			}
		}
	}
	half := area.Dx() * area.Dy() / 2
	if left+right < half/2 || left+right > half*3/2 || left == 0 || right == 0 {
		t.Errorf("halfway %d cells shown on the left and %d on the right, want about half spread over both", left, right)
	}
}

func TestWipeCrossesTheAreaFromOneSide(t *testing.T) {
	area := image.Rect(0, 1, 40, 9)
	shown := openingOver(t, effectWipe, area, 0.5)
	if !shown(area.Min.X, 3) || shown(area.Max.X-1, 3) {
		t.Errorf("halfway the left edge shows %v and the right edge %v, want only the left", shown(area.Min.X, 3), shown(area.Max.X-1, 3))
	}
	if done := openingOver(t, effectWipe, area, 1); !done(area.Max.X-1, 3) {
		t.Error("at the end the right edge still lacks its color")
	}
}

func TestGrowSpreadsFromTheCenter(t *testing.T) {
	area := image.Rect(0, 1, 40, 9)
	center := image.Pt(20, 5)
	shown := openingOver(t, effectGrow, area, 0.5)
	if !shown(center.X, center.Y) || shown(area.Min.X, area.Min.Y) || shown(area.Max.X-1, area.Max.Y-1) {
		t.Errorf("halfway the center shows %v and the corners %v and %v, want only the center", shown(center.X, center.Y), shown(area.Min.X, area.Min.Y), shown(area.Max.X-1, area.Max.Y-1))
	}
	if done := openingOver(t, effectGrow, area, 1); !done(area.Min.X, area.Min.Y) {
		t.Error("at the end a corner still lacks its color")
	}
}

func TestGrowClosesTowardTheCenter(t *testing.T) {
	area := image.Rect(0, 1, 40, 9)
	screen := closingOver(t, effectGrow, area, 0.5)
	if c := screen.CellAt(20, 5); c == nil || c.Content == "y" {
		t.Error("halfway through closing the center is gone, want it kept to the last")
	}
	if c := screen.CellAt(area.Min.X, area.Min.Y); c == nil || c.Content != "y" {
		t.Error("halfway through closing a corner is still there, want it gone first")
	}
}

func TestSlideRightBringsTheContentInFromTheLeft(t *testing.T) {
	a, _ := animatedApp(t)
	area := image.Rect(0, 1, 40, 3)
	screen := uv.NewScreenBuffer(a.size.X, a.size.Y)
	for x := area.Min.X; x < area.Max.X; x++ {
		screen.SetCell(x, 1, &uv.Cell{Content: string(rune('a' + x%26)), Width: 1})
	}
	a.applyEffect(&screen, &animation{moment: momentPanels, effect: effectSlideRight}, area, 0.5, time.Now(), nil)
	shift := int(math.Round((1 - easeOut(0.5)) * float64(area.Dx())))
	if c := screen.CellAt(area.Max.X-shift, 1); c == nil || c.Content != " " {
		t.Errorf("halfway, the right part shows %v, want it still empty", c)
	}
	if c := screen.CellAt(area.Max.X-shift-1, 1); c == nil || c.Content != string(rune('a'+(area.Max.X-1)%26)) {
		t.Errorf("halfway, the content's last cell is not where it got to: %v", c)
	}
	done := uv.NewScreenBuffer(a.size.X, a.size.Y)
	done.SetCell(5, 1, &uv.Cell{Content: "q", Width: 1})
	a.applyEffect(&done, &animation{moment: momentPanels, effect: effectSlideRight}, area, 1, time.Now(), nil)
	if c := done.CellAt(5, 1); c == nil || c.Content != "q" {
		t.Errorf("at the end the content moved: %v", c)
	}
}

func TestSlideRightClosesBackOutToTheLeft(t *testing.T) {
	area := image.Rect(0, 1, 40, 9)
	if below := countContent(closingOver(t, effectSlideRight, area, 0.05), area, "y"); below > area.Dx()*area.Dy()/4 {
		t.Errorf("%d cells already gone at the start", below)
	}
	screen := closingOver(t, effectSlideRight, area, 0.5)
	if c := screen.CellAt(area.Max.X-1, 5); c == nil || c.Content != "y" {
		t.Errorf("halfway, the right edge shows %v, want what is below", c)
	}
	if c := screen.CellAt(area.Min.X, 5); c == nil || c.Content != "x" {
		t.Errorf("halfway, the left edge shows %v, want the closing pane", c)
	}
}

func TestSlideLeftComesInFromTheRightAndGoesBack(t *testing.T) {
	a, _ := animatedApp(t)
	area := image.Rect(0, 1, 40, 3)
	screen := uv.NewScreenBuffer(a.size.X, a.size.Y)
	for x := area.Min.X; x < area.Max.X; x++ {
		screen.SetCell(x, 1, &uv.Cell{Content: string(rune('a' + x%26)), Width: 1})
	}
	a.applyEffect(&screen, &animation{moment: momentPanels, effect: effectSlideLeft}, area, 0.5, time.Now(), nil)
	shift := int(math.Round((1 - easeOut(0.5)) * float64(area.Dx())))
	if c := screen.CellAt(area.Min.X, 1); c == nil || c.Content != " " {
		t.Errorf("halfway, the left edge shows %v, want it still empty", c)
	}
	if c := screen.CellAt(area.Min.X+shift, 1); c == nil || c.Content != "a" {
		t.Errorf("halfway, the content's first cell is not where it got to: %v", c)
	}
	closing := closingOver(t, effectSlideLeft, image.Rect(0, 1, 40, 9), 0.5)
	if c := closing.CellAt(0, 5); c == nil || c.Content != "y" {
		t.Errorf("halfway through closing, the left edge shows %v, want what is below", c)
	}
	if c := closing.CellAt(39, 5); c == nil || c.Content != "x" {
		t.Errorf("halfway through closing, the right edge shows %v, want the pane going out", c)
	}
}

func TestShineCrossesTheTextAndLeavesItAsItWas(t *testing.T) {
	a, _ := animatedApp(t)
	area := image.Rect(0, 1, 40, 2)
	paint := func() uv.ScreenBuffer {
		buf := uv.NewScreenBuffer(a.size.X, a.size.Y)
		for x := area.Min.X; x < area.Max.X; x++ {
			buf.SetCell(x, 1, &uv.Cell{Content: "t", Width: 1, Style: uv.Style{Fg: closedColor}})
		}
		return buf
	}
	an := &animation{moment: momentHover, effect: effectShine}
	halfway := paint()
	a.applyEffect(&halfway, an, area, 0.5, time.Now(), nil)
	middle, edge := halfway.CellAt(20, 1).Style.Fg, halfway.CellAt(0, 1).Style.Fg
	if colorsEqual(middle, closedColor) || !colorsEqual(edge, closedColor) {
		t.Errorf("halfway the middle is %v and the edge %v, want only the middle lit", middle, edge)
	}
	done := paint()
	a.applyEffect(&done, an, area, 1, time.Now(), nil)
	for x := area.Min.X; x < area.Max.X; x++ {
		if !colorsEqual(done.CellAt(x, 1).Style.Fg, closedColor) {
			t.Fatalf("after the shine, cell %d is %v, want its own color", x, done.CellAt(x, 1).Style.Fg)
		}
	}
}

func TestShineGoesTowardTheColorThatStandsOut(t *testing.T) {
	if towardContrast(color.RGBA{0x1a, 0x1b, 0x26, 0xff}) != color.White || towardContrast(color.RGBA{0xf5, 0xf5, 0xf0, 0xff}) != color.Black {
		t.Error("a dark background does not shine white, or a light one black")
	}
}
