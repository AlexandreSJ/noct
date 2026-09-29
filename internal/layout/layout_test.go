package layout

import (
	"image"
	"maps"
	"slices"
	"testing"
)

var screen = image.Rect(0, 0, 100, 40)

// threePanes builds: pane 1 on the left, panes 2 (top) and 3 (bottom) on the right.
func threePanes(t *testing.T) *Tree {
	t.Helper()
	tree := New(1)
	if !tree.Split(1, 2, SideBySide) || !tree.Split(2, 3, Stacked) {
		t.Fatal("split failed")
	}
	return tree
}

func TestRectsDivideTheArea(t *testing.T) {
	rects := threePanes(t).Rects(screen)

	want := map[PaneID]image.Rectangle{
		1: image.Rect(0, 0, 50, 40),
		2: image.Rect(50, 0, 100, 20),
		3: image.Rect(50, 20, 100, 40),
	}
	for pane, rect := range want {
		if rects[pane] != rect {
			t.Errorf("pane %d: got %v, want %v", pane, rects[pane], rect)
		}
	}
}

func TestPanesAreInReadingOrder(t *testing.T) {
	if got := threePanes(t).Panes(); !slices.Equal(got, []PaneID{1, 2, 3}) {
		t.Errorf("got %v", got)
	}
}

func TestSplitUnknownPaneFails(t *testing.T) {
	if New(1).Split(9, 2, Stacked) {
		t.Error("split of a missing pane succeeded")
	}
}

func TestRemoveGivesAreaToSibling(t *testing.T) {
	tree := threePanes(t)

	if empty := tree.Remove(2); empty {
		t.Fatal("tree reported empty")
	}
	rects := tree.Rects(screen)
	if rects[3] != image.Rect(50, 0, 100, 40) {
		t.Errorf("pane 3 did not take the freed area: %v", rects[3])
	}
	if tree.Contains(2) {
		t.Error("pane 2 still in the tree")
	}
}

func TestRemoveLastPaneEmptiesTree(t *testing.T) {
	tree := New(1)
	if empty := tree.Remove(1); !empty {
		t.Error("tree not empty after removing its only pane")
	}
	if len(tree.Rects(screen)) != 0 {
		t.Error("empty tree still has rects")
	}
}

