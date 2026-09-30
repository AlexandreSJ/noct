package layout

import (
	"errors"
	"fmt"
)

// Shape is a tree as plain data, to save a tab's layout and build it again later. A shape with
// children is a container; one without is a pane.
type Shape struct {
	Pane      PaneID    `json:"pane,omitempty"`
	Direction Direction `json:"direction,omitempty"`
	Stacked   bool      `json:"stacked,omitempty"`
	// Shown is the child a stack shows whole.
	Shown    int     `json:"shown,omitempty"`
	Share    float64 `json:"share"`
	Children []Shape `json:"children,omitempty"`
}

// Shape gives the tree as plain data; an empty tree gives an empty shape.
func (t *Tree) Shape() Shape {
	if t.root == nil {
		return Shape{}
	}
	return t.root.shape()
}

func (n *node) shape() Shape {
	if n.isLeaf() {
		return Shape{Pane: n.pane, Share: n.share}
	}
	s := Shape{Direction: n.direction, Stacked: n.stacked, Shown: n.shownIndex(), Share: n.share}
	for _, child := range n.children {
		s.Children = append(s.Children, child.shape())
	}
	return s
}

// FromShape builds the tree a shape describes, giving each pane the ID newID returns for it, as
// a saved layout's panes start again under new IDs. Shares are scaled to fill each container.
func FromShape(s Shape, newID func(saved PaneID) PaneID) (*Tree, error) {
	seen := make(map[PaneID]bool)
	root, err := buildNode(s, nil, newID, seen)
	if err != nil {
		return nil, err
	}
	root.share = 1
	return &Tree{root: root}, nil
}

func buildNode(s Shape, parent *node, newID func(PaneID) PaneID, seen map[PaneID]bool) (*node, error) {
	n := &node{parent: parent, share: s.Share}
	if len(s.Children) == 0 {
		if s.Pane <= 0 || seen[s.Pane] {
			return nil, fmt.Errorf("layout: pane %d is missing or repeated", s.Pane)
		}
		seen[s.Pane] = true
		n.pane = newID(s.Pane)
		return n, nil
	}
	if len(s.Children) == 1 {
		return nil, errors.New("layout: a container holds a single pane")
	}
	n.direction, n.stacked = s.Direction, s.Stacked
	for _, childShape := range s.Children {
		child, err := buildNode(childShape, n, newID, seen)
		if err != nil {
			return nil, err
		}
		n.children = append(n.children, child)
	}
	fillContainer(n.children)
	n.shown = n.children[min(max(s.Shown, 0), len(n.children)-1)]
	return n, nil
}

// fillContainer scales saved shares so they sum to 1, splitting evenly when one cannot be trusted.
func fillContainer(children []*node) {
	total := 0.0
	for _, child := range children {
		if child.share <= 0 {
			total = 0
			break
		}
		total += child.share
	}
	for _, child := range children {
		if total > 0 {
			child.share /= total
		} else {
			child.share = 1 / float64(len(children))
		}
	}
}
