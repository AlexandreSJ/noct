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
			got, ok := tree.Neighbor(tt.from, tt.heading, screen, nil)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("got (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestNeighborPrefersThePaneFocusedMostRecently(t *testing.T) {
	tree := threePanes(t)
	tests := []struct {
		name   string
		recent []PaneID
		want   PaneID
	}{
		{"the bottom pane, focused last", []PaneID{2, 3}, 3},
		{"the top pane, focused last", []PaneID{3, 2}, 2},
		{"the only one focused before", []PaneID{3}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := tree.Neighbor(1, Right, screen, tt.recent); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
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
// wantRects fails the test unless the panes are at these rects in screen.
func wantRects(t *testing.T, tree *Tree, want map[PaneID]image.Rectangle) {
	t.Helper()
	if got := tree.Rects(screen); !maps.Equal(got, want) {
		t.Errorf("rects %v, want %v", got, want)
	}
}

func TestMoveSidewaysKeepsThePlaceInTheColumn(t *testing.T) {
	tree := threePanes(t) // 1 | [2 over 3]
	if !tree.Move(2, Left) {
		t.Fatal("move failed")
	}
	wantRects(t, tree, map[PaneID]image.Rectangle{
		2: image.Rect(0, 0, 50, 20),
		1: image.Rect(0, 20, 50, 40),
		3: image.Rect(50, 0, 100, 40),
	})

	tree = threePanes(t)
	tree.Move(3, Left) // second in its column, and the next has one pane: its bottom
	if got := tree.Panes(); !slices.Equal(got, []PaneID{1, 3, 2}) {
		t.Errorf("3 did not go under 1: %v", got)
	}
}

func TestMoveSidewaysFromAloneGoesToTheBottomOfTheNextColumn(t *testing.T) {

	tree := threePanes(t)
	tree.Move(1, Right)
	if got := tree.Panes(); !slices.Equal(got, []PaneID{2, 3, 1}) || tree.Rects(screen)[1].Dx() != 100 {
		t.Errorf("1 did not go under 3 in the only column left: %v", tree.Rects(screen))
	}
}

func TestMoveSidewaysDoesNotJustTradePlaces(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.Move(1, Right)
	wantRects(t, tree, map[PaneID]image.Rectangle{
		2: image.Rect(0, 0, 100, 20),
		1: image.Rect(0, 20, 100, 40),
	})
	tree.Move(1, Right) // past the edge of the tab: a column of its own
	wantRects(t, tree, map[PaneID]image.Rectangle{
		2: image.Rect(0, 0, 50, 40),
		1: image.Rect(50, 0, 100, 40),
	})
	if tree.Move(1, Right) {
		t.Error("a column of its own moved past the edge of the tab")
	}
}

// Panes side by side inside a column are columns of their own: moving one sideways takes it to
// the bottom of its neighbor.
func TestPanesSideBySideInAColumnAreColumns(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked)
	tree.Split(1, 3, SideBySide) // [1 | 3] over 2
	tree.Move(1, Right)
	if got := tree.Panes(); !slices.Equal(got, []PaneID{3, 1, 2}) || tree.Rects(screen)[1].Dx() != 100 {
		t.Errorf("1 did not go under 3: %v", tree.Rects(screen))
	}
}

func TestMoveUpOrDownGoesAlongTheColumnThenTakesTheWidth(t *testing.T) {
	tree := threePanes(t)
	tree.Move(2, Down)
	if got := tree.Panes(); !slices.Equal(got, []PaneID{1, 3, 2}) {
		t.Fatalf("2 did not go under 3: %v", got)
	}
	tree.Move(2, Down)
	wantRects(t, tree, map[PaneID]image.Rectangle{
		1: image.Rect(0, 0, 50, 20),
		3: image.Rect(50, 0, 100, 20),
		2: image.Rect(0, 20, 100, 40),
	})
	if tree.Move(2, Down) {
		t.Error("a pane as wide as the tab moved past its bottom")
	}
	tree.Move(3, Up) // out of the row above 2: the width of the tab
	if got := tree.Rects(screen)[3]; got.Min.Y != 0 || got.Dx() != 100 {
		t.Errorf("3 at %v, want across the top", got)
	}
}

func TestAPaneBecomingAColumnSharesTheWidthEvenly(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked)
	tree.Split(2, 3, SideBySide) // 1 across the top, 2 | 3 under it
	tree.Move(1, Right)
	wantRects(t, tree, map[PaneID]image.Rectangle{
		2: image.Rect(0, 0, 33, 40),
		3: image.Rect(33, 0, 66, 40),
		1: image.Rect(66, 0, 100, 40),
	})
}

// Moving the middle of three panes to a column of one leaves two columns of two, every pane the
// same size and the borders lined up, however the columns were built.
func TestMovingSharesTheColumnsEvenly(t *testing.T) {
	area := image.Rect(0, 0, 120, 37)
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.Split(2, 3, Stacked)
	tree.Split(3, 4, Stacked)
	tree.Resize(2, Stacked, 6, area, image.Pt(1, 1)) // sizes the move evens out again
	tree.Move(3, Left)
	want := map[PaneID]image.Rectangle{
		1: image.Rect(0, 0, 60, 18),
		3: image.Rect(0, 18, 60, 37),
		2: image.Rect(60, 0, 120, 18),
		4: image.Rect(60, 18, 120, 37),
	}
	if got := tree.Rects(area); !maps.Equal(got, want) {
		t.Errorf("rects %v, want %v", got, want)
	}

	tree = New(1) // columns put together by moves, whose shares pick up float error
	tree.Split(1, 2, SideBySide)
	tree.Split(2, 3, SideBySide)
	tree.Split(3, 4, SideBySide)
	tree.Move(3, Left)
	tree.Move(4, Left)
	tree.Move(4, Left)
	if got := tree.Rects(area); got[1].Dx() != 60 || got[2].Dx() != 60 || got[1].Dy() != got[2].Dy() {
		t.Errorf("columns of two are not even: %v", got)
	}
}

func TestMoveAcrossTurnsTheTab(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked) // 1 over 2

	if !tree.Move(2, Left) {
		t.Fatal("move failed")
	}
	wantRects(t, tree, map[PaneID]image.Rectangle{
		2: image.Rect(0, 0, 50, 40),
		1: image.Rect(50, 0, 100, 40),
	})
}

