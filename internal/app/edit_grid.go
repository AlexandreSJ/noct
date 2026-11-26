package app

import (
	"cmp"
	"image"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// An editGrid is rows of cells laid out as lines of code. The arrows or hjkl move between cells,
// and enter or a click works the one focused: it types in a text cell, opens an option cell's
// options or a list cell's items under it, flips a toggle, or presses a button row such as
// "+ add param". It lives in the inspect, so nothing holds the keys: the prefix still reaches
// every other panel and tab.

type cellKind int

const (
	textCell cellKind = iota
	// optionCell holds one of its options, picked from a menu under it.
	optionCell
	toggleCell
	// listCell holds items, edited in a list under it; a marking one marks an item as its default.
	listCell
)

type gridCell struct {
	kind  cellKind
	field textField
	// options are what an option cell can hold.
	options []string
	on      bool
	// items are a list cell's, marked the one marked or -1, and noun what an item is called.
	items   []string
	marking bool
	marked  int
	noun    string
	// skip marks a cell that does not apply to its row: drawn dim, never focused.
	skip bool
	// template completes the variables in scope and the functions as a name is typed: after a $,
	// or alone in an expression; secret shows a dot a character.
	template, expression bool
	secret               bool
	names                bool
	autoFor              string
}

func textCellOf(text string) gridCell { return gridCell{kind: textCell, field: newTextField(text)} }

func optionCellOf(value string, options []string) gridCell {
	return gridCell{kind: optionCell, field: newTextField(value), options: options}
}

func listCellOf(items []string, marked int, marking bool, noun string) gridCell {
	return gridCell{kind: listCell, items: items, marked: marked, marking: marking, noun: noun}
}

func (c *gridCell) value() string {
	switch c.kind {
	case toggleCell:
		return map[bool]string{true: "true", false: ""}[c.on]
	case listCell:
		if c.marking {
			return c.markedItem()
		}
		return strings.Join(c.items, "\n")
	}
	return c.field.String()
}

func (c *gridCell) markedItem() string {
	if c.marked < 0 || c.marked >= len(c.items) {
		return ""
	}
	return c.items[c.marked]
}

func (c gridCell) clone() gridCell {
	c.field.text = slices.Clone(c.field.text)
	c.items = slices.Clone(c.items)
	return c
}

type gridRow struct {
	cells []gridCell
	// key is the name the row is saved under, "" until it is.
	key string
	// hidden rows are left out by a filter: never drawn or focused.
	hidden bool
}

func (r gridRow) clone() gridRow {
	r.cells = slices.Clone(r.cells)
	for i := range r.cells {
		r.cells[i] = r.cells[i].clone()
	}
	return r
}

// gridOutcome tells the grid's owner what a key or a click did besides moving the focus.
type gridOutcome int

const (
	// gridChanged: a value, or the rows, changed.
	gridChanged gridOutcome = 1 << iota
	// gridPressed: the focused button row was pressed.
	gridPressed
	gridNothing gridOutcome = 0
)

// gridHit is where a draw put a cell, for clicks; the columns below 0 are the arrows that move
// the focused row.
type gridHit struct {
	rect     image.Rectangle
	row, col int
}

const (
	hitMoveUp = -1 - iota
	hitMoveDown
)

// gridState is what undo and redo bring back.
type gridState struct {
	rows     []gridRow
	row, col int
}

// maxGridHistory is how many changes undo goes back.
const maxGridHistory = 100

type editGrid struct {
	rows     []gridRow
	row, col int
	// editing types in the focused text cell; from is its text before, to tell a change.
	editing bool
	from    string
	// buttons are rows after the last, pressed with enter or a click.
	buttons []string
	// arranged rows move with J and K and go with ctrl+delete, as a flow's params.
	arranged bool
	// variables are what template cells complete; suggested is the completion picked.
	variables []flow.Variable
	suggested int
	// top is the first of the rows shown that the last draw showed.
	top int
	// history and future are the states undo and redo bring back; before is the state an edit,
	// a menu or a list started from, kept once it changes something.
	history, future []gridState
	before          *gridState
	menu            *gridMenu
	list            *gridList
	hits            []gridHit
	// dragging marks the text of the cell typed in as the mouse moves; copied is the text a
	// release or ctrl+c copied, for the owner to put on the clipboard.
	dragging bool
	copied   string
}

func (g *editGrid) at(row, col int) *gridCell { return &g.rows[row].cells[col] }

// cell is the focused cell, or nil on a button.
func (g *editGrid) cell() *gridCell {
	if g.row >= len(g.rows) || g.col >= len(g.rows[g.row].cells) {
		return nil
	}
	return g.at(g.row, g.col)
}

// button is the button row focused.
func (g *editGrid) button() (int, bool) {
	return g.row - len(g.rows), g.row >= len(g.rows)
}

// busy tells a cell is being typed in, or a menu or list is open: esc closes that first.
func (g *editGrid) busy() bool { return g.editing || g.menu != nil || g.list != nil }

// typing tells the keys type text as they are pressed, in a cell or a filter.
func (g *editGrid) typing() bool {
	return g.editing || g.menu != nil && g.menu.filter.typing || g.list != nil && g.list.typing()
}

func (g *editGrid) handleKey(key uv.KeyPressEvent) gridOutcome {
	switch {
	case g.menu != nil:
		return g.menuKey(key)
	case g.list != nil:
		return g.listKey(key)
	case g.editing:
		return g.editKey(key)
	}
	return g.navKey(key.String())
}

// gridMoves are the keys that move the focus, by their default names.
var gridMoves = func() map[string]func(g *editGrid) {
	moves := map[string]func(g *editGrid){}
	add := func(move func(g *editGrid), keys ...string) {
		for _, key := range keys {
			moves[key] = move
		}
	}
	add(func(g *editGrid) { g.changeRow(-1) }, "up", "k")
	add(func(g *editGrid) { g.changeRow(1) }, "down", "j")
	add(func(g *editGrid) { g.stepInRow(-1) }, "left", "h")
	add(func(g *editGrid) { g.stepInRow(1) }, "right", "l")
	add(func(g *editGrid) { g.step(1) }, "tab")
	add(func(g *editGrid) { g.step(-1) }, "shift+tab")
	add(func(g *editGrid) { g.changeRow(-len(g.rows) - len(g.buttons)) }, "ctrl+up", "ctrl+k")
	add(func(g *editGrid) { g.changeRow(len(g.rows) + len(g.buttons)) }, "ctrl+down", "ctrl+j")
	add(func(g *editGrid) { g.stepInRow(-len(g.cellsOf(g.row))) }, "ctrl+left", "ctrl+h")
	add(func(g *editGrid) { g.stepInRow(len(g.cellsOf(g.row))) }, "ctrl+right", "ctrl+l")
	return moves
}()

func (g *editGrid) navKey(name string) gridOutcome {
	if move, ok := gridMoves[name]; ok {
		move(g)
		return gridNothing
	}
	switch name {
	case "enter":
		return g.activate()
	case "space":
		if c := g.cell(); c != nil && c.kind != textCell {
			return g.activate()
		}
	case "K", "shift+k", "shift+up":
		return g.moveRow(-1)
	case "J", "shift+j", "shift+down":
		return g.moveRow(1)
	case "ctrl+delete":
		return g.removeRow()
	case "ctrl+z":
		return g.undo()
	case "ctrl+y":
		return g.redo()
	}
	return gridNothing
}

// activate works the focused cell or button, as enter or a click does.
func (g *editGrid) activate() gridOutcome {
	if _, ok := g.button(); ok {
		return gridPressed
	}
	c := g.cell()
	if c == nil || c.skip {
		return gridNothing
	}
	switch c.kind {
	case textCell:
		g.startEdit()
	case toggleCell:
		g.remember()
		c.on = !c.on
		return gridChanged
	case optionCell:
		g.openMenu()
	case listCell:
		g.openList()
	}
	return gridNothing
}

func (g *editGrid) startEdit() {
	c := g.cell()
	g.begin()
	g.editing, g.from, g.suggested = true, c.field.String(), 0
	c.field.cursor, c.field.shown, c.field.selecting = len(c.field.text), 0, false // it showed from its start
}

// settle keeps what is being typed, or listed, and closes what is open.
func (g *editGrid) settle() gridOutcome {
	switch {
	case g.list != nil:
		return g.closeList()
	case g.menu != nil:
		g.menu = nil
	case g.editing:
		return g.stopEdit()
	}
	return gridNothing
}

// stopEdit leaves the text typed in the cell.
func (g *editGrid) stopEdit() gridOutcome {
	g.editing = false
	return g.commit(g.cell().field.String() != g.from)
}

func (g *editGrid) editKey(key uv.KeyPressEvent) gridOutcome {
	name := key.String()
	if g.completionKey(name) {
		return gridNothing
	}
	if field := &g.cell().field; field.multiline {
		switch {
		case name == "enter":
			field.insert("\n")
			return gridNothing
		case name == "up" && field.moveLine(-1), name == "down" && field.moveLine(1):
			return gridNothing
		}
	}
	switch name {
	case "enter", "esc", "ctrl+enter", "alt+enter":
		return g.stopEdit()
	case "tab", "shift+tab", "up", "down":
		out := g.stopEdit()
		g.navKey(name)
		return out
	case "ctrl+z":
		g.stopEdit()
		return g.undo()
	case "ctrl+c", "ctrl+shift+c":
		g.copy()
		return gridNothing
	}
	g.suggested = 0
	g.cell().field.handleKey(key)
	return gridNothing
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

func (g *editGrid) paste(text string) {
	switch {
	case g.list != nil:
		g.list.paste(text)
	case g.menu != nil:
		if g.menu.filter.typing {
			g.menu.filter.field.insert(text)
		}
	default:
		c := g.cell()
		if c == nil || c.kind != textCell || c.skip {
			return
		}
		if !g.editing {
			g.startEdit()
		}
		c.field.insert(text)
	}
}

// shown are the rows drawn and focused: the ones no filter hides, then the buttons.
func (g *editGrid) shown() []int {
	var rows []int
	for i, row := range g.rows {
		if !row.hidden {
			rows = append(rows, i)
		}
	}
	for i := range g.buttons {
		rows = append(rows, len(g.rows)+i)
	}
	return rows
}

func (g *editGrid) cellsOf(row int) []gridCell {
	if row >= len(g.rows) {
		return nil
	}
	return g.rows[row].cells
}

// changeRow moves the focus by rows shown, staying at either end, to the cell nearest the column
// that applies there.
func (g *editGrid) changeRow(by int) {
	shown := g.shown()
	if len(shown) == 0 {
		return
	}
	at := max(slices.Index(shown, g.row), 0)
	g.focusNear(shown[min(max(at+by, 0), len(shown)-1)], g.col)
}

func (g *editGrid) focusNear(row, col int) {
	cells := g.cellsOf(row)
	for spread := 0; spread < len(cells); spread++ {
		for _, c := range []int{col - spread, col + spread} {
			if c >= 0 && c < len(cells) && !cells[c].skip {
				g.focus(row, c)
				return
			}
		}
	}
	g.focus(row, 0)
}

// stepInRow moves the focus by cells that apply within its row, staying at either end.
func (g *editGrid) stepInRow(by int) {
	cells := g.cellsOf(g.row)
	unit := map[bool]int{true: 1, false: -1}[by > 0]
	col := g.col
	for c := g.col + unit; c >= 0 && c < len(cells) && by != 0; c += unit {
		if !cells[c].skip {
			col = c
			by -= unit
		}
	}
	g.focus(g.row, col)
}

// step moves the focus to the next cell that applies, or button, in reading order, staying at
// either end.
func (g *editGrid) step(by int) {
	var stops [][2]int
	for _, row := range g.shown() {
		cells := g.cellsOf(row)
		if cells == nil {
			stops = append(stops, [2]int{row, 0})
		}
		for col := range cells {
			if !cells[col].skip {
				stops = append(stops, [2]int{row, col})
			}
		}
	}
	at := slices.Index(stops, [2]int{g.row, g.col})
	if at < 0 || len(stops) == 0 {
		return
	}
	stop := stops[min(max(at+by, 0), len(stops)-1)]
	g.focus(stop[0], stop[1])
}

func (g *editGrid) focus(row, col int) {
	g.row, g.col, g.suggested = row, col, 0
}

// keepFocus moves the focus back onto a row shown, after rows went.
func (g *editGrid) keepFocus() {
	shown := g.shown()
	if len(shown) == 0 || slices.Contains(shown, g.row) {
		return
	}
	for _, row := range shown {
		if row >= g.row {
			g.focusNear(row, g.col)
			return
		}
	}
	g.focusNear(shown[len(shown)-1], g.col)
}

func (g *editGrid) addRow(row gridRow) {
	g.remember()
	g.rows = append(g.rows, row)
	g.focusNear(len(g.rows)-1, 0)
}

// moveRow swaps the focused row with the one shown above or below it.
func (g *editGrid) moveRow(by int) gridOutcome {
	if _, onButton := g.button(); !g.arranged || onButton {
		return gridNothing
	}
	var rows []int
	for _, row := range g.shown() {
		if row < len(g.rows) {
			rows = append(rows, row)
		}
	}
	to := slices.Index(rows, g.row) + by
	if to < 0 || to >= len(rows) {
		return gridNothing
	}
	g.remember()
	g.rows[g.row], g.rows[rows[to]] = g.rows[rows[to]], g.rows[g.row]
	g.row = rows[to]
	return gridChanged
}

func (g *editGrid) removeRow() gridOutcome {
	if _, onButton := g.button(); !g.arranged || onButton {
		return gridNothing
	}
	g.remember()
	g.rows = slices.Delete(g.rows, g.row, g.row+1)
	g.keepFocus()
	return gridChanged
}

func (g *editGrid) snapshot() gridState {
	rows := make([]gridRow, len(g.rows))
	for i, row := range g.rows {
		rows[i] = row.clone()
	}
	return gridState{rows: rows, row: g.row, col: g.col}
}

// remember keeps the state before a change, for undo.
func (g *editGrid) remember() {
	g.history = append(g.history, g.snapshot())
	if len(g.history) > maxGridHistory {
		g.history = g.history[1:]
	}
	g.future = nil
}

// begin keeps the state before an edit, a menu or a list, which commit remembers once it changed.
func (g *editGrid) begin() {
	state := g.snapshot()
	g.before = &state
}

func (g *editGrid) commit(changed bool) gridOutcome {
	before := g.before
	g.before = nil
	if !changed || before == nil {
		return gridNothing
	}
	g.history = append(g.history, *before)
	g.future = nil
	return gridChanged
}

func (g *editGrid) undo() gridOutcome {
	return g.travel(&g.history, &g.future)
}

func (g *editGrid) redo() gridOutcome {
	return g.travel(&g.future, &g.history)
}

// travel brings back the last state of from, keeping the one now in to.
func (g *editGrid) travel(from, to *[]gridState) gridOutcome {
	if len(*from) == 0 {
		return gridNothing
	}
	*to = append(*to, g.snapshot())
	state := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	g.rows = state.rows
	g.focus(min(state.row, len(g.rows)+len(g.buttons)-1), state.col)
	g.keepFocus()
	return gridChanged
}

// windowFit are the rows shown that fit height lines, each heightOf lines tall, scrolled to keep
// the focus in sight.
func (g *editGrid) windowFit(height int, heightOf func(row int) int) []int {
	shown := g.shown()
	at := max(slices.Index(shown, g.row), 0)
	g.top = min(g.top, at)
	used := func(from, to int) int {
		sum := 0
		for _, row := range shown[from : to+1] {
			sum += heightOf(row)
		}
		return sum
	}
	for g.top < at && used(g.top, at) > height {
		g.top++
	}
	end, lines := g.top, 0
	for end < len(shown) && lines+heightOf(shown[end]) <= max(height, heightOf(shown[g.top])) {
		lines += heightOf(shown[end])
		end++
	}
	return shown[g.top:end]
}

// drawBlock draws a cell of several lines in rect: the field's lines with the cursor while typed
// in, else lines, lit while focused.
func (g *editGrid) drawBlock(screen uv.Screen, rect image.Rectangle, row, col int, lines []styledLine, pal *palette) {
	g.hit(rect, row, col)
	field := &g.at(row, col).field
	switch {
	case g.lit(row, col) && g.editing:
		at := field.drawLines(screen, rect, pal.typing(), pal.selectedRow)
		drawGhost(screen, at, image.Rect(rect.Min.X, at.Y, rect.Max.X, at.Y+1), g.ghost(), pal)
		return
	case g.lit(row, col):
		fillRect(screen, rect, pal.selectedRow)
		for i := range lines {
			lines[i].style = pal.selectedRow
		}
	}
	for i, line := range lines[:min(len(lines), rect.Dy())] {
		drawTextCut(screen, rect.Min.X, rect.Min.Y+i, rect.Max.X, line.text, line.style)
	}
}

// window are the rows shown that fit height rows, scrolled to keep the focus in sight.
func (g *editGrid) window(height int) []int {
	shown := g.shown()
	from, to := scrollWindow(&g.top, slices.Index(shown, g.row), len(shown), height)
	return shown[from:to]
}

// scrollWindow moves top so at, of count lines, is among the height shown from it, and gives
// the lines shown.
func scrollWindow(top *int, at, count, height int) (int, int) {
	height = max(height, 1)
	at = max(at, 0)
	*top = min(*top, at)
	if at >= *top+height {
		*top = at - height + 1
	}
	*top = max(min(*top, count-height), 0)
	return *top, min(*top+height, count)
}

func (g *editGrid) hit(rect image.Rectangle, row, col int) {
	g.hits = append(g.hits, gridHit{rect, row, col})
}

// focusRect is where the last draw put the focused cell.
func (g *editGrid) focusRect() (image.Rectangle, bool) {
	for _, h := range g.hits {
		if h.row == g.row && h.col == g.col {
			return h.rect, true
		}
	}
	return image.Rectangle{}, false
}

// copy keeps the text marked in the cell typed in, or all of it, to be copied; never a secret's.
func (g *editGrid) copy() {
	if c := g.cell(); c != nil && !c.secret {
		g.copied = cmp.Or(c.field.selected(), c.field.String())
	}
}

// takeCopied is the text copied since it was last asked, here or in an open list.
func (g *editGrid) takeCopied() string {
	text := g.copied
	g.copied = ""
	if g.list != nil {
		text = cmp.Or(text, g.list.grid.takeCopied())
	}
	return text
}

// mouse clicks or scrolls the grid, at a point on the screen; pressed on a text cell and
// dragged, it marks its text, which the release copies.
func (g *editGrid) mouse(event uv.MouseEvent, at image.Point) gridOutcome {
	m := event.Mouse()
	switch event.(type) {
	case uv.MouseClickEvent:
		if m.Button == uv.MouseLeft {
			out := g.click(at)
			g.startDrag()
			return out
		}
	case uv.MouseMotionEvent:
		g.drag(event, at)
	case uv.MouseReleaseEvent:
		g.drop(event, at)
	case uv.MouseWheelEvent:
		return g.wheel(map[uv.MouseButton]int{uv.MouseWheelUp: -1, uv.MouseWheelDown: 1}[m.Button])
	}
	return gridNothing
}

// click works what is at the point as enter would, once focused: a click outside an open menu
// or list closes it.
func (g *editGrid) click(at image.Point) gridOutcome {
	if g.menu != nil {
		if option, ok := g.menu.optionAt(at); ok {
			return g.pick(option)
		}
		g.menu = nil
		return gridNothing
	}
	if g.list != nil {
		if at.In(g.list.box) {
			g.list.click(at)
			return gridNothing
		}
		return g.closeList()
	}
	for _, h := range g.hits {
		if at.In(h.rect) {
			return g.clickHit(h, at)
		}
	}
	if g.editing {
		return g.stopEdit()
	}
	return gridNothing
}

func (g *editGrid) clickHit(h gridHit, at image.Point) gridOutcome {
	if g.editing && h.row == g.row && h.col == g.col {
		c := g.cell()
		c.field.cursor, c.field.selecting = c.field.posAtIn(at, h.rect), false
		return gridNothing
	}
	out := gridNothing
	if g.editing {
		out = g.stopEdit()
	}
	switch h.col {
	case hitMoveUp:
		return out | g.moveRow(-1)
	case hitMoveDown:
		return out | g.moveRow(1)
	}
	g.focus(h.row, h.col)
	out |= g.activate()
	if c := g.cell(); g.editing && c != nil {
		c.field.cursor, c.field.selecting = c.field.posAtIn(at, h.rect), false
	}
	return out
}

// startDrag starts marking the text of the cell a click left typed in, from where it was clicked.
func (g *editGrid) startDrag() {
	if g.list != nil {
		g.list.grid.startDrag()
		return
	}
	if c := g.cell(); g.editing && c != nil && !c.secret {
		c.field.mark()
		g.dragging = true
	}
}

// drag puts the cursor, and the end of the text marked, under the mouse.
func (g *editGrid) drag(event uv.MouseEvent, at image.Point) {
	if g.list != nil {
		g.list.grid.drag(event, at)
		return
	}
	rect, ok := g.focusRect()
	if c := g.cell(); g.dragging && ok && c != nil {
		c.field.cursor = c.field.posAtIn(at, rect)
	}
}

// drop ends a drag, and copies the text it marked.
func (g *editGrid) drop(event uv.MouseEvent, at image.Point) {
	if g.list != nil {
		g.list.grid.drop(event, at)
		return
	}
	if !g.dragging {
		return
	}
	g.drag(event, at)
	g.dragging = false
	if text := g.cell().field.selected(); text != "" {
		g.copied = text
	}
}

// wheel moves the focus a row, or through an open menu or list.
func (g *editGrid) wheel(by int) gridOutcome {
	switch {
	case by == 0:
	case g.menu != nil:
		g.menu.move(by)
	case g.list != nil:
		g.list.grid.wheel(by)
	default:
		out := gridNothing
		if g.editing {
			out = g.stopEdit()
		}
		g.changeRow(by)
		return out
	}
	return gridNothing
}

// suggestions are the variables that complete what is typed in a template cell before its
// cursor, and where the part they replace starts.
func (g *editGrid) suggestions() ([]string, int) {
	c := g.cell()
	switch {
	case !g.editing || c == nil:
		return nil, 0
	case c.names:
		return variableNameSuggestions(c.field.String(), g.variables), 0
	case c.template:
		return templateSuggestions(&c.field, g.variables, c.expression)
	}
	return nil, 0
}

func (g *editGrid) ghost() string {
	matches, from := g.suggestions()
	field := &g.cell().field
	if len(matches) == 0 || field.cursor != len(field.text) {
		return ""
	}
	typed := string(field.text[from:field.cursor])
	rest, ok := strings.CutPrefix(matches[min(g.suggested, len(matches)-1)], typed)
	if !ok {
		return ""
	}
	return rest
}

func (g *editGrid) complete() {
	matches, from := g.suggestions()
	if len(matches) > 0 {
		completeTemplate(&g.cell().field, matches[min(g.suggested, len(matches)-1)], from)
		g.suggested = 0
	}
}

// lit tells the cell is focused with nothing open over it, so it is drawn lit.
func (g *editGrid) lit(row, col int) bool {
	return row == g.row && col == g.col && g.menu == nil && g.list == nil
}

// drawCell draws a cell's text in style, lit while focused, or its field with the cursor while
// typed in, and keeps where it is for clicks.
func (g *editGrid) drawCell(screen uv.Screen, rect image.Rectangle, row, col int, text string, style uv.Style, pal *palette) {
	g.hit(rect, row, col)
	switch {
	case g.lit(row, col) && g.editing:
		field := &g.at(row, col).field
		end := field.draw(screen, rect.Min.X, rect.Min.Y, rect.Dx(), g.at(row, col).secret, pal.typing(), pal.selectedRow)
		drawGhost(screen, image.Pt(end-1, rect.Min.Y), rect, g.ghost(), pal)
	case g.lit(row, col):
		fillRect(screen, rect, pal.selectedRow)
		drawTextCut(screen, rect.Min.X, rect.Min.Y, rect.Max.X, text, pal.selectedRow)
	default:
		drawTextCut(screen, rect.Min.X, rect.Min.Y, rect.Max.X, text, style)
	}
}

// hints are the keys of what is open, or idle's while nothing is.
func (g *editGrid) hints(idle string) string {
	switch {
	case g.menu != nil && g.menu.filter.typing, g.list != nil && g.list.filter.typing:
		return "{enter} keep  esc clear"
	case g.editing && g.cell().field.multiline:
		return "{enter} new line  esc or ctrl+enter done  {tab} next"
	case g.editing, g.list != nil && g.list.grid.editing:
		return "{enter} done  {tab} next"
	case g.menu != nil:
		return "{enter} pick  {f} filter  esc back"
	case g.list != nil && g.list.marking:
		return "{enter} edit  {space} mark  {f} filter  esc back"
	case g.list != nil:
		return "{enter} edit  {f} filter  esc back"
	}
	return idle
}

func drawGhost(screen uv.Screen, at image.Point, rect image.Rectangle, ghost string, pal *palette) {
	if ghost == "" || at.X >= rect.Max.X {
		return
	}
	drawTextCut(screen, at.X, at.Y, rect.Max.X, ghost, pal.panelMuted)
	if cell := screen.CellAt(at.X, at.Y); cell != nil {
		cursor := *cell
		cursor.Style = pal.selectedRow
		screen.SetCell(at.X, at.Y, &cursor)
	}
}
