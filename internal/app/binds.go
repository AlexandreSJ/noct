package app

import (
	"maps"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
)

// bindsHints are the keys of the binds page; its own keys (a, del) do not change.
const bindsHints = "{up}{down} select  {enter} change  a add a key  del back to default  esc back"

// bindRowBase is the first row of the binds page: one row per action, in keyContexts' order.
const bindRowBase configRow = 1000

// bindRow is the action a row of the binds page changes.
type bindRow struct {
	context keyContext
	action  keyAction
}

func bindRows() []bindRow {
	var rows []bindRow
	for _, c := range keyContexts {
		for _, action := range c.actions {
			rows = append(rows, bindRow{c, action})
		}
	}
	return rows
}

func bindRowAt(row configRow) (bindRow, bool) {
	rows := bindRows()
	i := int(row - bindRowBase)
	if i < 0 || i >= len(rows) {
		return bindRow{}, false
	}
	return rows[i], true
}

// bindCapture changes a key of an action. With several keys, choosing picks which one first,
// with ←→; then active takes the key pressed, in the place of slot, or after the others with add.
type bindCapture struct {
	choosing bool
	active   bool
	add      bool
	slot     int
	pending  string
	// problem says why the last key pressed cannot be used there.
	problem string
}

// startBindEdit changes the selected action's key: right away when it has one or none, or after
// choosing which when it has several.
func (a *App) startBindEdit() {
	row, ok := bindRowAt(a.configPanel.selected)
	if !ok {
		return
	}
	keys := a.shownKeys(row)
	a.configPanel.bindCapture = bindCapture{choosing: len(keys) > 1, active: len(keys) <= 1, add: len(keys) == 0}
}

// chooseBindSlot moves among the keys of the action, or takes the key in the chosen place.
func (a *App) chooseBindSlot(key uv.KeyPressEvent) {
	c := &a.configPanel.bindCapture
	row, _ := bindRowAt(a.configPanel.selected)
	switch key.String() {
	case "esc":
		*c = bindCapture{}
	case "left", "h":
		c.slot = max(c.slot-1, 0)
	case "right", "l":
		c.slot = min(c.slot+1, len(a.shownKeys(row))-1)
	case "enter", "space":
		c.choosing, c.active = false, true
	}
}

// clickBind takes a key for the one of the action's keys at column x of its value, or for its
// only one.
func (a *App) clickBind(row bindRow, x int) {
	keys := a.shownKeys(row)
	if len(keys) < 2 {
		a.startBindEdit()
		return
	}
	if _, changed := a.config.Keys[row.context.id][row.action.id]; changed {
		x -= ansi.StringWidth("• ")
	}
	slot := 0
	for i, key := range keys {
		if x >= 0 {
			slot = i
		}
		x -= ansi.StringWidth(prettyKey(key)) + len("  ")
	}
	a.configPanel.bindCapture = bindCapture{active: true, slot: slot}
}

// shownKeys are an action's keys, one per key pressed: shift+g is left out after G.
func (a *App) shownKeys(row bindRow) []string {
	var keys, pressed []string
	for _, key := range a.keymap.actionKeys(row.context.id, row.action) {
		event, _ := keyEvent(key)
		if !slices.Contains(pressed, event.String()) {
			pressed = append(pressed, event.String())
			keys = append(keys, key)
		}
	}
	return keys
}

// openBinds shows the binds page, on its first action.
func (a *App) openBinds() {
	a.commitTyped()
	a.configPanel.page, a.configPanel.selected = bindsPage, bindRowBase
	a.configPanel.resetArmed = false
}

// handleBindsKey runs the binds page's keys; esc goes back to the settings.
func (a *App) handleBindsKey(key uv.KeyPressEvent) {
	switch c := a.configPanel.bindCapture; {
	case c.active:
		a.captureBind(key)
		return
	case c.choosing:
		a.chooseBindSlot(key)
		return
	}
	switch key.String() {
	case "esc":
		a.configPanel.page, a.configPanel.selected = settingsPage, bindsRow
	case "up", "k":
		a.moveConfigSelection(-1)
	case "down", "j":
		a.moveConfigSelection(1)
	case "pgup", "pgdown", "home", "end":
		a.jumpConfigSelection(key.String())
	case "enter", "space", "right", "l":
		a.changeSetting(1)
	case "a":
		if _, ok := bindRowAt(a.configPanel.selected); ok {
			a.configPanel.bindCapture = bindCapture{active: true, add: true}
		}
	case "delete", "backspace":
		if row, ok := bindRowAt(a.configPanel.selected); ok {
			a.setBind(row, nil)
		}
	}
}

// captureBind takes the key pressed for the selected action, to use with enter; another key
// replaces it, and esc leaves the action as it was.
func (a *App) captureBind(key uv.KeyPressEvent) {
	c := &a.configPanel.bindCapture
	row, _ := bindRowAt(a.configPanel.selected)
	switch {
	case key.MatchString("esc"):
		*c = bindCapture{}
	case key.MatchString("enter") && c.pending != "":
		keys := slices.Clone(a.shownKeys(row))
		switch {
		case c.add:
			keys = append(keys, c.pending)
		case c.slot < len(keys):
			keys[c.slot] = c.pending
		default:
			keys = []string{c.pending}
		}
		a.setBind(row, keys)
		*c = bindCapture{}
	default:
		c.pending, c.problem = "", a.bindProblem(row, key)
		if c.problem == "" {
			c.pending = key.String()
		}
	}
}

// bindProblem says why a key cannot be an action's, or "".
func (a *App) bindProblem(row bindRow, key uv.KeyPressEvent) string {
	name := key.String()
	switch _, ok := keyEvent(name); {
	case !ok:
		return name + " cannot be told apart by noct; try another"
	case a.prefix.matches(key):
		return name + " is the prefix"
	case row.context.typing && isPlainText(key):
		return name + " types text here; use it with ctrl or alt"
	}
	return ""
}

