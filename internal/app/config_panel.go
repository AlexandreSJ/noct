package app

import (
	"fmt"
	"image"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/script"
)

type configRow int

const (
	prefixRow configRow = iota
	bindsRow
	shortcutCtrlRow
	shortcutAltRow
	themeRow
	backgroundRow
	animateIntroRow
	mouseRow
	newTabRow
	closeEmptyTabsRow
	editorRow
	editorCommandRow
	fileManagerRow
	fileManagerCommandRow
	scriptsDirRow
	scriptHintsRow
	scriptsSortRow
	logsKeepRow
	logsSizeRow
	logsTerminalsRow
	flowsOutputRow
	updateCheckRow
	resetRow
	resetBindsRow
)

// configPage is what the config panel shows: the settings, or the keys of every screen.
type configPage int

const (
	settingsPage configPage = iota
	bindsPage
)

const configHints = "{up}{down} select  {left}{right} change  esc close"

// textColumn is where the text of a typed setting starts, after its label.
const textColumn = 20

type configPanel struct {
	page     configPage
	selected configRow
	// first is the first line shown when the page does not fit, kept between draws so the
	// selection moves inside the view before the view moves.
	first int
	// bindCapture takes a key for the selected action on the binds page.
	bindCapture bindCapture
	// capture takes the prefix's new combination while it is on.
	capture keyCapture
	// resetArmedAt is when enter first asked for a reset; a second enter within resetWindow does it.
	resetArmedAt time.Time
	// number is a log limit being typed, nil while none is; numberProblem says why enter did not
	// take it.
	number        *textField
	numberProblem string
	// logUsage is what the logs take on disk, nil when it cannot be read.
	logUsage    *logUsage
	editor      programSetting
	fileManager programSetting
	scriptsDir  textField
	// typing is on while the selected typed row takes text, from enter until enter, esc or leaving
	// the row; typedBefore is its text when typing started, put back by esc.
	typing      bool
	typedBefore string
	// copied says the location was just copied, until the selection or the text changes.
	copied bool
	// hover is the row under the mouse, when hovering is true.
	hover    configRow
	hovering bool
}

// configLine is one line of the panel: a section heading, a row that can be selected, or a note
// under the row before it.
type configLine struct {
	label   string
	value   string
	row     configRow
	heading bool
	note    bool
	// warning notes show in the attention color.
	warning bool
	// typed rows take text: enter opens their value for editing in place.
	typed bool
	// bar notes show a bar filled for fraction, then the label, as the logs view's usage bar.
	bar      bool
	fraction float64
}

func (a *App) openConfig() {
	a.configPanel = configPanel{
		editor:      newProgramSetting(editorChoices(isInstalled), a.config.Editor),
		fileManager: newProgramSetting(fileManagerChoices(isInstalled), a.config.FileManager),
		scriptsDir:  newTextField(a.scriptsDir()),
	}
	a.refreshLogUsage()
	a.mode = configMode
}

func (a *App) handleConfigKey(key uv.KeyPressEvent) {
	if a.configPanel.capture.active {
		a.capturePrefix(key)
		return
	}
	if a.configPanel.page == bindsPage {
		a.handleBindsKey(key)
		return
	}
	if a.configPanel.number != nil {
		a.typeNumber(key)
		return
	}
	if a.startsNumberEdit(key) || a.pressReset(key) {
		return
	}
	if a.configPanel.typing && key.String() == "esc" {
		a.cancelTyping()
		return
	}
	switch key.String() {
	case "esc":
		a.closeConfig()
		return
	case "up":
		a.moveConfigSelection(-1)
		return
	case "down":
		a.moveConfigSelection(1)
		return
	case "pgup", "pgdown", "home", "end":
		a.jumpConfigSelection(key.String())
		return
	}
	if a.isTypedRow(a.configPanel.selected) && a.startsTyping(key) {
		return
	}
	switch key.String() {
	case "k":
		a.moveConfigSelection(-1)
	case "j":
		a.moveConfigSelection(1)
	case "left", "h":
		a.changeSetting(-1)
	case "right", "l", "enter", "space":
		a.changeSetting(1)
	}
}

