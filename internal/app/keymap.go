package app

import (
	"slices"
	"strings"
	"unicode/utf8"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// keyAction is something a key does in a context. Its keys do the same thing everywhere in the
// context; the handler understands the first of them, the one a changed key stands for.
type keyAction struct {
	id       string // its name in the config file
	label    string
	defaults []string
}

// keyContext is a screen whose keys can change: noct's mode after the prefix, a panel, a popup.
// Where typing marks it, characters without ctrl or alt type text: they can be neither taken
// for an action nor blocked. Esc always cancels and is never part of a context.
type keyContext struct {
	id      string // its section in the config file
	label   string
	typing  bool
	actions []keyAction
}

// keyContexts are the contexts in the order the binds list them. Keys a handler knows but no
// action names (tab numbers, a choice's letters, what programs in panes get) do not change.
var keyContexts = []keyContext{
	{id: "commands", label: "commands (after the prefix)", actions: []keyAction{
		{"focus-left", "focus left, or the previous tab", []string{"left", "h"}},
		{"focus-down", "focus down", []string{"down", "j"}},
		{"focus-up", "focus up", []string{"up", "k"}},
		{"focus-right", "focus right, or the next tab", []string{"right"}},
		{"next-tab", "next tab", []string{"tab"}},
		{"previous-tab", "previous tab", []string{"shift+tab"}},
		{"new-tab", "new tab", []string{"t"}},
		{"new-panel", "new panel", []string{"p"}},
		{"scripts", "scripts panel", []string{"s"}},
		{"flows", "inspect flows", []string{"i"}},
		{"logs", "logs and calendar", []string{"v"}},
		{"floating", "floating panel", []string{"g"}},
		{"search", "search", []string{"space"}},
		{"mcp", "MCP", []string{"y"}},
		{"sessions", "sessions", []string{"o"}},
		{"program-help", "program help", []string{"?"}},
		{"move", "move", []string{"a"}},
		{"resize", "resize", []string{"n"}},
		{"stack", "stacked or split", []string{"w"}},
		{"fullscreen", "fullscreen", []string{"f"}},
		{"rename-tab", "rename tab", []string{"r"}},
		{"rename-panel", "rename panel", []string{"e"}},
		{"scroll", "scroll history", []string{"["}},
		{"close", "close", []string{"x"}},
		{"config", "config", []string{"c"}},
		{"detach", "detach", []string{"d"}},
		{"quit", "quit", []string{"q"}},
		{"lock", "lock or unlock the shortcuts", []string{"l"}},
	}},
	{id: "move", label: "move", actions: []keyAction{
		{"left", "move left", []string{"left", "h"}},
		{"down", "move down", []string{"down", "j"}},
		{"up", "move up", []string{"up", "k"}},
		{"right", "move right", []string{"right", "l"}},
		{"done", "done", []string{"enter"}},
	}},
	{id: "resize", label: "resize", actions: []keyAction{
		{"narrower", "narrower", []string{"left", "h"}},
		{"wider", "wider", []string{"right", "l"}},
		{"shorter", "shorter", []string{"up", "k"}},
		{"taller", "taller", []string{"down", "j"}},
		{"done", "done", []string{"enter"}},
	}},
	{id: "scroll", label: "scroll history", actions: []keyAction{
		{"line-up", "a line up", []string{"up", "k"}},
		{"line-down", "a line down", []string{"down", "j"}},
		{"page-up", "a page up", []string{"pgup", "ctrl+b"}},
		{"page-down", "a page down", []string{"pgdown", "ctrl+f"}},
		{"top", "the oldest line", []string{"home", "g"}},
		{"bottom", "back to live", []string{"end", "G", "shift+g"}},
		{"done", "done", []string{"q", "enter"}},
	}},
	{id: "scripts", label: "scripts panel", actions: []keyAction{
		{"up", "previous", []string{"up", "k"}},
		{"down", "next", []string{"down", "j"}},
		{"first", "first", []string{"home", "g"}},
		{"last", "last", []string{"end", "G", "shift+g"}},
		{"page-up", "a page up", []string{"pgup"}},
		{"page-down", "a page down", []string{"pgdown"}},
		{"enter", "run the script, open or close the folder", []string{"enter"}},
		{"toggle", "open or close the folder", []string{"space"}},
		{"left", "close the folder, or go to it", []string{"left", "h"}},
		{"right", "open the folder", []string{"right", "l"}},
		{"edit", "edit", []string{"e"}},
		{"new", "new script or directory", []string{"n", "ctrl+n"}},
		{"add-folder", "add a folder", []string{"a"}},
		{"open-folder", "open the folder", []string{"ctrl+o"}},
		{"promote", "promote a draft", []string{"P", "shift+p"}},
		{"delete", "delete", []string{"delete"}},
		{"close", "close the panel", []string{"ctrl+d"}},
	}},
	{id: "flows", label: "inspect flows", actions: []keyAction{
		{"up", "previous flow, or the step above", []string{"up", "k"}},
		{"down", "next flow, or the step below", []string{"down", "j"}},
		{"first", "the first flow", []string{"home"}},
		{"last", "the last flow", []string{"end"}},
		{"page-up", "a page up of flows", []string{"pgup"}},
		{"page-down", "a page down of flows", []string{"pgdown"}},

		{"left", "the block to the left", []string{"left", "h"}},
		{"right", "open the chart, or the block to the right", []string{"right", "l"}},
		{"enter", "open the chart, or about the step", []string{"enter"}},
		{"about", "about the step", []string{"space"}},
		{"switch", "list or chart", []string{"tab", "shift+tab"}},
		{"add", "new flow, or add a step", []string{"a"}},
		{"run", "run", []string{"r"}},
		{"params", "run with params", []string{"p"}},
		{"test", "test the step or range", []string{"t"}},
		{"edit", "edit the step", []string{"e"}},
		{"edit-file", "edit the file", []string{"E"}},
		{"range-up", "grow the range up", []string{"shift+up"}},
		{"range-down", "grow the range down", []string{"shift+down"}},
		{"move-down", "move the step down", []string{"J", "shift+j"}},
		{"move-up", "move the step up", []string{"K", "shift+k"}},
		{"scope", "show the scope", []string{"s"}},
		{"triggers", "triggers on or off", []string{"o"}},
		{"add-trigger", "add a trigger", []string{"T", "shift+t"}},
		{"promote", "promote a draft", []string{"P", "shift+p"}},
		{"zoom-in", "zoom in", []string{"+", "="}},
		{"zoom-out", "zoom out", []string{"-"}},
		{"center", "center", []string{"0"}},
		{"pan-up", "pan up", []string{"ctrl+up"}},
		{"pan-down", "pan down", []string{"ctrl+down"}},
		{"pan-left", "pan left", []string{"ctrl+left"}},
		{"pan-right", "pan right", []string{"ctrl+right"}},
		{"delete", "delete the flow, or remove the step", []string{"delete"}},
		{"close", "close the panel", []string{"ctrl+d"}},
	}},
	{id: "logs", label: "logs", actions: []keyAction{
		{"up", "a week back, or the previous log", []string{"up", "k"}},
		{"down", "a week on, or the next log", []string{"down", "j"}},
		{"left", "a day back, or back to the calendar", []string{"left", "h"}},
		{"right", "a day on", []string{"right", "l"}},
		{"enter", "the day's logs, or open the log", []string{"enter"}},
		{"today", "today", []string{"t"}},
		{"home", "today, or the first log", []string{"home"}},
		{"first", "the first log", []string{"g"}},
		{"last", "the last log", []string{"end", "G", "shift+g"}},
		{"previous-month", "previous month", []string{"["}},
		{"next-month", "next month", []string{"]"}},
		{"page-up", "previous month, or a page up the list", []string{"pgup"}},
		{"page-down", "next month, or a page down the list", []string{"pgdown"}},
		{"switch", "calendar or list", []string{"tab", "shift+tab"}},
		{"filter", "filter", []string{"f"}},
		{"sort", "sort", []string{"s"}},
		{"all-days", "all days", []string{"a"}},
		{"inspect", "the flow's run", []string{"i"}},
		{"clear", "clear", []string{"delete"}},
		{"close", "close the panel", []string{"ctrl+d"}},
	}},
	{id: "rerun", label: "run again after reopening", actions: []keyAction{
		{"run", "run again", []string{"enter"}},
	}},
	{id: "run", label: "finished run", actions: []keyAction{
		{"restart", "run again", []string{"r"}},
		{"close", "close", []string{"enter", "q", "ctrl+d"}},
	}},
	{id: "sessions", label: "sessions", actions: []keyAction{
		{"up", "previous", []string{"up", "k"}},
		{"down", "next", []string{"down", "j"}},
		{"first", "first", []string{"home"}},
		{"last", "last", []string{"end"}},
		{"page-up", "a page up", []string{"pgup"}},
		{"page-down", "a page down", []string{"pgdown"}},
		{"open", "open", []string{"enter", "o"}},
		{"new", "new", []string{"n"}},
		{"rename", "rename", []string{"r"}},
		{"close-session", "close the session", []string{"x"}},
		{"delete", "delete", []string{"d", "delete"}},
		{"close", "close", []string{"q"}},
	}},
	{id: "mcp", label: "MCP", actions: []keyAction{
		{"up", "previous", []string{"up", "k"}},
		{"down", "next", []string{"down", "j"}},
		{"first", "first", []string{"home"}},
		{"last", "last", []string{"end"}},
		{"page-up", "a page up", []string{"pgup"}},
		{"page-down", "a page down", []string{"pgdown"}},
		{"enter", "approve, or copy the setup", []string{"enter"}},
		{"approve", "approve", []string{"y"}},
		{"deny", "deny", []string{"n", "delete"}},
		{"copy", "copy the setup", []string{"c"}},
		{"close", "close", []string{"q"}},
	}},
	{id: "confirm", label: "questions", actions: []keyAction{
		{"yes", "yes", []string{"enter", "y"}},
		{"no", "no", []string{"n"}},
	}},
	{id: "info", label: "about popups", actions: []keyAction{
		{"up", "scroll up", []string{"up", "k"}},
		{"down", "scroll down", []string{"down", "j"}},
		{"page-up", "a page up", []string{"pgup"}},
		{"page-down", "a page down", []string{"pgdown"}},
		{"top", "the top", []string{"home"}},
		{"bottom", "the end", []string{"end"}},
	}},
	{id: "choice", label: "choices", typing: true, actions: []keyAction{
		{"previous", "previous", []string{"up", "k", "shift+tab"}},
		{"next", "next", []string{"down", "j", "tab"}},
		{"first", "first", []string{"home", "pgup"}},
		{"last", "last", []string{"end", "pgdown"}},
		{"choose", "choose", []string{"enter"}},
	}},
	{id: "search", label: "search", typing: true, actions: []keyAction{
		{"up", "previous", []string{"up", "ctrl+p"}},
		{"down", "next", []string{"down", "ctrl+n"}},
		{"first", "first", []string{"home"}},
		{"last", "last", []string{"end"}},
		{"page-up", "a page up", []string{"pgup"}},
		{"page-down", "a page down", []string{"pgdown"}},
		{"open", "open", []string{"enter"}},
		{"erase", "erase a character", []string{"backspace"}},
	}},
	{id: "prompt", label: "text prompts", typing: true, actions: []keyAction{
		{"submit", "submit", []string{"enter"}},
		{"erase", "erase a character", []string{"backspace"}},
		{"clear", "erase to the start", []string{"ctrl+u"}},
		{"pick", "pick from a list", []string{"ctrl+t"}},
	}},
	{id: "form", label: "params form", typing: true, actions: []keyAction{
		{"run", "run", []string{"enter"}},
		{"next", "next field", []string{"tab", "down"}},
		{"previous", "previous field", []string{"shift+tab", "up"}},
		{"choice-back", "previous choice", []string{"left"}},
		{"choice-on", "next choice", []string{"right"}},
		{"erase", "erase a character", []string{"backspace"}},
		{"clear", "erase to the start", []string{"ctrl+u"}},
	}},
	{id: "script-form", label: "script form", typing: true, actions: []keyAction{
		{"enter", "edit the part, or the next field", []string{"enter"}},
		{"save", "save", []string{"ctrl+s"}},
		{"next", "next field", []string{"tab"}},
		{"previous", "previous field", []string{"shift+tab"}},
		{"up", "field above", []string{"up"}},
		{"down", "field below", []string{"down"}},
		{"left", "field to the left", []string{"left"}},
		{"right", "field to the right", []string{"right"}},
		{"erase", "erase a character", []string{"backspace"}},
		{"clear", "erase to the start", []string{"ctrl+u"}},
	}},
	{id: "config", label: "config", typing: true, actions: []keyAction{
		{"up", "previous setting", []string{"up", "k"}},
		{"down", "next setting", []string{"down", "j"}},
		{"first", "the first setting", []string{"home"}},
		{"last", "the last setting", []string{"end"}},
		{"page-up", "a page up", []string{"pgup"}},
		{"page-down", "a page down", []string{"pgdown"}},
		{"back", "change back", []string{"left", "h"}},
		{"on", "change", []string{"right", "l", "space"}},
		{"enter", "change, or save the text", []string{"enter"}},
		{"copy", "copy the location", []string{"ctrl+c"}},
		{"erase", "erase a character", []string{"backspace"}},
		{"clear", "erase to the start", []string{"ctrl+u"}},
	}},
}

func keyContextNamed(id string) (keyContext, bool) {
	i := slices.IndexFunc(keyContexts, func(c keyContext) bool { return c.id == id })
	if i < 0 {
		return keyContext{}, false
	}
	return keyContexts[i], true
}

// keymap is the keys of every context once the config's changes are in: which default key each
// key stands for, and the default keys an action no longer has. A key in neither passes as it is.
type keymap struct {
	// keys are each action's keys, by context and action.
	keys map[string]map[string][]string
	// stands maps a key to the default key the handler understands, by context.
	stands  map[string]map[string]string
	blocked map[string]map[string]bool
}

// newKeymap puts the config's keys over the defaults. An action set to no keys has none; one
// the config does not name keeps its defaults. Names noct does not know are left out.
func newKeymap(changed map[string]map[string][]string) keymap {
	k := keymap{keys: map[string]map[string][]string{}, stands: map[string]map[string]string{}, blocked: map[string]map[string]bool{}}
	for _, c := range keyContexts {
		k.keys[c.id], k.stands[c.id], k.blocked[c.id] = map[string][]string{}, map[string]string{}, map[string]bool{}
		for _, action := range c.actions {
			k.keys[c.id][action.id] = action.defaults
			if keys, ok := changed[c.id][action.id]; ok {
				k.keys[c.id][action.id] = validKeys(keys)
			}
		}
		// unchanged keys stand for themselves, as before any change; they go first, so a key
		// the config gives to another action goes to that one
		for _, action := range c.actions {
			if _, ok := changed[c.id][action.id]; !ok {
				for _, key := range action.defaults {
					k.stands[c.id][key] = key
				}
			}
		}
		for _, action := range c.actions {
			if _, ok := changed[c.id][action.id]; !ok {
				continue
			}
			for _, key := range k.keys[c.id][action.id] {
				k.stands[c.id][key] = action.defaults[0]
			}
			for _, key := range action.defaults {
				if _, taken := k.stands[c.id][key]; !taken {
					k.blocked[c.id][key] = true
				}
			}
		}
	}
	return k
}

// validKeys keeps the keys noct can read back.
func validKeys(keys []string) []string {
	valid := []string{}
	for _, key := range keys {
		if _, ok := keyEvent(key); ok && key != "esc" {
			valid = append(valid, key)
		}
	}
	return valid
}

// resolve turns a key pressed in a context into the key its handler understands, or reports
// that the key no longer does anything there.
func (k keymap) resolve(context string, key uv.KeyPressEvent) (uv.KeyPressEvent, bool) {
	name := key.String()
	c, known := keyContextNamed(context)
	if !known || name == "esc" || c.typing && isPlainText(key) {
		return key, true
	}
	if standsFor, ok := k.stands[context][name]; ok {
		if standsFor == name {
			return key, true
		}
		return keyEvent(standsFor)
	}
	return key, !k.blocked[context][name]
}

// keysOf are an action's keys, by the default key it is known by in the handler.
func (k keymap) keysOf(context, defaultKey string) []string {
	c, ok := keyContextNamed(context)
	if !ok {
		return []string{defaultKey}
	}
	for _, action := range c.actions {
		if slices.Contains(action.defaults, defaultKey) {
			if keys, ok := k.keys[context][action.id]; ok {
				return keys
			}
			return action.defaults
		}
	}
	return []string{defaultKey}
}

// namedKeys are the keys without modifiers that have a name, such as enter or f2, by name.
var namedKeys = func() map[string]rune {
	codes := []rune{
		uv.KeyEnter, uv.KeyTab, uv.KeySpace, uv.KeyBackspace, uv.KeyDelete, uv.KeyInsert, uv.KeyEscape,
		uv.KeyUp, uv.KeyDown, uv.KeyLeft, uv.KeyRight, uv.KeyHome, uv.KeyEnd, uv.KeyPgUp, uv.KeyPgDown,
	}
	for code := uv.KeyF1; code <= uv.KeyF12; code++ {
		codes = append(codes, code)
	}
	names := map[string]rune{}
	for _, code := range codes {
		names[uv.Key{Code: code}.Keystroke()] = code
	}
	return names
}()

// keyEvent is the key press a name stands for, as key.String() writes it: "enter", "ctrl+x",
// "shift+tab", "G" or "?".
func keyEvent(name string) (uv.KeyPressEvent, bool) {
	parts := strings.Split(name, "+")
	last := parts[len(parts)-1]
	if last == "" && (name == "+" || strings.HasSuffix(name, "++")) { // the + key, maybe with modifiers
		parts, last = parts[:len(parts)-2], "+"
	} else {
		parts = parts[:len(parts)-1]
	}
	var mod uv.KeyMod
	for _, part := range parts {
		switch part {
		case "ctrl":
			mod |= uv.ModCtrl
		case "alt":
			mod |= uv.ModAlt
		case "shift":
			mod |= uv.ModShift
		default:
			return uv.KeyPressEvent{}, false
		}
	}
	if code, ok := namedKeys[last]; ok {
		return uv.KeyPressEvent{Code: code, Mod: mod}, true
	}
	r, size := utf8.DecodeRuneInString(last)
	if size != len(last) || r == utf8.RuneError {
		return uv.KeyPressEvent{}, false
	}
	key := uv.KeyPressEvent{Code: r, Mod: mod}
	if lower := strings.ToLower(last); lower != last { // G is shift+g
		key.Code, key.Mod = []rune(lower)[0], mod|uv.ModShift
	}
	if key.Mod&(uv.ModCtrl|uv.ModAlt) == 0 { // a character typed, as the terminal sends it
		key.Text = string(key.Code)
		if key.Mod&uv.ModShift != 0 {
			key.Text = strings.ToUpper(key.Text)
		}
	}
	return key, true
}

// modeKeyContexts are the contexts of noct's modes; normal mode's is the focused panel's.
var modeKeyContexts = map[mode]string{
	commandMode: "commands", moveMode: "move", resizeMode: "resize", scrollMode: "scroll",
	confirmMode: "confirm", infoMode: "info", choiceMode: "choice", finderMode: "search",
	promptMode: "prompt", formMode: "form", scriptFormMode: "script-form", configMode: "config",
	mcpMode: "mcp", sessionsMode: "sessions",
}

// modeKeyContext is the context of the mode on, or none while a combination is being taken,
// which must get the keys as they are pressed.
func (a *App) modeKeyContext() string {
	if a.mode == configMode && (a.configPanel.capture.active || a.configPanel.bindCapture.active) {
		return ""
	}
	return modeKeyContexts[a.mode]
}

// viewKeyContext is the context of a panel of noct; programs in panes get their keys as they are.
func viewKeyContext(v view) string {
	switch v := v.(type) {
	case *scriptsPanel:
		return "scripts"
	case *inspectPanel:
		return "flows"
	case *logsPanel:
		return "logs"
	case *runView:
		if !v.running {
			return "run"
		}
	}
	return ""
}

// commandKey is the noct mode key a key after the prefix stands for, as the direct shortcuts
// need it, or false when that key no longer does anything.
func (a *App) commandKey(name string) (string, bool) {
	key, ok := keyEvent(name)
	if !ok {
		return name, true
	}
	resolved, ok := a.keymap.resolve("commands", key)
	return resolved.String(), ok
}

// actionKeys are an action's keys now: the config's, or its defaults.
func (k keymap) actionKeys(context string, action keyAction) []string {
	if keys, ok := k.keys[context][action.id]; ok {
		return keys
	}
	return action.defaults
}

// hint writes a line of hints with the keys as they are now. A default key in braces, as in
// "{enter} run", becomes the key in the same place among its action's keys, or nothing when the
// action has fewer; braces next to each other make a group, as in "{up}{down} select".
func (a *App) hint(context, text string) string {
	c, _ := keyContextNamed(context)
	var out strings.Builder
	var group []string
	flush := func() {
		out.WriteString(joinKeys(group))
		group = nil
	}
	for text != "" {
		open := strings.IndexByte(text, '{')
		end := strings.IndexByte(text, '}')
		if open != 0 || end < 0 {
			flush()
			if open < 0 || end < 0 {
				open = len(text)
			}
			out.WriteString(text[:max(open, 1)])
			text = text[max(open, 1):]
			continue
		}
		group = append(group, a.keyAt(c, text[1:end]))
		text = text[end+1:]
	}
	flush()
	return out.String()
}

// keyAt is the key now in the place a default key has among its action's keys.
func (a *App) keyAt(c keyContext, defaultKey string) string {
	for _, action := range c.actions {
		if i := slices.Index(action.defaults, defaultKey); i >= 0 {
			if keys := a.keymap.actionKeys(c.id, action); i < len(keys) {
				return prettyKey(keys[i])
			}
			return ""
		}
	}
	return prettyKey(defaultKey)
}

// joinKeys writes a group of keys: together when each is one character or they share their
// modifiers (shift+↑↓), and apart with / otherwise.
func joinKeys(keys []string) string {
	keys = slices.DeleteFunc(keys, func(key string) bool { return key == "" })
	if len(keys) < 2 {
		return strings.Join(keys, "")
	}
	modifiers := func(key string) string { return key[:strings.LastIndexByte(key, '+')+1] }
	shared := modifiers(keys[0])
	var bare []string
	for _, key := range keys {
		rest := strings.TrimPrefix(key, shared)
		if modifiers(key) != shared || utf8.RuneCountInString(rest) != 1 {
			return strings.Join(keys, "/")
		}
		bare = append(bare, rest)
	}
	return shared + strings.Join(bare, "")
}

// prettyKey writes a key as hints show it.
func prettyKey(key string) string {
	for name, arrow := range map[string]string{"up": "↑", "down": "↓", "left": "←", "right": "→", "delete": "del"} {
		if key == name || strings.HasSuffix(key, "+"+name) {
			return strings.TrimSuffix(key, name) + arrow
		}
	}
	return key
}
