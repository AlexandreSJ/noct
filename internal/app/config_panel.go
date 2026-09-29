package app

import (
	"fmt"
	"image"
	"os"
	"reflect"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

type configRow int

const (
	prefixCtrlRow configRow = iota
	prefixAltRow
	prefixShiftRow
	prefixKeyRow
	shortcutCtrlRow
	shortcutAltRow
	themeRow
	backgroundRow
	mouseRow
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
)

const configHints = "↑↓ select  ←→ change  esc close"

// textColumn is where the text of a typed setting starts, after its label.
const textColumn = 20

type configPanel struct {
	selected     configRow
	capturingKey bool
	editor       programSetting
	fileManager  programSetting
	scriptsDir   []rune
	// copied says the location was just copied, until the selection or the text changes.
	copied bool
}

// configLine is one line of the panel: a section heading, a row that can be selected, or a note
// under the row before it.
type configLine struct {
	label   string
	value   string
	row     configRow
	heading bool
	note    bool
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
	if a.configPanel.capturingKey {
		a.capturePrefixKey(key)
		return
	}
	switch key.String() {
	case "esc":
		a.commitTyped()
		a.configPanel.editor.dropEmptyCustom()
		a.configPanel.fileManager.dropEmptyCustom()
		a.mode = normalMode
		return
	case "up":
		a.moveConfigSelection(-1)
		return
	case "down":
		a.moveConfigSelection(1)
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

// moveConfigSelection selects the next row shown, saving what was typed in the row it leaves.
func (a *App) moveConfigSelection(step int) {
	a.commitTyped()
	a.configPanel.copied = false
	var rows []configRow
	for _, line := range a.configLines() {
		if !line.heading && !line.note {
			rows = append(rows, line.row)
		}
	}
	index := max(slices.Index(rows, a.configPanel.selected), 0)
	a.configPanel.selected = rows[(index+step+len(rows))%len(rows)]
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

func (a *App) changeSetting(step int) {
	next := a.config
	switch a.configPanel.selected {
	case prefixCtrlRow:
		next.Prefix.Ctrl = !next.Prefix.Ctrl
	case prefixAltRow:
		next.Prefix.Alt = !next.Prefix.Alt
	case prefixShiftRow:
		next.Prefix.Shift = !next.Prefix.Shift
	case prefixKeyRow:
		a.configPanel.capturingKey = true
		return
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

func (a *App) capturePrefixKey(key uv.KeyPressEvent) {
	a.configPanel.capturingKey = false
	if key.MatchString("esc") {
		return
	}
	name, ok := prefixKeyName(key.Code)
	if !ok {
		a.statusMessage = "the prefix key must be a character, space or F1–F12"
		return
	}
	next := a.config
	next.Prefix.Key = name
	a.applyConfig(next)
}

// applyConfig puts a new config into effect right away and saves it. Invalid ones are refused.
func (a *App) applyConfig(next config.Config) {
	prefix, err := prefixFromConfig(next.Prefix)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.config, a.prefix, a.shortcut = next, prefix, shortcutFromConfig(next.Shortcut)
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
	key := a.config.Prefix.Key
	if a.configPanel.capturingKey {
		key = "press a key…"
	}
	lines := []configLine{
		{label: "Prefix", value: a.prefix.String(), heading: true},
		{label: "ctrl", value: onOff(a.config.Prefix.Ctrl), row: prefixCtrlRow},
		{label: "alt", value: onOff(a.config.Prefix.Alt), row: prefixAltRow},
		{label: "shift", value: onOff(a.config.Prefix.Shift), row: prefixShiftRow},
		{label: "key", value: key, row: prefixKeyRow},
		{label: "Shortcut", value: a.shortcut.String(), heading: true},
		{label: "ctrl", value: onOff(a.config.Shortcut.Ctrl), row: shortcutCtrlRow},
		{label: "alt", value: onOff(a.config.Shortcut.Alt), row: shortcutAltRow},
		{label: "Appearance", heading: true},
		{label: "theme", value: choice(themeNamed(a.config.Appearance.Theme).name), row: themeRow},
		{label: "background", value: a.config.Appearance.Background, row: backgroundRow},
		{label: "mouse", value: onOff(a.config.Mouse), row: mouseRow},
		{label: "Programs", heading: true},
	}
	lines = append(lines, a.programLines("editor", &a.configPanel.editor, a.config.Editor, editorRow, editorCommandRow)...)
	lines = append(lines, a.programLines("file manager", &a.configPanel.fileManager, a.config.FileManager, fileManagerRow, fileManagerCommandRow)...)
	lines = append(lines,
		configLine{label: "Scripts", heading: true},
		configLine{label: "location", value: string(a.configPanel.scriptsDir), row: scriptsDirRow, typed: true},
	)
	if a.configPanel.selected == scriptsDirRow {
		copyNote := "ctrl+c copy"
		if a.configPanel.copied {
			copyNote = "copied to clipboard"
		}
		lines = append(lines, configLine{label: copyNote, note: true})
	}
	if note := scriptsDirNote(a.configPanel.scriptsDir); note != "" {
		lines = append(lines, configLine{label: note, note: true})
	}
	return append(lines,
		configLine{label: "hints in new scripts", value: onOff(a.config.ScriptHints), row: scriptHintsRow},
		configLine{label: "sort", value: "◂ " + sortLabels[a.config.ScriptsSort] + " ▸", row: scriptsSortRow},
		configLine{label: "Logs", heading: true},
		configLine{label: "keep", value: fmt.Sprintf("◂ %d days ▸", a.config.Logs.KeepDays), row: logsKeepRow},
		configLine{label: "max size", value: "◂ " + logstore.PrettySize(int64(a.config.Logs.MaxSizeMB)<<20) + " ▸", row: logsSizeRow},
		configLine{label: "older and, past the size, oldest logs are deleted", note: true},
		configLine{label: "Flows", heading: true},
		configLine{label: "step output kept", value: "◂ " + flowOutputLabel(a.config.Flows.OutputKB) + " ▸", row: flowsOutputRow},
		configLine{label: "in each flow's last result; tests keep 4 KB", note: true},
	)
}

// programLines are a program setting's row and, while custom is chosen, the command typed for it
// with a note when that command is not installed.
func (a *App) programLines(label string, setting *programSetting, current string, row, commandRow configRow) []configLine {
	lines := []configLine{{label: label, value: "◂ " + setting.shown(current) + " ▸", row: row}}
	if !setting.custom {
		return lines
	}
	lines = append(lines, configLine{label: "command", value: string(setting.draft), row: commandRow, typed: true})
	if note := commandNote(string(setting.draft)); note != "" {
		lines = append(lines, configLine{label: note, note: true})
	}
	return lines
}

// commandNote warns about a typed command whose program is not installed.
func commandNote(command string) string {
	fields := strings.Fields(command)
	switch {
	case len(fields) == 0:
		return "type the command, e.g. code -w"
	case !isInstalled(fields[0]):
		return "command not found"
	}
	return ""
}

// scriptsDirNote says that the scripts folder does not exist yet.
func scriptsDirNote(typed []rune) string {
	if _, err := os.Stat(script.ResolveDir(string(typed))); err != nil {
		return "folder not found; it is created with the first script"
	}
	return ""
}

// drawConfigPanel draws the config centered over the workspace, a blank line before each
// section. When it does not fit, it scrolls to keep the selected row in view.
func (a *App) drawConfigPanel(screen uv.Screen) {
	lines := a.configLines()
	var rows []configLine // the panel's content, blank lines included
	for i, line := range lines {
		if line.heading && i > 0 {
			rows = append(rows, configLine{note: true})
		}
		rows = append(rows, line)
	}
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, 72, panelHeight(len(rows)+2)), "config")
	visible := inner.Dy() - 2 // the hints take the last two rows
	first := 0
	for i, line := range rows {
		if !line.heading && !line.note && line.row == a.configPanel.selected {
			first = max(i-visible+1, 0)
		}
	}
	for i := first; i < len(rows) && i-first < visible; i++ {
		a.drawConfigLine(screen, inner, inner.Min.Y+i-first, rows[i])
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, configHints, a.palette.panelMuted)
}

func (a *App) drawConfigLine(screen uv.Screen, inner image.Rectangle, y int, line configLine) {
	style := a.configLineStyle(line)
	switch {
	case line.note:
		drawText(screen, inner.Min.X+textColumn, y, inner.Max.X-1, line.label, a.noteStyle(line.label))
	case line.typed:
		fillRect(screen, image.Rect(inner.Min.X, y, inner.Max.X, y+1), style)
		drawText(screen, inner.Min.X+1, y, inner.Max.X, "  "+line.label, style)
		value := line.value
		if line.row == a.configPanel.selected {
			value += "▏"
		}
		drawText(screen, inner.Min.X+textColumn, y, inner.Max.X-1, tail(value, inner.Dx()-textColumn-1), style)
	case line.heading:
		drawRow(screen, inner, y, line.label, line.value, style)
	default:
		drawRow(screen, inner, y, "  "+line.label, line.value, style)
	}
}

func (a *App) noteStyle(note string) uv.Style {
	if note == "command not found" {
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
