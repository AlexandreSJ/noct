package app

import (
	"image"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// mode decides where keys go.
type mode int

const (
	normalMode       mode = iota // keys go to the focused view
	commandMode                  // after the prefix: noct commands
	moveMode                     // arrows move the focused pane
	promptMode                   // typing a line of text, such as a tab name
	configMode                   // the config panel is open
	confirmMode                  // asking before doing something that cannot be undone
	scrollMode                   // paging through the focused terminal's history
	formMode                     // filling in a script's parameters
	scriptFormMode               // editing a script's name, command line and description
	resizeMode                   // arrows resize the focused pane
	choiceMode                   // choosing one of a few things to do
	finderMode                   // searching actions, scripts and tabs
	infoMode                     // reading about something, such as a flow's step
	mcpMode                      // AI approvals, connected clients and how to add noct to one
	sessionsMode                 // the sessions: open, create, rename, close, delete
	updatesMode                  // new versions of noct and the changelog
	colorsMode                   // the focused pane's and its tab's own colors
	templateSaveMode             // saving the tab as a template
	templatesMode                // the templates, to open one as a new tab
)

// shortcut is one command as listed in the popups and on the home screen.
type shortcut struct {
	keys  string
	label string
	// needsTab hides the command on the home screen, where there is nothing to act on.
	needsTab bool
}

var commandShortcuts = []shortcut{
	{"←↓↑→ hjkl", "focus", true},
	{"tab", "change", true},
	{"1-9", "go to tab", true},
	{"t", "new tab", false},
	{"T", "open a template", false},
	{"p", "new panel", false},
	{"s", "scripts panel", false},
	{"i", "inspect flows", false},
	{"v", "logs and calendar", false},
	{"space", "search", false},
	{"y", "MCP", false},
	{"o", "sessions", false},
	{"?", "program help", true},
	{"a", "move", true},
	{"n", "resize", true},
	{"w", "stacked or split", true},
	{"f", "fullscreen", true},
	{"g", "floating panel", false},
	{"r", "rename tab", true},
	{"e", "rename panel", true},
	{"P", "colors of the panel and tab", true},
	{"R", "save the tab as a template", true},
	{"u", "scroll history", true},
	{"x", "close", true},
	{",", "config", false},
	{"c", "updates and changelog", false},
	{"d", "detach", false},
	{"q", "quit", false},
	{"b", "lock shortcuts", false},
	{"esc", "cancel", false},
}

var moveShortcuts = []shortcut{
	{"←↓↑→ hjkl", "move pane", true},
	{"esc", "done", true},
}

var resizeShortcuts = []shortcut{
	{"← → h l", "narrower, wider", true},
	{"↑ ↓ k j", "shorter, taller", true},
	{"esc", "done", true},
}

// arrowHeadings take vim's hjkl as well as the arrows, everywhere noct moves something.
var arrowHeadings = map[string]layout.Heading{
	"left": layout.Left, "right": layout.Right, "up": layout.Up, "down": layout.Down,
	"h": layout.Left, "l": layout.Right, "k": layout.Up, "j": layout.Down,
}

// actionKeys are the noct mode commands that act, as opposed to moving around.
var actionKeys = []string{"t", "T", "p", "s", "i", "v", "g", "space", "y", "o", "?", "a", "n", "w", "f", "r", "e", "P", "R", "u", "x", ",", "c", "d", "q"}

var tabNumberKeys = map[string]int{"1": 0, "2": 1, "3": 2, "4": 3, "5": 4, "6": 5, "7": 6, "8": 7, "9": 8}

func (a *App) handleKey(key uv.KeyPressEvent) {
	a.statusMessage, a.notice, a.lockHint = "", "", false
	if a.selection != nil && !a.selection.dragging {
		a.selection = nil
	}
	if context := a.modeKeyContext(); context != "" && !a.prefix.matches(key) {
		var ok bool
		if key, ok = a.keymap.resolve(context, key); !ok {
			return
		}
	}
	if field, _ := a.typedField(); field != nil && isPasteKey(key) {
		a.requestClipboard()
		return
	}
	switch a.mode {
	case commandMode:
		a.handleCommandKey(key)
	case moveMode:
		a.handleMoveKey(key)
	case promptMode:
		a.handlePromptKey(key)
	case configMode:
		a.handleConfigKey(key)
	case confirmMode:
		a.handleConfirmKey(key)
	case scrollMode:
		a.handleScrollKey(key)
	case formMode:
		a.handleFormKey(key)
	case scriptFormMode:
		a.handleScriptFormKey(key)
	case resizeMode:
		a.handleResizeKey(key)
	case choiceMode:
		a.handleChoiceKey(key)
	case finderMode:
		a.handleFinderKey(key)
	case infoMode:
		a.handleInfoKey(key)
	case mcpMode:
		a.handleMCPKey(key)
	case sessionsMode:
		a.handleSessionsKey(key)
	case updatesMode:
		a.handleUpdatesKey(key)
	case colorsMode:
		a.handleColorsKey(key)
	case templateSaveMode:
		a.handleTemplateSaveKey(key)
	case templatesMode:
		a.handleTemplatesKey(key)
	default:
		a.handleNormalKey(key)
	}
}

func (a *App) handleNormalKey(key uv.KeyPressEvent) {
	if a.prefix.matches(key) {
		a.mode = commandMode
		return
	}
	if name, move, ok := a.shortcut.command(key, a.commandKey); ok && !a.lockedOut(name, move) {
		a.runShortcut(name, move)
		return
	}
	if t := a.currentTab(); t != nil && !t.isEmpty() {
		if a.pendingRerun(t.focused) != "" {
			a.handleRerunKey(t, key)
			return
		}
		v := t.focusedView()
		if context := viewKeyContext(v); context != "" {
			var ok bool
			if key, ok = a.keymap.resolve(context, key); !ok {
				return
			}
		}
		v.SendKey(key)
		return
	}
	a.handleHomeKey(key)
}

// handleCommandKey runs noct commands. Navigation keeps the mode on, so moving around takes a
// single prefix; other commands end it. Unknown keys are ignored.
func (a *App) handleCommandKey(key uv.KeyPressEvent) {
	if key.MatchString("esc") || a.prefix.matches(key) {
		a.mode = normalMode
		return
	}
	name := key.String()
	if name == lockKey {
		a.toggleLock()
		return
	}
	if a.currentTab() != nil && a.navigate(name) {
		return
	}
	if a.lockedOut(name, false) {
		a.mode = normalMode
		return
	}
	a.runAction(name)
}

func (a *App) navigate(name string) bool {
	if heading, ok := arrowHeadings[name]; ok {
		if heading == layout.Left || heading == layout.Right {
			a.moveFocusAcrossTabs(heading)
		} else {
			a.moveFocus(heading)
		}
		return true
	}
	switch name {
	case "tab":
		a.selectTab(a.session.activeTab + 1)
	case "shift+tab":
		a.selectTab(a.session.activeTab - 1)
	default:
		index, ok := tabNumberKeys[name]
		if ok {
			a.selectTab(index)
		}
		return ok
	}
	return true
}

func (a *App) runAction(name string) {
	if kind, ok := panelKindWithKey(name); ok {
		a.openPanel(kind.start(a))
		a.mode = normalMode
		return
	}
	switch name {
	case "t":
		a.openNewTab()
	case "g":
		a.openFloating()
	case ",":
		a.openConfig()
		return
	case "c":
		a.openUpdates()
		return
	case "d":
		if a.client != nil {
			a.detachClient("detached")
		}
	case "q":
		again := "q"
		if keys := a.keymap.keysOf("commands", "q"); len(keys) > 0 {
			again = keys[0] // the quit key as it is now, pressed twice, quits
		}
		a.askToConfirmWith("quit noct?", again, func() { a.quitting = true })
		return
	case "space":
		a.openFinder()
		return
	case "T":
		a.openTemplates()
		return
	case "y":
		a.openMCP()
		return
	case "o":
		a.sessionFilter = ""
		a.openSessions()
		return
	case "?":
		a.showHelp()
		return
	default:
		a.runTabAction(name)
		return
	}
	a.mode = normalMode
}

// runTabAction runs the commands that act on the current tab. A tab without panes, opened empty,
// can only be renamed or closed.
func (a *App) runTabAction(name string) {
	t := a.currentTab()
	if t == nil {
		return
	}
	if t.isEmpty() && name != "r" {
		if name == "x" {
			a.session.removeTabIfEmpty(t)
		}
		a.mode = normalMode
		return
	}
	switch name {
	case "x":
		a.closeFocused()
		a.mode = normalMode
	case "P":
		a.openColors()
	case "R":
		a.openTemplateSave()
	case "a":
		if !a.currentTab().isEmpty() {
			a.mode = moveMode
		}
	case "n":
		if !a.currentTab().isEmpty() {
			a.mode = resizeMode
		}
	case "f":
		if !a.currentTab().isEmpty() {
			a.toggleFullscreen()
		}
		a.mode = normalMode
	case "w":
		a.toggleStacked()
		a.mode = normalMode
	case "r":
		a.startRename()
	case "e":
		a.startPaneRename()
	case "u":
		a.startScroll()
	}
}

func (a *App) handleMoveKey(key uv.KeyPressEvent) {
	name := key.String()
	if heading, ok := arrowHeadings[name]; ok {
		if a.currentTab() != nil {
			a.moveFocusedPane(heading)
		}
		return
	}
	if name == "esc" || name == "enter" || a.prefix.matches(key) {
		a.mode = normalMode
	}
}

// shortcutTemplate marks the keys of a popup's row for hint: a key alone, or the arrows and
// their letters together. Other rows, such as 1-9, show as they are.
func shortcutTemplate(keys string) string {
	switch keys {
	case "←↓↑→ hjkl":
		return "{left}{down}{up}{right} {h}{j}{k}{l}"
	case "← → h l":
		return "{left} {right} {h} {l}"
	case "↑ ↓ k j":
		return "{up} {down} {k} {j}"
	}
	if _, ok := keyEvent(keys); ok && !strings.Contains(keys, "{") {
		return "{" + keys + "}"
	}
	return keys
}

// availableShortcuts leaves out the commands that need a tab when none is open.
func (a *App) availableShortcuts(shortcuts []shortcut) []shortcut {
	if a.currentTab() != nil {
		return shortcuts
	}
	var available []shortcut
	for _, s := range shortcuts {
		if !s.needsTab {
			available = append(available, s)
		}
	}
	return available
}

// shortcutsPopupArea is where drawShortcutsPopup puts its panel, for clicks to find the rows.
func (a *App) shortcutsPopupArea(visible []shortcut, note string) image.Rectangle {
	lines := len(visible)
	if note != "" {
		lines += 2
	}
	return centeredRect(image.Rect(0, 0, a.size.X, a.size.Y), 36, panelHeight(lines))
}

// drawShortcutsPopup lists the keys of the current mode, as they are in its key context, in the
// middle of the screen, with an optional note under them.
func (a *App) drawShortcutsPopup(screen uv.Screen, title, context string, shortcuts []shortcut, note string) {
	visible := a.availableShortcuts(shortcuts)
	inner := a.drawPanel(screen, a.shortcutsPopupArea(visible, note), title)
	for i, s := range visible {
		drawRow(screen, inner, inner.Min.Y+i, a.hint(context, shortcutTemplate(s.keys)), s.label, a.palette.panelText)
	}
	if note != "" {
		drawCentered(screen, inner, inner.Max.Y-1, note, a.palette.panelMuted)
	}
}

// clickCommandShortcut runs the command whose row of the noct popup was clicked, as its key
// would. The rows of focus and tab numbers stand for several keys and do nothing; a click
// outside the popup closes it.
func (a *App) clickCommandShortcut(point image.Point) {
	visible := a.availableShortcuts(commandShortcuts)
	area := a.shortcutsPopupArea(visible, a.shortcutNote())
	if !point.In(area) {
		a.mode = normalMode
		return
	}
	row := point.Y - area.Inset(2).Min.Y
	if row < 0 || row >= len(visible) {
		return
	}
	switch keys := visible[row].keys; {
	case keys == "esc":
		a.mode = normalMode
	case keys == "tab":
		a.navigate(keys)
	case keys == lockKey:
		a.toggleLock()
	case a.lockedOut(keys, false):
		a.mode = normalMode
	case slices.Contains(actionKeys, keys):
		a.runAction(keys)
	}
}

// shortcutNote reminds, in the commands popup, that the keys also work without the prefix.
func (a *App) shortcutNote() string {
	if a.shortcut.mods == 0 {
		return ""
	}
	return "or " + a.shortcut.String() + " without prefix"
}