// closeConfig saves what was typed and closes the panel.
func (a *App) closeConfig() {
	a.cancelNumberEdit()
	a.commitTyped()
	a.configPanel.editor.dropEmptyCustom()
	a.configPanel.fileManager.dropEmptyCustom()
	a.mode = normalMode
}

// selectConfigRow selects a row, saving what was typed in the row it leaves.
func (a *App) selectConfigRow(row configRow) {
	if row == a.configPanel.selected {
		return
	}
	a.commitTyped()
	a.cancelNumberEdit()
	a.configPanel.copied, a.configPanel.resetArmedAt, a.configPanel.typing = false, time.Time{}, false
	a.configPanel.selected = row
}

// moveConfigSelection selects the next row shown, saving what was typed in the row it leaves.
func (a *App) moveConfigSelection(step int) {
	rows := a.selectableConfigRows()
	index := max(slices.Index(rows, a.configPanel.selected), 0)
	a.selectConfigRow(rows[(index+step+len(rows))%len(rows)])
}

// jumpConfigSelection moves a page up or down, or to the first or last row, as listJump does.
func (a *App) jumpConfigSelection(key string) {
	rows := a.selectableConfigRows()
	index := max(slices.Index(rows, a.configPanel.selected), 0)
	if to, ok := listJump(key, index, len(rows), a.configLayout().visible()); ok {
		a.selectConfigRow(rows[to])
	}
}

// selectableConfigRows are the rows of the page shown, in order.
func (a *App) selectableConfigRows() []configRow {
	var rows []configRow
	for _, line := range a.configLines() {
		if line.selectable() {
			rows = append(rows, line.row)
		}
	}
	return rows
}

func (a *App) isTypedRow(row configRow) bool {
	return row == editorCommandRow || row == fileManagerCommandRow || row == scriptsDirRow
}

// typedText is the text a typed row edits.
func (a *App) typedText(row configRow) *textField {
	switch row {
	case editorCommandRow:
		return &a.configPanel.editor.draft
	case fileManagerCommandRow:
		return &a.configPanel.fileManager.draft
	case logsKeepRow, logsSizeRow:
		return a.configPanel.number
	}
	return &a.configPanel.scriptsDir
}

// startsTyping handles a key on a typed row: enter opens it for typing, and while it is open every
// key types, so j and k only move the selection while it is closed. ctrl+c copies the location
// either way. It tells whether it used the key.
func (a *App) startsTyping(key uv.KeyPressEvent) bool {
	p := &a.configPanel
	p.copied = false
	switch key.String() {
	case "ctrl+c", "ctrl+shift+c":
		if p.selected == scriptsDirRow {
			a.copyToClipboard(script.ResolveDir(p.scriptsDir.String()))
			p.copied = true
		}
		return true
	case "enter":
		if p.typing {
			a.commitTyped()
		} else {
			p.typedBefore = a.typedText(p.selected).String()
		}
		p.typing = !p.typing
		return true
	}
	if p.typing {
		a.typeKey(key)
	}
	return p.typing
}

// cancelTyping closes the selected typed row, putting back the text it had when typing started.
func (a *App) cancelTyping() {
	p := &a.configPanel
	*a.typedText(p.selected) = newTextField(p.typedBefore)
	p.typing = false
}

// configTypedField is the selected row's text while it is open for typing.
func (a *App) configTypedField() (*textField, func()) {
	p := &a.configPanel
	if p.number != nil {
		return p.number, func() { p.numberProblem = "" }
	}
	if p.capture.active || p.page == bindsPage || !p.typing || !a.isTypedRow(p.selected) {
		return nil, nil
	}
	return a.typedText(p.selected), func() { p.copied = false }
}

