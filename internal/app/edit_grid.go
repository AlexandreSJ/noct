package app

import (
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// An editGrid is rows of cells typed in place, as one line of code each: tab and shift+tab go
// from cell to cell, the arrows move through the text and on past its ends, and up and down change
// rows. It edits a flow's params and a step's fields inside the inspect, without a popup.

type cellKind int

const (
	// textCell is typed, with the shell's editing keys.
	textCell cellKind = iota
	// optionCell holds one of its options: space steps to the next, a letter to the next one
	// starting with it.
	optionCell
	// toggleCell is on or off: space flips it.
	toggleCell
)

type gridCell struct {
	kind  cellKind
	field textField
	// options are what an option cell can hold.
	options []string
	on      bool
	// skip marks a cell that does not apply to its row: drawn dim, never focused.
	skip bool
	// template completes the variables in scope as {{. is typed.
	template bool
}

func textCellOf(text string) gridCell { return gridCell{kind: textCell, field: newTextField(text)} }

func optionCellOf(value string, options []string) gridCell {
	return gridCell{kind: optionCell, field: newTextField(value), options: options}
}

func (c *gridCell) value() string {
	if c.kind == toggleCell {
		return map[bool]string{true: "true", false: ""}[c.on]
	}
	return c.field.String()
}

// stepOption moves an option cell to the option step places away, or to the next one starting
// with letter when it is not "".
func (c *gridCell) stepOption(step int, letter string) {
	if len(c.options) == 0 {
		return
	}
	at := slices.Index(c.options, c.value())
	for range c.options {
		at = (at + step + len(c.options)) % len(c.options)
		if letter == "" || strings.HasPrefix(c.options[at], letter) {
			c.field = newTextField(c.options[at])
			return
		}
	}
}

type editGrid struct {
	rows     [][]gridCell
	row, col int
	// variables are what template cells complete; suggested is the completion picked.
	variables []flow.Variable
	suggested int
}

func (g *editGrid) cell() *gridCell {
	if g.row >= len(g.rows) || g.col >= len(g.rows[g.row]) {
		return nil
	}
	return &g.rows[g.row][g.col]
}

// handleKey runs the key on the focused cell or moves the focus, and tells whether it changed a
// cell's value.
func (g *editGrid) handleKey(key uv.KeyPressEvent) (edited bool) {
	c := g.cell()
	if c == nil {
		return false
	}
	name := key.String()
	if g.completionKey(name) {
		return name == "tab"
	}
	switch name {
	case "tab":
		g.step(1)
	case "shift+tab":
		g.step(-1)
	case "up":
		g.changeRow(-1)
	case "down":
		g.changeRow(1)
	case "left", "right":
		g.arrow(c, key)
	default:
		return g.typeIn(c, key)
	}
	return false
}

// completionKey picks or puts in a variable while some complete what is typed.
func (g *editGrid) completionKey(name string) bool {
	matches, _ := g.suggestions()
	if len(matches) == 0 {
		return false
	}
	switch name {
	case "up", "down":
		g.suggested = (g.suggested + map[string]int{"up": -1, "down": 1}[name] + len(matches)) % len(matches)
	case "tab":
		g.complete()
	default:
		return false
	}
	return true
}

// arrow moves the cursor through a text cell's text, or past its end to the cell beside.
func (g *editGrid) arrow(c *gridCell, key uv.KeyPressEvent) {
	step := map[string]int{"left": -1, "right": 1}[key.String()]
	atEdge := c.kind != textCell || step < 0 && c.field.cursor == 0 || step > 0 && c.field.cursor == len(c.field.text)
	if atEdge {
		g.step(step)
		return
	}
	c.field.handleKey(key)
}

func (g *editGrid) typeIn(c *gridCell, key uv.KeyPressEvent) bool {
	before := c.value()
	switch {
	case c.kind == toggleCell && key.String() == "space":
		c.on = !c.on
	case c.kind == optionCell && key.String() == "space":
		c.stepOption(1, "")
	case c.kind == optionCell && isPlainText(key):
		c.stepOption(1, strings.ToLower(key.Text))
	case c.kind == textCell:
		g.suggested = 0
		c.field.handleKey(key)
	}
	return c.value() != before
}

func (g *editGrid) paste(text string) bool {
	c := g.cell()
	if c == nil || c.kind != textCell {
		return false
	}
	c.field.insert(text)
	return true
}

// step moves the focus to the next cell that applies, in reading order, staying at either end.
func (g *editGrid) step(by int) {
	row, col := g.row, g.col
	for {
		col += by
		if col < 0 || col >= len(g.rows[row]) {
			row += by
			if row < 0 || row >= len(g.rows) {
				return
			}
			col = map[bool]int{true: 0, false: len(g.rows[row]) - 1}[by > 0]
		}
		if !g.rows[row][col].skip {
			g.focus(row, col)
			return
		}
	}
}

// changeRow moves the focus a row up or down, to the nearest cell that applies there.
func (g *editGrid) changeRow(by int) {
	row := g.row + by
	if row < 0 || row >= len(g.rows) {
		return
	}
	for spread := 0; spread < len(g.rows[row]); spread++ {
		for _, col := range []int{g.col - spread, g.col + spread} {
			if col >= 0 && col < len(g.rows[row]) && !g.rows[row][col].skip {
				g.focus(row, col)
				return
			}
		}
	}
}

func (g *editGrid) focus(row, col int) {
	g.row, g.col, g.suggested = row, col, 0
}

// suggestions are the variables that complete what is typed in a template cell before its
// cursor, and where the part they replace starts.
func (g *editGrid) suggestions() ([]string, int) {
	c := g.cell()
	if c == nil || !c.template {
		return nil, 0
	}
	return templateSuggestions(&c.field, g.variables)
}

func (g *editGrid) complete() {
	matches, from := g.suggestions()
	if len(matches) > 0 {
		completeTemplate(&g.cell().field, matches[min(g.suggested, len(matches)-1)], from)
		g.suggested = 0
	}
}
