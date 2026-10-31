package layout

import "image"

// Rects divides area among the panes.
func (t *Tree) Rects(area image.Rectangle) map[PaneID]image.Rectangle {
	rects := make(map[PaneID]image.Rectangle)
	if t.root != nil {
		collectRects(t.root, area, rects)
	}
	return rects
}

func collectRects(n *node, area image.Rectangle, rects map[PaneID]image.Rectangle) {
	if n.isLeaf() {
		rects[n.pane] = area
		return
	}
	for i, child := range n.children {
		collectRects(child, n.childArea(area, i), rects)
	}
}

// childArea is the part of a container's area that its child i takes. Boundaries come from the
// shares summed so far, so rounding never leaves a gap.
func (n *node) childArea(area image.Rectangle, i int) image.Rectangle {
	if n.stacked {
		return n.stackedArea(area, i)
	}
	start, end := n.boundary(area, i), n.boundary(area, i+1)
	if n.direction == SideBySide {
		return image.Rect(start, area.Min.Y, end, area.Max.Y)
	}
	return image.Rect(area.Min.X, start, area.Max.X, end)
}

// stackedArea gives each child of a stack one row, in order, and the rest to the child shown.
func (n *node) stackedArea(area image.Rectangle, i int) image.Rectangle {
	shown, count := n.shownIndex(), len(n.children)
	top := area.Min.Y + i
	if i > shown {
		top = area.Max.Y - (count - i)
	}
	bottom := top + 1
	if i == shown {
		bottom = area.Max.Y - (count - 1 - i)
	}
	top, bottom = min(max(top, area.Min.Y), area.Max.Y), min(max(bottom, area.Min.Y), area.Max.Y)
	return image.Rect(area.Min.X, top, area.Max.X, max(bottom, top))
}

// boundary is where child i starts, along the container's direction.
func (n *node) boundary(area image.Rectangle, i int) int {
	origin, length := span(area, n.direction)
	if i >= len(n.children) {
		return origin + length
	}
	// The shares of n children sum to a hair under k/n at times; rounding that up keeps equal
	// neighbors equal and lines up the boundaries of containers split alike.
	return origin + int(float64(length)*n.sharesBefore(i)+1e-9)
}

// sharesBefore is the part of the container's length its children before child i take.
func (n *node) sharesBefore(i int) float64 {
	sum := 0.0
	for _, child := range n.children[:i] {
		sum += child.share
	}
	return sum
}

func span(area image.Rectangle, direction Direction) (origin, length int) {
	if direction == SideBySide {
		return area.Min.X, area.Dx()
	}
	return area.Min.Y, area.Dy()
}

// areaOf finds the area of target inside n, which covers area.
func areaOf(target, n *node, area image.Rectangle) (image.Rectangle, bool) {
	if n == target {
		return area, true
	}
	for i, child := range n.children {
		if found, ok := areaOf(target, child, n.childArea(area, i)); ok {
			return found, true
		}
	}
	return image.Rectangle{}, false
}