func TestNeighbor(t *testing.T) {
	tree := threePanes(t)
	tests := []struct {
		name    string
		from    PaneID
		heading Heading
		want    PaneID
		wantOK  bool
	}{
		{"right of the left pane is the top one", 1, Right, 2, true},
		{"left of the bottom pane", 3, Left, 1, true},
		{"below the top pane", 2, Down, 3, true},
		{"above the bottom pane", 3, Up, 2, true},
		{"nothing left of the left pane", 1, Left, 0, false},
		{"nothing above the top pane", 2, Up, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tree.Neighbor(tt.from, tt.heading, screen)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("got (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestSwapExchangesPlaces(t *testing.T) {
	tree := threePanes(t)
	before := tree.Rects(screen)
	if !tree.Swap(1, 3) {
		t.Fatal("swap failed")
	}
	after := tree.Rects(screen)
	if after[1] != before[3] || after[3] != before[1] || after[2] != before[2] {
		t.Errorf("before %v, after %v", before, after)
	}
	if tree.Swap(1, 9) {
		t.Error("swap with a missing pane succeeded")
	}
}

func TestDraggingADividerResizesBothSides(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	area := image.Rect(0, 1, 100, 31)

	divider, ok := tree.DividerAt(area, image.Pt(49, 10))
	if !ok {
		t.Fatal("no divider where the frames of the two panes meet")
	}
	if _, ok := tree.DividerAt(area, image.Pt(20, 10)); ok {
		t.Error("found a divider inside a pane")
	}
	tree.MoveDivider(divider, area, image.Pt(30, 10), image.Pt(12, 5))
	rects := tree.Rects(area)
	if rects[1].Max.X != 30 || rects[2].Min.X != 30 {
		t.Errorf("after dragging to 30: %v %v", rects[1], rects[2])
	}
	tree.MoveDivider(divider, area, image.Pt(95, 10), image.Pt(12, 5))
	if rects := tree.Rects(area); rects[2].Dx() != 12 {
		t.Errorf("dragging to the edge left the second pane %d wide, want the minimum 12", rects[2].Dx())
	}
}

func TestDividerAtFindsNestedSplits(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.Split(2, 3, Stacked)
	area := image.Rect(0, 0, 100, 30)

	divider, ok := tree.DividerAt(area, image.Pt(70, 15))
	if !ok {
		t.Fatal("no divider between the stacked panes")
	}
	tree.MoveDivider(divider, area, image.Pt(70, 20), image.Pt(12, 5))
	if rects := tree.Rects(area); rects[2].Max.Y != 20 || rects[1].Dy() != 30 {
		t.Errorf("rects %v", rects)
	}
}

func TestSplitAlongTheContainerAddsASibling(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.Split(2, 3, SideBySide)
	rects := tree.Rects(image.Rect(0, 0, 90, 30))
	for pane, x := range map[PaneID]int{1: 0, 2: 30, 3: 60} {
		if rects[pane].Min.X != x || rects[pane].Dx() != 30 {
			t.Errorf("pane %d at %v, want a third starting at %d", pane, rects[pane], x)
		}
	}
}

// The example from i3: pane 1 on the left, 2 above 3 on the right.
func TestMoveLeavesAContainerAndLandsBesideIt(t *testing.T) {
	tree := threePanes(t)

	if !tree.Move(2, Left) {
		t.Fatal("move failed")
	}
	rects := tree.Rects(image.Rect(0, 0, 90, 30))
	want := map[PaneID]image.Rectangle{
		1: image.Rect(0, 0, 30, 30),
		2: image.Rect(30, 0, 60, 30),
		3: image.Rect(60, 0, 90, 30),
	}
	for pane, rect := range want {
		if rects[pane] != rect {
			t.Errorf("after moving 2 left, pane %d at %v, want %v", pane, rects[pane], rect)
		}
	}

	tree.Move(2, Left)
	if got := tree.Panes(); !slices.Equal(got, []PaneID{2, 1, 3}) {
		t.Errorf("moving again traded places: got %v", got)
	}
	if tree.Move(2, Left) {
		t.Error("moved past the edge of the tab")
	}
}

func TestMoveIntoANeighborContainer(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.Split(2, 3, Stacked)
	tree.Split(1, 4, SideBySide) // 1 | 4 | [2 over 3]

	tree.Move(4, Right)
	rects := tree.Rects(image.Rect(0, 0, 100, 30))
	if rects[4].Min.X != rects[2].Min.X || rects[4].Min.Y != 0 || rects[2].Min.Y <= rects[4].Min.Y {
		t.Errorf("4 did not enter the column on the right at its top: %v", rects)
	}
}

func TestMoveAcrossTurnsTheTab(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked) // 1 over 2

	if !tree.Move(2, Left) {
		t.Fatal("move failed")
	}
	rects := tree.Rects(image.Rect(0, 0, 100, 30))
	if rects[2] != image.Rect(0, 0, 50, 30) || rects[1] != image.Rect(50, 0, 100, 30) {
		t.Errorf("rects %v, want 2 on the left and 1 on the right", rects)
	}
}

func TestMovingKeepsSharesWhole(t *testing.T) {
	tree := threePanes(t)
	tree.Split(3, 4, SideBySide)
	for _, move := range []struct {
		pane    PaneID
		heading Heading
	}{{4, Up}, {4, Left}, {1, Down}, {3, Right}, {2, Up}, {4, Down}} {
		tree.Move(move.pane, move.heading)
		total := 0
		for _, rect := range tree.Rects(screen) {
			total += rect.Dx() * rect.Dy()
		}
		if total != screen.Dx()*screen.Dy() || len(tree.Panes()) != 4 {
			t.Fatalf("after moving %d %v: panes %v cover %d of %d cells", move.pane, move.heading, tree.Panes(), total, screen.Dx()*screen.Dy())
		}
	}
}

func TestResizeMovesTheBoundaryWithTheNeighbor(t *testing.T) {
	tree := threePanes(t) // 1 | [2 over 3]
	area := image.Rect(0, 0, 100, 40)
	min := image.Pt(12, 5)

	tree.Resize(1, SideBySide, 10, area, min)
	if rects := tree.Rects(area); rects[1].Dx() != 60 || rects[2].Dx() != 40 {
		t.Fatalf("growing 1 by 10: %v", rects)
	}
	tree.Resize(2, SideBySide, 5, area, min) // 2 is last in its row: it grows to the left
	if rects := tree.Rects(area); rects[2].Min.X != 55 || rects[3].Min.X != 55 {
		t.Fatalf("growing 2 by 5: %v", rects)
	}
	tree.Resize(3, Stacked, -4, area, min)
	if rects := tree.Rects(area); rects[3].Dy() != 16 || rects[2].Dy() != 24 {
		t.Fatalf("shrinking 3 by 4: %v", rects)
	}
	if tree.Resize(1, Stacked, 3, area, min) {
		t.Error("resized 1 in a direction it has no neighbor")
	}
}

// Three panes side by side turn into a stack: the pane shown takes the height, the others fold
// to one row above and below it, and toggling again gives the split back.
func TestStackFoldsAllButTheShownPane(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.Split(2, 3, SideBySide)
	if !tree.ToggleStacked(2) {
		t.Fatal("toggle failed")
	}
	want := map[PaneID]image.Rectangle{
		1: image.Rect(0, 0, 100, 1),
		2: image.Rect(0, 1, 100, 39),
		3: image.Rect(0, 39, 100, 40),
	}
	if got := tree.Rects(screen); !maps.Equal(got, want) {
		t.Errorf("stacked on 2: got %v, want %v", got, want)
	}
	if !tree.Folded(1) || tree.Folded(2) || !tree.Folded(3) {
		t.Error("folded panes are not 1 and 3")
	}
	tree.Show(3)
	if got := tree.Rects(screen)[3]; got != image.Rect(0, 2, 100, 40) {
		t.Errorf("showing 3: got %v", got)
	}
	tree.ToggleStacked(3)
	if got := tree.Rects(screen)[1]; got.Dx() != 33 || got.Dy() != 40 {
		t.Errorf("unstacked: pane 1 is %v, want a third of the width", got)
	}
}

func TestStackTakesNewPanesAndMovesAlongIt(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.ToggleStacked(1)
	tree.Split(1, 3, SideBySide) // a stack takes the pane whatever the direction
	if got := tree.Panes(); !slices.Equal(got, []PaneID{1, 3, 2}) {
		t.Fatalf("panes = %v", got)
	}
	if tree.Folded(3) {
		t.Error("the new pane is folded")
	}
	if !tree.Move(3, Down) || !slices.Equal(tree.Panes(), []PaneID{1, 2, 3}) {
		t.Errorf("moving down the stack: %v", tree.Panes())
	}
	if _, ok := tree.DividerAt(screen, image.Pt(50, 1)); ok {
		t.Error("a stack has a divider")
	}
	if tree.ToggleStacked(4) || New(1).ToggleStacked(1) {
		t.Error("toggled a pane that is missing or alone")
	}
}
