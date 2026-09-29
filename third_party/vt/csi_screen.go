package vt

import (
	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// eraseCharacter erases n characters starting from the cursor position. It
// does not move the cursor. This is equivalent to [ansi.ECH].
func (e *Emulator) eraseCharacter(n int) {
	if n <= 0 {
		n = 1
	}
	x, y := e.scr.CursorPosition()
	rect := uv.Rect(x, y, n, 1)
	e.scr.FillArea(e.scr.blankCell(), rect)
	e.atPhantom = false
	// ECH does not move the cursor.
}
