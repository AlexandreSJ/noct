package app

import (
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// textPrompt asks for one line of text, such as a tab's or a new script's name.
type textPrompt struct {
	title  string
	hints  string
	text   []rune
	submit func(text string)
	// cancel, when set, runs after esc.
	cancel func()
	// pick, when set, lets ctrl+v search for text to type, such as a flow's variables.
	pick func()
}

// confirmation asks before doing something that cannot be undone.
type confirmation struct {
	question string
	accept   func()
	// again is the key that asked, which also confirms: prefix+q, then q again, quits.
	again string
}

func (a *App) openPrompt(title, initial, hints string, submit func(text string)) {
	a.prompt = textPrompt{title: title, hints: hints, text: []rune(initial), submit: submit}
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
		a.prompt.submit(strings.TrimSpace(string(a.prompt.text)))
	case "backspace":
		a.prompt.text = deleteLastRune(a.prompt.text)
	case "ctrl+u":
		a.prompt.text = nil
	case "ctrl+v":
		if a.prompt.pick != nil {
			a.prompt.pick()
		}
	default:
		if isPlainText(key) {
			a.prompt.text = append(a.prompt.text, []rune(key.Text)...)
		}
	}
}

func deleteLastRune(text []rune) []rune {
	if len(text) == 0 {
		return text
	}
	return text[:len(text)-1]
}

func isPlainText(key uv.KeyPressEvent) bool {
	return key.Text != "" && key.Mod&(uv.ModCtrl|uv.ModAlt) == 0
}

func (a *App) drawPrompt(screen uv.Screen) {
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, 50, panelHeight(3)), a.prompt.title)
	drawRow(screen, inner, inner.Min.Y, string(a.prompt.text)+"▏", "", a.palette.selectedRow)
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
	}
}
