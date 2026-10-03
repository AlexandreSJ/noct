package app

import (
	"fmt"
	"image"
	"os"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

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
		{label: "mouse", value: onOff(a.config.Mouse), row: mouseRow},
		{label: "Tabs", heading: true},
		{label: "new tab opens", value: choice(a.newTabLabel(a.config.NewTab)), row: newTabRow},
		{label: "close when empty", value: onOff(a.config.CloseEmptyTabs), row: closeEmptyTabsRow},
		{label: "Programs", heading: true},
	}...)
	lines = append(lines, a.programLines("editor", &a.configPanel.editor, a.config.Editor, editorRow, editorCommandRow)...)
	lines = append(lines, a.programLines("file manager", &a.configPanel.fileManager, a.config.FileManager, fileManagerRow, fileManagerCommandRow)...)
	lines = append(lines, configLine{label: "Scripts", heading: true})
	lines = append(lines, a.scriptsLocationLines()...)
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
		configLine{label: "Defaults", heading: true},
		configLine{label: "reset all to defaults", value: a.resetValue(), row: resetRow},
	)
}

// scriptsLocationLines are the scripts folder's typed row, how to copy it, and whether it exists.
func (a *App) scriptsLocationLines() []configLine {
	copyNote := "ctrl+c copy"
	if a.configPanel.copied {
		copyNote = "copied to clipboard"
	}
	lines := []configLine{
		{label: "location", value: string(a.configPanel.scriptsDir), row: scriptsDirRow, typed: true},
		{label: copyNote, note: true},
	}
	if note := scriptsDirNote(a.configPanel.scriptsDir); note != "" {
		lines = append(lines, configLine{label: note, note: true})
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

func (a *App) resetValue() string {
	if a.configPanel.resetArmed {
		return "enter again to reset"
	}
	return ""
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
			l.first = l.firstShowing(i, a.configPanel.first)
		}
	}
	a.configPanel.first = l.first // the same selection gives the same first, so later calls agree
	return l
}

// firstShowing is the first line to show so the row at index stays in view with the notes under
// it, such as what a key being taken would take.
func (l configLayout) firstShowing(index, previousFirst int) int {
	last := index
	for last+1 < len(l.rows) && l.rows[last+1].note && l.rows[last+1].label != "" {
		last++
	}
	first := listFirst(previousFirst, index, len(l.rows), l.visible())
	return max(first, last-l.visible()+1)
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
// forward anywhere else, as ←→ do; on a typed row, it only selects, for typing. A click outside
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
	case line.note:
		drawText(screen, inner.Min.X+textColumn, y, inner.Max.X-1, line.label, a.noteStyle(line))
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
