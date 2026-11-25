package app

import (
	"fmt"
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// infoPopup tells about something, such as what a flow's step does or a screen's keys, until
// closed. f or / filters its lines, the arrows, the page keys and the wheel scroll it.
type infoPopup struct {
	title string
	lines []styledLine
	first int
	// visible and wrapped are how many lines were shown and how many the text took, as drawn.
	visible, wrapped int
	// back is the mode closing it goes back to, such as the config it was opened from.
	back mode
	// filter keeps the lines that hold every word typed; rect is where the popup was drawn, so a
	// click outside closes it and one inside does not.
	filter listFilter
	rect   image.Rectangle
}

func (a *App) openInfo(title string, lines []styledLine) {
	a.info = infoPopup{title: title, lines: lines, back: normalMode}
	a.mode = infoMode
}

// infoWheelLines is how far a turn of the wheel scrolls the popup.
const infoWheelLines = 3

// handleInfoKey types in the filter, scrolls a long text or filters it; esc clears the filter,
// then closes, and any other key closes.
func (a *App) handleInfoKey(key uv.KeyPressEvent) {
	p := &a.info
	if p.filter.typing {
		if used, changed := p.filter.handleKey(key); used {
			if changed {
				p.first = 0
			}
			return
		}
	}
	switch name := key.String(); {
	case isFilterKey(name):
		p.filter.start()
	case name == "esc" && p.filter.on():
		p.filter.clear()
		p.first = 0
	case name == "up" || name == "k":
		p.scroll(-1)
	case name == "down" || name == "j":
		p.scroll(1)
	case name == "pgup" || name == "pgdown" || name == "home" || name == "end":
		last := max(p.wrapped-p.visible, 0) // the first line that shows the end
		p.first, _ = listJump(name, p.first, last+1, p.visible)
	default:
		a.mode = p.back
	}
}

func (p *infoPopup) scroll(by int) {
	p.first = min(max(p.first+by, 0), max(p.wrapped-p.visible, 0))
}

// clickInfo closes the popup on a click outside it; the wheel scrolls it.
func (a *App) clickInfo(at image.Point) {
	if !at.In(a.info.rect) {
		a.mode = a.info.back
	}
}

func (a *App) wheelInfo(button uv.MouseButton) {
	a.info.scroll(map[uv.MouseButton]int{uv.MouseWheelUp: -infoWheelLines, uv.MouseWheelDown: infoWheelLines}[button])
}

// infoTextWidth is how wide a line of the about popup can be before it wraps.
func (a *App) infoTextWidth() int { return min(78, a.size.X-4) - 6 }

// wrappedLines are the popup's lines wrapped to width, those the filter keeps.
func (p *infoPopup) wrappedLines(width int) []styledLine {
	var lines []styledLine
	for _, group := range infoGroups(p.lines) {
		if p.filter.query() != "" && !p.filter.keeps(groupTexts(group)...) {
			continue
		}
		for _, line := range group {
			for _, part := range strings.Split(ansi.Wrap(line.text, width, ""), "\n") {
				lines = append(lines, styledLine{part, line.style})
			}
		}
	}
	return lines
}

// infoGroups are the lines kept together by the filter: a line that starts with a space goes on
// the one before, as a key's label under its key. A blank line is a group of its own.
func infoGroups(lines []styledLine) [][]styledLine {
	var groups [][]styledLine
	for _, line := range lines {
		if len(groups) > 0 && strings.HasPrefix(line.text, " ") && strings.TrimSpace(line.text) != "" {
			groups[len(groups)-1] = append(groups[len(groups)-1], line)
			continue
		}
		groups = append(groups, []styledLine{line})
	}
	return groups
}

func groupTexts(group []styledLine) []string {
	var texts []string
	for _, line := range group {
		texts = append(texts, line.text)
	}
	return texts
}

func (a *App) drawInfo(screen uv.Screen) {
	p := &a.info
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	width := min(78, a.size.X-4)
	all := (&infoPopup{lines: p.lines}).wrappedLines(a.infoTextWidth())
	lines := p.wrappedLines(a.infoTextWidth())
	p.rect = centeredRect(area, width, panelHeight(len(all)+2)) // as tall as the whole text, filtered or not
	inner := a.drawPanel(screen, p.rect, p.title)
	if p.filter.on() {
		p.filter.draw(screen, inner.Min.X+1, inner.Min.Y, inner.Max.X-1, filterCount(len(lines), len(all)), &a.palette)
		inner.Min.Y += 2
	}
	visible := max(inner.Dy()-2, 1)
	p.first = min(p.first, max(len(lines)-visible, 0))
	p.visible, p.wrapped = visible, len(lines)
	for i := p.first; i < len(lines) && i-p.first < visible; i++ {
		drawText(screen, inner.Min.X+1, inner.Min.Y+i-p.first, inner.Max.X-1, lines[i].text, lines[i].style)
	}
	drawHints(screen, inner, a.infoHints(len(lines), visible), a.palette.panelMuted)
}

// infoHints are the popup's keys, and where the lines shown are in a text taller than it.
func (a *App) infoHints(total, visible int) string {
	p := &a.info
	if p.filter.typing {
		return a.hint("info", "{enter} keep  esc clear")
	}
	hints := "{f} filter  esc closes"
	if total > visible {
		end := min(p.first+visible, total)
		hints = fmt.Sprintf("{up}{down} scroll  {pgup}{pgdown} page  {f} filter  esc closes  %d-%d of %d", p.first+1, end, total)
	}
	return a.hint("info", hints)
}
