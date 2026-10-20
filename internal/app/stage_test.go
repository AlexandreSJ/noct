package app

import (
	"image"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/theme"
)

func stageBuffer(th theme.Theme, area image.Rectangle, lit theme.Component, blink bool) uv.ScreenBuffer {
	buf := uv.NewScreenBuffer(area.Max.X, area.Max.Y)
	p := newPalette(th, true)
	drawStage(buf, area, &p, lit, blink)
	return buf
}

func sameCells(a, b uv.ScreenBuffer, area image.Rectangle) bool {
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			ca, cb := a.CellAt(x, y), b.CellAt(x, y)
			if (ca == nil) != (cb == nil) {
				return false
			}
			if ca != nil && (ca.Content != cb.Content || !ca.Style.Equal(&cb.Style)) {
				return false
			}
		}
	}
	return true
}

func TestTheStageShowsEveryComponent(t *testing.T) {
	area := image.Rect(0, 0, 40, 14)
	noct := theme.Builtins()[0]
	before := stageBuffer(noct, area, theme.Background, false)
	for c := range theme.Count {
		changed := noct
		changed.Colors[c] = "#123456"
		if sameCells(before, stageBuffer(changed, area, theme.Background, false), area) {
			t.Errorf("the stage does not show %s", theme.Components[c].Label)
		}
	}
}

func TestTheStageBlinksTheLitComponent(t *testing.T) {
	area := image.Rect(0, 0, 40, 14)
	noct := theme.Builtins()[0]
	border := func(buf uv.ScreenBuffer) *uv.Cell { return buf.CellAt(0, 5) } // the left panel's left side
	if c := border(stageBuffer(noct, area, theme.FocusedBorder, true)); c == nil || c.Content != "│" || c.Style.Attrs&uv.AttrReverse == 0 {
		t.Errorf("the lit focused border is %+v, want it reversed", c)
	}
	if c := border(stageBuffer(noct, area, theme.FocusedBorder, false)); c == nil || c.Style.Attrs&uv.AttrReverse != 0 {
		t.Errorf("the focused border is %+v without the blink, want it plain", c)
	}
}

func TestATinyStageDrawsNothing(t *testing.T) {
	area := image.Rect(0, 0, 20, 8)
	empty := uv.NewScreenBuffer(20, 8)
	if !sameCells(empty, stageBuffer(theme.Builtins()[0], area, theme.Background, false), area) {
		t.Error("a stage too small to read was drawn")
	}
}
