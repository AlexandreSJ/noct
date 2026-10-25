package app

import (
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
)

// The binds page's custom section: a row per bind of the user's, after the row that adds one.
const (
	addCustomBindRow  configRow = 2999
	customBindRowBase configRow = 3000
)

// customBindDraft is a bind being added: its key is taken first, then what it runs.
type customBindDraft struct {
	afterPrefix bool
	key         string
	// choosing is on once the key is taken, until c, s or f picks what the bind runs.
	choosing bool
	// save takes the finished bind and back returns to the screen it was made from; both are nil
	// on the binds page, which saves it to the config.
	save func(config.CustomBind)
	back func()
}

func customBindAt(row configRow, binds []config.CustomBind) (int, bool) {
	i := int(row - customBindRowBase)
	return i, i >= 0 && i < len(binds)
}

// startCustomBind takes a key for a new bind; the prefix pressed first puts the key after it.
func (a *App) startCustomBind() {
	a.configPanel.bindCapture = bindCapture{active: true, custom: &customBindDraft{}}
}

// handleCustomBindKey takes the new bind's key, then c, s or f for what it runs.
func (a *App) handleCustomBindKey(key uv.KeyPressEvent) {
	c := &a.configPanel.bindCapture
	draft := c.custom
	switch {
	case key.MatchString("esc"):
		*c = bindCapture{}
	case draft.choosing:
		a.chooseCustomBindKind(key.String())
	case a.prefix.matches(key) && !draft.afterPrefix:
		draft.afterPrefix = true
	default:
		name := key.String()
		if draft.afterPrefix {
			name = config.AfterPrefix + name
		}
		if _, _, problem := a.customBindKey(config.CustomBind{Key: name, Command: "-"}); problem != "" {
			c.problem = problem
			return
		}
		draft.key, draft.choosing, c.problem = name, true, ""
	}
}

func (a *App) chooseCustomBindKind(pressed string) {
	kinds := map[string]string{"c": config.CustomCommand, "s": config.CustomScript, "f": config.CustomFlow}
	kind, ok := kinds[pressed]
	if !ok {
		return
	}
	draft := a.configPanel.bindCapture.custom
	b := config.CustomBind{Key: draft.key}
	a.configPanel.bindCapture = bindCapture{}
	save, back := draft.save, draft.back
	if save == nil {
		save, back = func(b config.CustomBind) { a.saveCustomBind(b, -1) }, a.backToConfig
	}
	a.askCustomBindTarget(b, kind, "", save, back)
}

func (a *App) backToConfig() {
	a.mode = configMode
}

// askCustomBindTarget asks what the bind runs, then its name, and hands the bind to save.
func (a *App) askCustomBindTarget(b config.CustomBind, kind, initial string, save func(config.CustomBind), back func()) {
	a.openPrompt(kind+" for "+prettyKey(b.Key), initial, customBindPromptHints(kind), func(what string) {
		if what == "" {
			back()
			return
		}
		b.Command, b.Script, b.Flow = "", "", ""
		*customBindField(&b, kind) = what
		name := b.Name
		if name == "" {
			name = strings.Fields(what)[0]
		}
		a.openPrompt("name of "+prettyKey(b.Key), name, "{enter} save  esc cancel  the search shows it", func(name string) {
			b.Name = name
			save(b)
			back()
		})
		a.prompt.cancel = back
	})
	a.prompt.cancel = back
}

func customBindPromptHints(kind string) string {
	if kind == config.CustomCommand {
		return "{enter} next  esc cancel  each {{.name}} is asked before it runs"
	}
	return "{enter} next  esc cancel  by its name, as the " + kind + "s list it"
}

func customBindField(b *config.CustomBind, kind string) *string {
	switch kind {
	case config.CustomScript:
		return &b.Script
	case config.CustomFlow:
		return &b.Flow
	}
	return &b.Command
}

func (a *App) saveCustomBind(b config.CustomBind, index int) {
	next := a.config
	next.CustomBinds = slices.Clone(a.config.CustomBinds)
	if index < 0 {
		next.CustomBinds = append(next.CustomBinds, b)
	} else {
		next.CustomBinds[index] = b
	}
	a.applyConfig(next)
}

// editCustomBind asks again what the selected bind runs, keeping its key and kind.
func (a *App) editCustomBind(index int) {
	b := a.config.CustomBinds[index]
	kind, what, err := b.Target()
	if err != nil {
		kind, what = config.CustomCommand, ""
	}
	a.askCustomBindTarget(b, kind, what, func(b config.CustomBind) { a.saveCustomBind(b, index) }, a.backToConfig)
}

func (a *App) removeCustomBind(index int) {
	next := a.config
	next.CustomBinds = slices.Delete(slices.Clone(a.config.CustomBinds), index, index+1)
	if len(next.CustomBinds) == 0 {
		next.CustomBinds = nil
	}
	a.applyConfig(next)
}

// handleCustomBindRowKey runs enter and delete on the custom section's rows.
func (a *App) handleCustomBindRowKey(name string) bool {
	selected := a.configPanel.selected
	if selected == addCustomBindRow && (name == "enter" || name == "space") {
		a.startCustomBind()
		return true
	}
	index, ok := customBindAt(selected, a.config.CustomBinds)
	if !ok {
		return false
	}
	switch name {
	case "enter", "space", "right", "l":
		a.editCustomBind(index)
	case "delete", "backspace":
		a.removeCustomBind(index)
	default:
		return false
	}
	return true
}

// customBindLines are the custom section: the row that adds a bind, a row per bind and, while a
// bind is being added, what to press.
func (a *App) customBindLines() []configLine {
	lines := []configLine{{label: "custom (yours)", heading: true}}
	value := "n"
	if draft := a.configPanel.bindCapture.custom; draft != nil {
		value = "[" + a.customBindDraftValue(draft) + "]"
	}
	lines = append(lines, configLine{label: "add a bind", value: value, row: addCustomBindRow})
	if c := a.configPanel.bindCapture; c.custom != nil {
		if c.problem != "" {
			lines = append(lines, configLine{label: c.problem, note: true, warning: true})
		}
		lines = append(lines, configLine{label: customBindDraftNote(c.custom), note: true})
	}
	for i, b := range a.config.CustomBinds {
		kind, what, _ := b.Target()
		label := customBindName(b)
		lines = append(lines, configLine{label: label, value: prettyKey(b.Key) + "  " + kind + " " + what, row: customBindRowBase + configRow(i)})
		if _, _, problem := a.customBindKey(b); problem != "" {
			lines = append(lines, configLine{label: "⚠ " + problem + "; left out", note: true, warning: true})
		}
	}
	return lines
}

func (a *App) customBindDraftValue(draft *customBindDraft) string {
	switch {
	case draft.choosing:
		return prettyKey(draft.key)
	case draft.afterPrefix:
		return a.prefix.String() + " …"
	}
	return "press a key…"
}

func customBindDraftNote(draft *customBindDraft) string {
	if draft.choosing {
		return "c a command · s a script · f a flow · esc cancels"
	}
	return "the key pressed, with ctrl or alt, or the prefix then a key · esc cancels"
}
