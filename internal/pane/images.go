package pane

import (
	"fmt"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/AlexandreSJ/noct/third_party/vt"

	"github.com/AlexandreSJ/noct/internal/kittygfx"
)

// placeImage fills the placement with placeholder cells from the cursor on and then moves the
// cursor as kitty does: right by the image's columns and down by its rows, unless asked to stay.
// An image reaching past the bottom scrolls the screen first, as it would in kitty.
func placeImage(screen *vt.Emulator, placement kittygfx.Placement) {
	width, height := screen.Width(), screen.Height()
	origin := screen.CursorPosition()
	rowsBelowCursor := placement.Rows // the cursor ends on the row below the image
	if placement.KeepCursor {
		rowsBelowCursor = placement.Rows - 1
	}
	if overflow := origin.Y + rowsBelowCursor - (height - 1); overflow > 0 {
		moveCursor(screen, origin.X, height-1)
		_, _ = screen.WriteString(strings.Repeat("\n", overflow)) // line feeds at the bottom scroll
		origin.Y -= overflow
	}

	fg := kittygfx.PlaceholderColor(placement.ImageID)
	for row := range placement.Rows {
		y := origin.Y + row
		if y < 0 {
			continue // taller than the screen: the top already scrolled away
		}
		for col := 0; col < placement.Cols && origin.X+col < width; col++ {
			screen.SetCell(origin.X+col, y, &uv.Cell{
				Content: kittygfx.PlaceholderText(placement.ImageID, row, col),
				Width:   1,
				Style:   uv.Style{Fg: fg},
			})
		}
	}

	if placement.KeepCursor {
		moveCursor(screen, origin.X, max(origin.Y, 0))
		return
	}
	moveCursor(screen, min(origin.X+placement.Cols, width-1), min(origin.Y+placement.Rows, height-1))
}

func moveCursor(screen *vt.Emulator, x, y int) {
	_, _ = fmt.Fprintf(screen, "\x1b[%d;%dH", y+1, x+1) // the emulator never fails on write
}
