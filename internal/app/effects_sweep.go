package app

import (
	"image"
	"image/color"
	"math"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// slide moves the content in from one edge and, closing, back out to it, like a drawer; shine
// sweeps a bright band across the text. Both draw the cells where they are at the moment and
// never resize a program.

// slideDirection is where a slide moves the content as it opens: 1 to the right, -1 to the left.
func (e effect) slideDirection() (int, bool) {
	switch e {
	case effectSlideRight:
		return 1, true
	case effectSlideLeft:
		return -1, true
	}
	return 0, false
}

// slideIn draws area's content moving dir as p goes from 0 to 1, from beyond the edge it comes
// in from to its place; where the content has not come yet, blank shows.
func slideIn(screen uv.Screen, area image.Rectangle, p float64, dir int, blank uv.Cell) {
	shift := int(math.Round((1 - p) * float64(area.Dx())))
	if shift <= 0 {
		return
	}
	row := make([]uv.Cell, area.Dx())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for i := range row {
			row[i] = uv.Cell{Content: " ", Width: 1}
			if c := screen.CellAt(area.Min.X+i, y); c != nil {
				row[i] = *c
			}
		}
		for i := range row {
			from := i + dir*shift // the cell that is at i now
			cell := blank
			if from >= 0 && from < len(row) {
				cell = row[from]
			}
			screen.SetCell(area.Min.X+i, y, &cell)
		}
	}
}

// slideOut draws a closing pane's cells going back out the edge they came in from as p goes on,
// what is below showing where they were.
func slideOut(screen uv.Screen, an *animation, p float64, dir int) {
	shift := int(math.Round(p * p * float64(an.area.Dx()))) // slow to start, as a pane let go
	for y := an.area.Min.Y; y < an.area.Max.Y; y++ {
		for x := an.area.Min.X; x < an.area.Max.X; x++ {
			from := x + dir*shift
			if from < an.area.Min.X || from >= an.area.Max.X {
				continue
			}
			if old := an.cells.CellAt(from, y); old != nil {
				cell := *old
				screen.SetCell(x, y, &cell)
			}
		}
	}
}

// shineWidth is how much of the area the bright band covers.
const shineWidth = 0.3

// shine lights the text under a band that crosses area from left to right as p goes on: toward
// white on a dark background, toward black on a light one, each cell back to its own color once
// the band is past.
func (a *App) shine(screen uv.Screen, area image.Rectangle, p float64, only func(x, y int) bool) {
	light := towardContrast(a.backgroundColor())
	center := -shineWidth + p*(1+2*shineWidth)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			c := screen.CellAt(x, y)
			if c == nil || c.Style.Fg == nil || only != nil && !only(x, y) {
				continue
			}
			across := (float64(x-area.Min.X) + 0.5) / float64(max(area.Dx(), 1))
			weight := 1 - math.Abs(across-center)/shineWidth
			if weight <= 0 {
				continue
			}
			cell := *c
			cell.Style.Fg = mix(cell.Style.Fg, light, 0.75*weight)
			screen.SetCell(x, y, &cell)
		}
	}
}

// towardContrast is the color that stands out most on background: white, or black on a light one.
func towardContrast(background color.Color) color.Color {
	if background == nil {
		return color.White
	}
	r, g, b, _ := background.RGBA()
	if 0.2126*float64(r)+0.7152*float64(g)+0.0722*float64(b) > 0.5*0xffff {
		return color.Black
	}
	return color.White
}
