package app

import (
	"image"
	"image/color"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/theme"
)

// colorsView prints a in red, b in color 9 on color 4, and c in the 256-color 196, as a program would.
type colorsView struct{ fakeView }

func (v *colorsView) Draw(screen uv.Screen, area uv.Rectangle) {
	screen.SetCell(area.Min.X, area.Min.Y, &uv.Cell{Content: "a", Width: 1, Style: uv.Style{Fg: ansi.Red}})
	screen.SetCell(area.Min.X+1, area.Min.Y, &uv.Cell{Content: "b", Width: 1, Style: uv.Style{Fg: ansi.IndexedColor(9), Bg: ansi.Blue}})
	screen.SetCell(area.Min.X+2, area.Min.Y, &uv.Cell{Content: "c", Width: 1, Style: uv.Style{Fg: ansi.IndexedColor(196)}})
}

func ansiApp(t *testing.T, paintBackground bool) (*App, image.Point) {
	t.Helper()
	a := testApp(t, newTab(1, &colorsView{}))
	th := theme.Builtins()[0]
	th.ANSI[1], th.ANSI[4], th.ANSI[9] = "#aa0000", "#0000aa", "#ff5555"
	a.palette = newPalette(th, paintBackground)
	frame, _ := a.currentTab().frameOf(1, a.workspaceArea())
	return a, contentRect(frame).Min
}

func TestTheThemesSixteenColorsColorPrograms(t *testing.T) {
	a, at := ansiApp(t, true)
	screen := drawnApp(a)
	want := []struct {
		fg, bg color.Color
	}{
		{color.RGBA{0xaa, 0, 0, 0xff}, nil},
		{color.RGBA{0xff, 0x55, 0x55, 0xff}, color.RGBA{0, 0, 0xaa, 0xff}},
		{ansi.IndexedColor(196), nil},
	}
	for i, w := range want {
		c := screen.CellAt(at.X+i, at.Y)
		if !colorsEqual(c.Style.Fg, w.fg) || w.bg != nil && !colorsEqual(c.Style.Bg, w.bg) {
			t.Errorf("cell %q: fg %v bg %v, want fg %v bg %v", c.Content, c.Style.Fg, c.Style.Bg, w.fg, w.bg)
		}
	}
}

func TestTheTerminalsBackgroundKeepsItsOwnColors(t *testing.T) {
	a, at := ansiApp(t, false)
	if c := drawnApp(a).CellAt(at.X, at.Y); c.Style.Fg != ansi.Red {
		t.Errorf("with the terminal's background, red drew as %v, want the terminal's own", c.Style.Fg)
	}
}
