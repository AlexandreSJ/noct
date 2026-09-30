package app

import (
	"strings"

	"github.com/AlexandreSJ/noct/internal/config"
)

// panelKind is a kind of panel a noct mode key opens: its name, as the config calls it, the key,
// and how it starts.
type panelKind struct {
	name  string
	key   string
	start func(*App) viewStarter
}

// panelKinds are the panels the keys open, in the order the config offers them for a new tab.
// A kind added here opens with its key and is one of a new tab's choices; its key also needs a
// line in commandShortcuts and a place in actionKeys, which a test checks.
var panelKinds = []panelKind{
	{config.NewTabTerminal, "p", func(a *App) viewStarter { return a.startTerminal }},
	{"scripts", "s", func(a *App) viewStarter { return a.startScripts }},
	{"flows", "i", func(a *App) viewStarter { return a.startInspect }},
	{"logs", "v", func(a *App) viewStarter { return a.startLogs }},
}

// panelKindWithKey is the kind of panel a noct mode key opens.
func panelKindWithKey(key string) (panelKind, bool) {
	for _, k := range panelKinds {
		if k.key == key {
			return k, true
		}
	}
	return panelKind{}, false
}

// openNewTab opens a tab with the panel the config chooses: none, or a terminal when the config
// names a panel noct does not have.
func (a *App) openNewTab() {
	a.openEmptyTab()
	if a.config.NewTab == config.NewTabEmpty {
		return
	}
	kind := panelKinds[0]
	for _, k := range panelKinds {
		if k.name == a.config.NewTab {
			kind = k
		}
	}
	a.openPanel(kind.start(a))
}

// newTabChoices are what a new tab can open with, as the config cycles through them.
func newTabChoices() []string {
	choices := []string{config.NewTabEmpty}
	for _, k := range panelKinds {
		choices = append(choices, k.name)
	}
	return choices
}

// newTabLabel names a choice for a new tab with the keys that open that panel.
func (a *App) newTabLabel(choice string) string {
	for _, k := range panelKinds {
		if k.name == choice {
			return k.name + " (" + a.keysFor(k.key) + ")"
		}
	}
	return choice
}

// keysFor is how to press a noct mode key: with the direct shortcut, or after the prefix.
func (a *App) keysFor(key string) string {
	if a.shortcut.mods == 0 {
		return a.prefix.String() + " " + key
	}
	return strings.TrimSuffix(a.shortcut.String(), "key") + key
}
