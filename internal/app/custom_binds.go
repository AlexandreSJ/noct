package app

import (
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/script"
)

// programKeys stay with the program in the pane: interrupting, ending input and suspending.
var programKeys = []string{"ctrl+c", "ctrl+d", "ctrl+z", "ctrl+\\"}

// customBindFor is the user's bind on the key, pressed as it is or after the prefix.
func (a *App) customBindFor(key uv.KeyPressEvent, afterPrefix bool) (config.CustomBind, bool) {
	for _, b := range a.config.CustomBinds {
		pressed, prefixed, problem := a.customBindKey(b)
		if problem == "" && prefixed == afterPrefix && pressed.String() == key.String() {
			return b, true
		}
	}
	return config.CustomBind{}, false
}

// customBindProblems say why binds of the config are left out.
func (a *App) customBindProblems() []string {
	var problems []string
	for _, b := range a.config.CustomBinds {
		if _, _, problem := a.customBindKey(b); problem != "" {
			problems = append(problems, problem)
		}
	}
	return problems
}

// customBindKey is the key a bind takes and whether it comes after the prefix, or why the bind
// cannot be used.
func (a *App) customBindKey(b config.CustomBind) (key uv.KeyPressEvent, afterPrefix bool, problem string) {
	if _, _, err := b.Target(); err != nil {
		return key, false, err.Error()
	}
	name, afterPrefix := strings.CutPrefix(b.Key, config.AfterPrefix)
	key, ok := keyEvent(name)
	switch {
	case !ok:
		return key, false, b.Key + " cannot be told apart by noct"
	case afterPrefix && a.isCommandsKey(key.String()):
		return key, false, b.Key + " is a noct command"
	case afterPrefix:
		return key, true, ""
	case a.prefix.matches(key):
		return key, false, b.Key + " is the prefix"
	case isPlainText(key) || key.Mod&(uv.ModCtrl|uv.ModAlt) == 0:
		return key, false, b.Key + " needs ctrl or alt, or the prefix before it"
	case slices.Contains(programKeys, key.String()):
		return key, false, b.Key + " stays with the program in the pane"
	}
	return key, false, ""
}

// isCommandsKey tells whether a key after the prefix already does something in noct mode.
func (a *App) isCommandsKey(name string) bool {
	c, _ := keyContextNamed("commands")
	for _, action := range c.actions {
		if slices.Contains(a.keymap.actionKeys(c.id, action), name) {
			return true
		}
	}
	_, isTabNumber := tabNumberKeys[name]
	return isTabNumber || name == "esc"
}

// customBindName names a bind in the search and its pane: its name, or its key.
func customBindName(b config.CustomBind) string {
	if strings.TrimSpace(b.Name) != "" {
		return b.Name
	}
	return b.Key
}

// runCustomBind runs what the bind names. A command or a script runs beside the focused pane,
// which closes when it ends well; a flow runs as from the inspect.
func (a *App) runCustomBind(b config.CustomBind) {
	kind, what, err := b.Target()
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	switch kind {
	case config.CustomCommand:
		s, err := script.FromCommand(customBindName(b), what, a.terminalDir(a.currentTab()))
		if err != nil {
			a.statusMessage = "bind " + b.Key + ": " + err.Error()
			return
		}
		a.askAndRun(s, true)
	case config.CustomScript:
		s, ok := a.scripts.Find(what)
		if !ok {
			a.statusMessage = "bind " + b.Key + ": no script " + what
			return
		}
		a.askAndRun(s, true)
	case config.CustomFlow:
		a.reloadFlows()
		for _, f := range a.flows {
			if f.Name == what {
				a.runFlow(f)
				return
			}
		}
		a.statusMessage = "bind " + b.Key + ": no flow " + what
	}
}

// customBindItems are the binds the search lists.
func (a *App) customBindItems() []finderItem {
	var items []finderItem
	for _, b := range a.config.CustomBinds {
		if _, _, problem := a.customBindKey(b); problem == "" {
			items = append(items, finderItem{label: customBindName(b), kind: "bind", open: func() { a.runCustomBind(b) }})
		}
	}
	return items
}
