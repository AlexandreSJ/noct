package app

import (
	"image"
	"maps"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// copiedRows are the rows c or x took from a grid, which v puts in a grid of as many columns.
var copiedRows []gridRow

// rowKey works the keys on an arranged grid's rows while nothing is typed: shift+up and
// shift+down mark rows, d doubles, x cuts, c copies and v pastes them, the marked ones or the
// focused one, and esc lets the marks go.
func (g *editGrid) rowKey(name string) (gridOutcome, bool) {
	if !g.arranged {
		return gridNothing, false
	}
	switch name {
	case "shift+up", "shift+down":
		g.markTo(map[string]int{"shift+up": -1, "shift+down": 1}[name])
	case "esc":
		if len(g.marked) == 0 {
			return gridNothing, false
		}
		g.marked = nil
	case "d":
		return g.doubleRows(), true
	case "c":
		g.copyRows()
	case "x":
		g.copyRows()
		return g.removeRows(), true
	case "v":
		return g.pasteRows(), true
	case "delete", "ctrl+delete":
		return g.removeRows(), true
	default:
		return gridNothing, false
	}
	return gridNothing, true
}

func (g *editGrid) markTo(by int) {
	if g.row >= len(g.rows) {
		return
	}
	if g.marked == nil {
		g.marked = map[int]bool{g.row: true}
	}
	g.changeRow(by)
	if g.row < len(g.rows) {
		g.marked[g.row] = true
	}
}

// toggleMark marks a row clicked with ctrl, or lets its mark go.
func (g *editGrid) toggleMark(row int) {
	if g.marked == nil {
		g.marked = map[int]bool{}
	}
	if g.marked[row] {
		delete(g.marked, row)
	} else {
		g.marked[row] = true
	}
}

// targets are the rows a row key works on: the marked ones, or else the focused one.
func (g *editGrid) targets() []int {
	if len(g.marked) > 0 {
		return slices.Sorted(maps.Keys(g.marked))
	}
	if g.row < len(g.rows) {
		return []int{g.row}
	}
	return nil
}

func (g *editGrid) copyRows() {
	copiedRows = nil
	var text []string
	for _, row := range g.targets() {
		copiedRows = append(copiedRows, g.rows[row].clone())
		var cells []string
		for _, cell := range g.rows[row].cells {
			if !cell.secret {
				cells = append(cells, cell.value())
			}
		}
		text = append(text, strings.Join(cells, "\t"))
	}
	g.copied = strings.Join(text, "\n")
}

func (g *editGrid) insertRows(after int, rows []gridRow) gridOutcome {
	if len(rows) == 0 {
		return gridNothing
	}
	g.remember()
	at := min(after+1, len(g.rows))
	clones := make([]gridRow, len(rows))
	for i, row := range rows {
		clones[i] = row.clone()
	}
	g.rows = slices.Insert(g.rows, at, clones...)
	g.marked = nil
	g.focusNear(at, g.col)
	return gridChanged
}

func (g *editGrid) doubleRows() gridOutcome {
	targets := g.targets()
	if len(targets) == 0 {
		return gridNothing
	}
	rows := make([]gridRow, len(targets))
	for i, row := range targets {
		rows[i] = g.rows[row]
	}
	return g.insertRows(targets[len(targets)-1], rows)
}

func (g *editGrid) pasteRows() gridOutcome {
	if len(copiedRows) == 0 || len(g.rows) > 0 && len(copiedRows[0].cells) != len(g.rows[0].cells) {
		return gridNothing
	}
	return g.insertRows(min(g.row, len(g.rows)-1), copiedRows)
}

func (g *editGrid) removeRows() gridOutcome {
	targets := g.targets()
	if len(targets) == 0 {
		return gridNothing
	}
	g.remember()
	for _, row := range slices.Backward(targets) {
		g.rows = slices.Delete(g.rows, row, row+1)
	}
	g.marked = nil
	g.row = targets[0]
	g.keepFocus()
	return gridChanged
}

// drawMark shows a marked row, drawn in rect, reversed.
func (g *editGrid) drawMark(screen uv.Screen, rect image.Rectangle, row int) {
	if !g.marked[row] {
		return
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		lightCells(screen, rect.Min.X, rect.Max.X, y)
	}
}

func (g *editGrid) rowAt(at image.Point) (int, bool) {
	for _, h := range g.hits {
		if at.In(h.rect) && h.row < len(g.rows) && h.col >= 0 {
			return h.row, true
		}
	}
	return 0, false
}
