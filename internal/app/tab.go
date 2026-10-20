package app

import (
	"image"
	"slices"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
)

// minPaneRect is the smallest pane, frame included, that a split may create.
var minPaneRect = image.Pt(12, 5)

// tab holds panes in a layout, and floating panes over it. A tab is empty only while its first
// pane is being opened: the workspace closes a tab when its last pane leaves.
type tab struct {
	tree    *layout.Tree // nil while the tab is empty
	views   map[layout.PaneID]view
	focused layout.PaneID
	// history holds the panes focused before, the most recent last: closing a pane gives the focus
	// back to the one used before it, as i3 does.
	history []layout.PaneID
	// name is the title the user gave the tab, or "" to follow the focused view.
	name string
	// paneNames are the titles the user gave panes; a pane without one shows its view's title.
	paneNames map[layout.PaneID]string
	// paneColors are panes' own colors over the theme, and colors the tab's own in the tab bar.
	paneColors map[layout.PaneID]colorSet
	colors     colorSet
	// floating panes sit over the tiles in frames of their own, the last on top.
	floating []*floatingPane
	// fullscreen shows only the focused pane, over the whole tab; the layout stays as it is.
	fullscreen bool
}

type floatingPane struct {
	id    layout.PaneID
	frame image.Rectangle
}

// placedPane is a pane as it shows on screen.
type placedPane struct {
	id    layout.PaneID
	frame image.Rectangle
}

func newEmptyTab() *tab {
	return &tab{views: make(map[layout.PaneID]view), paneNames: make(map[layout.PaneID]string), paneColors: make(map[layout.PaneID]colorSet)}
}

func newTab(id layout.PaneID, v view) *tab {
	t := newEmptyTab()
	t.placeFirst(id, v)
	return t
}

func (t *tab) isEmpty() bool {
	return t.tree == nil
}

// placeFirst puts a view into an empty tab, filling it.
func (t *tab) placeFirst(id layout.PaneID, v view) {
	t.tree = layout.New(id)
	t.views[id] = v
	t.focused = id
}

// focus moves the focus to a pane, remembering the one it leaves.
func (t *tab) focus(id layout.PaneID) {
	if id == t.focused {
		return
	}
	t.history = append(slices.DeleteFunc(t.history, func(p layout.PaneID) bool { return p == id || p == t.focused }), t.focused)
	t.focused = id
}

// focusedView is nil for an empty tab.
func (t *tab) focusedView() view {
	return t.views[t.focused]
}

// rects are the frames of the tiled panes in the layout, with the focused tile unfolded in the
// stacks it is in.
func (t *tab) rects(area image.Rectangle) map[layout.PaneID]image.Rectangle {
	if t.isEmpty() {
		return nil
	}
	t.tree.Show(t.tiledFocus())
	return t.tree.Rects(area)
}

// visible lists the panes to draw, bottom to top: the tiles, then the floating panes. In
// fullscreen only the focused pane shows, over the whole area.
func (t *tab) visible(area image.Rectangle) []placedPane {
	if t.isEmpty() {
		return nil
	}
	if t.fullscreen {
		return []placedPane{{t.focused, area}}
	}
	rects := t.rects(area)
	var panes []placedPane
	for _, id := range t.tree.Panes() {
		panes = append(panes, placedPane{id, rects[id]})
	}
	for _, f := range t.floating {
		panes = append(panes, placedPane{f.id, f.frame.Intersect(area)})
	}
	return panes
}

// frameOf is where a pane shows, if it does.
func (t *tab) frameOf(id layout.PaneID, area image.Rectangle) (image.Rectangle, bool) {
	for _, p := range t.visible(area) {
		if p.id == id {
			return p.frame, true
		}
	}
	return image.Rectangle{}, false
}

// paneAt is the topmost pane shown at point.
func (t *tab) paneAt(point image.Point, area image.Rectangle) (layout.PaneID, bool) {
	panes := t.visible(area)
	for i := len(panes) - 1; i >= 0; i-- {
		if point.In(panes[i].frame) {
			return panes[i].id, true
		}
	}
	return 0, false
}

// layoutFrame is where a pane sits in the layout, fullscreen aside: its tile or floating frame.
func (t *tab) layoutFrame(id layout.PaneID, area image.Rectangle) image.Rectangle {
	if f := t.floatingPane(id); f != nil {
		return f.frame
	}
	return t.rects(area)[id]
}

