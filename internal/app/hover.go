package app

import (
	"fmt"
	"image"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// Hovering shows what a click would pick as selected: a tab's name, a pane's title or a list's
// row. Each visit plays the hover's effect once, and what the mouse left blends back.

// hoverAt hovers what is under the mouse, once per visit: it shows as selected, the hover's
// effect playing from its own colors, and what the mouse left blends back. It tells whether what
// is hovered changed.
func (a *App) hoverAt(point image.Point) bool {
	a.hoverPoint = point
	name, on := a.hoverTargetAt(point)
	if name == a.hoverTarget {
		return false
	}
	if a.hoverTarget != "" && a.effectFor(momentHover).blendsColors() {
		left := a.hoverOn
		left.closing = left.target.kind == targetListRow
		_, hovered := a.hoverStyles(left)
		a.playHover(left, hovered)
	}
	a.hoverTarget, a.hoverOn = name, on
	if name != "" {
		on.seed = int64(point.X)
		normal, _ := a.hoverStyles(on)
		a.playHover(on, normal)
	}
	return true
}

func (a *App) playHover(on animation, from uv.Style) {
	on.fromColor = from.Fg
	if an := a.newAnimation(momentHover, on, a.now()); an != nil {
		a.addAnimation(an)
	}
}

// hoverStyles are how what a hover plays on looks without the mouse and under it: a tab's name
// in the active tab's color, a pane's title as the focused pane's. A list row's own colors stay,
// each blending on its own.
func (a *App) hoverStyles(on animation) (normal, hovered uv.Style) {
	switch on.target.kind {
	case targetTabName:
		active, inactive := a.palette.forTab(on.target.tab.colors)
		if a.currentTab() == on.target.tab {
			return active, active
		}
		return inactive, hoveredTab(active, inactive)
	case targetListRow:
		return uv.Style{}, uv.Style{}
	}
	t := a.currentTab()
	if t == nil {
		return a.palette.unfocusedTitle, a.palette.focusedTitle
	}
	pal := a.palette.forPane(t.paneColors[on.pane])
	if on.pane == t.focused {
		return pal.focusedTitle, pal.focusedTitle
	}
	return pal.unfocusedTitle, pal.focusedTitle
}

// hoveredTab is an inactive tab's name under the mouse: on its own background, in the color the
// active tab stands out with.
func hoveredTab(active, inactive uv.Style) uv.Style {
	inactive.Fg = orColor(active.Bg, active.Fg)
	return inactive
}

func tabHoverName(index int) string { return fmt.Sprintf("tab %d", index) }

func paneHoverName(id layout.PaneID) string { return fmt.Sprintf("pane %d", id) }

func rowHoverName(id layout.PaneID, row image.Rectangle) string {
	return fmt.Sprintf("row %d %d", id, row.Min.Y)
}

// hoverTargetAt names what is under the mouse, a tab's name, a pane's title or a list's row not
// selected, with the animation that plays on it; "" when it is none.
func (a *App) hoverTargetAt(point image.Point) (string, animation) {
	if point.Y == 0 {
		for _, span := range a.tabBar() {
			if !span.mark && point.X >= span.start && point.X < span.end {
				return tabHoverName(span.tab), animation{pane: hoverPane, target: target{kind: targetTabName, tab: a.session.tabs[span.tab]}}
			}
		}
		return "", animation{}
	}
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		return "", animation{}
	}
	for _, p := range t.visible(a.workspaceArea()) {
		if point.Y == p.frame.Min.Y && point.X >= p.frame.Min.X && point.X < p.frame.Max.X {
			return paneHoverName(p.id), animation{pane: p.id, target: target{kind: targetPaneTitle}}
		}
	}
	if id, row, selected, ok := a.listRowAt(point); ok && !selected {
		return rowHoverName(id, row), animation{pane: id, target: target{kind: targetListRow, row: row}}
	}
	return "", animation{}
}

// rowLister is a view whose list rows light under the mouse, as rows a click selects.
type rowLister interface {
	// RowAt is the list's row at point, relative to the view, and whether it is the selected one.
	RowAt(point image.Point) (row image.Rectangle, selected, ok bool)
}

// listRowAt is the row at point of a list drawn in list, showing n rows from first, with the one
// at index selected selected.
func listRowAt(list image.Rectangle, point image.Point, first, n, selected int) (image.Rectangle, bool, bool) {
	index := first + point.Y - list.Min.Y
	if !point.In(list) || index >= n {
		return image.Rectangle{}, false, false
	}
	return image.Rect(list.Min.X, point.Y, list.Max.X, point.Y+1), index == selected, true
}

// listRowAt is the list row on screen at point, of the pane shown there, and whether it is the
// selected one. Rows light only while no popup is over the panes.
func (a *App) listRowAt(point image.Point) (layout.PaneID, image.Rectangle, bool, bool) {
	t := a.currentTab()
	if a.mode != normalMode || t == nil || t.isEmpty() {
		return 0, image.Rectangle{}, false, false
	}
	id, ok := t.paneAt(point, a.workspaceArea())
	lister, isList := t.views[id].(rowLister)
	if !ok || !isList {
		return 0, image.Rectangle{}, false, false
	}
	frame, _ := t.frameOf(id, a.workspaceArea())
	content := contentRect(frame)
	if !point.In(content) {
		return 0, image.Rectangle{}, false, false
	}
	row, selected, ok := lister.RowAt(point.Sub(content.Min))
	return id, row.Add(content.Min), selected, ok
}

// drawRowHovers lights the text of the list row under the mouse, unless it is selected, and of
// the rows the mouse left, as they blend back.
func (a *App) drawRowHovers(screen uv.Screen, at time.Time) {
	_, hovered, selected, lit := a.listRowAt(a.hoverPoint)
	lit = lit && !selected
	for _, an := range a.animations {
		if an.moment != momentHover || an.target.kind != targetListRow || !an.effect.blendsColors() {
			continue
		}
		row, p := an.target.row, an.progress(at)
		switch {
		case an.closing && (row != hovered || !lit):
			a.lightRow(screen, row, func(x, y int) float64 { return 1 - colorWeight(an, row, x, y, p) })
		case !an.closing && row == hovered && lit:
			a.lightRow(screen, row, func(x, y int) float64 { return colorWeight(an, row, x, y, p) })
			lit = false
		}
	}
	if lit {
		a.lightRow(screen, hovered, func(int, int) float64 { return 1 })
	}
}

// lightRow takes the text of row toward the selection's color, as far as weight says for each
// cell. A row selected meanwhile is left as it is.
func (a *App) lightRow(screen uv.Screen, row image.Rectangle, weight func(x, y int) float64) {
	if _, _, selected, ok := a.listRowAt(row.Min); !ok || selected {
		return
	}
	light := orColor(a.palette.selectedRow.Bg, a.palette.selectedRow.Fg)
	if light == nil {
		return
	}
	for y := row.Min.Y; y < row.Max.Y; y++ {
		for x := row.Min.X; x < row.Max.X; x++ {
			c := screen.CellAt(x, y)
			if c == nil || c.Width != 1 {
				continue
			}
			cell := *c
			switch own, w := orColor(cell.Style.Fg, a.palette.panelText.Fg), weight(x, y); {
			case own != nil:
				cell.Style.Fg = mix(own, light, w)
			case w >= 0.5:
				cell.Style.Fg = light
			}
			screen.SetCell(x, y, &cell)
		}
	}
}
