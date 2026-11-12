package app

import (
	"image"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// choice asks which of a few things to do, such as what to create.
type choice struct {
	title    string
	options  []choiceOption
	selected int
}

type choiceOption struct {
	key   string
	label string
	pick  func()
}

func (a *App) openChoice(title string, options ...choiceOption) {
	a.choice = choice{title: title, options: options}
	a.mode = choiceMode
}

// handleChoiceKey picks with enter or an option's key; esc gives up.
func (a *App) handleChoiceKey(key uv.KeyPressEvent) {
	c := &a.choice
	name := key.String()
	switch name {
	case "esc":
		a.mode = normalMode
		return
	case "up", "k", "shift+tab":
		c.selected = (c.selected + len(c.options) - 1) % len(c.options)
		return
	case "down", "j", "tab":
		c.selected = (c.selected + 1) % len(c.options)
		return
	case "pgup", "pgdown", "home", "end":
		c.selected, _ = listJump(name, c.selected, len(c.options), len(c.options))
		return
	}
	for i, option := range c.options {
		if name == option.key || (name == "enter" && i == c.selected) {
			a.mode = normalMode // before pick, which may open a prompt
			option.pick()
			return
		}
	}
}

func (a *App) drawChoice(screen uv.Screen) {
	c := &a.choice
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	hints := a.hint("choice", "{enter} choose  esc cancel")
	inner := a.drawPanel(screen, centeredRect(area, 34, panelHeight(len(c.options)+2+moreHintRows(hints, 34))), c.title)
	for i, option := range c.options {
		style := a.palette.panelText
		if i == c.selected {
			style = a.palette.selectedRow
		}
		drawRow(screen, inner, inner.Min.Y+i, option.label, option.key, style)
	}
	drawHints(screen, inner, hints, a.palette.panelMuted)
}
