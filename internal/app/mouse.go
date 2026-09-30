package app

import (
	"image"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// wheelLines is how many lines one wheel step scrolls a terminal's history.
const wheelLines = 3

// mouseAware views take mouse events at some places, positioned relative to their content:
// terminals whose program asked for them, and the clickable parts of noct's own panels. Anywhere
// else, dragging selects text.
type mouseAware interface {
	WantsMouseAt(point image.Point) bool
	SendMouse(event uv.MouseEvent)
}

type dragKind int

const (
	dragDivider        dragKind = iota // the line between two tiles, resizing them
	dragFloatingMove                   // a floating pane by its border, moving it
	dragFloatingResize                 // sides of a floating pane, resizing it
)

// mouseDrag is something being dragged with the mouse.
type mouseDrag struct {
	kind    dragKind
	tab     *tab
	divider layout.Divider
	pane    layout.PaneID
	// grab is where the pane was grabbed, relative to its frame.
	grab  image.Point
	edges floatingEdges
}

// applyMouseMode asks the attached terminal to report the mouse, or to stop, as configured.
// Reports include motion while a button is held, which dragging a divider needs. On the home they
// include every motion, so the art can follow the pointer; nowhere else, as panes need none.
func (a *App) applyMouseMode() {
	if a.screen == nil {
		return
	}
	mode := uv.MouseModeNone
	if a.config.Mouse {
		mode = uv.MouseModeDrag
		if a.onHome() {
			mode = uv.MouseModeMotion
		}
	}
	if mode == a.screen.MouseMode() {
		return
	}
	if a.screen.MouseMode() == uv.MouseModeMotion {
		_ = a.screen.SetMouseMode(uv.MouseModeNone) // turning on drag reports leaves every motion on
	}
	_ = a.screen.SetMouseMode(mode) // only queued; the next frame's flush reports failures
}

// onHome tells whether the home shows, with no menu over it.
func (a *App) onHome() bool {
	t := a.currentTab()
	return a.mode == normalMode && (t == nil || t.isEmpty())
}

func (a *App) handleMouse(event uv.MouseEvent) {
	if !a.config.Mouse {
		return
	}
	switch event.(type) {
	case uv.MouseClickEvent:
		a.handleClick(event)
	case uv.MouseMotionEvent:
		switch {
		case a.drag != nil && a.drag.kind == dragDivider:
			a.dragDivider(event.Mouse())
		case a.drag != nil:
			a.dragFloating(image.Pt(event.Mouse().X, event.Mouse().Y))
		case a.selection != nil && a.selection.dragging:
			a.extendSelection(event.Mouse())
		case a.onHome():
			a.hoverHome(image.Pt(event.Mouse().X, event.Mouse().Y))
		default:
			a.forwardMouse(event)
		}
	case uv.MouseReleaseEvent:
		switch {
		case a.drag != nil:
			a.drag = nil
		case a.selection != nil && a.selection.dragging:
			a.finishSelection()
		default:
			a.forwardMouse(event)
		}
	case uv.MouseWheelEvent:
		a.handleWheel(event)
	}
}

// handleClick runs the command clicked in the noct popup, selects the tab clicked in the tab bar,
// starts dragging a divider, or focuses the pane clicked and passes the click on to it; on text,
// it starts selecting instead.
func (a *App) handleClick(event uv.MouseEvent) {
	m := event.Mouse()
	// A release outside the window never arrives; a new click ends that drag or selection.
	a.drag, a.selection = nil, nil
	if a.mode == infoMode {
		a.mode = normalMode // a click closes what it tells, as a key does
		return
	}
	if a.mode == commandMode {
		a.clickCommandShortcut(image.Pt(m.X, m.Y))
		return
	}
	if a.mode != normalMode && a.mode != scrollMode {
		return
	}
	if m.Y == 0 {
		if index, ok := a.tabAt(m.X); ok {
			a.session.activeTab = index
		}
		return
	}
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		return
	}
	point := image.Pt(m.X, m.Y)
	id, ok := t.paneAt(point, a.workspaceArea())
	if !ok {
		return
	}
	if a.mode == scrollMode && id != t.focused {
		a.mode = normalMode // another pane: scrolling this one ends
	}
	if a.startDrag(t, id, point, m.Button) {
		return
	}
	a.focusPane(t, id)
	t.raise(id)
	frame, _ := t.frameOf(id, a.workspaceArea())
	content := contentRect(frame)
	if !point.In(content) || a.forwardMouse(event) {
		return
	}
	if m.Button == uv.MouseLeft {
		a.startSelection(t, id, point)
	}
}

