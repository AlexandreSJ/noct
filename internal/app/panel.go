package app

import (
	"image"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Panels are the framed boxes noct draws over the workspace: the config, the list of noct mode
// commands and the rename prompt all share this look.

// drawPanel clears rect, frames it with the title and returns the area inside the frame and padding.
func (a *App) drawPanel(screen uv.Screen, rect image.Rectangle, title string) image.Rectangle {
	fillRect(screen, rect, uv.Style{Bg: a.palette.background})
	drawFrame(screen, rect, title, a.palette.focusedFrame, a.palette.focusedTitle)
	return rect.Inset(2)
}

// panelHeight is the height of a panel holding the given number of content lines.
func panelHeight(lines int) int {
	return lines + 4
}

// drawRow fills one line of area with style, the label on the left and the value on the right.
func drawRow(screen uv.Screen, area image.Rectangle, y int, label, value string, style uv.Style) {
	fillRect(screen, image.Rect(area.Min.X, y, area.Max.X, y+1), style)
	drawText(screen, area.Min.X+1, y, area.Max.X, label, style)
	valueX := area.Max.X - 1 - ansi.StringWidth(value)
	drawText(screen, valueX, y, area.Max.X, value, style)
}

// drawCentered writes text horizontally centered in area on row y.
func drawCentered(screen uv.Screen, area image.Rectangle, y int, text string, style uv.Style) {
	x := area.Min.X + max(area.Dx()-ansi.StringWidth(text), 0)/2
	drawText(screen, x, y, area.Max.X, text, style)
}

func centeredRect(area image.Rectangle, width, height int) image.Rectangle {
	width, height = min(width, area.Dx()), min(height, area.Dy())
	x := area.Min.X + (area.Dx()-width)/2
	y := area.Min.Y + (area.Dy()-height)/2
	return image.Rect(x, y, x+width, y+height)
}
