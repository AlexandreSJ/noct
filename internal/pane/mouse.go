package pane

import (
	"image"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/AlexandreSJ/noct/third_party/vt"
	"github.com/charmbracelet/x/ansi"
)

// mouseModeBits are the mouse reporting modes a program can ask for, from clicks only to every
// motion.
var mouseModeBits = map[ansi.Mode]uint32{
	ansi.ModeMouseX10:         1 << 0,
	ansi.ModeMouseNormal:      1 << 1,
	ansi.ModeMouseButtonEvent: 1 << 2,
	ansi.ModeMouseAnyEvent:    1 << 3,
}

// Modes that also report releases and dragging.
const (
	reportsRelease = 1<<1 | 1<<2 | 1<<3
	reportsMotion  = 1<<2 | 1<<3
)

func (p *Pane) setMouseMode(mode ansi.Mode, on bool) {
	bit, ok := mouseModeBits[mode]
	if !ok {
		return
	}
	if on {
		p.mouseModes.Or(bit)
	} else {
		p.mouseModes.And(^bit)
	}
}

// WantsMouse tells whether the program asked for mouse events, as vim and htop do.
func (p *Pane) WantsMouse() bool {
	return p.mouseModes.Load() != 0
}

// WantsMouseAt is WantsMouse: a program takes the mouse over its whole screen.
func (p *Pane) WantsMouseAt(image.Point) bool {
	return p.WantsMouse()
}

// SendMouse passes a mouse event, positioned relative to the pane, to the program, if the modes
// it turned on report that kind of event.
func (p *Pane) SendMouse(event uv.MouseEvent) {
	modes := p.mouseModes.Load()
	switch event.(type) {
	case uv.MouseReleaseEvent:
		if modes&reportsRelease == 0 {
			return
		}
	case uv.MouseMotionEvent:
		if modes&reportsMotion == 0 {
			return
		}
	}
	if modes != 0 {
		p.withOpenScreen(func(screen *vt.Emulator) { screen.SendMouse(event) })
	}
}
