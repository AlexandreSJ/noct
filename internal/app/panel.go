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
	fillRect(screen, rect, uv.Style{Bg: a.palette.popupBackground})
	drawFrame(screen, rect, title, a.palette.focusedFrame, a.palette.focusedTitle)
	return rect.Inset(2)
}

// panelHeight is the height of a panel holding the given number of content lines.
func panelHeight(lines int) int {
	return lines + 4
}

// scrollOff is the fewest rows a list keeps in view around its selection, above and below, so
// moving through it shows what comes next before reaching it. Taller lists keep scrollShare of
// their height.
const (
	scrollOff   = 4
	scrollShare = 30 // percent
)

// scrollMargin is how many rows a list height rows tall keeps around its selection: scrollShare
// of it, at least scrollOff, and never more than half, where the selection would stop moving.
func scrollMargin(height int) int {
	return min(max(scrollOff, height*scrollShare/100), (height-1)/2)
}

// listFirst is the first row a list of n rows shows in height rows. It moves from first, where the
// list was, only as much as keeping scrollOff rows around selected takes, and never leaves blank
// rows past the end.
func listFirst(first, selected, n, height int) int {
	if height <= 0 {
		return 0
	}
	margin := scrollMargin(height)
	first = min(first, selected-margin)
	first = max(first, selected+margin-height+1)
	return max(min(first, n-height), 0)
}

// listJump is where a key that jumps leaves the selection of a list of n rows shown height at a
// time: pgup and pgdown a page away, keeping one row of the page left in view, home and end at
// the ends. Other keys do not jump.
func listJump(key string, selected, n, height int) (int, bool) {
	page := max(height-1, 1)
	switch key {
	case "pgup":
		return max(selected-page, 0), true
	case "pgdown":
		return max(min(selected+page, n-1), 0), true
	case "home":
		return 0, true
	case "end":
		return max(n-1, 0), true
	}
	return selected, false
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