// startDrag grabs what is under a click on a border: a floating pane's frame (left button moves
// it, right button resizes it) or the divider between two tiles.
func (a *App) startDrag(t *tab, id layout.PaneID, point image.Point, button uv.MouseButton) bool {
	if f := t.floatingPane(id); f != nil && !t.fullscreen {
		edges, onBorder := edgesAt(f.frame, point)
		if !onBorder || (button != uv.MouseLeft && button != uv.MouseRight) {
			return false
		}
		t.focus(id)
		t.raise(id)
		a.drag = &mouseDrag{kind: dragFloatingMove, tab: t, pane: id, grab: point.Sub(f.frame.Min)}
		if button == uv.MouseRight {
			a.drag.kind, a.drag.edges = dragFloatingResize, edges
		}
		return true
	}
	if t.fullscreen || button != uv.MouseLeft {
		return false
	}
	divider, ok := t.tree.DividerAt(a.workspaceArea(), point)
	if ok {
		a.drag = &mouseDrag{kind: dragDivider, tab: t, divider: divider}
	}
	return ok
}

func (a *App) dragDivider(m uv.Mouse) {
	if !a.drag.tab.tree.MoveDivider(a.drag.divider, a.workspaceArea(), image.Pt(m.X, m.Y), minPaneRect) {
		a.drag = nil // the split went away while dragging
		return
	}
	a.drag.tab.resizeViews(a.workspaceArea())
}

// handleWheel over the tab bar changes tab. Elsewhere it gives the wheel to a program or panel
// that takes the mouse; otherwise it scrolls the terminal's history, entering scroll mode and
// leaving it back at the live screen.
func (a *App) handleWheel(event uv.MouseEvent) {
	t := a.currentTab()
	if t == nil || (a.mode != normalMode && a.mode != scrollMode) {
		return
	}
	m := event.Mouse()
	if m.Y == 0 && (a.selection == nil || !a.selection.dragging) {
		a.wheelTabs(m.Button)
		return
	}
	if t.isEmpty() {
		return
	}
	id, ok := t.paneAt(image.Pt(m.X, m.Y), a.workspaceArea())
	if a.selection != nil && a.selection.dragging {
		id, ok = a.selection.pane, true // while selecting, the wheel scrolls the selected pane
	}
	if !ok {
		return
	}
	if a.mode == scrollMode && id != t.focused {
		a.mode = normalMode // the wheel over another pane scrolls that one instead
	}
	a.focusPane(t, id)
	if a.forwardMouse(event) {
		return
	}
	s, ok := t.focusedView().(scrollable)
	if !ok || s.ScrollbackLen() == 0 {
		return
	}
	switch m.Button {
	case uv.MouseWheelUp:
		if a.mode == normalMode {
			a.mode, a.scrollOffset = scrollMode, 0
		}
		a.scrollTo(s, a.scrollOffset+wheelLines)
	case uv.MouseWheelDown:
		a.scrollTo(s, a.scrollOffset-wheelLines)
		if a.scrollOffset == 0 {
			a.mode = normalMode
		}
	}
	if a.selection != nil && a.selection.dragging {
		a.selection.mouse = image.Pt(m.X, m.Y)
		a.followSelection() // selecting while scrolling: the end stays under the mouse
	}
}

// forwardMouse passes the event to the focused view when it is over the view's content and the
// view takes mouse events. It reports whether the view got it.
func (a *App) forwardMouse(event uv.MouseEvent) bool {
	t := a.currentTab()
	if a.mode != normalMode || t == nil || t.isEmpty() {
		return false
	}
	target, ok := t.focusedView().(mouseAware)
	if !ok {
		return false
	}
	frame, _ := t.frameOf(t.focused, a.workspaceArea())
	content := contentRect(frame)
	m := event.Mouse()
	point := image.Pt(m.X, m.Y)
	if !point.In(content) || !target.WantsMouseAt(point.Sub(content.Min)) {
		return false
	}
	target.SendMouse(relativeTo(event, content.Min))
	return true
}

// relativeTo moves the event's position to be relative to origin, keeping its kind.
func relativeTo(event uv.MouseEvent, origin image.Point) uv.MouseEvent {
	m := event.Mouse()
	m.X, m.Y = m.X-origin.X, m.Y-origin.Y
	switch event.(type) {
	case uv.MouseClickEvent:
		return uv.MouseClickEvent(m)
	case uv.MouseReleaseEvent:
		return uv.MouseReleaseEvent(m)
	case uv.MouseMotionEvent:
		return uv.MouseMotionEvent(m)
	}
	return uv.MouseWheelEvent(m)
}
