package app

import (
	"image"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

const (
	stepPickerWidth = 56
	stepPickerRows  = 12
)

type floatingEditor interface {
	boxSize() image.Point
}

type stepPicker struct {
	pal         *palette
	list        finder
	stage       int
	path, again []string
	added       func(path []string)
	rows        image.Rectangle
	first       int
	box         image.Rectangle
}

func newStepPicker(pal *palette, items []finderItem, again []string, added func([]string)) *stepPicker {
	s := &stepPicker{pal: pal, again: again, added: added}
	s.show("add step", items)
	return s
}

func (s *stepPicker) show(title string, items []finderItem) {
	s.list = finder{title: title, items: items}
	s.list.filter()
	s.stage++
}

func (s *stepPicker) title() string        { return s.list.title }
func (s *stepPicker) typing() bool         { return true }
func (s *stepPicker) paste(text string)    { s.list.query.insert(text); s.list.filter() }
func (s *stepPicker) takeCopied() string   { return "" }
func (s *stepPicker) help() []styledLine   { return nil }
func (s *stepPicker) boxSize() image.Point { return image.Pt(stepPickerWidth, stepPickerRows+6) }
func (s *stepPicker) hints() string {
	return "type to filter  {up}{down} select  {enter} add  tab again  esc cancel"
}

func (s *stepPicker) pick(item finderItem) bool {
	s.path = append(s.path, item.label)
	stage := s.stage
	item.open()
	if s.stage != stage {
		return false
	}
	s.added(s.path)
	return true
}

// ghost is the rest of the step added last after what is typed, while it starts so; tab adds it.
func (s *stepPicker) ghost() (string, bool) {
	if len(s.again) == 0 || s.stage > 1 {
		return "", false
	}
	typed := s.list.query.String()
	if !strings.HasPrefix(strings.ToLower(s.again[0]), strings.ToLower(typed)) {
		return "", false
	}
	return s.again[0][len(typed):], true
}

func (s *stepPicker) pickAgain() bool {
	for _, label := range s.again {
		i := slices.IndexFunc(s.list.items, func(item finderItem) bool { return item.label == label })
		if i < 0 {
			return false
		}
		if s.pick(s.list.items[i]) {
			return true
		}
	}
	return false
}

func (s *stepPicker) handleKey(key uv.KeyPressEvent) bool {
	f := &s.list
	switch name := key.String(); name {
	case "esc":
		return true
	case "enter":
		return len(f.matches) > 0 && s.pick(f.matches[f.selected])
	case "tab":
		if _, ok := s.ghost(); ok {
			return s.pickAgain()
		}
	case "up", "ctrl+p":
		f.selected = max(f.selected-1, 0)
	case "down", "ctrl+n":
		f.selected = min(f.selected+1, max(len(f.matches)-1, 0))
	case "pgup", "pgdown":
		f.selected, _ = listJump(name, f.selected, len(f.matches), stepPickerRows)
	default:
		if f.query.handleKey(key) {
			f.filter()
		}
	}
	return false
}

func (s *stepPicker) mouse(event uv.MouseEvent, at image.Point) bool {
	f := &s.list
	switch e := event.(type) {
	case uv.MouseWheelEvent:
		by := map[uv.MouseButton]int{uv.MouseWheelUp: -1, uv.MouseWheelDown: 1}[e.Button]
		f.selected = min(max(f.selected+by, 0), max(len(f.matches)-1, 0))
	case uv.MouseClickEvent:
		if !at.In(s.box) {
			return true
		}
		if i, ok := s.itemAt(at); ok {
			return s.pick(f.matches[i])
		}
	}
	return false
}

func (s *stepPicker) itemAt(at image.Point) (int, bool) {
	i := s.first + at.Y - s.rows.Min.Y
	return i, at.In(s.rows) && i < len(s.list.matches)
}

func (s *stepPicker) hitAt(at image.Point) (image.Rectangle, bool, bool) {
	i, ok := s.itemAt(at)
	if !ok {
		return image.Rectangle{}, false, false
	}
	return image.Rect(s.rows.Min.X, at.Y, s.rows.Max.X, at.Y+1), i == s.list.selected, true
}

func (s *stepPicker) draw(screen uv.Screen, area image.Rectangle, _ bool) {
	f, pal := &s.list, s.pal
	s.box = area.Inset(-1)
	x := drawText(screen, area.Min.X+1, area.Min.Y, area.Max.X, "› ", pal.panelHeading)
	end := f.query.draw(screen, x, area.Min.Y, area.Max.X-1-x, false, pal.typing(), pal.selectedRow)
	if ghost, ok := s.ghost(); ok {
		drawGhost(screen, image.Pt(end-1, area.Min.Y), image.Rect(x, area.Min.Y, area.Max.X-1, area.Min.Y+1), ghost, pal)
	}
	s.rows = image.Rect(area.Min.X, area.Min.Y+2, area.Max.X, min(area.Min.Y+2+stepPickerRows, area.Max.Y-2))
	s.first = listFirst(s.first, f.selected, len(f.matches), s.rows.Dy())
	if len(f.matches) == 0 {
		drawText(screen, area.Min.X+1, s.rows.Min.Y, area.Max.X, "nothing matches", pal.panelMuted)
	}
	for i := s.first; i < len(f.matches) && i-s.first < s.rows.Dy(); i++ {
		style := pal.panelText
		if i == f.selected {
			style = pal.selectedRow
		}
		drawRow(screen, s.rows, s.rows.Min.Y+i-s.first, f.matches[i].label, f.matches[i].kind, style)
	}
	if _, ok := s.ghost(); ok {
		drawText(screen, area.Min.X+1, area.Max.Y-1, area.Max.X, "tab adds "+strings.Join(s.again, " ")+" again", pal.panelMuted)
	}
}