// commitTyped saves what was typed in the selected row, when it is a typed row.
func (a *App) commitTyped() {
	next := a.config
	switch a.configPanel.selected {
	case editorCommandRow:
		next.Editor = a.configPanel.editor.command(next.Editor)
	case fileManagerCommandRow:
		next.FileManager = a.configPanel.fileManager.command(next.FileManager)
	case scriptsDirRow:
		next.ScriptsDir = strings.TrimSpace(a.configPanel.scriptsDir.String())
		if script.ResolveDir(next.ScriptsDir) == script.PersonalDir() {
			next.ScriptsDir = "" // the standard place stays portable across machines
		}
	default:
		return
	}
	if reflect.DeepEqual(next, a.config) {
		return
	}
	a.applyConfig(next)
	a.reloadScripts()
}

func (a *App) changeSetting(step int) {
	if _, ok := bindRowAt(a.configPanel.selected); ok {
		a.startBindEdit()
		return
	}
	next := a.config
	switch a.configPanel.selected {
	case prefixRow:
		a.configPanel.capture = keyCapture{active: true}
		return
	case bindsRow:
		a.openBinds()
		return
	case resetBindsRow, resetRow:
		return // only enter resets, twice, so neither an arrow nor a click does it by accident
	case shortcutCtrlRow:
		next.Shortcut.Ctrl = !next.Shortcut.Ctrl
	case shortcutAltRow:
		next.Shortcut.Alt = !next.Shortcut.Alt
	case themeRow:
		next.Appearance.Theme = nextThemeName(next.Appearance.Theme, step)
	case backgroundRow:
		next.Appearance.Background = cycle([]string{config.BackgroundTerminal, config.BackgroundTheme}, next.Appearance.Background, step)
	case animateIntroRow:
		next.Appearance.AnimateIntro = cycle([]string{config.IntroEmpty, config.IntroAlways, config.IntroNever}, next.Appearance.AnimateIntro, step)
	case mouseRow:
		next.Mouse = !next.Mouse
	case newTabRow:
		next.NewTab = cycle(newTabChoices(), next.NewTab, step)
	case closeEmptyTabsRow:
		next.CloseEmptyTabs = !next.CloseEmptyTabs
	case editorRow:
		next.Editor = a.configPanel.editor.step(next.Editor, step)
	case fileManagerRow:
		next.FileManager = a.configPanel.fileManager.step(next.FileManager, step)
	case scriptHintsRow:
		next.ScriptHints = !next.ScriptHints
	case scriptsSortRow:
		next.ScriptsSort = cycle(sortOrders, next.ScriptsSort, step)
		defer a.sortScripts() // after the new order is in effect
	case logsKeepRow:
		next.Logs.KeepDays = stepNumber(keepDaysChoices, next.Logs.KeepDays, step)
		a.applyLogLimits(next, nil)
		return
	case logsSizeRow:
		next.Logs.MaxSizeKB = stepNumber(logSizeChoices, next.Logs.MaxSizeKB, step)
		a.applyLogLimits(next, nil)
		return
	case logsTerminalsRow:
		next.Logs.Terminals = !next.Logs.Terminals
	case flowsOutputRow:
		i := max(slices.Index(flowOutputChoices, next.Flows.OutputKB), 0)
		next.Flows.OutputKB = flowOutputChoices[min(max(i+step, 0), len(flowOutputChoices)-1)]
	case updateCheckRow:
		next.Update.Check = !next.Update.Check
		if next.Update.Check && !a.updates.checked {
			defer a.checkForUpdates()
		}
	}
	a.applyConfig(next)
}

// Choices of the log limits; a value set by hand in the file steps to the nearest one.
var (
	keepDaysChoices = []int{7, 14, 30, 60, 90, 180, 365}
	logSizeChoices  = []int{50 << 10, 100 << 10, 200 << 10, 500 << 10, 1000 << 10, 2000 << 10, 5000 << 10} // KB
	// flowOutputChoices go from keeping all of a step's output (0) to keeping less.
	flowOutputChoices = []int{0, 256, 64, 16, 4}
)

func flowOutputLabel(kb int) string {
	if kb == 0 {
		return "all (up to 1 MB)"
	}
	return fmt.Sprintf("%d KB", kb)
}

