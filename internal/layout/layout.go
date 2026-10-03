// Package layout arranges the panes of a tab the way i3 does: a tree of containers, each laying
// its children side by side or stacked, with a share of the container for each child. A container
// can also be a stack, as in zellij: one child shown whole, the others folded to a title row.
package layout

import (
	"slices"
)

type PaneID int

type Direction int

const (
	SideBySide Direction = iota
	Stacked
)

type Heading int

const (
	Left Heading = iota
	Right
	Up
	Down
)

// direction is the orientation a heading moves along.
func (h Heading) direction() Direction {
	if h == Left || h == Right {
		return SideBySide
	}
	return Stacked
}

// step is -1 toward the start of a container (left, up) and 1 toward its end.
func (h Heading) step() int {
	if h == Left || h == Up {
		return -1
	}
	return 1
}

// Tree is a tab's layout. A leaf holds a pane; a container holds two or more children, laid out
// in its direction. No container has a child with its own direction: that child's children
// belong to the container directly, as in i3.
type Tree struct {
	root *node
}

type node struct {
	pane      PaneID
	direction Direction
	children  []*node
	// share is the part of the parent's length this node takes; a container's shares sum to 1.
	share  float64
	parent *node
	// stacked folds every child but shown to one row; direction is kept for when it unfolds.
	stacked bool
	shown   *node
}

func (n *node) isLeaf() bool {
	return len(n.children) == 0
}

func (n *node) index() int {
	return slices.Index(n.parent.children, n)
}

// axis is the way a container lays out its children: a stack runs top to bottom.
func (n *node) axis() Direction {
	if n.stacked {
		return Stacked
	}
	return n.direction
}

func New(pane PaneID) *Tree {
	return &Tree{root: &node{pane: pane, share: 1}}
}

// Split puts newPane right after target in the given direction. When target's container already
// runs that way, or is a stack, the new pane joins it, taking an equal part from each child, and
// shows in the stack; otherwise target becomes a container of the two. It reports whether target
// was found.
func (t *Tree) Split(target, newPane PaneID, direction Direction) bool {
	leaf := t.find(target)
	if leaf == nil {
		return false
	}
	parent := leaf.parent
	if parent == nil || !parent.stacked && parent.direction != direction {
		parent = t.wrap(leaf, direction)
	}
	added := &node{pane: newPane}
	insert(parent, leaf.index()+1, added)
	parent.shown = added
	return true
}

// ToggleStacked turns the container of the pane into a stack showing the pane, or back into
// a split. It reports false when the pane is alone in the tab or not in it.
func (t *Tree) ToggleStacked(pane PaneID) bool {
	leaf := t.find(pane)
	if leaf == nil || leaf.parent == nil {
		return false
	}
	leaf.parent.stacked = !leaf.parent.stacked
	leaf.parent.shown = leaf
	return true
}

// Show unfolds the pane in every stack it is in.
func (t *Tree) Show(pane PaneID) {
	for n := t.find(pane); n != nil && n.parent != nil; n = n.parent {
		n.parent.shown = n
	}
}

// Folded tells whether the pane is folded to its title row in a stack.
func (t *Tree) Folded(pane PaneID) bool {
	for n := t.find(pane); n != nil && n.parent != nil; n = n.parent {
		if n.parent.stacked && n.parent.shownIndex() != n.index() {
			return true
		}
	}
	return false
}

// shownIndex is the child a stack shows whole: the first when the one shown has left.
func (n *node) shownIndex() int {
	return max(slices.Index(n.children, n.shown), 0)
}

// wrap turns a leaf into a container of the given direction holding it, and returns the container.
func (t *Tree) wrap(leaf *node, direction Direction) *node {
	container := &node{direction: direction, share: leaf.share, parent: leaf.parent}
	t.replace(leaf, container)
	leaf.parent, leaf.share = container, 1
	container.children = []*node{leaf}
	return container
}

// replace puts with where old is, in old's parent or as the root.
func (t *Tree) replace(old, with *node) {
	if old.parent == nil {
		t.root = with
		return
	}
	old.parent.children[old.index()] = with
}

// insert adds child to container at index, taking an equal part of the length from the others.
func insert(container *node, index int, child *node) {
	count := float64(len(container.children))
	for _, sibling := range container.children {
		sibling.share *= count / (count + 1)
	}
	child.parent, child.share = container, 1/(count+1)
	container.children = slices.Insert(container.children, index, child)
}

