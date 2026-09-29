package app

import (
	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/pty"
)

// view is what a pane of the layout shows: a terminal running a program, or one of noct's own
// panels. *pane.Pane is the terminal implementation.
type view interface {
	Draw(screen uv.Screen, area uv.Rectangle)
	Resize(size pty.Size)
	SendKey(key uv.KeyPressEvent)
	Paste(text string)
	// Cursor is relative to the view, with whether it should be shown.
	Cursor() (uv.Position, bool)
	// Title is the view's own title, or "" to let noct choose one.
	Title() string
	Close()
}
