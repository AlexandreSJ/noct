package vt

import (
	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// handleSgr handles SGR escape sequences.
// handleSgr handles Select Graphic Rendition (SGR) escape sequences.
func (e *Emulator) handleSgr(params ansi.Params) {
	uv.ReadStyle(params, &e.scr.cur.Pen)
}