// Swap exchanges the places of two panes. It reports whether both were found.
func (t *Tree) Swap(first, second PaneID) bool {
	a, b := t.find(first), t.find(second)
	if a == nil || b == nil {
		return false
	}
	a.pane, b.pane = b.pane, a.pane
	return true
}

// Remove deletes the pane; its siblings share its length. It reports whether the tree is now empty.
func (t *Tree) Remove(pane PaneID) (empty bool) {
	leaf := t.find(pane)
	if leaf == nil {
		return t.root == nil
	}
	if leaf.parent == nil {
		t.root = nil
		return true
	}
	t.detach(leaf)
	return false
}

// detach takes a node out of its container, giving its length to the siblings in proportion, and
// dissolves a container left with a single child.
func (t *Tree) detach(n *node) {
	container := n.parent
	container.children = slices.Delete(container.children, n.index(), n.index()+1)
	for _, sibling := range container.children {
		sibling.share /= 1 - n.share
	}
	n.parent = nil
	if len(container.children) == 1 {
		t.dissolve(container)
	}
}

// dissolve replaces a container that holds one child with that child. When the child is itself a
// container running like its new parent, its children join that parent directly.
func (t *Tree) dissolve(container *node) {
	child := container.children[0]
	child.share, child.parent = container.share, container.parent
	t.replace(container, child)
	parent := child.parent
	if parent == nil || child.isLeaf() || child.direction != parent.direction || child.stacked || parent.stacked {
		return
	}
	at := child.index()
	for _, grandchild := range child.children {
		grandchild.share *= child.share
		grandchild.parent = parent
	}
	parent.children = slices.Replace(parent.children, at, at+1, child.children...)
}

// Move moves the pane one step toward heading, as i3's move does. Within a container running
// that way it trades places with the neighbor, or enters the neighbor when that is a container.
// At a container's edge it leaves it and lands beside it in the next container out that runs
// that way. When no container runs that way, the whole tab turns that way with the pane at its
// edge. It reports false when the pane is already at the tab's edge.
func (t *Tree) Move(pane PaneID, heading Heading) bool {
	leaf := t.find(pane)
	if leaf == nil || leaf.parent == nil {
		return false
	}
	direction, step := heading.direction(), heading.step()
	for n := leaf; n.parent != nil; n = n.parent {
		container := n.parent
		next := n.index() + step
		if container.axis() != direction || next < 0 || next >= len(container.children) {
			continue
		}
		if n == leaf {
			t.moveIntoNeighbor(leaf, container.children[next], step)
		} else {
			t.relocate(leaf, container, n.index()+max(step, 0))
		}
		return true
	}
	if t.root.axis() == direction {
		return false // already at the edge of a tab that runs this way
	}
	t.detach(leaf)
	outer := t.wrap(t.root, direction)
	insert(outer, max(step, 0), leaf)
	return true
}

// moveIntoNeighbor trades places with a neighbor pane, or enters a neighbor container at the side
// facing the pane.
func (t *Tree) moveIntoNeighbor(leaf, neighbor *node, step int) {
	if neighbor.isLeaf() {
		container := leaf.parent
		i, j := leaf.index(), neighbor.index()
		container.children[i], container.children[j] = neighbor, leaf
		leaf.share, neighbor.share = neighbor.share, leaf.share
		return
	}
	at := len(neighbor.children)
	if step > 0 {
		at = 0
	}
	t.relocate(leaf, neighbor, at)
}

// relocate moves leaf into container at index. The place is reserved first: taking the leaf out
// may dissolve its old container and merge nodes around the destination.
func (t *Tree) relocate(leaf, container *node, index int) {
	slot := &node{}
	insert(container, index, slot)
	t.detach(leaf)
	leaf.parent, leaf.share = slot.parent, slot.share
	slot.parent.children[slot.index()] = leaf
}

// Panes lists pane IDs in reading order: left to right, top to bottom.
func (t *Tree) Panes() []PaneID {
	var panes []PaneID
	t.walk(t.root, func(leaf *node) { panes = append(panes, leaf.pane) })
	return panes
}

func (t *Tree) Contains(pane PaneID) bool {
	return t.find(pane) != nil
}

func (t *Tree) find(pane PaneID) *node {
	var found *node
	t.walk(t.root, func(leaf *node) {
		if leaf.pane == pane {
			found = leaf
		}
	})
	return found
}

func (t *Tree) walk(n *node, visit func(leaf *node)) {
	if n == nil {
		return
	}
	if n.isLeaf() {
		visit(n)
		return
	}
	for _, child := range n.children {
		t.walk(child, visit)
	}
}
