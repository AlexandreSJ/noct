package app

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

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
	flowsOutputRow
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
	// resetArmed asks for the reset again before doing it, until the selection moves.
	resetArmed  bool
	editor      programSetting
	fileManager programSetting
	scriptsDir  []rune
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
	// typed rows take text: their value is edited in place.
	typed bool
}

func (a *App) openConfig() {
	a.configPanel = configPanel{
		editor:      newProgramSetting(editorChoices(isInstalled), a.config.Editor),
		fileManager: newProgramSetting(fileManagerChoices(isInstalled), a.config.FileManager),
		scriptsDir:  []rune(a.scriptsDir()),
	}
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
	if a.isTypedRow(a.configPanel.selected) {
		a.typeInConfig(key)
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
	a.configPanel.copied, a.configPanel.resetArmed = false, false
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
func (a *App) typedText(row configRow) *[]rune {
	switch row {
	case editorCommandRow:
		return &a.configPanel.editor.draft
	case fileManagerCommandRow:
		return &a.configPanel.fileManager.draft
	}
	return &a.configPanel.scriptsDir
}

func (a *App) typeInConfig(key uv.KeyPressEvent) {
	text := a.typedText(a.configPanel.selected)
	a.configPanel.copied = false
	switch key.String() {
	case "ctrl+c", "ctrl+shift+c":
		if a.configPanel.selected == scriptsDirRow {
			a.copyToClipboard(script.ResolveDir(string(*text)))
			a.configPanel.copied = true
		}
	case "enter":
		a.commitTyped()
	case "backspace":
		*text = deleteLastRune(*text)
	case "ctrl+u":
		*text = nil
	default:
		if isPlainText(key) {
			*text = append(*text, []rune(key.Text)...)
		}
	}
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
		next.ScriptsDir = strings.TrimSpace(string(a.configPanel.scriptsDir))
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

// runConfigAction does what a row that is an action rather than a value does, and tells whether
// the selected row is one.
func (a *App) runConfigAction() bool {
	if _, ok := bindRowAt(a.configPanel.selected); ok {
		a.startBindEdit()
		return true
	}
	switch a.configPanel.selected {
	case prefixRow:
		a.configPanel.capture = keyCapture{active: true}
	case bindsRow:
		a.openBinds()
	case resetBindsRow:
		a.armOrReset(a.resetBinds)
	case resetRow:
		a.armOrReset(a.resetConfig)
	default:
		return false
	}
	return true
}

// armOrReset runs reset on the second ask only, so a stray key cannot wipe the settings.
func (a *App) armOrReset(reset func()) {
	if !a.configPanel.resetArmed {
		a.configPanel.resetArmed = true
		return
	}
	reset()
}

func (a *App) changeSetting(step int) {
	if a.runConfigAction() {
		return
	}
	next := a.config
	switch a.configPanel.selected {
	case shortcutCtrlRow:
		next.Shortcut.Ctrl = !next.Shortcut.Ctrl
	case shortcutAltRow:
		next.Shortcut.Alt = !next.Shortcut.Alt
	case themeRow:
		next.Appearance.Theme = nextThemeName(next.Appearance.Theme, step)
	case backgroundRow:
		next.Appearance.Background = cycle([]string{config.BackgroundTerminal, config.BackgroundTheme}, next.Appearance.Background, step)
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
		defer a.pruneLogs()
	case logsSizeRow:
		next.Logs.MaxSizeMB = stepNumber(logSizeChoices, next.Logs.MaxSizeMB, step)
		defer a.pruneLogs()
	case flowsOutputRow:
		i := max(slices.Index(flowOutputChoices, next.Flows.OutputKB), 0)
		next.Flows.OutputKB = flowOutputChoices[min(max(i+step, 0), len(flowOutputChoices)-1)]
	}
	a.applyConfig(next)
}

// Choices of the log limits; a value set by hand in the file steps to the nearest one.
var (
	keepDaysChoices = []int{7, 14, 30, 60, 90, 180, 365}
	logSizeChoices  = []int{50, 100, 200, 500, 1000, 2000, 5000}
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
	a.applyConfig(next)
	a.reloadScripts()
	a.sortScripts()
	a.pruneLogs()
	selected := a.configPanel.selected
	a.openConfig() // the program choices and the typed location follow the new values
	a.configPanel.selected = selected
	a.notice = "settings and keys reset to their defaults"
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

// sortOrders are the orders of the scripts list, as the config cycles through them.
var sortOrders = []string{config.SortByName, config.SortByEdited, config.SortByRun}

var sortLabels = map[string]string{config.SortByName: "alphabetical", config.SortByEdited: "last edited", config.SortByRun: "last run"}
