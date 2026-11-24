package app

import (
	"testing"
)

func testGrid() *editGrid {
	skipped := textCellOf("x")
	skipped.skip = true
	return &editGrid{rows: [][]gridCell{
		{optionCellOf("text", []string{"text", "number", "option", "object"}), textCellOf("ab"), skipped, {kind: toggleCell}},
		{textCellOf(""), textCellOf("cd"), textCellOf(""), {kind: toggleCell}},
	}}
}

// Tab and shift+tab go through the cells in reading order, past the ones that do not apply, and
// stay at either end.
func TestTheGridStepsOverCellsThatDoNotApply(t *testing.T) {
	g := testGrid()
	var visited [][2]int
	for range 8 {
		visited = append(visited, [2]int{g.row, g.col})
		g.handleKey(tabPress)
	}
	want := [][2]int{{0, 0}, {0, 1}, {0, 3}, {1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 3}}
	for i := range want {
		if visited[i] != want[i] {
			t.Fatalf("tab visited %v, want %v", visited, want)
		}
	}
	g.focus(1, 0)
	g.handleKey(backTab)
	if g.row != 0 || g.col != 3 {
		t.Errorf("shift+tab from the second row's first cell went to %d,%d", g.row, g.col)
	}
}

// The arrows move through a cell's text and on past its ends, as along one line; up and down
// keep the column, or the nearest cell that applies.
func TestTheGridArrowsReadAsOneLine(t *testing.T) {
	g := testGrid()
	g.focus(0, 1)
	g.cell().field.cursor = 1
	g.handleKey(rightPress)
	if g.col != 1 || g.cell().field.cursor != 2 {
		t.Fatalf("right inside the text: cell %d cursor %d", g.col, g.cell().field.cursor)
	}
	g.handleKey(rightPress)
	if g.col != 3 {
		t.Errorf("right at the text's end went to cell %d, want 3 past the one that does not apply", g.col)
	}
	g.handleKey(downPress)
	g.handleKey(upPress)
	g.focus(1, 2)
	g.handleKey(upPress)
	if g.row != 0 || g.col != 1 {
		t.Errorf("up onto a cell that does not apply went to %d,%d, want the nearest", g.row, g.col)
	}
}

// A letter picks the next option starting with it, space the next, and space flips a toggle;
// each tells it changed a value.
func TestTheGridPicksOptionsAndFlipsToggles(t *testing.T) {
	g := testGrid()
	if !g.handleKey(letter('o')) || g.cell().value() != "option" {
		t.Fatalf("o picked %q", g.cell().value())
	}
	g.handleKey(letter('o'))
	if g.cell().value() != "object" {
		t.Errorf("o again picked %q, want the next starting with o", g.cell().value())
	}
	g.handleKey(spacePress)
	if g.cell().value() != "text" {
		t.Errorf("space after the last picked %q, want the first", g.cell().value())
	}
	g.focus(0, 3)
	if !g.handleKey(spacePress) || !g.cell().on {
		t.Error("space did not flip the toggle")
	}
	if g.handleKey(tabPress) {
		t.Error("tab told a change")
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
