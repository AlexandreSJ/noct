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
	"github.com/AlexandreSJ/noct/internal/theme"
)

type configRow int

const (
	prefixRow configRow = iota
	bindsRow
	shortcutCtrlRow
	shortcutAltRow
	themeRow
	newThemeRow
	editThemeRow
	modeRow
	backgroundRow
	animateIntroRow
	animationsRow
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
	logsColumnsRow
	logsPreviewRow
	logsPreviewPercentRow
	flowsOutputRow
	notifyRow
	updateCheckRow
	resetRow
	resetBindsRow
)

// configPage is what the config panel shows: the settings, or the keys of every screen.
type configPage int

const (
	settingsPage configPage = iota
	bindsPage
	themePage
)

const configHints = "{up}{down} select  {left}{right} change  esc close"

// textColumn is where the text of a typed setting starts, after its label.
const textColumn = 20

type configPanel struct {
	page     configPage
	selected configRow
	// selectedAt is when the selection last moved, which a note too wide for the panel scrolls from.
	selectedAt time.Time
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
	// logColumns is the logs list's columns as typed, names a space apart.
	logColumns textField
	// typing is on while the selected typed row takes text, from enter until enter, esc or leaving
	// the row; typedBefore is its text when typing started, put back by esc.
	typing      bool
	typedBefore string
	// copied says the location was just copied, until the selection or the text changes.
	copied bool
	// hover is the row under the mouse, when hovering is true, and hoverX its column.
	hover    configRow
	hoverX   int
	hovering bool
	// edit is the theme being edited on the theme page.
	edit *themeEdit
	// preview is the animation the stage plays for the moment selected, nil when none plays.
	preview *animation
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
		logColumns:  newTextField(strings.Join(a.config.Logs.Columns, " ")),
		selectedAt:  a.now(),
	}
	a.refreshLogUsage()
	a.mode = configMode
}

