package app

import (
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
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
	// variables are what a template typed here can read, offered as {{. is typed; suggested is
	// the one picked.
	variables []flow.Variable
	suggested int
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
	case "tab":
		a.prompt.complete()
	case "up", "down":
		a.prompt.moveSuggestion(map[string]int{"up": -1, "down": 1}[key.String()])
	default:
		a.prompt.suggested = 0
		a.typeKey(key)
	}
}

func isPlainText(key uv.KeyPressEvent) bool {
	return key.Text != "" && key.Mod&(uv.ModCtrl|uv.ModAlt) == 0
}

func (a *App) drawPrompt(screen uv.Screen) {
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	matches, _ := a.prompt.suggestions()
	rows := min(len(matches), maxSuggestions)
	inner := a.drawPanel(screen, centeredRect(area, 50+20*min(rows, 1), panelHeight(3+rows)), a.prompt.title)
	a.prompt.input.draw(screen, inner.Min.X+1, inner.Min.Y, inner.Dx()-2, false, a.palette.typing(), a.palette.selectedRow)
	a.drawSuggestions(inner, inner.Min.Y+1, func(x, y, maxX int, text string, selected bool) {
		style := a.palette.panelMuted
		if selected {
			style = a.palette.typing()
		}
		drawText(screen, x, y, maxX, text, style)
	})
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("prompt", a.prompt.hints), a.palette.panelMuted)
}

func (a *App) askToConfirm(question string, accept func()) {
	a.askToConfirmWith(question, "", accept)
}

func (a *App) askToConfirmWith(question, again string, accept func()) {
	a.confirm = confirmation{question: question, accept: accept, again: again}
	a.mode = confirmMode
}

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
