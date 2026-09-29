package app

import (
	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// Steps of one resize key: cells are about twice as tall as wide.
const (
	resizeStepX = 4
	resizeStepY = 2
)

// handleResizeKey resizes the focused pane as i3's resize mode does: right and down grow it,
// left and up shrink it. A tile moves its border with a neighbor; a floating pane changes size.
func (a *App) handleResizeKey(key uv.KeyPressEvent) {
	name := key.String()
	heading, isArrow := arrowHeadings[name]
	t := a.currentTab()
	if !isArrow || t == nil || t.isEmpty() {
		if name == "esc" || name == "enter" || a.prefix.matches(key) {
			a.mode = normalMode
		}
		return
	}
	if f := t.floatingPane(t.focused); f != nil {
		a.resizeFloating(t, f, heading)
		return
	}
	direction, cells := layout.SideBySide, resizeStepX
	if heading == layout.Up || heading == layout.Down {
		direction, cells = layout.Stacked, resizeStepY
	}
	if heading == layout.Left || heading == layout.Up {
		cells = -cells
	}
	if t.tree.Resize(t.focused, direction, cells, a.workspaceArea(), minPaneRect) {
		t.resizeViews(a.workspaceArea())
	}
}
