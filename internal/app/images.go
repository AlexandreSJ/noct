package app

import (
	"image"
	"strings"
)

// outerShowsPlaceholders reports whether the terminal noct runs in draws kitty's Unicode image
// placeholders (kitty 0.28+ and Ghostty). Elsewhere, images from pane programs are dropped.
func outerShowsPlaceholders(env []string) bool {
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		switch {
		case name == "TERM" && value == "xterm-kitty",
			name == "KITTY_WINDOW_ID",
			name == "TERM_PROGRAM" && strings.EqualFold(value, "ghostty"):
			return true
		}
	}
	return false
}

func (a *App) newImageID() uint32 {
	return a.nextImageID.Add(1)
}

// cellSize is the outer terminal's cell size in pixels, or zero when the terminal does not say.
func (a *App) cellSize() image.Point {
	size, _ := a.cellPixels.Load().(image.Point)
	return size
}

// forwardGraphics hands image commands to the main loop and returns once they are queued for the
// terminal, so they always reach it before the frame showing their placeholders.
func (a *App) forwardGraphics(commands []byte) {
	select {
	case a.terminalGraphics <- commands:
	case <-a.stopped:
	}
}
