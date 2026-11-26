package app

import (
	"image"
	"slices"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

var (
	ctrlZ      = uv.KeyPressEvent{Code: 'z', Mod: uv.ModCtrl}
	ctrlY      = uv.KeyPressEvent{Code: 'y', Mod: uv.ModCtrl}
	ctrlS      = uv.KeyPressEvent{Code: 's', Mod: uv.ModCtrl}
	ctrlDelete = uv.KeyPressEvent{Code: uv.KeyDelete, Mod: uv.ModCtrl}
	ctrlDown   = uv.KeyPressEvent{Code: uv.KeyDown, Mod: uv.ModCtrl}
	ctrlRight  = uv.KeyPressEvent{Code: uv.KeyRight, Mod: uv.ModCtrl}
	moveUp     = uv.KeyPressEvent{Code: 'k', Mod: uv.ModShift, Text: "K"}
	moveDown   = uv.KeyPressEvent{Code: 'j', Mod: uv.ModShift, Text: "J"}
)

func testGrid() *editGrid {
	skipped := textCellOf("x")
	skipped.skip = true
	return &editGrid{rows: []gridRow{
		{cells: []gridCell{optionCellOf("text", []string{"text", "number", "option", "object"}), textCellOf("ab"), skipped, {kind: toggleCell}}},
		{cells: []gridCell{textCellOf(""), textCellOf("cd"), textCellOf(""), {kind: toggleCell}}},
	}, buttons: []string{"+ add"}, arranged: true}
}

func pressGrid(g *editGrid, keys ...uv.KeyPressEvent) gridOutcome {
	out := gridNothing
	for _, key := range keys {
		out |= g.handleKey(key)
	}
	return out
}

func typeGrid(g *editGrid, text string) {
	for _, r := range text {
		g.handleKey(letter(r))
	}
}

// Tab and shift+tab go through the cells in reading order, past the ones that do not apply, then
// onto the buttons, and stay at either end.
func TestTheGridStepsOverCellsThatDoNotApply(t *testing.T) {
	g := testGrid()
	var visited [][2]int
	for range 9 {
		visited = append(visited, [2]int{g.row, g.col})
		g.handleKey(tabPress)
	}
	want := [][2]int{{0, 0}, {0, 1}, {0, 3}, {1, 0}, {1, 1}, {1, 2}, {1, 3}, {2, 0}, {2, 0}}
	if !slices.Equal(visited, want) {
		t.Fatalf("tab visited %v, want %v", visited, want)
	}
	g.focus(1, 0)
	g.handleKey(backTab)
	if g.row != 0 || g.col != 3 {
		t.Errorf("shift+tab from the second row's first cell went to %d,%d", g.row, g.col)
	}
}

// hjkl and the arrows move a cell at a time, never past a row's ends; with ctrl they go to the
// last row or cell.
func TestTheGridMovesWithHJKLAndTheArrows(t *testing.T) {
	g := testGrid()
	pressGrid(g, letter('l'), rightPress)
	if g.row != 0 || g.col != 3 {
		t.Fatalf("l then right went to %d,%d, want 0,3 past the cell that does not apply", g.row, g.col)
	}
	pressGrid(g, letter('l'))
	if g.col != 3 {
		t.Errorf("l at the row's end went on to %d", g.col)
	}
	pressGrid(g, letter('j'), letter('h'))
	if g.row != 1 || g.col != 2 {
		t.Errorf("j then h went to %d,%d", g.row, g.col)
	}
	pressGrid(g, letter('k'))
	if g.row != 0 || g.col != 1 {
		t.Errorf("k onto a cell that does not apply went to %d,%d, want the nearest", g.row, g.col)
	}
	pressGrid(g, ctrlDown)
	if _, onButton := g.button(); !onButton {
		t.Errorf("ctrl+down went to %d, want the button last", g.row)
	}
	g.focus(1, 0)
	pressGrid(g, ctrlRight)
	if g.col != 3 {
		t.Errorf("ctrl+right went to %d", g.col)
	}
}

// Enter types in a text cell, where every letter types, hjkl too; enter again keeps the text, and
// tells it changed only when it did.
func TestTheGridTypesOnlyAfterEnter(t *testing.T) {
	g := testGrid()
	g.focus(0, 1)
	if out := pressGrid(g, letter('j')); out != gridNothing || g.row != 1 || g.at(0, 1).value() != "ab" {
		t.Fatal("j before enter typed instead of moving down")
	}
	g.focus(0, 1)
	pressGrid(g, enterPress)
	typeGrid(g, "jk")
	if !g.typing() || g.at(0, 1).value() != "abjk" {
		t.Fatalf("typed %q", g.at(0, 1).value())
	}
	if out := pressGrid(g, enterPress); out != gridChanged || g.editing {
		t.Errorf("enter after typing: %v, editing %v", out, g.editing)
	}
	if out := pressGrid(g, enterPress, escPress); out != gridNothing || len(g.history) != 1 {
		t.Errorf("an edit that changed nothing: %v, %d undos", out, len(g.history))
	}
}

// Enter opens an option cell's options under it, the filter narrows them, and enter picks one.
func TestTheGridPicksAnOptionFromItsMenu(t *testing.T) {
	g := testGrid()
	pressGrid(g, enterPress)
	if g.menu == nil {
		t.Fatal("enter on an option cell opened no menu")
	}
	if out := pressGrid(g, letter('j'), letter('j'), enterPress); out != gridChanged || g.at(0, 0).value() != "option" {
		t.Fatalf("j j enter picked %q, %v", g.at(0, 0).value(), out)
	}
	pressGrid(g, enterPress, letter('/'))
	typeGrid(g, "obj")
	if shown := g.menu.shown(); len(shown) != 1 || g.menu.options[shown[0]] != "object" {
		t.Fatalf("the filter kept %v", shown)
	}
	pressGrid(g, enterPress, enterPress)
	if g.menu != nil || g.at(0, 0).value() != "object" {
		t.Errorf("picked %q from the filtered menu", g.at(0, 0).value())
	}
	pressGrid(g, enterPress, letter('k'), escPress)
	if g.menu != nil || g.at(0, 0).value() != "object" {
		t.Error("esc closed the menu changing the value")
	}
	g.focus(0, 3)
	if out := pressGrid(g, spacePress); out != gridChanged || !g.at(0, 3).on {
		t.Error("space did not flip the toggle")
	}
}

// A list cell's items are a grid of their own: + add types a new one, space marks the default,
// ctrl+delete removes one, and esc puts them back in the cell.
func TestTheGridEditsAListCellsItems(t *testing.T) {
	g := &editGrid{rows: []gridRow{{cells: []gridCell{listCellOf([]string{"dev", "prod"}, 0, true, "option")}}}}
	pressGrid(g, enterPress)
	if g.list == nil || g.list.grid.row != 0 {
		t.Fatal("enter on a list cell opened no list on its marked item")
	}
	pressGrid(g, letter('j'), letter('j'), enterPress)
	typeGrid(g, "stage")
	pressGrid(g, enterPress, spacePress)
	if got := g.list.marked(); got != 2 {
		t.Fatalf("space marked row %d, want the new one", got)
	}
	pressGrid(g, letter('k'), ctrlDelete)
	if out := pressGrid(g, escPress); out != gridChanged {
		t.Fatalf("closing a changed list told %v", out)
	}
	c := g.at(0, 0)
	if !slices.Equal(c.items, []string{"dev", "stage"}) || c.markedItem() != "stage" {
		t.Errorf("items %v marked %q", c.items, c.markedItem())
	}
	pressGrid(g, ctrlZ)
	if !slices.Equal(g.at(0, 0).items, []string{"dev", "prod"}) || g.at(0, 0).markedItem() != "dev" {
		t.Errorf("undo left %v", g.at(0, 0).items)
	}
}

func TestAListFilterHidesItemsAndKeepsThem(t *testing.T) {
	g := &editGrid{rows: []gridRow{{cells: []gridCell{listCellOf([]string{"apple", "banana", "avocado"}, -1, false, "item")}}}}
	pressGrid(g, enterPress, letter('f'))
	typeGrid(g, "av")
	pressGrid(g, enterPress)
	if shown := g.list.grid.shown(); !slices.Equal(shown, []int{2, 3}) || g.list.grid.row != 2 {
		t.Fatalf("the filter shows %v with the focus on %d", shown, g.list.grid.row)
	}
	pressGrid(g, escPress, escPress)
	if g.list != nil || len(g.at(0, 0).items) != 3 {
		t.Errorf("esc cleared the filter, then closed with %v", g.at(0, 0).items)
	}
}

// Rows move with J and K and go with ctrl+delete; undo and redo walk the changes.
func TestTheGridArrangesRowsAndUndoes(t *testing.T) {
	g := testGrid()
	g.focus(0, 1)
	if out := pressGrid(g, moveDown); out != gridChanged || g.row != 1 || g.at(1, 1).value() != "ab" {
		t.Fatalf("J left the row at %d", g.row)
	}
	pressGrid(g, moveDown)
	if g.row != 1 {
		t.Error("J moved the last row past the end")
	}
	pressGrid(g, ctrlDelete)
	if len(g.rows) != 1 || g.at(0, 1).value() != "cd" {
		t.Fatalf("ctrl+delete left %d rows", len(g.rows))
	}
	pressGrid(g, ctrlZ, ctrlZ)
	if len(g.rows) != 2 || g.at(0, 1).value() != "ab" {
		t.Fatalf("two undos left %d rows, the first %q", len(g.rows), g.at(0, 1).value())
	}
	pressGrid(g, ctrlY)
	if g.at(1, 1).value() != "ab" {
		t.Errorf("redo did not move the row again")
	}
	pressGrid(g, moveUp)
	if len(g.future) != 0 {
		t.Error("a change after undo kept what redo would bring back")
	}
}

// A click focuses a cell and works it as enter does; the arrows of the focused row move it.
func TestTheGridTakesClicks(t *testing.T) {
	g := testGrid()
	g.hit(image.Rect(10, 1, 20, 2), 1, 1)
	g.hit(image.Rect(0, 3, 20, 4), 2, 0)
	g.hit(image.Rect(0, 1, 1, 2), 1, hitMoveUp)
	g.click(image.Pt(11, 1))
	if g.row != 1 || g.col != 1 || !g.editing || g.cell().field.cursor != 1 {
		t.Fatalf("a click on a text cell: %d,%d editing %v cursor %d", g.row, g.col, g.editing, g.cell().field.cursor)
	}
	if out := g.click(image.Pt(5, 3)); out != gridPressed {
		t.Errorf("a click on the button told %v", out)
	}
	g.focus(1, 1)
	if out := g.click(image.Pt(0, 1)); out != gridChanged || g.at(0, 1).value() != "cd" {
		t.Errorf("a click on the up arrow: %v", out)
	}
}

func TestAListCellShowsItsItemAndExactCount(t *testing.T) {
	for _, c := range []struct {
		cell gridCell
		want string
	}{
		{listCellOf([]string{"dev", "prod"}, 1, true, "option"), `"prod" · 2 options`},
		{listCellOf([]string{"dev", "prod"}, -1, true, "option"), "2 options"},
		{listCellOf(make([]string, 10001), 0, true, "option"), `"" · 10001 options`},
		{listCellOf([]string{"a", "3"}, -1, false, "item"), `"a" · 2 items`},
		{listCellOf(nil, -1, false, "item"), "no items"},
	} {
		if got := listText(&c.cell); got != c.want {
			t.Errorf("%q, want %q", got, c.want)
		}
	}
}

func TestHintsTakeOneRow(t *testing.T) {
	hints := "tab next  enter new param  esc close  ? all keys"
	for width, want := range map[int]string{
		60: hints,
		40: "tab next  enter new param  ? all keys",
		30: "tab next  ? all keys",
		10: "? all keys",
	} {
		if got := hintsInOneRow(hints, width); got != want {
			t.Errorf("width %d: %q, want %q", width, got, want)
		}
	}
}

// Enter on an item's mark moves the default there, above the old one too; an undo in a filtered
// list keeps hidden what the filter leaves out.
func TestAListMarksOnceAndUndoKeepsTheFilter(t *testing.T) {
	g := &editGrid{rows: []gridRow{{cells: []gridCell{listCellOf([]string{"web-1", "wiki-2", "db-1"}, 2, true, "option")}}}}
	pressGrid(g, enterPress)
	g.list.grid.focus(0, 0)
	pressGrid(g, enterPress)
	if got := g.list.marked(); got != 0 || g.list.markCount() != 1 {
		t.Fatalf("enter on the first mark left row %d marked, %d marks", got, g.list.markCount())
	}
	pressGrid(g, letter('f'))
	typeGrid(g, "w")
	pressGrid(g, enterPress, letter('j'), ctrlDelete, letter('f'), clearLine)
	typeGrid(g, "web")
	pressGrid(g, enterPress, ctrlZ)
	if shown := g.list.grid.shown(); !slices.Equal(shown, []int{0, 3}) {
		t.Errorf("after undo the filter web shows %v", shown)
	}
}
