package app

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/theme"
)

// pickerRows are the lines under the grid: the selected color and the hex field.
const pickerRows = 2

// colorPicker picks one of the terminal's 256 colors on a grid, or a typed #rrggbb.
type colorPicker struct {
	// current is the color the picker opened on, kept until the selection moves.
	current theme.Color
	moved   bool
	cell    uint8
	// first is the first grid row shown when the grid does not fit.
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
	p := &colorPicker{current: current, emptyName: emptyName, done: done}
	if n, err := strconv.Atoi(string(current)); err == nil {
		p.cell = uint8(n)
	} else if c := current.Resolve(); c != nil {
		p.cell = uint8(ansi.Convert256(c))
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
		if p.cell%16 > 0 {
			p.cell--
		}
	case "right", "l":
		if p.cell%16 < 15 {
			p.cell++
		}
	case "up", "k":
		if p.cell >= 16 {
			p.cell -= 16
		}
	case "down", "j":
		if p.cell < 240 {
			p.cell += 16
		}
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
	hints := "{left}{right}{up}{down} move  enter use  # hex"
	if p.emptyName != "" {
		hints += "  t " + p.emptyName
	}
	return hints + "  esc back"
}

// draw shows the grid in area, rows scrolled to keep the selection in view, and under it the
// selected color and the hex field.
func (p *colorPicker) draw(screen uv.Screen, area image.Rectangle, pal *palette) {
	rows := max(area.Dy()-pickerRows, 1)
	p.first = listFirst(p.first, int(p.cell)/16, 16, rows)
	for row := p.first; row < 16 && row-p.first < rows; row++ {
		y := area.Min.Y + row - p.first
		for col := range 16 {
			n := uint8(row*16 + col)
			x := area.Min.X + 2*col
			if n == p.cell && !p.typing {
				drawText(screen, x, y, area.Max.X, "[]", pal.selectedRow)
				continue
			}
			drawText(screen, x, y, area.Max.X, "  ", uv.Style{Bg: theme.Indexed(n).Resolve()})
		}
	}
	y := area.Min.Y + min(rows, 16)
	drawText(screen, area.Min.X, y, area.Max.X, fmt.Sprintf("%d  %s", p.cell, theme.Indexed(p.cell)), pal.panelText)
	x := drawText(screen, area.Min.X, y+1, area.Max.X, "hex  ", pal.panelMuted)
	field := p.hex.String()
	if p.typing {
		field = p.hex.view(area.Max.X-x, false)
	}
	x = drawText(screen, x, y+1, area.Max.X, field, pal.panelText)
	drawText(screen, x+1, y+1, area.Max.X, p.problem, pal.status)
}
