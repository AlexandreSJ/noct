package app

import (
	"image"
	"slices"
	"strconv"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// What opens under a grid's cell: the menu of an option cell's options and the list of a list
// cell's items. Both are drawn over the grid, take its keys and clicks while open, and close with
// esc or a click outside; f or / filters them, as any list of noct.

// maxListRows is how many options or items show at once; the rest scroll.
const maxListRows = 10

// gridMenu is an option cell's options, one lit.
type gridMenu struct {
	options []string
	filter  listFilter
	lit     int
	top     int
	box     image.Rectangle
	hits    []gridHit
}

func (g *editGrid) openMenu() {
	c := g.cell()
	g.menu = &gridMenu{options: c.options, lit: max(slices.Index(c.options, c.value()), 0)}
}

// shown are the options the filter keeps, by index.
func (m *gridMenu) shown() []int {
	var shown []int
	for i, option := range m.options {
		if m.filter.keeps(optionText(option)) {
			shown = append(shown, i)
		}
	}
	return shown
}

func (m *gridMenu) move(by int) {
	shown := m.shown()
	if len(shown) == 0 {
		return
	}
	at := max(slices.Index(shown, m.lit), 0)
	m.lit = shown[min(max(at+by, 0), len(shown)-1)]
}

func (g *editGrid) menuKey(key uv.KeyPressEvent) gridOutcome {
	m := g.menu
	if m.filter.typing {
		if used, _ := m.filter.handleKey(key); used {
			m.move(0)
			return gridNothing
		}
	}
	switch key.String() {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "ctrl+up", "ctrl+k", "home":
		m.move(-len(m.options))
	case "ctrl+down", "ctrl+j", "end":
		m.move(len(m.options))
	case "enter", "space":
		if slices.Contains(m.shown(), m.lit) {
			return g.pick(m.options[m.lit])
		}
	case "esc":
		if m.filter.on() {
			m.filter.clear()
			return gridNothing
		}
		g.menu = nil
	default:
		if isFilterKey(key.String()) {
			m.filter.start()
		}
	}
	return gridNothing
}

// pick puts option in the focused cell and closes the menu.
func (g *editGrid) pick(option string) gridOutcome {
	g.menu = nil
	c := g.cell()
	if c.value() == option {
		return gridNothing
	}
	g.remember()
	c.field = newTextField(option)
	return gridChanged
}

func (m *gridMenu) optionAt(at image.Point) (string, bool) {
	for _, h := range m.hits {
		if at.In(h.rect) {
			return m.options[h.row], true
		}
	}
	return "", false
}

// optionText is how an option shows: the empty one is none.
func optionText(option string) string {
	if option == "" {
		return "none"
	}
	return option
}

func (m *gridMenu) draw(screen uv.Screen, anchor, bounds image.Rectangle, pal *palette) {
	shown := m.shown()
	width := anchor.Dx() + 2
	for _, option := range m.options {
		width = max(width, ansi.StringWidth(optionText(option))+4)
	}
	lines := max(min(len(shown), maxListRows), 1)
	if m.filter.on() {
		lines++
	}
	m.box = placeBox(anchor, bounds, width, lines+2)
	inner := drawBox(screen, m.box, "", pal)
	y := inner.Min.Y
	if m.filter.on() {
		m.filter.draw(screen, inner.Min.X+1, y, inner.Max.X, filterCount(len(shown), len(m.options)), pal)
		y++
	}
	m.hits = m.hits[:0]
	from, to := scrollWindow(&m.top, slices.Index(shown, m.lit), len(shown), inner.Max.Y-y)
	for _, i := range shown[from:to] {
		rect := image.Rect(inner.Min.X, y, inner.Max.X, y+1)
		style := pal.panelText
		if i == m.lit {
			style = pal.selectedRow
			fillRect(screen, rect, style)
		}
		drawTextCut(screen, rect.Min.X+1, y, rect.Max.X, optionText(m.options[i]), style)
		m.hits = append(m.hits, gridHit{rect: rect, row: i})
		y++
	}
	if len(shown) == 0 {
		drawText(screen, inner.Min.X+1, y, inner.Max.X, "nothing matches", pal.panelMuted)
	}
}

// gridList edits a list cell's items as a grid of its own, an item a row, with a mark before
// each in a marking list.
type gridList struct {
	grid    editGrid
	marking bool
	noun    string
	filter  listFilter
	box     image.Rectangle
}

func (g *editGrid) openList() {
	c := g.cell()
	g.begin()
	l := &gridList{marking: c.marking, noun: c.noun}
	l.grid = editGrid{buttons: []string{"+ add " + c.noun}, arranged: true}
	for i, item := range c.items {
		l.grid.rows = append(l.grid.rows, l.itemRow(item, i == c.marked))
	}
	l.grid.focusNear(max(c.marked, 0), l.textColumn())
	g.list = l
}

func (l *gridList) itemRow(item string, marked bool) gridRow {
	text := textCellOf(item)
	if !l.marking {
		return gridRow{cells: []gridCell{text}}
	}
	return gridRow{cells: []gridCell{{kind: toggleCell, on: marked}, text}}
}

func (l *gridList) textColumn() int {
	if l.marking {
		return 1
	}
	return 0
}

func (l *gridList) typing() bool { return l.filter.typing || l.grid.typing() }

func (g *editGrid) listKey(key uv.KeyPressEvent) gridOutcome {
	l := g.list
	if l.filter.typing {
		if used, _ := l.filter.handleKey(key); used {
			l.refilter()
			return gridNothing
		}
	}
	if !l.grid.busy() {
		switch key.String() {
		case "esc":
			if l.filter.on() {
				l.filter.clear()
				l.refilter()
				return gridNothing
			}
			return g.closeList()
		case "f", "/":
			l.filter.start()
			return gridNothing
		case "space":
			l.mark()
			return gridNothing
		}
	}
	marked := l.marked()
	l.after(l.grid.handleKey(key), marked)
	return gridNothing
}

// after adds an item when + add is pressed, typed in at once, keeps one item marked, the one
// marked before going, and filters again what a change, as an undo, brought back.
func (l *gridList) after(out gridOutcome, marked int) {
	if out&gridPressed != 0 {
		l.grid.addRow(l.itemRow("", false))
		l.grid.focus(len(l.grid.rows)-1, l.textColumn())
		l.grid.startEdit()
	}
	if out&gridChanged == 0 {
		return
	}
	if marked >= 0 && l.markCount() > 1 {
		l.grid.at(marked, 0).on = false
	}
	l.refilter()
}

// mark marks the focused item as the default, or unmarks it.
func (l *gridList) mark() {
	if _, onButton := l.grid.button(); !l.marking || onButton || len(l.grid.rows) == 0 {
		return
	}
	marked := l.marked()
	l.grid.remember()
	cell := l.grid.at(l.grid.row, 0)
	cell.on = !cell.on
	l.after(gridChanged, marked)
}

// marked is the row marked, or -1.
func (l *gridList) marked() int {
	for i, row := range l.grid.rows {
		if l.marking && row.cells[0].on {
			return i
		}
	}
	return -1
}

func (l *gridList) markCount() int {
	count := 0
	for _, row := range l.grid.rows {
		if l.marking && row.cells[0].on {
			count++
		}
	}
	return count
}

func (l *gridList) click(at image.Point) {
	marked := l.marked()
	l.after(l.grid.click(at), marked)
}

func (l *gridList) paste(text string) {
	if l.filter.typing {
		l.filter.field.insert(text)
		l.refilter()
		return
	}
	l.grid.paste(text)
}

func (l *gridList) refilter() {
	for i := range l.grid.rows {
		l.grid.rows[i].hidden = !l.filter.keeps(l.grid.at(i, l.textColumn()).value())
	}
	l.grid.keepFocus()
}

// items are the items typed, empty ones left out, and the one marked, or -1.
func (l *gridList) items() ([]string, int) {
	var items []string
	marked := -1
	for _, row := range l.grid.rows {
		text := strings.TrimSpace(row.cells[l.textColumn()].value())
		if text == "" {
			continue
		}
		if l.marking && row.cells[0].on {
			marked = len(items)
		}
		items = append(items, text)
	}
	return items, marked
}

// closeList puts the items back in the cell.
func (g *editGrid) closeList() gridOutcome {
	l := g.list
	g.list = nil
	if l.grid.editing {
		l.grid.stopEdit()
	}
	items, marked := l.items()
	c := g.cell()
	changed := !slices.Equal(items, c.items) || marked != c.marked
	c.items, c.marked = items, marked
	if changed {
		c.field = textField{} // a default the file had among none of the items goes
	}
	return g.commit(changed)
}

func (l *gridList) draw(screen uv.Screen, anchor, bounds image.Rectangle, pal *palette) {
	inner := &l.grid
	shown := inner.shown()
	lines := min(len(shown), maxListRows)
	if l.filter.on() {
		lines++
	}
	l.box = placeBox(anchor, bounds, max(anchor.Dx()+2, 34), lines+2)
	area := drawBox(screen, l.box, plural(len(inner.rows), l.noun), pal)
	y := area.Min.Y
	if l.filter.on() {
		kept := len(shown) - len(inner.buttons)
		l.filter.draw(screen, area.Min.X+1, y, area.Max.X, filterCount(kept, len(inner.rows)), pal)
		y++
	}
	inner.hits = inner.hits[:0]
	for _, row := range inner.window(area.Max.Y - y) {
		l.drawRow(screen, image.Rect(area.Min.X, y, area.Max.X, y+1), row, pal)
		y++
	}
}

func (l *gridList) drawRow(screen uv.Screen, line image.Rectangle, row int, pal *palette) {
	inner := &l.grid
	if b, ok := inner.buttonAt(row); ok {
		inner.drawButton(screen, line, row, b, pal)
		return
	}
	x := line.Min.X
	if l.marking {
		mark := map[bool]string{true: "★", false: "·"}[inner.at(row, 0).on]
		inner.drawCell(screen, image.Rect(x, line.Min.Y, x+2, line.Max.Y), row, 0, mark, pal.panelHeading, pal)
		x += 2
	}
	end := line.Max.X
	if row == inner.row && !inner.editing {
		end -= 2
		inner.drawMoveArrows(screen, image.Pt(end, line.Min.Y), pal)
	}
	inner.drawCell(screen, image.Rect(x, line.Min.Y, end, line.Max.Y), row, l.textColumn(), inner.at(row, l.textColumn()).value(), pal.panelText, pal)
}

func (g *editGrid) buttonAt(row int) (int, bool) {
	return row - len(g.rows), row >= len(g.rows)
}

// drawButton draws a button row, with the key that presses it while it is focused.
func (g *editGrid) drawButton(screen uv.Screen, line image.Rectangle, row, button int, pal *palette) {
	g.hit(line, row, 0)
	text, style := g.buttons[button], pal.panelMuted
	if g.lit(row, 0) {
		text, style = "[enter] "+text, pal.selectedRow
		fillRect(screen, line, style)
	}
	drawTextCut(screen, line.Min.X, line.Min.Y, line.Max.X, text, style)
}

// drawMoveArrows draws the arrows that move the focused row, two cells wide at, for clicks.
func (g *editGrid) drawMoveArrows(screen uv.Screen, at image.Point, pal *palette) {
	if !g.arranged {
		return
	}
	drawText(screen, at.X, at.Y, at.X+1, "↑", pal.panelMuted)
	drawText(screen, at.X+1, at.Y, at.X+2, "↓", pal.panelMuted)
	g.hit(image.Rect(at.X, at.Y, at.X+1, at.Y+1), g.row, hitMoveUp)
	g.hit(image.Rect(at.X+1, at.Y, at.X+2, at.Y+1), g.row, hitMoveDown)
}

// drawOverlay draws the menu or list open under the focused cell, within bounds.
func (g *editGrid) drawOverlay(screen uv.Screen, bounds image.Rectangle, pal *palette) {
	anchor, ok := g.focusRect()
	switch {
	case !ok:
	case g.menu != nil:
		g.menu.draw(screen, anchor, bounds, pal)
	case g.list != nil:
		g.list.draw(screen, anchor, bounds, pal)
	}
}

// placeBox puts a box width by height under anchor, or over it when there is more room there,
// within bounds.
func placeBox(anchor, bounds image.Rectangle, width, height int) image.Rectangle {
	width = min(width, bounds.Dx())
	x := min(max(anchor.Min.X-1, bounds.Min.X), bounds.Max.X-width)
	below, above := bounds.Max.Y-anchor.Max.Y, anchor.Min.Y-bounds.Min.Y
	if below >= height || below >= above {
		return image.Rect(x, anchor.Max.Y, x+width, anchor.Max.Y+min(height, below))
	}
	return image.Rect(x, anchor.Min.Y-min(height, above), x+width, anchor.Min.Y)
}

// drawBox clears and frames box, and gives the room inside.
func drawBox(screen uv.Screen, box image.Rectangle, title string, pal *palette) image.Rectangle {
	fillRect(screen, box, pal.panelText)
	drawFrame(screen, box, title, pal.focusedFrame, pal.focusedTitle)
	return box.Inset(1)
}

// moreText tells how many items there are past the one shown, as +22, or +9999 and more.
func moreText(n int) string {
	if n > 9999 {
		return "+9999 and more"
	}
	return "+" + strconv.Itoa(n)
}
