package app

import (
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// popupSelection is text dragged over in a popup, inside its frame, in reading order from where
// the button went down; releasing the button copies what the frame showed there.
type popupSelection struct {
	anchor, head image.Point
	area         image.Rectangle
	dragging     bool
	cells        uv.ScreenBuffer
}

func (a *App) startPopupSelection(at image.Point) {
	area := a.popupArea.Inset(1)
	if !isPopupMode(a.mode) || !at.In(area) {
		return
	}
	a.popupSel = &popupSelection{anchor: at, head: at, area: area, dragging: true}
}

func (a *App) extendPopupSelection(at image.Point) {
	s := a.popupSel
	s.head = image.Pt(min(max(at.X, s.area.Min.X), s.area.Max.X-1), min(max(at.Y, s.area.Min.Y), s.area.Max.Y-1))
}

func (a *App) finishPopupSelection() {
	s := a.popupSel
	s.dragging = false
	if s.anchor == s.head {
		a.popupSel = nil
		return
	}
	if text := s.text(); text != "" {
		a.copyToClipboard(text)
		a.notice = "copied to the clipboard"
	}
}

func (s *popupSelection) ordered() (image.Point, image.Point) {
	if s.head.Y < s.anchor.Y || s.head.Y == s.anchor.Y && s.head.X < s.anchor.X {
		return s.head, s.anchor
	}
	return s.anchor, s.head
}

// span is the columns of row y the selection covers, up to and not with to.
func (s *popupSelection) span(y int) (from, to int, ok bool) {
	first, last := s.ordered()
	if y < first.Y || y > last.Y {
		return 0, 0, false
	}
	from, to = s.area.Min.X, s.area.Max.X
	if y == first.Y {
		from = first.X
	}
	if y == last.Y {
		to = last.X + 1
	}
	return from, to, true
}

func (s *popupSelection) text() string {
	var lines []string
	for y := s.area.Min.Y; y < s.area.Max.Y; y++ {
		from, to, ok := s.span(y)
		if !ok {
			continue
		}
		var line strings.Builder
		for x := from; x < to; x++ {
			if c := s.cells.CellAt(x, y); c != nil {
				line.WriteString(c.Content)
			}
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	return strings.Join(lines, "\n")
}

// drawPopupSelection keeps the popup's cells for the copy and shows the selected ones reversed.
func (a *App) drawPopupSelection(screen uv.Screen) {
	s := a.popupSel
	if s == nil {
		return
	}
	if !isPopupMode(a.mode) {
		a.popupSel = nil
		return
	}
	if s.dragging {
		s.cells = a.copyArea(screen, s.area)
	}
	for y := s.area.Min.Y; y < s.area.Max.Y; y++ {
		from, to, ok := s.span(y)
		if ok {
			lightCells(screen, from, to, y)
		}
	}
}