func TestMoveIntoAStackShowsThePane(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.Split(2, 3, Stacked)
	tree.Split(3, 4, Stacked) // 1 | [2 over 3 over 4]
	tree.ToggleStacked(2)
	tree.Move(1, Right)
	if got := tree.Panes(); !slices.Equal(got, []PaneID{2, 3, 4, 1}) || tree.Folded(1) {
		t.Errorf("1 did not go to the bottom of the stack, shown: %v, folded %v", got, tree.Folded(1))
	}

	tree = New(1)
	tree.Split(1, 2, Stacked)
	tree.Split(2, 3, Stacked)
	tree.ToggleStacked(2) // 1 over a stack of 2 and 3
	tree.Move(1, Down)
	if tree.Folded(1) || !tree.Folded(2) || !tree.Folded(3) {
		t.Error("moving down did not enter the stack at its top, shown")
	}
	tree.Move(1, Down)
	if got := tree.Panes(); !slices.Equal(got, []PaneID{2, 1, 3}) || tree.Folded(1) {
		t.Errorf("moving down the stack: %v, 1 folded %v", got, tree.Folded(1))
	}
}

func TestAStackLosingItsPaneShownShowsTheNext(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked)
	tree.Split(2, 3, Stacked)
	tree.ToggleStacked(1)
	tree.Show(2)
	tree.Remove(2)
	if tree.Folded(3) {
		t.Error("the stack did not show 3, which took 2's place")
	}
}

func TestStackHeirIsThePaneShownAfterTheRemove(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked)
	tree.Split(2, 3, Stacked)
	tree.ToggleStacked(1)
	for _, c := range []struct {
		shown, removed, heir PaneID
	}{{2, 2, 3}, {3, 3, 2}, {1, 1, 2}, {1, 2, 1}} {
		tree.Show(c.shown)
		if heir, ok := tree.StackHeir(c.removed); !ok || heir != c.heir {
			t.Errorf("showing %d, removing %d: heir %d (%v), want %d", c.shown, c.removed, heir, ok, c.heir)
		}
	}
	if _, ok := New(1).StackHeir(1); ok {
		t.Error("a pane outside any stack has an heir")
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

// A column of three panes turns into a stack from its top: the pane shown takes the height, the
// others fold to one row above and below it, and toggling again gives the split back.
func TestStackFoldsAllButTheShownPane(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked)
	tree.Split(2, 3, Stacked)
	if !tree.ToggleStacked(1) {
		t.Fatal("toggle failed")
	}
	tree.Show(2)
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
	if got := tree.Rects(screen)[1]; got.Dx() != 100 || got.Dy() != 13 {
		t.Errorf("unstacked: pane 1 is %v, want a third of the height", got)
	}
}

func TestStackTakesThePaneAndThoseBelowIt(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked)
	tree.Split(2, 3, Stacked)
	tree.Split(3, 4, Stacked) // a column of 1 to 4, ten rows each
	if !tree.ToggleStacked(2) {
		t.Fatal("toggle failed")
	}
	wantRects(t, tree, map[PaneID]image.Rectangle{
		1: image.Rect(0, 0, 100, 10),
		2: image.Rect(0, 10, 100, 38),
		3: image.Rect(0, 38, 100, 39),
		4: image.Rect(0, 39, 100, 40),
	})
	tree = New(1)
	tree.Split(1, 2, Stacked)
	tree.Split(2, 3, Stacked)
	tree.Split(3, 4, Stacked)
	tree.ToggleStacked(2)
	tree.ToggleStacked(3)
	if got := tree.Rects(screen); got[1].Dy() != 10 || got[4].Dy() != 10 || tree.Folded(3) {
		t.Errorf("unstacking did not give the column back: %v", got)
	}
	if tree.ToggleStacked(4) {
		t.Error("stacked the last pane of the column, with nothing below it")
	}
}

func TestStackTakesNewPanesAndMovesAlongIt(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, Stacked)
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

func TestStackLeavesOtherColumnsAlone(t *testing.T) {
	tree := threePanes(t) // 1 | [2 over 3]
	if tree.ToggleStacked(1) {
		t.Error("stacked a pane alone in its column, and so the whole tab")
	}
	if !tree.ToggleStacked(2) || tree.Rects(screen)[1] != image.Rect(0, 0, 50, 40) || !tree.Folded(3) {
		t.Errorf("stacking 2's column: %v", tree.Rects(screen))
	}
}

func TestNeighborGoesToThePaneAStackShows(t *testing.T) {
	tree := threePanes(t)
	tree.ToggleStacked(2)
	tree.Show(3)
	if got, _ := tree.Neighbor(1, Right, screen, []PaneID{3, 2}); got != 3 {
		t.Errorf("got the folded %d, want 3, the pane shown", got)
	}
}