func (a *App) handleConfigKey(key uv.KeyPressEvent) {
	if a.configPanel.capture.active {
		a.capturePrefix(key)
		return
	}
	if name, _, ok := a.shortcut.command(key, a.commandKey); ok && name == "," && !a.configPanel.bindCapture.active {
		a.closeConfig() // the shortcut that opens the config closes it again
		return
	}
	switch a.configPanel.page {
	case bindsPage:
		a.handleBindsKey(key)
		return
	case themePage:
		a.handleThemeEditorKey(key)
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
	if m, ok := previewedMoment(a.configPanel.selected); ok && key.String() == "space" {
		a.playStagePreview(m) // space plays it again; the arrows change it
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
	a.configPanel.selected, a.configPanel.selectedAt = row, a.now()
	if c, ok := themeRowAt(row); ok && a.configPanel.edit != nil {
		a.lightComponent(c)
	}
	a.configPanel.preview = nil
	if m, ok := previewedMoment(row); ok {
		a.playStagePreview(m)
	}
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
	return row == editorCommandRow || row == fileManagerCommandRow || row == scriptsDirRow || row == logsColumnsRow
}

// typedText is the text a typed row edits.
func (a *App) typedText(row configRow) *textField {
	switch row {
	case editorCommandRow:
		return &a.configPanel.editor.draft
	case fileManagerCommandRow:
		return &a.configPanel.fileManager.draft
	case logsColumnsRow:
		return &a.configPanel.logColumns
	}
	if isNumberRow(row) {
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
	if p.page == themePage && p.edit.picker != nil && p.edit.picker.typing {
		return &p.edit.picker.hex, func() { p.edit.picker.problem = "" }
	}
	if p.capture.active || p.page == bindsPage || p.page == themePage || !p.typing || !a.isTypedRow(p.selected) {
		return nil, nil
	}
	return a.typedText(p.selected), func() { p.copied = false }
}

// commitTyped saves what was typed in the selected row, when it is a typed row.
func (a *App) commitTyped() {
	next := a.config
	switch a.configPanel.selected {
	case logsColumnsRow:
		next.Logs.Columns = config.LogColumnsFrom(strings.Fields(a.configPanel.logColumns.String()))
		a.configPanel.logColumns = newTextField(strings.Join(next.Logs.Columns, " "))
		if !slices.Equal(next.Logs.Columns, a.config.Logs.Columns) {
			a.applyConfig(next)
		}
		return
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

// changeSetting steps the selected row's setting back (-1) or on (1), or does what a row that is
// no setting does.
func (a *App) changeSetting(step int) {
	row := a.configPanel.selected
	if a.actOnRow(row, step) {
		return
	}
	next := a.config
	after := a.stepSetting(&next, row, step)
	a.applyConfig(next)
	if after != nil {
		after()
	}
}

// actOnRow does what a row does when it opens something, or applies its change its own way. It
// tells whether the row is one of those.
func (a *App) actOnRow(row configRow, step int) bool {
	if _, ok := bindRowAt(row); ok {
		a.startBindEdit()
		return true
	}
	if _, ok := themeRowAt(row); ok {
		a.pickColor()
		return true
	}
	if m, ok := animationRowAt(row); ok {
		a.changeAnimation(m, step)
		return true
	}
	if m, ok := animationTimeRowAt(row); ok {
		a.changeAnimationTime(m, step)
		return true
	}
	switch row {
	case prefixRow:
		a.configPanel.capture = keyCapture{active: true}
	case bindsRow:
		a.openBinds()
	case newThemeRow:
		a.startNewTheme()
	case editThemeRow:
		a.editCurrentTheme()
	case resetBindsRow, resetRow:
		// only enter resets, twice, so neither an arrow nor a click does it by accident
	case logsKeepRow, logsSizeRow:
		a.stepLogLimit(row, step)
	default:
		return false
	}
	return true
}

// stepLogLimit steps how long or how much the logs keep; the logs past the new limit go at once.
func (a *App) stepLogLimit(row configRow, step int) {
	next := a.config
	if row == logsKeepRow {
		next.Logs.KeepDays = stepNumber(keepDaysChoices, next.Logs.KeepDays, step)
	} else {
		next.Logs.MaxSizeKB = stepNumber(logSizeChoices, next.Logs.MaxSizeKB, step)
	}
	a.applyLogLimits(next, nil)
}

// stepSetting changes the setting of a row in next, step places back or on. It returns what to do
// once the change is in effect, or nil.
func (a *App) stepSetting(next *config.Config, row configRow, step int) (after func()) {
	switch row {
	case shortcutCtrlRow:
		next.Shortcut.Ctrl = !next.Shortcut.Ctrl
	case shortcutAltRow:
		next.Shortcut.Alt = !next.Shortcut.Alt
	case themeRow:
		next.Appearance.Theme = a.nextThemeName(a.sessionTheme(a.session), step)
		a.session.theme = next.Appearance.Theme // the session on screen, and sessions without a theme
	case modeRow:
		next.Appearance.Mode = cycle([]string{config.ModeDark, config.ModeLight}, next.Appearance.Mode, step)
	case backgroundRow:
		next.Appearance.Background = cycle([]string{config.BackgroundTerminal, config.BackgroundTheme}, next.Appearance.Background, step)
	case animationsRow:
		next.Appearance.Animations = config.AnimationsOff
		if a.config.Appearance.Animations == config.AnimationsOff {
			next.Appearance.Animations = config.AnimationsOn
		}
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
		return a.sortScripts
	case logsTerminalsRow:
		next.Logs.Terminals = !next.Logs.Terminals
	case logsPreviewRow:
		next.Logs.Preview = cycle(config.LogPreviewSides, next.Logs.Preview, step)
	case logsPreviewPercentRow:
		next.Logs.PreviewPercent = min(max(next.Logs.PreviewPercent+step*previewPercentStep, config.MinPreviewPercent), config.MaxPreviewPercent)
	case flowsOutputRow:
		i := max(slices.Index(flowOutputChoices, next.Flows.OutputKB), 0)
		next.Flows.OutputKB = flowOutputChoices[min(max(i+step, 0), len(flowOutputChoices)-1)]
	case notifyRow:
		i := slices.Index(config.NotifyChoices, next.Notify)
		next.Notify = config.NotifyChoices[min(max(i+step, 0), len(config.NotifyChoices)-1)]
	case updateCheckRow:
		next.Update.Check = !next.Update.Check
		if next.Update.Check && !a.updates.checked {
			return a.checkForUpdates
		}
	}
	return nil
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
	a.applyPalette()
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

// configLines are the lines of the page shown; the settings page goes section by section.
func (a *App) configLines() []configLine {
	switch a.configPanel.page {
	case bindsPage:
		return a.bindLines()
	case themePage:
		return a.themeEditorLines()
	}
	return slices.Concat(
		a.keysLines(), a.appearanceLines(), a.animationLines(), a.tabsLines(), a.programsLines(),
		a.scriptsLines(), a.logsLines(), a.flowsLines(), a.updatesLines(), a.defaultsLines(),
	)
}

// choiceValue marks a value that the arrows step through.
func choiceValue(value string) string { return "< " + value + " >" }

func (a *App) keysLines() []configLine {
	binds := "defaults >"
	if n := changedBinds(a.config); n > 0 {
		binds = fmt.Sprintf("%d changed >", n)
	}
	lines := []configLine{{label: "Keys", heading: true}}
	lines = append(lines, a.prefixLines()...)
	return append(lines,
		configLine{label: "binds of every screen", value: binds, row: bindsRow},
		configLine{label: "Shortcut", value: a.shortcut.String(), heading: true},
		configLine{label: "ctrl", value: onOff(a.config.Shortcut.Ctrl), row: shortcutCtrlRow},
		configLine{label: "alt", value: onOff(a.config.Shortcut.Alt), row: shortcutAltRow},
	)
}

func (a *App) appearanceLines() []configLine {
	current := a.themeNamed(a.sessionTheme(a.session))
	edit := "enter"
	if current.Builtin {
		edit = "copy"
	}
	return []configLine{
		{label: "Appearance", heading: true},
		{label: "theme", value: choiceValue(current.Name), row: themeRow},
		{label: "new theme", value: "enter", row: newThemeRow},
		{label: "edit theme", value: edit, row: editThemeRow},
		{label: "mode", value: a.config.Appearance.Mode, row: modeRow},
		{label: "background", value: a.config.Appearance.Background, row: backgroundRow},
		{label: "animate intro", value: choiceValue(a.config.Appearance.AnimateIntro), row: animateIntroRow},
		{label: "mouse", value: onOff(a.config.Mouse), row: mouseRow},
	}
}

func (a *App) tabsLines() []configLine {
	return []configLine{
		{label: "Tabs", heading: true},
		{label: "new tab opens", value: choiceValue(a.newTabLabel(a.config.NewTab)), row: newTabRow},
		{label: "close when empty", value: onOff(a.config.CloseEmptyTabs), row: closeEmptyTabsRow},
	}
}

func (a *App) programsLines() []configLine {
	lines := []configLine{{label: "Programs", heading: true}}
	lines = append(lines, a.programLines("editor", &a.configPanel.editor, a.config.Editor, editorRow, editorCommandRow)...)
	return append(lines, a.programLines("file manager", &a.configPanel.fileManager, a.config.FileManager, fileManagerRow, fileManagerCommandRow)...)
}

func (a *App) scriptsLines() []configLine {
	lines := []configLine{
		{label: "Scripts", heading: true},
		{label: "location", value: a.configPanel.scriptsDir.String(), row: scriptsDirRow, typed: true},
		{label: a.scriptsDirHelp(), note: true},
	}
	if note := scriptsDirNote(a.configPanel.scriptsDir.String()); note != "" {
		lines = append(lines, configLine{label: note, note: true})
	}
	return append(lines,
		configLine{label: "hints in new scripts", value: onOff(a.config.ScriptHints), row: scriptHintsRow},
		configLine{label: "sort", value: choiceValue(sortLabels[a.config.ScriptsSort]), row: scriptsSortRow},
	)
}

// scriptsDirHelp is the note under the scripts' location: its keys, or that it was just copied.
func (a *App) scriptsDirHelp() string {
	switch {
	case a.configPanel.copied:
		return "copied to clipboard"
	case a.configPanel.typing && a.configPanel.selected == scriptsDirRow:
		return "enter saves · esc cancels · ctrl+c copy"
	}
	return "enter edit · ctrl+c copy"
}

func (a *App) logsLines() []configLine {
	lines := []configLine{{label: "Logs", heading: true}}
	lines = append(lines, a.numberLines("keep", logsKeepRow, fmt.Sprintf("%d days", a.config.Logs.KeepDays))...)
	lines = append(lines, a.numberLines("max size", logsSizeRow, sizeText(a.config.Logs.MaxSizeKB))...)
	lines = append(lines, a.logUsageLine()...)
	lines = append(lines,
		configLine{label: "logs not written to for longer and, past the size, the least recent are deleted", note: true},
		configLine{label: "terminals", value: onOff(a.config.Logs.Terminals), row: logsTerminalsRow},
		configLine{label: "log what terminal panes print; runs are always logged", note: true},
	)
	lines = append(lines, a.logColumnsLines()...)
	return append(lines,
		configLine{label: "log", value: choiceValue(a.config.Logs.Preview), row: logsPreviewRow},
		configLine{label: "log size", value: choiceValue(fmt.Sprintf("%d%%", a.config.Logs.PreviewPercent)), row: logsPreviewPercentRow},
		configLine{label: "where the logs view shows the selected log; there, p moves it and + - size it", note: true},
	)
}

func (a *App) flowsLines() []configLine {
	return []configLine{
		{label: "Flows", heading: true},
		{label: "step output kept", value: choiceValue(flowOutputLabel(a.config.Flows.OutputKB)), row: flowsOutputRow},
		{label: "in each flow's last result; tests keep 4 KB", note: true},
		{label: "tell when runs end", value: choiceValue(a.config.Notify), row: notifyRow},
		{label: "runs out of sight: system notification, the terminal's bell, or off", note: true},
	}
}

func (a *App) updatesLines() []configLine {
	return []configLine{
		{label: "Updates", heading: true},
		{label: "check for new versions", value: onOff(a.config.Update.Check), row: updateCheckRow},
		{label: "only checks; install with " + a.prefix.String() + "+" + a.hint("commands", "{c}"), note: true},
	}
}

func (a *App) defaultsLines() []configLine {
	return []configLine{
		{label: "Defaults", heading: true},
		{label: "reset all to defaults", value: a.resetValue(), row: resetRow},
	}
}

// logColumnsLines are the logs list's columns, typed in order, with the names there are and the
// words typed that are none of them.
func (a *App) logColumnsLines() []configLine {
	p := &a.configPanel
	note := "enter edit"
	if p.typing && p.selected == logsColumnsRow {
		note = "enter saves · esc cancels"
	}
	lines := []configLine{
		{label: "list columns", value: p.logColumns.String(), row: logsColumnsRow, typed: true},
		{label: note + " · in order, out of " + strings.Join(config.LogColumns, " "), note: true},
	}
	var unknown []string
	for _, word := range strings.Fields(strings.ToLower(p.logColumns.String())) {
		if !slices.Contains(config.LogColumns, word) {
			unknown = append(unknown, word)
		}
	}
	if len(unknown) > 0 {
		lines = append(lines, configLine{label: "not a column, left out: " + strings.Join(unknown, " "), note: true, warning: true})
	}
	return lines
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
	lines := []configLine{{label: label, value: choiceValue(setting.shown(current)), row: row}}
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
// to keep the selected row in view. Drawing and clicks both go by list, the area the lines are
// drawn in, so a click lands on what is drawn.
type configLayout struct {
	panel image.Rectangle
	rows  []configLine // the panel's content, blank lines included
	first int
	list  image.Rectangle
	// stage is where the animations' example plays under the list, empty when it does not show.
	stage image.Rectangle
}

// stageShare is the part of the panel's height the animations' example takes, under the rows.
const stageShare = 0.3

func (a *App) configLayout() configLayout {
	var l configLayout
	for i, line := range a.configLines() {
		if line.heading && i > 0 {
			l.rows = append(l.rows, configLine{note: true})
		}
		l.rows = append(l.rows, line)
	}
	width, height := 72, panelHeight(len(l.rows)+2)
	if a.configPanel.page == themePage {
		width = themePanelWidth                           // the list of components and the stage side by side
		height = max(height, panelHeight(pickerHeight+2)) // the picker and the hints under it
	}
	showsStage := a.configShowsStage()
	if showsStage {
		height = a.size.Y // the example takes its part of the height, under the rows
	}
	l.panel = centeredRect(image.Rect(0, 0, a.size.X, a.size.Y), width, height)
	l.list = l.inner()
	l.list.Max.Y -= 2 // the hints
	if stage := int(float64(l.list.Dy()) * stageShare); showsStage && stage > stageMinHeight {
		l.stage = image.Rect(l.list.Min.X, l.list.Max.Y-stage+1, l.list.Max.X, l.list.Max.Y) // under a rule
		l.list.Max.Y -= stage
	}
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

// visible is how many lines fit in the list.
func (l configLayout) visible() int { return l.list.Dy() }

// lineAt is the line drawn on row y of the screen.
func (l configLayout) lineAt(y int) (configLine, bool) {
	i := y - l.list.Min.Y
	if i < 0 || i >= l.visible() || l.first+i >= len(l.rows) {
		return configLine{}, false
	}
	return l.rows[l.first+i], true
}

func (line configLine) selectable() bool { return !line.heading && !line.note }

// rowAbove is the row the note at i is under, past the notes between them.
func (l configLayout) rowAbove(i int) (configRow, bool) {
	for i--; i >= 0; i-- {
		switch line := l.rows[i]; {
		case line.selectable():
			return line.row, true
		case !line.note || line.label == "":
			return 0, false // a heading or the blank line before one
		}
	}
	return 0, false
}

// A note too wide for the panel scrolls while its row is selected, slowly enough to read: it
// holds at its start, moves a cell per noteStep until its end shows, holds there and starts over.
const (
	noteHold = 1500 * time.Millisecond
	noteStep = 250 * time.Millisecond
)

// scrolledNote is the part of text that shows in width now, asking for the frame that moves it.
func (a *App) scrolledNote(text string, width int) string {
	overflow := ansi.StringWidth(text) - width
	if overflow <= 0 || width < 1 {
		return text
	}
	if a.noteTick == nil {
		a.noteTick = time.After(noteStep)
	}
	period := 2*noteHold + time.Duration(overflow)*noteStep
	elapsed := max(a.now().Sub(a.configPanel.selectedAt), 0) % period
	offset := min(max(int((elapsed-noteHold)/noteStep), 0), overflow)
	return ansi.Cut(text, offset, offset+width)
}

func (a *App) drawConfigPanel(screen uv.Screen) {
	l := a.configLayout()
	title, hints := "config", configHints
	switch a.configPanel.page {
	case bindsPage:
		title, hints = "config › binds", bindsHints
	case themePage:
		title, hints = "config › theme › "+a.configPanel.edit.theme.Name, themeEditorHints
		if picker := a.configPanel.edit.picker; picker != nil {
			title += " › " + theme.Components[a.configPanel.edit.selected].Label
			hints = picker.hints()
		}
	}
	inner := a.drawPanel(screen, l.panel, title)
	if a.configPanel.page == themePage {
		a.drawThemeEditor(screen, l, inner)
		drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("config", hints), a.palette.panelMuted)
		return
	}
	if !l.stage.Empty() {
		rule := l.stage.Min.Y - 1
		for x := inner.Min.X; x < inner.Max.X; x++ {
			setRune(screen, x, rule, "─", a.palette.unfocusedFrame)
		}
		drawText(screen, inner.Min.X+2, rule, inner.Max.X, " example · space repeats it ", a.palette.panelMuted)
		a.drawStagePreview(screen, l.stage)
	}
	for i := l.first; i < len(l.rows) && i-l.first < l.visible(); i++ {
		line := l.rows[i]
		if row, ok := l.rowAbove(i); ok && line.note && !line.bar && row == a.configPanel.selected {
			line.label = a.scrolledNote(line.label, l.list.Dx()-textColumn-1)
		}
		a.drawConfigLine(screen, l.list, l.list.Min.Y+i-l.first, line)
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("config", hints), a.palette.panelMuted)
}

// clickConfig selects the row clicked. On a choice's arrows, a cell around each included, a click
// steps the setting back or forward, as ←→ do; on another value, such as on/off, it steps
// forward. On a choice's text or a typed row it only selects, and enter opens a typed row for
// typing. A click outside the panel closes it, as esc does.
func (a *App) clickConfig(point image.Point) {
	l := a.configLayout()
	if !point.In(l.panel) {
		a.closeConfig()
		return
	}
	a.configPanel.capture, a.configPanel.bindCapture = keyCapture{}, bindCapture{}
	if a.configPanel.page == themePage {
		a.clickThemeEditor(point, l)
		return
	}
	line, ok := l.lineAt(point.Y)
	if !ok || !line.selectable() {
		return
	}
	a.selectConfigRow(line.row)
	step, acts := valueStep(l.list, line, point.X)
	if !acts {
		return
	}
	if row, ok := bindRowAt(line.row); ok {
		start, _ := valueSpan(l.list, line)
		a.clickBind(row, point.X-start)
		return
	}
	a.changeSetting(step)
}

// valueSpan is where line's value is drawn in list: its first column and the one past its last.
func valueSpan(list image.Rectangle, line configLine) (int, int) {
	end := list.Max.X - 1
	return end - ansi.StringWidth(line.value), end
}

// valueStep is what a click at column x does to line's value: on a choice, -1 on its < and 1 on
// its >, each with the cell on either side; on another value, 1, stepping forward or opening it.
// acts is false where a click only selects the row.
func valueStep(list image.Rectangle, line configLine, x int) (step int, acts bool) {
	start, end := valueSpan(list, line)
	switch {
	case line.typed || line.value == "":
		return 0, false
	case !strings.HasPrefix(line.value, "<"):
		return 1, x >= start
	case x >= start-1 && x <= start+1:
		return -1, true
	case x >= end-2 && x <= end:
		return 1, true
	}
	return 0, false
}

// clickable is the columns of line, from and up to, that a click at column x acts on, which
// hovering lights: a choice's arrow that the click takes, another value whole, or a typed
// row's field.
func clickable(list image.Rectangle, line configLine, x int) (from, to int, ok bool) {
	if line.typed {
		from = list.Min.X + textColumn
		return from, list.Max.X - 1, x >= from
	}
	step, acts := valueStep(list, line, x)
	start, end := valueSpan(list, line)
	switch {
	case !acts:
		return 0, 0, false
	case !strings.HasPrefix(line.value, "<"):
		return start, end, true
	case step < 0:
		return start, start + 1, true
	}
	return end - 1, end, true
}

// hoverConfig marks the row under the mouse, and where on it the mouse is.
func (a *App) hoverConfig(point image.Point) {
	l := a.configLayout()
	line, ok := l.lineAt(point.Y)
	a.configPanel.hovering = ok && line.selectable() && point.In(l.list)
	a.configPanel.hover, a.configPanel.hoverX = line.row, point.X
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
	if !a.configPanel.hovering || line.row != a.configPanel.hover || a.configPanel.page == themePage {
		return
	}
	if from, to, ok := clickable(inner, line, a.configPanel.hoverX); ok {
		lightCells(screen, from, to, y)
	}
}

// lightCells shows the cells of line y from column from up to to in reverse video, marking what
// a click under the mouse acts on; the terminal swaps the colors, its defaults included.
func lightCells(screen uv.Screen, from, to, y int) {
	for x := from; x < to; x++ {
		if c := screen.CellAt(x, y); c != nil {
			cell := *c
			cell.Style.Attrs |= uv.AttrReverse
			screen.SetCell(x, y, &cell)
		}
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
