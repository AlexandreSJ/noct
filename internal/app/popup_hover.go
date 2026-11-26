package app

import (
	"image"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// popupRowAt is the row of the open popup at point that a click works, and whether it is the
// selected one: the rows hovering lights in popups, as in the panes' lists.
func (a *App) popupRowAt(point image.Point) (image.Rectangle, bool, bool) {
	row := func(list image.Rectangle) image.Rectangle {
		return image.Rect(list.Min.X, point.Y, list.Max.X, point.Y+1)
	}
	switch a.mode {
	case configMode:
		l := a.configLayout()
		line, ok := l.lineAt(point.Y)
		if !ok || !line.selectable() || !point.In(l.list) || a.configPanel.page == themePage {
			return image.Rectangle{}, false, false
		}
		return row(l.list), line.row == a.configPanel.selected, true
	case commandMode:
		inner := a.shortcutsPopupArea(a.availableShortcuts(commandShortcuts), a.shortcutNote()).Inset(2)
		i := point.Y - inner.Min.Y
		return row(inner), false, point.In(inner) && i < len(a.availableShortcuts(commandShortcuts))
	case agentsMode:
		rect, lines, _ := a.agentsLayout()
		inner := rect.Inset(2)
		i := point.Y - inner.Min.Y
		if !point.In(inner) || i >= len(lines) || lines[i].row < 0 {
			return image.Rectangle{}, false, false
		}
		selected := a.agentsPopup.selected
		if a.agentsPopup.models != nil {
			selected = a.agentsPopup.models.selected
		}
		return row(inner), lines[i].row == selected, true
	case choiceMode:
		i, ok := a.choiceAt(point)
		return row(a.choiceInner()), ok && i == a.choice.selected, ok
	case finderMode:
		i, ok := a.finderAt(point)
		return row(a.finderRows()), ok && i == a.finder.selected, ok
	}
	return image.Rectangle{}, false, false
}

func (a *App) choicePanel() image.Rectangle {
	hints := a.hint("choice", choiceHints)
	return centeredRect(image.Rect(0, 0, a.size.X, a.size.Y), 34, panelHeight(len(a.choice.options)+2+moreHintRows(hints, 34)))
}

func (a *App) choiceInner() image.Rectangle { return a.choicePanel().Inset(2) }

func (a *App) choiceAt(point image.Point) (int, bool) {
	inner := a.choiceInner()
	i := point.Y - inner.Min.Y
	return i, point.In(inner) && i < len(a.choice.options)
}

func (a *App) clickChoice(point image.Point) {
	if i, ok := a.choiceAt(point); ok {
		a.choice.selected = i
		a.handleChoiceKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	} else if !point.In(a.choicePanel()) {
		a.mode = normalMode
	}
}

func (a *App) finderPanel() image.Rectangle {
	hints := a.hint("search", finderHints)
	return centeredRect(image.Rect(0, 0, a.size.X, a.size.Y), 60, panelHeight(finderResults+3+moreHintRows(hints, 60)))
}

func (a *App) finderRows() image.Rectangle {
	inner := a.finderPanel().Inset(2)
	return image.Rect(inner.Min.X, inner.Min.Y+2, inner.Max.X, inner.Min.Y+2+finderResults)
}

func (a *App) finderAt(point image.Point) (int, bool) {
	rows := a.finderRows()
	i := a.finder.first + point.Y - rows.Min.Y
	return i, point.In(rows) && i < len(a.finder.matches)
}

func (a *App) clickFinder(point image.Point) {
	if i, ok := a.finderAt(point); ok {
		a.finder.selected = i
		a.searchKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	} else if !point.In(a.finderPanel()) {
		a.searchKey(uv.KeyPressEvent{Code: uv.KeyEscape})
	}
}
