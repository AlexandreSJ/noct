package app

import (
	"image"
	"math"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// Keyboard steps for moving and resizing floating panes: cells are about twice as tall as wide.
const (
	floatStepX = 4
	floatStepY = 2
)

// openFloating opens a terminal in a floating pane, centered over the tab. A tab needs a tile
// first, so on the home screen or in an empty tab the terminal opens tiled.
func (a *App) openFloating() {
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		a.openPanel(a.startTerminal)
		return
	}
	area := a.workspaceArea()
	frame := centeredRect(area, max(area.Dx()*3/5, minPaneRect.X), max(area.Dy()*3/5, minPaneRect.Y))
	id := a.newPaneID()
	v, err := a.startTerminal(id, frame)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	t.views[id] = v
	t.floating = append(t.floating, &floatingPane{id: id, frame: frame})
	t.fullscreen = false
	t.focus(id)
	t.resizeViews(area)
}

// setFloatingFrame moves or resizes a floating pane, keeping it inside the tab and no smaller
// than a pane may be.
func (a *App) setFloatingFrame(t *tab, f *floatingPane, frame image.Rectangle) {
	area := a.workspaceArea()
	width := min(max(frame.Dx(), minPaneRect.X), area.Dx())
	height := min(max(frame.Dy(), minPaneRect.Y), area.Dy())
	x := min(max(frame.Min.X, area.Min.X), area.Max.X-width)
	y := min(max(frame.Min.Y, area.Min.Y), area.Max.Y-height)
	f.frame = image.Rect(x, y, x+width, y+height)
	t.resizeViews(area)
}

// moveFloating moves a floating pane one step toward heading.
func (a *App) moveFloating(t *tab, f *floatingPane, heading layout.Heading) {
	a.setFloatingFrame(t, f, f.frame.Add(headingStep(heading)))
}

// resizeFloating grows a floating pane (right, down) or shrinks it (left, up) by one step.
func (a *App) resizeFloating(t *tab, f *floatingPane, heading layout.Heading) {
	frame := f.frame
	frame.Max = frame.Max.Add(headingStep(heading))
	a.setFloatingFrame(t, f, frame)
}

func headingStep(heading layout.Heading) image.Point {
	return map[layout.Heading]image.Point{
		layout.Left: {-floatStepX, 0}, layout.Right: {floatStepX, 0},
		layout.Up: {0, -floatStepY}, layout.Down: {0, floatStepY},
	}[heading]
}

// neighbor is the pane focus moves to toward heading. Among tiles it is the layout's neighbor;
// a floating pane is reached from the tile under its middle, and from a floating pane the nearest
// pane whose middle lies that way.
func (a *App) neighbor(t *tab, heading layout.Heading) (layout.PaneID, bool) {
	area := a.workspaceArea()
	from := t.layoutFrame(t.focused, area)
	if t.floatingPane(t.focused) == nil {
		var over []placedPane
		for _, f := range t.floating {
			if center(f.frame).In(from) {
				over = append(over, placedPane{f.id, f.frame})
			}
		}
		if id, ok := nearestToward(over, from, heading); ok {
			return id, true
		}
		return t.tree.Neighbor(t.focused, heading, area)
	}
	var others []placedPane
	for id, frame := range t.rects(area) {
		others = append(others, placedPane{id, frame})
	}
	for _, f := range t.floating {
		if f.id != t.focused {
			others = append(others, placedPane{f.id, f.frame})
		}
	}
	return nearestToward(others, from, heading)
}

// nearestToward picks the pane whose middle is closest to from's middle toward heading, straying
// sideways counting double.
func nearestToward(panes []placedPane, from image.Rectangle, heading layout.Heading) (layout.PaneID, bool) {
	origin := center(from)
	best, bestScore := layout.PaneID(0), math.MaxInt
	for _, p := range panes {
		d := center(p.frame).Sub(origin)
		ahead, aside := d.X, d.Y
		switch heading {
		case layout.Left:
			ahead = -d.X
		case layout.Up:
			ahead, aside = -d.Y, d.X
		case layout.Down:
			ahead, aside = d.Y, d.X
		}
		if ahead <= 0 {
			continue
		}
		if score := ahead + 2*abs(aside); score < bestScore {
			best, bestScore = p.id, score
		}
	}
	return best, bestScore != math.MaxInt
}

func center(r image.Rectangle) image.Point {
	return image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
}

func abs(n int) int {
	return max(n, -n)
}

// floatingEdges are the sides of a floating pane's frame that a resize drag moves.
type floatingEdges struct {
	left, right, top, bottom bool
}

// edgesAt tells which sides of the frame point is on; none when it is inside.
func edgesAt(frame image.Rectangle, point image.Point) (floatingEdges, bool) {
	e := floatingEdges{
		left: point.X == frame.Min.X, right: point.X == frame.Max.X-1,
		top: point.Y == frame.Min.Y, bottom: point.Y == frame.Max.Y-1,
	}
	return e, e.left || e.right || e.top || e.bottom
}

// dragFloating moves the pane with the mouse (left button on its border) or moves the sides that
// were grabbed (right button).
func (a *App) dragFloating(m image.Point) {
	d := a.drag
	f := d.tab.floatingPane(d.pane)
	if f == nil {
		a.drag = nil // the pane closed while dragging
		return
	}
	if d.kind == dragFloatingMove {
		a.setFloatingFrame(d.tab, f, f.frame.Add(m.Sub(d.grab).Sub(f.frame.Min)))
		return
	}
	frame := f.frame
	if d.edges.left {
		frame.Min.X = min(m.X, frame.Max.X-minPaneRect.X)
	}
	if d.edges.right {
		frame.Max.X = max(m.X+1, frame.Min.X+minPaneRect.X)
	}
	if d.edges.top {
		frame.Min.Y = min(m.Y, frame.Max.Y-minPaneRect.Y)
	}
	if d.edges.bottom {
		frame.Max.Y = max(m.Y+1, frame.Min.Y+minPaneRect.Y)
	}
	a.setFloatingFrame(d.tab, f, frame)
}
