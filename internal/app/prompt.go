package app

import (
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// textPrompt asks for one line of text, such as a tab's or a new script's name.
type textPrompt struct {
	title  string
	hints  string
	input  textField
	submit func(text string)
	// cancel, when set, runs after esc.
	cancel func()
	// pick, when set, lets ctrl+t search for text to type, such as a flow's variables.
	pick func()
}

// confirmation asks before doing something that cannot be undone.
type confirmation struct {
	question string
	accept   func()
	// cancel, when set, runs after a no, such as going back to the panel that asked.
	cancel func()
	// again is the key that asked, which also confirms: prefix+q, then q again, quits.
	again string
}

func (a *App) openPrompt(title, initial, hints string, submit func(text string)) {
	a.prompt = textPrompt{title: title, hints: hints, input: newTextField(initial), submit: submit}
	a.mode = promptMode
}

func (a *App) startRename() {
	a.openPrompt("rename tab", a.currentTab().name, "{enter} save  esc cancel  empty follows the pane", func(name string) {
		if t := a.currentTab(); t != nil {
			t.name = name
		}
	})
}

// startPaneRename names the focused pane; an empty name gives it back its program's title.
func (a *App) startPaneRename() {
	t := a.currentTab()
	if t.isEmpty() {
		return
	}
	id := t.focused
	a.openPrompt("rename panel", t.paneNames[id], "{enter} save  esc cancel  empty follows the program", func(name string) {
		if name == "" {
			delete(t.paneNames, id)
			return
		}
		t.paneNames[id] = name
	})
}

// handlePromptKey edits the text; enter hands it, trimmed, to the prompt's owner.
func (a *App) handlePromptKey(key uv.KeyPressEvent) {
	switch key.String() {
	case "esc":
		a.mode = normalMode
		if a.prompt.cancel != nil {
			a.prompt.cancel()
		}
	case "enter":
		a.mode = normalMode // before submit, which may open another panel
		a.prompt.submit(strings.TrimSpace(a.prompt.input.String()))
	case "ctrl+t":
		if a.prompt.pick != nil {
			a.prompt.pick()
		}
	default:
		a.typeKey(key)
	}
}

func isPlainText(key uv.KeyPressEvent) bool {
	return key.Text != "" && key.Mod&(uv.ModCtrl|uv.ModAlt) == 0
}

func (a *App) drawPrompt(screen uv.Screen) {
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, 50, panelHeight(3)), a.prompt.title)
	drawRow(screen, inner, inner.Min.Y, a.prompt.input.view(inner.Dx()-2, false), "", a.palette.selectedRow)
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("prompt", a.prompt.hints), a.palette.panelMuted)
}

func (a *App) askToConfirm(question string, accept func()) {
	a.askToConfirmWith(question, "", accept)
}

// askToConfirmWith asks, taking again as a yes too.
func (a *App) askToConfirmWith(question, again string, accept func()) {
	a.confirm = confirmation{question: question, accept: accept, again: again}
	a.mode = confirmMode
}

// confirmShortcuts lists the keys of the question on screen.
func (a *App) confirmShortcuts() []shortcut {
	yes := "{enter}"
	if a.confirm.again != "" {
		yes += " / " + a.confirm.again
	}
	return []shortcut{{yes, "yes", false}, {"esc", "cancel", false}}
}

func (a *App) handleConfirmKey(key uv.KeyPressEvent) {
	name := key.String()
	if name == a.confirm.again {
		name = "enter"
	}
	switch name {
	case "enter", "y":
		a.mode = normalMode
		a.confirm.accept()
	case "esc", "n":
		a.mode = normalMode
		if a.confirm.cancel != nil {
			a.confirm.cancel()
		}
	}
}

// choice asks which of a few things to do, such as what to create.
type choice struct {
	title    string
	options  []choiceOption
	selected int
}

type choiceOption struct {
	// key picks the option directly.
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
	inner := a.drawPanel(screen, centeredRect(area, 34, panelHeight(len(c.options)+2)), c.title)
	for i, option := range c.options {
		style := a.palette.panelText
		if i == c.selected {
			style = a.palette.selectedRow
		}
		drawRow(screen, inner, inner.Min.Y+i, option.label, option.key, style)
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("choice", "{enter} choose  esc cancel"), a.palette.panelMuted)
}

// infoPopup tells about something, such as what a flow's step does, until closed.
type infoPopup struct {
	title string
	lines []styledLine
	first int
	// visible and wrapped are how many lines were shown and how many the text took, as drawn.
	visible, wrapped int
}

func (a *App) openInfo(title string, lines []styledLine) {
	a.info = infoPopup{title: title, lines: lines}
	a.mode = infoMode
}

// handleInfoKey scrolls a long text; any other key closes it.
func (a *App) handleInfoKey(key uv.KeyPressEvent) {
	switch key.String() {
	case "up", "k":
		a.info.first = max(a.info.first-1, 0)
	case "down", "j":
		a.info.first++
	case "pgup", "pgdown", "home", "end":
		last := max(a.info.wrapped-a.info.visible, 0) // the first line that shows the end
		a.info.first, _ = listJump(key.String(), a.info.first, last+1, a.info.visible)
	default:
		a.mode = normalMode
	}
}

func (a *App) drawInfo(screen uv.Screen) {
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	width := min(78, a.size.X-4)
	var lines []styledLine
	for _, line := range a.info.lines {
		for _, part := range strings.Split(ansi.Wrap(line.text, width-6, ""), "\n") {
			lines = append(lines, styledLine{part, line.style})
		}
	}
	inner := a.drawPanel(screen, centeredRect(area, width, panelHeight(len(lines)+2)), a.info.title)
	visible := max(inner.Dy()-2, 1)
	a.info.first = min(a.info.first, max(len(lines)-visible, 0))
	a.info.visible, a.info.wrapped = visible, len(lines)
	for i := a.info.first; i < len(lines) && i-a.info.first < visible; i++ {
		drawText(screen, inner.Min.X+1, inner.Min.Y+i-a.info.first, inner.Max.X-1, lines[i].text, lines[i].style)
	}
	hints := "any key closes"
	if len(lines) > visible {
		hints = a.hint("info", "{up}{down} scroll  {pgup} {pgdown} page  any other key closes")
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, hints, a.palette.panelMuted)
}
