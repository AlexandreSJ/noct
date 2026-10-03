package layout

import "image"

// Divider is the line between two neighbors in a container, which can be dragged to resize them.
type Divider struct {
	container *node
	// after is the child the divider follows.
	after int
}

// DividerAt finds the divider drawn at point: the two cells on either side of a boundary between
// neighbors, where their frames meet. Outer containers come first.
func (t *Tree) DividerAt(area image.Rectangle, point image.Point) (Divider, bool) {
	n := t.root
	for n != nil && !n.isLeaf() && point.In(area) {
		if divider, ok := n.ownDividerAt(area, point); ok {
			return divider, true
		}
		var found bool
		if n, area, found = n.childAt(area, point); !found {
			break
		}
	}
	return Divider{}, false
}

// ownDividerAt finds a divider between the container's own children; a stack has none.
func (n *node) ownDividerAt(area image.Rectangle, point image.Point) (Divider, bool) {
	if n.stacked {
		return Divider{}, false
	}
	along := point.X
	if n.direction == Stacked {
		along = point.Y
	}
	for i := 1; i < len(n.children); i++ {
		if b := n.boundary(area, i); along == b-1 || along == b {
			return Divider{container: n, after: i - 1}, true
		}
	}
	return Divider{}, false
}

// childAt finds the child of the container whose area holds point, and that area.
func (n *node) childAt(area image.Rectangle, point image.Point) (*node, image.Rectangle, bool) {
	for i, child := range n.children {
		if childArea := n.childArea(area, i); point.In(childArea) {
			return child, childArea, true
		}
	}
	return nil, area, false
}

// MoveDivider puts the divider at point, keeping the two neighbors at least minSize cells long.
// It reports whether the divider still belongs to the tree.
func (t *Tree) MoveDivider(d Divider, area image.Rectangle, point image.Point, minSize image.Point) bool {
	c := d.container
	containerArea, ok := areaOf(c, t.root, area)
	if !ok || d.after+1 >= len(c.children) {
		return false
	}
	origin, length := span(containerArea, c.direction)
	at, least := point.X, minSize.X
	if c.direction == Stacked {
		at, least = point.Y, minSize.Y
	}
	start, end := c.boundary(containerArea, d.after)-origin, c.boundary(containerArea, d.after+2)-origin
	if end-start < 2*least {
		return true // no room to move it
	}
	offset := min(max(at-origin, start+least), end-least)
	before := c.sharesBefore(d.after)
	pair := c.children[d.after].share + c.children[d.after+1].share
	// Half a cell keeps the truncation in boundary from landing one cell short.
	c.children[d.after].share = (float64(offset)+0.5)/float64(length) - before
	c.children[d.after+1].share = pair - c.children[d.after].share
	return true
}

// Resize grows the pane by cells along direction (shrinks it when cells is negative), as i3's
// resize does: in the nearest container running that way where it has a neighbor, the boundary
// with the next neighbor moves, or with the previous one when it is last. Neighbors keep at least
// minSize. It reports whether there was a boundary to move.
func (t *Tree) Resize(pane PaneID, direction Direction, cells int, area image.Rectangle, minSize image.Point) bool {
	n := t.find(pane)
	for n != nil && n.parent != nil {
		container := n.parent
		if container.stacked || container.direction != direction {
			n = container
			continue
		}
		containerArea, _ := areaOf(container, t.root, area)
		index := n.index()
		divider, boundary := Divider{container: container, after: index}, container.boundary(containerArea, index+1)
		if index == len(container.children)-1 {
			divider, boundary, cells = Divider{container: container, after: index - 1}, container.boundary(containerArea, index), -cells
		}
		return t.MoveDivider(divider, area, image.Pt(boundary+cells, boundary+cells), minSize)
	}
	return false
}
