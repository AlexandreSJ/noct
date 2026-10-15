package layout

import (
	"image"
	"reflect"
	"testing"
)

func TestShapeRebuildsTheSameLayout(t *testing.T) {
	tree := New(1)
	tree.Split(1, 2, SideBySide)
	tree.Split(2, 3, Stacked)
	tree.ToggleStacked(2)
	area := image.Rect(0, 0, 120, 40)
	if !tree.Resize(1, SideBySide, 20, area, image.Pt(12, 5)) {
		t.Fatal("resizing did not change the layout")
	}

	rebuilt, err := FromShape(tree.Shape(), func(saved PaneID) PaneID { return saved + 10 })
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[PaneID]image.Rectangle)
	for id, rect := range tree.Rects(area) {
		want[id+10] = rect
	}
	if got := rebuilt.Rects(area); !reflect.DeepEqual(got, want) {
		t.Errorf("rebuilt rects %v, want %v", got, want)
	}
	if rebuilt.Folded(12) || !rebuilt.Folded(13) {
		t.Error("the stack does not show the same pane")
	}
}

func TestFromShapeRejectsBrokenShapes(t *testing.T) {
	same := func(id PaneID) PaneID { return id }
	for name, shape := range map[string]Shape{
		"no pane":       {},
		"repeated pane": {Children: []Shape{{Pane: 1}, {Pane: 1}}},
		"lone child":    {Children: []Shape{{Pane: 1}}},
	} {
		if _, err := FromShape(shape, same); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestFromShapeFixesShares(t *testing.T) {
	shape := Shape{Children: []Shape{{Pane: 1, Share: 0}, {Pane: 2, Share: 3}}}
	tree, err := FromShape(shape, func(id PaneID) PaneID { return id })
	if err != nil {
		t.Fatal(err)
	}
	rects := tree.Rects(image.Rect(0, 0, 100, 10))
	if rects[1].Dx() != 50 || rects[2].Dx() != 50 {
		t.Errorf("rects %v, want an even split", rects)
	}
}
