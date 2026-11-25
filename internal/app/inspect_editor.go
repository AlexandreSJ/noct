package app

import (
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// inspectEditor is what the inspect shows in place of the chart while a flow's params or a step
// are edited. The keys go to it, and every other panel and tab stays a prefix away.
type inspectEditor interface {
	title() string
	// handleKey tells whether the editor is done and closes.
	handleKey(key uv.KeyPressEvent) (done bool)
	paste(text string)
	// mouse clicks or scrolls the editor at a point on the screen, and tells whether it is done.
	mouse(event uv.MouseEvent, at image.Point) (done bool)
	// typing tells the keys type text as they are pressed, so none of them is a shortcut.
	typing() bool
	// draw draws the editor in area; without the help beside it, it shows what it must itself.
	draw(screen uv.Screen, area image.Rectangle, helpBeside bool)
	// help is what the focused cell takes and what it can read, for the pane beside.
	help() []styledLine
	hints() string
}

// editorHelpWidth is the help pane beside an editor, in panels wide enough for both.
const editorHelpWidth = 40

// drawEditor frames the editor in area, with its help beside it when there is room.
func (p *inspectPanel) drawEditor(screen uv.Screen, area image.Rectangle) {
	pal := &p.app.palette
	helpBeside := !p.helpHidden && area.Dx() >= editorHelpWidth+40
	if helpBeside {
		helpArea := image.Rect(area.Max.X-editorHelpWidth, area.Min.Y, area.Max.X, area.Max.Y)
		area.Max.X = helpArea.Min.X
		drawFrame(screen, helpArea, "help", pal.unfocusedFrame, pal.panelHeading)
		drawWrappedLines(screen, helpArea.Inset(1), p.editor.help())
	}
	drawFrame(screen, area, p.editor.title(), pal.focusedFrame, pal.focusedTitle)
	p.editor.draw(screen, area.Inset(1), helpBeside)
}

// drawWrappedLines writes lines from the top of area, each wrapped to its width, as many as fit.
func drawWrappedLines(screen uv.Screen, area image.Rectangle, lines []styledLine) {
	y := area.Min.Y
	for _, line := range lines {
		for _, part := range strings.Split(ansi.Wrap(line.text, max(area.Dx()-2, 1), ""), "\n") {
			if y >= area.Max.Y {
				return
			}
			drawText(screen, area.Min.X+1, y, area.Max.X, part, line.style)
			y++
		}
	}
}

// hintsInOneRow keeps the hints that fit one row width wide, in order, and always the last one,
// which tells where the rest are (? all keys): an editor's hints never take a second row.
func hintsInOneRow(hints string, width int) string {
	parts := strings.Split(hints, "  ")
	if ansi.StringWidth(hints) <= width || len(parts) < 2 {
		return ansi.Truncate(hints, max(width, 0), "")
	}
	last := parts[len(parts)-1]
	var kept []string
	for _, part := range parts[:len(parts)-1] {
		if ansi.StringWidth(strings.Join(append(kept, part, last), "  ")) > width {
			break
		}
		kept = append(kept, part)
	}
	return ansi.Truncate(strings.Join(append(kept, last), "  "), max(width, 0), "")
}
