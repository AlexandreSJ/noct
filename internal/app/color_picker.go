package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/theme"
)

// pickerRows are the lines under the grid: the selected color and the hex field.
const pickerRows = 2

// pickerHeight is the picker's height with nothing scrolled away; the panels it opens in are at
// least this tall where the terminal allows.
const pickerHeight = swatchLines + pickerRows

// swatchBlock is a block of the picker's cells, each row a line of colors, its top-left corner
// at x, y of the picker. A cell is two columns wide.
type swatchBlock struct {
	rows [][]uint8
	x, y int
}

// swatchHeading names a part of the picker on its line y.
type swatchHeading struct {
	y            int
	name, colors string
}

// The picker shows the terminal's 16 colors as normal and bright, one slice of the 6×6×6 cube as
// a triangle, and the 24 grays.
const (
	// cubeSlices are the cube's slices, by the sum of a color's levels of red, green and blue.
	cubeSlices = 16
	// cubeTop is the triangle's first line, and cubeX its left edge: 17 columns centered in 39.
	cubeTop, cubeX = 6, 11
	// swatchLines is how many lines the picker's cells and headings take.
	swatchLines = 16
)

// cubeLevels is n's levels of red, green and blue, each from 0 to 5.
func cubeLevels(n uint8) (red, green, blue int) {
	k := int(n) - 16
	return k / 36, k / 6 % 6, k % 6
}

func inCube(n uint8) bool {
	return n >= 16 && n < 232
}

func cubeColor(red, green, blue int) uint8 {
	return uint8(16 + 36*red + 6*green + blue)
}

// swatchBlocks are the picker's cells with the cube's slice shown. The slice is the colors whose
// levels add up to slice, a line for each level of red, the strongest on top, blue rising across.
// Each line starts half a cell further left than the one above, so that red, green and blue are
// the triangle's corners and the colors between them mix them.
func swatchBlocks(slice int) []swatchBlock {
	blocks := []swatchBlock{{rows: swatchRun(0, 2, 8), x: 0, y: 1}}
	for red := 5; red >= 0; red-- {
		var row []uint8
		first := -1
		for blue := range 6 {
			if green := slice - red - blue; green >= 0 && green <= 5 {
				if first < 0 {
					first = blue
				}
				row = append(row, cubeColor(red, green, blue))
			}
		}
		if row != nil {
			blocks = append(blocks, swatchBlock{rows: [][]uint8{row}, x: cubeX + 2*first + red, y: cubeTop + 5 - red})
		}
	}
	return append(blocks, swatchBlock{rows: swatchRun(232, 2, 12), x: 0, y: 14})
}

// sliceColors are the colors of the cube's slice, whose levels add up to slice.
func sliceColors(slice int) []uint8 {
	var colors []uint8
	for red := range 6 {
		for blue := range 6 {
			if green := slice - red - blue; green >= 0 && green <= 5 {
				colors = append(colors, cubeColor(red, green, blue))
			}
		}
	}
	return colors
}

// swatchCells is where each color shown with slice sits, at x, y of the picker.
func swatchCells(slice int) map[uint8]image.Point {
	cells := map[uint8]image.Point{}
	for _, b := range swatchBlocks(slice) {
		for y, row := range b.rows {
			for x, n := range row {
				cells[n] = image.Pt(b.x+2*x, b.y+y)
			}
		}
	}
	return cells
}

// sliceShades are the ruler of slices: each slice's colors mixed into one.
var sliceShades = func() (shades [cubeSlices]color.Color) {
	for slice := range shades {
		var sum [3]int
		colors := sliceColors(slice)
		for _, n := range colors {
			r, g, b, _ := theme.Indexed(n).Resolve().RGBA()
			sum[0], sum[1], sum[2] = sum[0]+int(r>>8), sum[1]+int(g>>8), sum[2]+int(b>>8)
		}
		count := len(colors)
		shades[slice] = color.RGBA{R: uint8(sum[0] / count), G: uint8(sum[1] / count), B: uint8(sum[2] / count), A: 0xff}
	}
	return shades
}()

// swatchRun is the colors from first in rows of perRow.
func swatchRun(first, rows, perRow int) [][]uint8 {
	run := make([][]uint8, rows)
	for y := range run {
		for x := range perRow {
			run[y] = append(run[y], uint8(first+y*perRow+x))
		}
	}
	return run
}