// bindConflicts are the other actions of the context that have the key, which it would leave.
func (a *App) bindConflicts(row bindRow, key string) []keyAction {
	var conflicts []keyAction
	for _, action := range row.context.actions {
		if action.id != row.action.id && slices.Contains(a.keymap.actionKeys(row.context.id, action), key) {
			conflicts = append(conflicts, action)
		}
	}
	return conflicts
}

// setBind gives an action its keys, or its defaults back with nil, taking each key from any
// other action of the context that had it. Keys equal to the defaults are left out of the file.
func (a *App) setBind(row bindRow, keys []string) {
	next := a.config
	next.Keys = cloneKeys(a.config.Keys)
	set := func(action keyAction, keys []string) {
		changed := next.Keys[row.context.id]
		if changed == nil {
			changed = map[string][]string{}
		}
		if keys == nil || slices.Equal(keys, action.defaults) {
			delete(changed, action.id)
		} else {
			changed[action.id] = keys
		}
		next.Keys[row.context.id] = changed
		if len(changed) == 0 {
			delete(next.Keys, row.context.id)
		}
	}
	if keys == nil {
		keys = row.action.defaults
	}
	var unique []string
	for _, key := range keys {
		if !slices.Contains(unique, key) {
			unique = append(unique, key)
		}
	}
	keys = unique
	for _, key := range keys {
		for _, other := range a.bindConflicts(row, key) {
			rest := slices.DeleteFunc(slices.Clone(a.keymap.actionKeys(row.context.id, other)), func(k string) bool { return k == key })
			set(other, rest)
		}
	}
	set(row.action, keys)
	if len(next.Keys) == 0 {
		next.Keys = nil
	}
	a.applyConfig(next)
}

func cloneKeys(keys map[string]map[string][]string) map[string]map[string][]string {
	clone := map[string]map[string][]string{}
	for context, actions := range keys {
		clone[context] = maps.Clone(actions)
	}
	return clone
}

// resetBinds puts every key back to its default.
func (a *App) resetBinds() {
	next := a.config
	next.Keys = nil
	a.applyConfig(next)
	a.configPanel.resetArmed = false
	a.notice = "keys reset to their defaults"
}

// bindLines are the binds page: a heading per context and a row per action with its keys, then
// the reset. The selected action shows, while taking a key, what to press and what it would take.
func (a *App) bindLines() []configLine {
	var lines []configLine
	context := ""
	for i, row := range bindRows() {
		if row.context.id != context {
			context = row.context.id
			lines = append(lines, configLine{label: row.context.label, heading: true})
		}
		id := bindRowBase + configRow(i)
		lines = append(lines, configLine{label: row.action.label, value: a.bindValue(id, row), row: id})
		if id == a.configPanel.selected {
			lines = append(lines, a.bindNotes(row)...)
		}
	}
	value := ""
	if a.configPanel.resetArmed {
		value = "enter again to reset"
	}
	return append(lines,
		configLine{label: "Defaults", heading: true},
		configLine{label: "reset all keys to defaults", value: value, row: resetBindsRow},
	)
}

// bindValue shows an action's keys. While one is being changed, it is in brackets: the one
// chosen with ←→, then what is pressed in its place, or after the others when adding.
func (a *App) bindValue(id configRow, row bindRow) string {
	keys := a.shownKeys(row)
	shown := make([]string, len(keys))
	for i, key := range keys {
		shown[i] = prettyKey(key)
	}
	if c := a.configPanel.bindCapture; (c.choosing || c.active) && id == a.configPanel.selected {
		taken := "press a key…"
		if c.pending != "" {
			taken = prettyKey(c.pending)
		}
		switch {
		case c.choosing:
			shown[c.slot] = "[" + shown[c.slot] + "]"
		case c.add || len(shown) == 0:
			shown = append(shown, "["+taken+"]")
		default:
			shown[c.slot] = "[" + taken + "]"
		}
		return strings.Join(shown, "  ")
	}
	if len(keys) == 0 {
		return "none"
	}
	value := strings.Join(shown, "  ")
	if _, changed := a.config.Keys[row.context.id][row.action.id]; changed {
		value = "• " + value // changed from the defaults
	}
	return value
}

// bindNotes explain the key being taken: why it cannot be used, what it takes, what to press.
func (a *App) bindNotes(row bindRow) []configLine {
	c := a.configPanel.bindCapture
	if c.choosing {
		return []configLine{{label: "←→ choose · enter changes it · esc cancels", note: true}}
	}
	if !c.active {
		return nil
	}
	var notes []configLine
	if c.problem != "" {
		notes = append(notes, configLine{label: c.problem, note: true, warning: true})
	}
	if c.pending == "" {
		verb := "is added to its keys"
		if keys := a.shownKeys(row); !c.add && c.slot < len(keys) {
			verb = "replaces " + prettyKey(keys[c.slot])
		}
		return append(notes, configLine{label: "the key pressed " + verb + "; esc cancels", note: true})
	}
	for _, other := range a.bindConflicts(row, c.pending) {
		notes = append(notes, configLine{label: "⚠ " + prettyKey(c.pending) + " is " + other.label + "; enter moves it here", note: true, warning: true})
	}
	return append(notes, configLine{label: "enter uses it · esc cancels · or press another", note: true})
}

// changedBinds tells how many actions have keys other than their defaults.
func changedBinds(cfg config.Config) int {
	n := 0
	for _, actions := range cfg.Keys {
		n += len(actions)
	}
	return n
}