// stepNumber moves from current to the next choice up or down, stopping at the ends.
func stepNumber(choices []int, current, step int) int {
	if step > 0 {
		for _, choice := range choices {
			if choice > current {
				return choice
			}
		}
		return choices[len(choices)-1]
	}
	for i := len(choices) - 1; i >= 0; i-- {
		if choices[i] < current {
			return choices[i]
		}
	}
	return choices[0]
}

// cycle returns the choice step places after current, wrapping around.
func cycle(choices []string, current string, step int) string {
	index := max(slices.Index(choices, current), 0)
	return choices[(index+step+len(choices))%len(choices)]
}

// capturePrefix takes the combination pressed as the prefix's new one, to use with enter; another
// combination replaces it, and esc leaves the prefix as it was.
func (a *App) capturePrefix(key uv.KeyPressEvent) {
	c := &a.configPanel.capture
	switch {
	case key.MatchString("esc"):
		*c = keyCapture{}
	case key.MatchString("enter") && c.pending != nil:
		next := a.config
		next.Prefix = *c.pending
		*c = keyCapture{}
		a.applyConfig(next)
	default:
		c.take(key)
	}
}

// resetConfig puts back the default of every setting the panel shows. Script folders and the AI
// policy, which are set elsewhere, stay.
func (a *App) resetConfig() {
	next := config.Default()
	next.ScriptFolders, next.MCP = a.config.ScriptFolders, a.config.MCP
	a.applyLogLimits(next, func() {
		a.reloadScripts()
		a.sortScripts()
		selected := a.configPanel.selected
		a.openConfig() // the program choices and the typed location follow the new values
		a.configPanel.selected = selected
		a.notice = "settings and keys reset to their defaults"
	})
}