// closestInSlice is the color of slice to whose mix of red, green and blue is closest to that of
// n, a color of slice from: lighter or darker, a red stays red. Of two as close, the one with
// less red, then less blue, wins.
func closestInSlice(n uint8, from, to int) uint8 {
	red, green, blue := cubeLevels(n)
	want := [3]float64{float64(to) / 3, float64(to) / 3, float64(to) / 3}
	if from > 0 {
		scale := float64(to) / float64(from)
		want = [3]float64{float64(red) * scale, float64(green) * scale, float64(blue) * scale}
	}
	best, bestDistance := n, math.Inf(1)
	for _, c := range sliceColors(to) {
		r, g, b := cubeLevels(c)
		d := math.Pow(float64(r)-want[0], 2) + math.Pow(float64(g)-want[1], 2) + math.Pow(float64(b)-want[2], 2)
		if d < bestDistance-1e-9 {
			best, bestDistance = c, d
		}
	}
	return best
}

// render is the block's lines, its cells in their colors and the selected one marked.
func (b swatchBlock) render(selected int) []string {
	var lines []string
	for _, row := range b.rows {
		var cells []string
		for _, n := range row {
			cells = append(cells, swatch(n, int(n) == selected))
		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}
	return strings.Split(lipgloss.JoinVertical(lipgloss.Left, lines...), "\n")
}

// swatch is a cell in its color; the selected one is marked in black or white, whichever reads on it.
func swatch(n uint8, selected bool) string {
	c := theme.Indexed(n).Resolve()
	style := lipgloss.NewStyle().Background(c)
	if !selected {
		return style.Render("  ")
	}
	mark := lipgloss.Color("15")
	if luminance(c) > 0.18 {
		mark = lipgloss.Color("0")
	}
	return style.Foreground(mark).Bold(true).Render("[]")
}

// nextSwatch is the cell a move by dx or dy leaves n on: the nearest one that way on the same
// line, or on the nearest line that way the one closest to column, the left one of two as close.
// n stays at an edge.
func nextSwatch(cells map[uint8]image.Point, n uint8, dx, dy, column int) uint8 {
	from := cells[n]
	best, bestLine, bestAcross, found := n, 0, 0, false
	for c, at := range cells {
		line, across := (at.Y-from.Y)*dy, abs(at.X-column)
		if dy == 0 {
			line, across = (at.X-from.X)*dx, 0
			if at.Y != from.Y {
				continue
			}
		}
		if line <= 0 {
			continue
		}
		closer := line < bestLine || line == bestLine && (across < bestAcross || across == bestAcross && at.X < cells[best].X)
		if !found || closer {
			best, bestLine, bestAcross, found = c, line, across, true
		}
	}
	return best
}

// colorPicker picks one of the terminal's 256 colors, laid out by kind, or a typed #rrggbb.
type colorPicker struct {
	// current is the color the picker opened on, kept until the selection moves.
	current theme.Color
	moved   bool
	cell    uint8
	// slice is the cube's slice shown; a selection in the cube is in it.
	slice int
	// column is what moving up and down keeps close to, so that it goes straight through the
	// triangle's lines, each half a cell from the next; -1 until the first move.
	column int
	// first is the first line of cells shown when they do not fit.
	first  int
	hex    textField
	typing bool
	// problem says why the typed hex was not taken.
	problem string
	// emptyName is what t hands back "" for, such as the theme's own color; t does nothing
	// when it is "".
	emptyName string
	// done gets the color picked, or ok false when esc left it as it was.
	done func(c theme.Color, ok bool)
}

func newColorPicker(current theme.Color, emptyName string, done func(theme.Color, bool)) *colorPicker {
	p := &colorPicker{current: current, emptyName: emptyName, done: done, slice: 5, column: -1}
	if n, err := strconv.Atoi(string(current)); err == nil {
		p.cell = uint8(n)
	} else if c := current.Resolve(); c != nil {
		p.cell = uint8(ansi.Convert256(c))
	}
	if inCube(p.cell) {
		red, green, blue := cubeLevels(p.cell)
		p.slice = red + green + blue
	}
	return p
}

// candidate is the color under the selection, or the one typed when it reads as a color.
func (p *colorPicker) candidate() theme.Color {
	if p.typing {
		if c, err := p.typed(); err == nil {
			return c
		}
	}
	return p.selected()
}

// selected is the cell's color, or the color the picker opened on until the selection moves:
// a hex between two cells is not swapped for the nearest one by enter alone.
func (p *colorPicker) selected() theme.Color {
	if !p.moved {
		return p.current
	}
	return theme.Indexed(p.cell)
}

func (p *colorPicker) typed() (theme.Color, error) {
	text := p.hex.String()
	if !strings.HasPrefix(text, "#") {
		return "", fmt.Errorf("a typed color is #rrggbb")
	}
	return theme.ParseColor(text)
}

func (p *colorPicker) handleKey(key uv.KeyPressEvent) bool {
	if p.typing {
		return p.typeKey(key)
	}
	before := p.cell
	defer func() { p.moved = p.moved || p.cell != before }()
	switch key.String() {
	case "left", "h":
		p.move(-1, 0)
	case "right", "l":
		p.move(1, 0)
	case "up", "k":
		p.move(0, -1)
	case "down", "j":
		p.move(0, 1)
	case ",":
		p.toSlice(p.slice - 1)
	case ".":
		p.toSlice(p.slice + 1)
	case "enter":
		p.done(p.selected(), true)
	case "esc":
		p.done("", false)
	case "#":
		start := "#"
		if c := p.selected(); strings.HasPrefix(string(c), "#") {
			start = string(c)
		}
		p.typing, p.hex, p.problem = true, newTextField(start), ""
	case "t":
		if p.emptyName == "" {
			return false
		}
		p.done("", true)
	default:
		return false
	}
	return true
}

func (p *colorPicker) move(dx, dy int) {
	cells := swatchCells(p.slice)
	if p.column < 0 {
		p.column = cells[p.cell].X
	}
	p.cell = nextSwatch(cells, p.cell, dx, dy, p.column)
	if dx != 0 {
		p.column = cells[p.cell].X
	}
}

// toSlice shows the cube's slice, a selection in the cube going to its closest color there.
func (p *colorPicker) toSlice(slice int) {
	if slice < 0 || slice >= cubeSlices {
		return
	}
	if inCube(p.cell) {
		p.cell = closestInSlice(p.cell, p.slice, slice)
		p.column = swatchCells(slice)[p.cell].X
	}
	p.slice = slice
}

// typeKey edits the hex field; enter takes it when it reads as a color, esc goes back to the grid.
func (p *colorPicker) typeKey(key uv.KeyPressEvent) bool {
	switch key.String() {
	case "esc":
		p.typing, p.problem = false, ""
	case "enter":
		c, err := p.typed()
		if err != nil {
			p.problem = err.Error()
			return true
		}
		p.done(c, true)
	default:
		p.problem = ""
		return p.hex.handleKey(key)
	}
	return true
}

func (p *colorPicker) hints() string {
	if p.typing {
		return "enter use  esc the grid"
	}
	hints := "{left}{right}{up}{down} move  , . slice  enter use  # hex"
	if p.emptyName != "" {
		hints += "  t " + p.emptyName
	}
	return hints + "  esc back"
}

// draw shows the colors in area, lines scrolled to keep the selection in view, and under them
// the selected color and the hex field.
func (p *colorPicker) draw(screen uv.Screen, area image.Rectangle, pal *palette) {
	rows := max(area.Dy()-pickerRows, 1)
	p.first = listFirst(p.first, swatchCells(p.slice)[p.cell].Y, swatchLines, rows)
	visible := func(line int) (int, bool) {
		return area.Min.Y + line - p.first, line >= p.first && line-p.first < rows
	}
	headings := []swatchHeading{{0, "system", "0-15"}, {4, "cube", fmt.Sprintf("slice %d of %d", p.slice, cubeSlices-1)}, {13, "grays", "232-255"}}
	for _, h := range headings {
		if y, ok := visible(h.y); ok {
			x := drawText(screen, area.Min.X, y, area.Max.X, h.name+"  ", pal.panelHeading)
			drawText(screen, x, y, area.Max.X, h.colors, pal.panelMuted)
		}
	}
	if y, ok := visible(4); ok {
		for slice, shade := range sliceShades {
			mark, style := "▪", uv.Style{Fg: shade}
			if slice == p.slice {
				mark, style = "◆", pal.panelText
			}
			drawText(screen, area.Min.X+23+slice, y, area.Max.X, mark, style)
		}
	}
	selected := int(p.cell)
	if p.typing {
		selected = -1
	}
	for _, b := range swatchBlocks(p.slice) {
		for i, line := range b.render(selected) {
			if y, ok := visible(b.y + i); ok {
				x := area.Min.X + b.x
				uv.NewStyledString(line).Draw(screen, image.Rect(x, y, min(x+2*len(b.rows[i]), area.Max.X), y+1))
			}
		}
	}
	y := area.Min.Y + min(rows, swatchLines)
	drawText(screen, area.Min.X, y, area.Max.X, fmt.Sprintf("%d  %s", p.cell, theme.Indexed(p.cell)), pal.panelText)
	x := drawText(screen, area.Min.X, y+1, area.Max.X, "hex  ", pal.panelMuted)
	field := p.hex.String()
	if p.typing {
		field = p.hex.view(area.Max.X-x, false)
	}
	x = drawText(screen, x, y+1, area.Max.X, field, pal.panelText)
	drawText(screen, x+1, y+1, area.Max.X, p.problem, pal.status)
}