func (t *tab) floatingPane(id layout.PaneID) *floatingPane {
	for _, f := range t.floating {
		if f.id == id {
			return f
		}
	}
	return nil
}

// raise puts a floating pane on top of the others.
func (t *tab) raise(id layout.PaneID) {
	if f := t.floatingPane(id); f != nil {
		t.floating = append(slices.DeleteFunc(t.floating, func(g *floatingPane) bool { return g == f }), f)
	}
}

// tiledFocus is the tiled pane that has, or last had, the focus: where a new tile opens.
func (t *tab) tiledFocus() layout.PaneID {
	candidates := append(slices.Clone(t.history), t.focused)
	for i := len(candidates) - 1; i >= 0; i-- {
		if t.tree.Contains(candidates[i]) {
			return candidates[i]
		}
	}
	return t.tree.Panes()[0]
}

// resizeViews sizes every view for the frame it shows in; hidden by fullscreen, they keep their
// layout size, so leaving fullscreen does not resize them again.
func (t *tab) resizeViews(area image.Rectangle) {
	if t.isEmpty() {
		return
	}
	t.rects(area) // unfolds the focused tile before telling which panes are folded
	for id := range t.views {
		if t.tree.Folded(id) {
			continue // keeps its size, so unfolding does not resize it
		}
		frame := t.layoutFrame(id, area)
		if t.fullscreen && id == t.focused {
			frame = area
		}
		t.views[id].Resize(contentSize(frame))
	}
}

// removeView forgets the view and gives the focus back to the pane used before it, or, for a pane
// of a stack, to the pane the stack shows in its place, so the focus stays in the column. When the
// last tile goes, a floating pane takes its place; with no panes left the tab is empty.
func (t *tab) removeView(id layout.PaneID) {
	heir, inStack := t.tree.StackHeir(id)
	delete(t.views, id)
	delete(t.paneNames, id)
	delete(t.paneColors, id)
	t.history = slices.DeleteFunc(t.history, func(p layout.PaneID) bool { return p == id })
	if f := t.floatingPane(id); f != nil {
		t.floating = slices.DeleteFunc(t.floating, func(g *floatingPane) bool { return g == f })
	} else if t.tree.Remove(id) {
		t.tree = nil
		if len(t.floating) == 0 {
			return
		}
		t.tree, t.floating = layout.New(t.floating[0].id), t.floating[1:]
	}
	if t.focused != id {
		return
	}
	if _, ok := t.views[heir]; inStack && ok {
		t.focused = heir
		t.history = slices.DeleteFunc(t.history, func(p layout.PaneID) bool { return p == heir })
		return
	}
	t.focused = t.tree.Panes()[0]
	if len(t.history) > 0 {
		t.focused, t.history = t.history[len(t.history)-1], t.history[:len(t.history)-1]
	}
}

// edgePane is the pane a pane arriving from the given heading lands next to: moving right, a
// pane enters a tab from its left side, so it lands beside the first pane.
func (t *tab) edgePane(heading layout.Heading) layout.PaneID {
	panes := t.tree.Panes()
	if heading == layout.Left {
		return panes[len(panes)-1]
	}
	return panes[0]
}

// fits tells whether every pane but those folded in a stack is at least the smallest pane
// allowed. It reads the layout as it is, so a pane just added to a stack counts as shown.
func (t *tab) fits(area image.Rectangle) bool {
	for id, rect := range t.tree.Rects(area) {
		if t.tree.Folded(id) {
			continue
		}
		if rect.Dx() < minPaneRect.X || rect.Dy() < minPaneRect.Y {
			return false
		}
	}
	return true
}

// splitDirectionFor splits along the longer side, counting a cell as twice as tall as it is wide,
// so repeated splits alternate between side by side and stacked, like i3 with autotiling.
func splitDirectionFor(rect image.Rectangle) layout.Direction {
	if rect.Dx() > 2*rect.Dy() {
		return layout.SideBySide
	}
	return layout.Stacked
}

func canSplit(rect image.Rectangle, direction layout.Direction) bool {
	if direction == layout.SideBySide {
		return rect.Dx()/2 >= minPaneRect.X
	}
	return rect.Dy()/2 >= minPaneRect.Y
}

// contentRect is the part of a pane's frame where its view is drawn.
func contentRect(frame image.Rectangle) image.Rectangle {
	return frame.Inset(1)
}

func contentSize(frame image.Rectangle) pty.Size {
	content := contentRect(frame)
	return pty.Size{Cols: content.Dx(), Rows: content.Dy()}
}