// applyConfig puts a new config into effect right away and saves it. Invalid ones are refused.
func (a *App) applyConfig(next config.Config) {
	prefix, err := prefixFromConfig(next.Prefix)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.config, a.prefix, a.shortcut = next, prefix, shortcutFromConfig(next.Shortcut)
	a.keymap = newKeymap(next.Keys)
	a.applyMouseMode()
	a.palette = newPalette(themeNamed(next.Appearance.Theme), next.Appearance.Background == config.BackgroundTheme)
	if a.configPath == "" {
		return // no config directory; the problem was reported at startup
	}
	if err := config.Save(a.configPath, next); err != nil {
		a.statusMessage = err.Error()
	}
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func (a *App) configLines() []configLine {
	choice := func(value string) string { return "◂ " + value + " ▸" }
	if a.configPanel.page == bindsPage {
		return a.bindLines()
	}
	binds := "defaults ▸"
	if n := changedBinds(a.config); n > 0 {
		binds = fmt.Sprintf("%d changed ▸", n)
	}
	lines := []configLine{{label: "Keys", heading: true}}
	lines = append(lines, a.prefixLines()...)
	lines = append(lines, configLine{label: "binds of every screen", value: binds, row: bindsRow})
	lines = append(lines, []configLine{
		{label: "Shortcut", value: a.shortcut.String(), heading: true},
		{label: "ctrl", value: onOff(a.config.Shortcut.Ctrl), row: shortcutCtrlRow},
		{label: "alt", value: onOff(a.config.Shortcut.Alt), row: shortcutAltRow},
		{label: "Appearance", heading: true},
		{label: "theme", value: choice(themeNamed(a.config.Appearance.Theme).name), row: themeRow},
		{label: "background", value: a.config.Appearance.Background, row: backgroundRow},
		{label: "animate intro", value: choice(a.config.Appearance.AnimateIntro), row: animateIntroRow},
		{label: "mouse", value: onOff(a.config.Mouse), row: mouseRow},
		{label: "Tabs", heading: true},
		{label: "new tab opens", value: choice(a.newTabLabel(a.config.NewTab)), row: newTabRow},
		{label: "close when empty", value: onOff(a.config.CloseEmptyTabs), row: closeEmptyTabsRow},
		{label: "Programs", heading: true},
	}...)
	lines = append(lines, a.programLines("editor", &a.configPanel.editor, a.config.Editor, editorRow, editorCommandRow)...)
	lines = append(lines, a.programLines("file manager", &a.configPanel.fileManager, a.config.FileManager, fileManagerRow, fileManagerCommandRow)...)
	lines = append(lines,
		configLine{label: "Scripts", heading: true},
		configLine{label: "location", value: a.configPanel.scriptsDir.String(), row: scriptsDirRow, typed: true},
	)
	copyNote := "enter edit · ctrl+c copy"
	switch {
	case a.configPanel.copied:
		copyNote = "copied to clipboard"
	case a.configPanel.typing && a.configPanel.selected == scriptsDirRow:
		copyNote = "enter saves · esc cancels · ctrl+c copy"
	}
	lines = append(lines, configLine{label: copyNote, note: true})
	if note := scriptsDirNote(a.configPanel.scriptsDir.String()); note != "" {
		lines = append(lines, configLine{label: note, note: true})
	}
	lines = append(lines,
		configLine{label: "hints in new scripts", value: onOff(a.config.ScriptHints), row: scriptHintsRow},
		configLine{label: "sort", value: "◂ " + sortLabels[a.config.ScriptsSort] + " ▸", row: scriptsSortRow},
		configLine{label: "Logs", heading: true},
	)
	lines = append(lines, a.numberLines("keep", logsKeepRow, fmt.Sprintf("%d days", a.config.Logs.KeepDays))...)
	lines = append(lines, a.numberLines("max size", logsSizeRow, sizeText(a.config.Logs.MaxSizeKB))...)
	lines = append(lines, a.logUsageLine()...)
	return append(lines,
		configLine{label: "older and, past the size, oldest logs are deleted", note: true},
		configLine{label: "terminals", value: onOff(a.config.Logs.Terminals), row: logsTerminalsRow},
		configLine{label: "log what terminal panes print; runs are always logged", note: true},
		configLine{label: "Flows", heading: true},
		configLine{label: "step output kept", value: "◂ " + flowOutputLabel(a.config.Flows.OutputKB) + " ▸", row: flowsOutputRow},
		configLine{label: "in each flow's last result; tests keep 4 KB", note: true},
		configLine{label: "Updates", heading: true},
		configLine{label: "check for new versions", value: onOff(a.config.Update.Check), row: updateCheckRow},
		configLine{label: "on GitHub, at start and daily; installing always asks", note: true},
		configLine{label: "Defaults", heading: true},
		configLine{label: "reset all to defaults", value: a.resetValue(), row: resetRow},
	)
}

// prefixLines are the prefix's row and, while a new combination is being taken, what to press
// and what the combination pressed would take from.
func (a *App) prefixLines() []configLine {
	c := a.configPanel.capture
	value := a.prefix.String()
	switch {
	case c.pending != nil:
		pending, _ := prefixFromConfig(*c.pending) // taken only when valid
		value = pending.String()
	case c.active:
		value = "press the combination…"
	}
	lines := []configLine{{label: "prefix", value: value, row: prefixRow}}
	if !c.active {
		return lines
	}
	if c.problem != "" {
		lines = append(lines, configLine{label: c.problem, note: true, warning: true})
	}
	if c.pending == nil {
		return append(lines, configLine{label: "ctrl or alt with a key; esc cancels", note: true})
	}
	pending, _ := prefixFromConfig(*c.pending)
	for _, conflict := range a.prefixConflicts(pending) {
		lines = append(lines, configLine{label: "⚠ " + conflict, note: true, warning: true})
	}
	return append(lines, configLine{label: "enter uses it · esc cancels · or press another", note: true})
}

// resetWindow is how long the reset waits for the second enter before it lets go.
const resetWindow = 1000 * time.Millisecond

func (a *App) resetValue() string {
	if a.resetArmed() {
		return "enter again to reset"
	}
	return ""
}

func (a *App) resetArmed() bool {
	at := a.configPanel.resetArmedAt
	return !at.IsZero() && time.Since(at) < resetWindow
}

// pressReset runs enter on a reset row: the first arms it for resetWindow, a second within it
// resets. It tells whether the key was enter on a reset row.
func (a *App) pressReset(key uv.KeyPressEvent) bool {
	row := a.configPanel.selected
	if key.String() != "enter" || row != resetRow && row != resetBindsRow {
		return false
	}
	if !a.resetArmed() {
		a.configPanel.resetArmedAt = time.Now()
		a.resetExpired = time.After(resetWindow) // to redraw without the prompt once it lets go
		return true
	}
	a.configPanel.resetArmedAt = time.Time{}
	if row == resetBindsRow {
		a.resetBinds()
	} else {
		a.resetConfig()
	}
	return true
}

// programLines are a program setting's row and, while custom is chosen, the command typed for it
// with a note when that command is not installed.
func (a *App) programLines(label string, setting *programSetting, current string, row, commandRow configRow) []configLine {
	lines := []configLine{{label: label, value: "◂ " + setting.shown(current) + " ▸", row: row}}
	if !setting.custom {
		return lines
	}
	lines = append(lines, configLine{label: "command", value: setting.draft.String(), row: commandRow, typed: true})
	if note := commandNote(setting.draft.String()); note != "" {
		lines = append(lines, configLine{label: note, note: true})
	}
	return lines
}

// commandNote warns about a typed command whose program is not installed.
func commandNote(command string) string {
	fields := strings.Fields(command)
	switch {
	case len(fields) == 0:
		return "enter, then type the command, e.g. code -w"
	case !isInstalled(fields[0]):
		return "command not found"
	}
	return ""
}

// scriptsDirNote says that the scripts folder does not exist yet.
func scriptsDirNote(typed string) string {
	if _, err := os.Stat(script.ResolveDir(typed)); err != nil {
		return "folder not found; it is created with the first script"
	}
	return ""
}

// configLayout places the config centered over the workspace, a blank line before each section:
// the panel, the lines it holds and the first one shown. When they do not fit, the panel scrolls
// to keep the selected row in view.
type configLayout struct {
	panel image.Rectangle
	rows  []configLine // the panel's content, blank lines included
	first int
}

func (a *App) configLayout() configLayout {
	var l configLayout
	for i, line := range a.configLines() {
		if line.heading && i > 0 {
			l.rows = append(l.rows, configLine{note: true})
		}
		l.rows = append(l.rows, line)
	}
	l.panel = centeredRect(image.Rect(0, 0, a.size.X, a.size.Y), 72, panelHeight(len(l.rows)+2))
	for i, line := range l.rows {
		if line.selectable() && line.row == a.configPanel.selected {
			last := i // the row and the notes under it, such as what a key being taken would take
			for last+1 < len(l.rows) && l.rows[last+1].note && l.rows[last+1].label != "" {
				last++
			}
			l.first = listFirst(a.configPanel.first, i, len(l.rows), l.visible())
			l.first = max(l.first, last-l.visible()+1)
		}
	}
	a.configPanel.first = l.first // the same selection gives the same first, so later calls agree
	return l
}

func (l configLayout) inner() image.Rectangle { return l.panel.Inset(2) }

// visible is how many lines fit: the hints take the last two rows.
func (l configLayout) visible() int { return l.inner().Dy() - 2 }

// lineAt is the line drawn on row y of the screen.
func (l configLayout) lineAt(y int) (configLine, bool) {
	i := y - l.inner().Min.Y
	if i < 0 || i >= l.visible() || l.first+i >= len(l.rows) {
		return configLine{}, false
	}
	return l.rows[l.first+i], true
}

func (line configLine) selectable() bool { return !line.heading && !line.note }

func (a *App) drawConfigPanel(screen uv.Screen) {
	l := a.configLayout()
	title, hints := "config", configHints
	if a.configPanel.page == bindsPage {
		title, hints = "config › binds", bindsHints
	}
	inner := a.drawPanel(screen, l.panel, title)
	for i := l.first; i < len(l.rows) && i-l.first < l.visible(); i++ {
		a.drawConfigLine(screen, inner, inner.Min.Y+i-l.first, l.rows[i])
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("config", hints), a.palette.panelMuted)
}

// clickConfig selects the row clicked. On its value, a click steps the setting, back on ◂ and
// forward anywhere else, as ←→ do; on a typed row, it only selects, and enter opens it for typing. A click outside
// the panel closes it, as esc does.
func (a *App) clickConfig(point image.Point) {
	l := a.configLayout()
	if !point.In(l.panel) {
		a.closeConfig()
		return
	}
	a.configPanel.capture, a.configPanel.bindCapture = keyCapture{}, bindCapture{}
	line, ok := l.lineAt(point.Y)
	if !ok || !line.selectable() {
		return
	}
	a.selectConfigRow(line.row)
	valueX := l.inner().Max.X - 1 - ansi.StringWidth(line.value)
	if line.typed || point.X < valueX || line.value == "" {
		return
	}
	if row, ok := bindRowAt(line.row); ok {
		a.clickBind(row, point.X-valueX)
		return
	}
	if strings.HasPrefix(line.value, "◂") && point.X < valueX+2 {
		a.changeSetting(-1)
		return
	}
	a.changeSetting(1)
}

// hoverConfig marks the row under the mouse.
func (a *App) hoverConfig(point image.Point) {
	l := a.configLayout()
	line, ok := l.lineAt(point.Y)
	a.configPanel.hovering = ok && line.selectable() && point.In(l.inner())
	a.configPanel.hover = line.row
}

func (a *App) drawConfigLine(screen uv.Screen, inner image.Rectangle, y int, line configLine) {
	style := a.configLineStyle(line)
	switch {
	case line.bar:
		x := drawBar(screen, inner.Min.X+textColumn, y, barWidth, line.fraction, &a.palette)
		drawText(screen, x+1, y, inner.Max.X-1, line.label, a.palette.panelMuted)
	case line.note:
		drawText(screen, inner.Min.X+textColumn, y, inner.Max.X-1, line.label, a.noteStyle(line))
	case line.typed:
		fillRect(screen, image.Rect(inner.Min.X, y, inner.Max.X, y+1), style)
		drawText(screen, inner.Min.X+1, y, inner.Max.X, "  "+line.label, style)
		width := inner.Dx() - textColumn - 1
		value := tail(line.value, width)
		if line.row == a.configPanel.selected && (a.configPanel.typing || isNumberRow(line.row)) {
			value = a.typedText(line.row).view(width, false)
		}
		drawText(screen, inner.Min.X+textColumn, y, inner.Max.X-1, value, style)
	case line.heading:
		drawRow(screen, inner, y, line.label, line.value, style)
	default:
		drawRow(screen, inner, y, "  "+line.label, line.value, style)
	}
}

func (a *App) noteStyle(line configLine) uv.Style {
	if line.warning || line.label == "command not found" {
		return a.palette.status
	}
	return a.palette.panelMuted
}

func (a *App) configLineStyle(line configLine) uv.Style {
	switch {
	case line.heading:
		return a.palette.panelHeading
	case line.row == a.configPanel.selected:
		return a.palette.selectedRow
	case a.configPanel.hovering && line.row == a.configPanel.hover:
		return a.palette.focusedTitle
	}
	return a.palette.panelText
}

// sortOrders are the orders of the scripts list, as the config cycles through them.
var sortOrders = []string{config.SortByName, config.SortByEdited, config.SortByRun}

var sortLabels = map[string]string{config.SortByName: "alphabetical", config.SortByEdited: "last edited", config.SortByRun: "last run"}

// tail cuts text to its last width cells, marking the cut with "…": the end of a path or command
// is what tells it apart.
func tail(text string, width int) string {
	if width < 1 || ansi.StringWidth(text) <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 && ansi.StringWidth(string(runes))+1 > width {
		runes = runes[1:]
	}
	return "…" + string(runes)
}
