package app

import (
	"encoding/base64"
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// textPos is a cell of a pane's text: its column, and its line counted from the oldest line of
// the history, so it stays the same cell while the pane scrolls.
type textPos struct {
	col, line int
}

func (p textPos) before(q textPos) bool {
	return p.line < q.line || (p.line == q.line && p.col < q.col)
}

// textSelection is text being selected with the mouse in one pane, from the cell where the button
// went down to the cell under the mouse, in reading order. It never leaves that pane: the mouse
// outside it holds the selection at its edge, and past the top or bottom scrolls the history as a
// terminal does. Releasing the button copies it; it stays highlighted until the next click or key.
type textSelection struct {
	tab      *tab
	pane     layout.PaneID
	anchor   textPos
	head     textPos
	dragging bool
	// mouse is where the mouse was last seen, to follow it when the wheel scrolls.
	mouse image.Point
}

// ordered is the selection's first and last cell.
func (s *textSelection) ordered() (textPos, textPos) {
	if s.head.before(s.anchor) {
		return s.head, s.anchor
	}
	return s.anchor, s.head
}

func (s *textSelection) covers(p textPos) bool {
	first, last := s.ordered()
	return !p.before(first) && !last.before(p)
}

// footed views keep rows at the bottom of their content that are not text, such as a status line.
type footed interface {
	FooterRows() int
}

// textHolder views show the text of another view inside them, such as a script's terminal.
type textHolder interface {
	Terminal() view
}

// paneText is how a pane's text lies on screen: where it shows, which view holds it, and which
// line of the history is on the top row.
type paneText struct {
	content image.Rectangle
	source  view
	history int
	top     int
}

func (a *App) paneText(t *tab, id layout.PaneID) (paneText, bool) {
	frame, ok := t.frameOf(id, a.workspaceArea())
	if !ok {
		return paneText{}, false
	}
	v := t.views[id]
	text := paneText{content: contentRect(frame), source: v}
	if f, ok := v.(footed); ok {
		text.content.Max.Y -= f.FooterRows()
	}
	if holder, ok := v.(textHolder); ok {
		text.source = holder.Terminal()
	}
	if s, ok := text.source.(scrollable); ok {
		text.history = s.ScrollbackLen()
	}
	offset := 0
	if a.mode == scrollMode && id == t.focused {
		offset = a.scrollOffset
	}
	text.top = text.history - offset
	return text, true
}

// at is the text cell at a screen point, held inside the content.
func (p paneText) at(point image.Point) textPos {
	x := min(max(point.X, p.content.Min.X), p.content.Max.X-1)
	y := min(max(point.Y, p.content.Min.Y), p.content.Max.Y-1)
	return textPos{col: x - p.content.Min.X, line: p.top + y - p.content.Min.Y}
}

func (a *App) startSelection(t *tab, id layout.PaneID, at image.Point) {
	text, ok := a.paneText(t, id)
	if !ok || !at.In(text.content) {
		return
	}
	pos := text.at(at)
	a.selection = &textSelection{tab: t, pane: id, anchor: pos, head: pos, dragging: true, mouse: at}
}

// extendSelection follows the mouse. Past the top or bottom of the pane it scrolls the history one
// line, so a selection can grow beyond what fits on screen.
func (a *App) extendSelection(m uv.Mouse) {
	s := a.selection
	s.mouse = image.Pt(m.X, m.Y)
	text, ok := a.paneText(s.tab, s.pane)
	if !ok {
		a.selection = nil
		return
	}
	switch {
	case m.Y < text.content.Min.Y:
		a.scrollSelectedPane(1)
	case m.Y >= text.content.Max.Y:
		a.scrollSelectedPane(-1)
	}
	a.followSelection()
}

// followSelection puts the selection's end under the mouse again, after the pane scrolled.
func (a *App) followSelection() {
	s := a.selection
	if s == nil || !s.dragging {
		return
	}
	if text, ok := a.paneText(s.tab, s.pane); ok {
		s.head = text.at(s.mouse)
	}
}

// scrollSelectedPane scrolls the history of the pane being selected in by lines (up when
// positive), entering scroll mode, and leaving it at the live screen.
func (a *App) scrollSelectedPane(lines int) {
	t := a.currentTab()
	s, ok := t.views[a.selection.pane].(scrollable)
	if !ok || t.focused != a.selection.pane || s.ScrollbackLen() == 0 {
		return
	}
	if a.mode != scrollMode {
		if lines < 0 {
			return
		}
		a.mode, a.scrollOffset = scrollMode, 0
	}
	a.scrollTo(s, a.scrollOffset+lines)
	if a.scrollOffset == 0 {
		a.mode = normalMode
	}
}

// finishSelection copies the selected text; a click without dragging selects nothing.
func (a *App) finishSelection() {
	s := a.selection
	s.dragging = false
	if s.anchor == s.head {
		a.selection = nil
		return
	}
	text := a.selectedText()
	if text == "" {
		return
	}
	a.copyToClipboard(text)
	a.notice = "copied to the clipboard"
}

// copyToClipboard puts text on the clipboard of the attached terminal, with OSC 52.
func (a *App) copyToClipboard(text string) {
	if a.screen != nil {
		_, _ = a.screen.WriteString("\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\x07")
	}
}

// selectedText reads the selected cells, one line per row, without the spaces that pad each line.
// Lines scrolled out of view are read from the history.
func (a *App) selectedText() string {
	s := a.selection
	text, ok := a.paneText(s.tab, s.pane)
	if !ok {
		return ""
	}
	first, last := s.ordered()
	var lines []string
	for i, cells := range text.lines(first.line, last.line) {
		from, to := 0, len(cells)
		if i == 0 {
			from = min(first.col, len(cells))
		}
		if first.line+i == last.line {
			to = min(last.col+1, len(cells))
		}
		lines = append(lines, strings.TrimRight(strings.Join(cells[from:max(to, from)], ""), " "))
	}
	return strings.Join(lines, "\n")
}

// lines reads the cells of lines first to last, each cell's text in its column.
func (p paneText) lines(first, last int) [][]string {
	width, rows := p.content.Dx(), p.content.Dy()
	buffer := uv.NewScreenBuffer(width, rows)
	scrolled, isScrollable := p.source.(scrollable)
	var lines [][]string
	for start := first; start <= last; start += rows {
		// Draw the part of the history that starts at line start, or the live screen.
		top := min(start, p.history)
		buffer.Clear()
		if isScrollable {
			scrolled.DrawScrolled(buffer, buffer.Bounds(), p.history-top)
		} else {
			top = 0
			p.source.Draw(buffer, buffer.Bounds())
		}
		for line := start; line <= min(last, start+rows-1); line++ {
			lines = append(lines, rowCells(&buffer, line-top, width))
		}
	}
	return lines
}

// rowCells is the text of each column of a row; a wide character's text goes in its first column.
func rowCells(buffer *uv.ScreenBuffer, y, width int) []string {
	cells := make([]string, width)
	for x := 0; x < width; x++ {
		if cell := buffer.CellAt(x, y); cell != nil {
			cells[x] = cell.Content
		}
	}
	return cells
}

// highlightSelection shows the selected cells of a pane reversed.
func (a *App) highlightSelection(screen uv.Screen, t *tab, id layout.PaneID) {
	s := a.selection
	if s == nil || s.tab != t || s.pane != id {
		return
	}
	text, ok := a.paneText(t, id)
	if !ok {
		return
	}
	for y := text.content.Min.Y; y < text.content.Max.Y; y++ {
		for x := text.content.Min.X; x < text.content.Max.X; x++ {
			if !s.covers(textPos{col: x - text.content.Min.X, line: text.top + y - text.content.Min.Y}) {
				continue
			}
			cell := screen.CellAt(x, y)
			if cell == nil {
				continue
			}
			highlighted := *cell
			highlighted.Style.Attrs |= uv.AttrReverse
			screen.SetCell(x, y, &highlighted)
		}
	}
}
