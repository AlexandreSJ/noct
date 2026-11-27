package app

import (
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// A command of the "commands" context is a command everywhere: it has its row in the commands
// popup, the direct shortcut takes it, and runAction runs it. A command added to keyContexts and
// missing from one of them fails here, as move-tab once did with the direct shortcut and the
// search.
func TestEveryCommandIsWiredEverywhere(t *testing.T) {
	c, _ := keyContextNamed("commands")
	for _, action := range c.actions {
		if nonActingCommands[action.id] {
			continue
		}
		key := action.defaults[0]
		if !slices.ContainsFunc(commandShortcuts, func(s shortcut) bool { return s.keys == key }) {
			t.Errorf("%s: no row %q in commandShortcuts, the commands popup", action.id, key)
		}
		if !isCommandKey(key) {
			t.Errorf("%s: the direct shortcut does not take %q", action.id, key)
		}
		a := testApp(t, sideBySideTab(1, 2))
		a.updates.checked = true // no looking for new versions
		a.runAction(key)
		if strings.HasPrefix(a.statusMessage, "no command for") {
			t.Errorf("%s: runAction does not run %q", action.id, key)
		}
	}
}

// Each row of the commands popup is a command of keyContexts, so a row never shows a key that
// does nothing; the rows that act are in the search, but the search itself.
func TestEveryCommandsPopupRowIsACommand(t *testing.T) {
	c, _ := keyContextNamed("commands")
	commandOf := map[string]string{}
	for _, action := range c.actions {
		for _, key := range action.defaults {
			commandOf[key] = action.id
		}
	}
	a := testApp(t, sideBySideTab(1, 2))
	searched := map[string]bool{}
	for _, item := range a.finderItems() {
		searched[item.label] = true
	}
	for _, s := range commandShortcuts {
		id, isCommand := commandOf[s.keys]
		switch {
		case s.keys == "←↓↑→ hjkl" || s.keys == "1-9" || s.keys == "esc":
		case !isCommand:
			t.Errorf("row %q %s is no command of keyContexts", s.keys, s.label)
		case !nonActingCommands[id] && s.keys != "space" && !searched[s.label]:
			t.Errorf("row %q %s is missing from the search", s.keys, s.label)
		}
	}
}

func TestTheDirectShortcutMovesATab(t *testing.T) {
	first, second := newTab(1, &fakeView{}), newTab(2, &fakeView{})
	a := testApp(t, first, second)
	press(a, uv.KeyPressEvent{Code: 'm', Mod: uv.ModAlt}, rightPress, enterPress)
	if !slices.Equal(a.session.tabs, []*tab{second, first}) || a.session.activeTab != 1 {
		t.Errorf("tabs %v, active %d after alt+m right", a.session.tabs, a.session.activeTab)
	}
}
