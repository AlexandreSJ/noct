package app

import (
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// infoPopup tells about something, such as what a flow's step does, until closed.
type infoPopup struct {
	title string
	lines []styledLine
	first int
	// visible and wrapped are how many lines were shown and how many the text took, as drawn.
	visible, wrapped int
	// back is the mode closing it goes back to, such as the config it was opened from.
	back mode
}

func (a *App) openInfo(title string, lines []styledLine) {
	a.info = infoPopup{title: title, lines: lines, back: normalMode}
	a.mode = infoMode
}

// handleInfoKey scrolls a long text; any other key closes it.
func (a *App) handleInfoKey(key uv.KeyPressEvent) {
	switch key.String() {
	case "up", "k":
		a.info.first = max(a.info.first-1, 0)
	case "down", "j":
		a.info.first++
	case "pgup", "pgdown", "home", "end":
		last := max(a.info.wrapped-a.info.visible, 0) // the first line that shows the end
		a.info.first, _ = listJump(key.String(), a.info.first, last+1, a.info.visible)
	default:
		a.mode = a.info.back
	}
}

// infoTextWidth is how wide a line of the about popup can be before it wraps.
func (a *App) infoTextWidth() int { return min(78, a.size.X-4) - 6 }

func (a *App) drawInfo(screen uv.Screen) {
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	width := min(78, a.size.X-4)
	var lines []styledLine
	for _, line := range a.info.lines {
		for _, part := range strings.Split(ansi.Wrap(line.text, a.infoTextWidth(), ""), "\n") {
			lines = append(lines, styledLine{part, line.style})
		}
	}
	inner := a.drawPanel(screen, centeredRect(area, width, panelHeight(len(lines)+2)), a.info.title)
	visible := max(inner.Dy()-2, 1)
	a.info.first = min(a.info.first, max(len(lines)-visible, 0))
	a.info.visible, a.info.wrapped = visible, len(lines)
	for i := a.info.first; i < len(lines) && i-a.info.first < visible; i++ {
		drawText(screen, inner.Min.X+1, inner.Min.Y+i-a.info.first, inner.Max.X-1, lines[i].text, lines[i].style)
	}
	hints := "any key closes"
	if len(lines) > visible {
		hints = a.hint("info", "{up}{down} scroll  {pgup}{pgdown} page  any other key closes")
	}
	drawHints(screen, inner, hints, a.palette.panelMuted)
}
