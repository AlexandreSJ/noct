package app

import (
	"fmt"
	"image"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// setupStepNames name the steps in the setup's heading, from the keys to the summary.
var setupStepNames = []string{"keys", "themes", "animations", "demo", "AI", "binds", "done"}

// setupSwatch are the parts of a theme its row shows a cell of.
var setupSwatch = []theme.Component{theme.ActiveTabBackground, theme.FocusedBorder, theme.FocusedTitle, theme.PanelHeading, theme.Attention, theme.Logo}

// setupLine is a row of a step: what it is, its mark or value, and whether it is selected.
type setupLine struct {
	text     string
	muted    bool
	selected bool
	// theme, when set, draws the theme's swatch after the text.
	theme string
	// pinned, on the first lines, keeps them on top while the lines under them scroll.
	pinned bool
}

func (a *App) drawSetup(screen uv.Screen) {
	w := &a.setup
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, min(a.size.X-4, 96), min(a.size.Y-2, 26)), "noct setup")
	y := inner.Min.Y
	if w.step != setupStart {
		drawText(screen, inner.Min.X, y, inner.Max.X, a.setupHeading(), a.palette.panelMuted)
		y += 2
	}
	list := image.Rect(inner.Min.X, y, inner.Max.X, inner.Max.Y-2)
	if w.step == setupThemes && list.Dx() > 2*stageMinWidth {
		stage := list
		stage.Min.X = list.Max.X - stageMinWidth - 8
		list.Max.X = stage.Min.X - 2
		drawStage(screen, stage, &a.palette, -1, false)
	}
	a.drawSetupLines(screen, list, a.setupLines())
	drawText(screen, inner.Min.X, inner.Max.Y-1, inner.Max.X, a.setupHints(), a.palette.panelMuted)
}

func (a *App) setupHeading() string {
	parts := make([]string, len(setupStepNames))
	for i, name := range setupStepNames {
		parts[i] = name
		if setupStep(i+1) == a.setup.step {
			parts[i] = "[" + name + "]"
		}
	}
	return strings.Join(parts, " · ")
}

func (a *App) drawSetupLines(screen uv.Screen, area image.Rectangle, lines []setupLine) {
	for len(lines) > 0 && lines[0].pinned && area.Dy() > 1 {
		drawText(screen, area.Min.X+1, area.Min.Y, area.Max.X, lines[0].text, a.palette.panelMuted)
		lines, area.Min.Y = lines[1:], area.Min.Y+1
	}
	selected := 0
	for i, line := range lines {
		if line.selected {
			selected = i
		}
	}
	first := listFirst(0, selected, len(lines), area.Dy())
	for i := first; i < len(lines) && i-first < area.Dy(); i++ {
		line, y := lines[i], area.Min.Y+i-first
		style := a.palette.panelText
		switch {
		case line.selected:
			style = a.palette.selectedRow
			fillRect(screen, image.Rect(area.Min.X, y, area.Max.X, y+1), style)
		case line.muted:
			style = a.palette.panelMuted
		}
		x := drawText(screen, area.Min.X+1, y, area.Max.X, line.text, style)
		if line.theme != "" {
			a.drawSwatch(screen, x+1, y, area.Max.X, line.theme)
		}
	}
}

// drawSwatch shows a cell of each of a theme's main colors.
func (a *App) drawSwatch(screen uv.Screen, x, y, maxX int, name string) {
	t := a.themeNamed(name).In(a.setup.draft.Appearance.Mode == config.ModeLight)
	for _, c := range setupSwatch {
		x = drawText(screen, x, y, maxX, "██", uv.Style{Fg: t.Colors[c].Resolve()})
	}
}

func (a *App) setupLines() []setupLine {
	w := &a.setup
	switch w.step {
	case setupStart:
		return a.setupStartLines()
	case setupKeys:
		return []setupLine{
			{text: "prefix     ◂ " + prefixName(w.draft.Prefix) + " ▸", selected: w.selected == 0},
			{text: "shortcut   ◂ " + shortcutName(w.draft.Shortcut) + " ▸", selected: w.selected == 1},
			{},
			{text: "the prefix, then a key, runs a noct command; the shortcut held with that key runs it", muted: true},
			{text: "directly. ctrl+alt keeps alt+letter for the shell's word moves.", muted: true},
		}
	case setupThemes:
		return a.setupThemeLines()
	case setupAnimations:
		descriptions := map[string]string{
			animationsAll:  "every moment, as noct comes",
			animationsFew:  "hover, popups and activity only",
			animationsNone: "nothing moves",
		}
		var lines []setupLine
		for i, set := range animationSets {
			lines = append(lines, setupLine{text: fmt.Sprintf("%-6s %s", set, descriptions[set]), selected: w.selected == i})
		}
		return lines
	case setupDemo:
		return []setupLine{
			{text: mark(w.demo) + " install the demo", selected: true},
			{},
			{text: "scripts demo/* and the flows demo-deploy and demo-loops: a deploy with a health", muted: true},
			{text: "check and a rollback, made up, to learn flows by running them. start with", muted: true},
			{text: "demo/start-here in the scripts panel.", muted: true},
		}
	case setupMCP:
		return a.setupMCPLines()
	case setupBinds:
		return a.setupBindLines()
	}
	return a.setupSummaryLines()
}

func (a *App) setupStartLines() []setupLine {
	w := &a.setup
	lines := []setupLine{{text: "welcome to noct. set it up now, or later with noct setup.", muted: true}, {}}
	for i, choice := range w.startChoices() {
		lines = append(lines, setupLine{text: choice, selected: w.selected == i})
	}
	return lines
}

func (a *App) setupThemeLines() []setupLine {
	w := &a.setup
	lines := []setupLine{{text: fmt.Sprintf("mode %s (m)   background %s (b)", w.draft.Appearance.Mode, w.draft.Appearance.Background), muted: true, pinned: true}}
	for i, name := range w.themes {
		active := " "
		if name == w.draft.Appearance.Theme {
			active = "●"
		}
		lines = append(lines, setupLine{text: fmt.Sprintf("%s %s %-10s", mark(w.installed[name]), active, name), selected: w.selected == i, theme: name})
	}
	return lines
}

func (a *App) setupMCPLines() []setupLine {
	w := &a.setup
	if len(w.mcp) == 0 {
		return []setupLine{{text: "no AI client with a command line found (claude, codex, gemini, code).", muted: true}, {text: a.keysFor("y") + " shows how to add noct to others.", muted: true}}
	}
	var lines []setupLine
	for i, s := range w.mcp {
		lines = append(lines, setupLine{text: fmt.Sprintf("%s %-12s %s", mark(w.mcpMarked[i]), s.client, s.text), selected: w.selected == i})
	}
	return append(lines, setupLine{}, setupLine{text: "the ones marked run in a pane once the setup ends.", muted: true})
}

func (a *App) setupBindLines() []setupLine {
	w := &a.setup
	var lines []setupLine
	for _, b := range w.draft.CustomBinds {
		kind, what, _ := b.Target()
		lines = append(lines, setupLine{text: fmt.Sprintf("%-12s %-10s %s %s", prettyKey(b.Key), customBindName(b), kind, what)})
	}
	if len(lines) == 0 {
		lines = append(lines, setupLine{text: "no shortcuts of your own yet.", muted: true})
	}
	create := "+ create new shortcut"
	if draft := a.configPanel.bindCapture.custom; draft != nil {
		create += "  [" + a.customBindDraftValue(draft) + "]"
	}
	lines = append(lines, setupLine{}, setupLine{text: create, selected: w.selected == setupNewShortcut})
	if c := a.configPanel.bindCapture; c.custom != nil {
		if c.problem != "" {
			lines = append(lines, setupLine{text: "  " + c.problem, muted: true})
		}
		lines = append(lines, setupLine{text: "  " + customBindDraftNote(c.custom), muted: true})
	}
	return append(lines,
		setupLine{text: "continue", selected: w.selected == setupBindsNext},
		setupLine{},
		setupLine{text: "a shortcut is a key that runs a command, a script or a flow; the pane of a", muted: true},
		setupLine{text: "command or script closes when it ends well. more on the config's binds page.", muted: true})
}

func (a *App) setupSummaryLines() []setupLine {
	w := &a.setup
	themes := "all"
	if n := countMarked(w.themes, w.installed); n < len(w.themes) {
		themes = fmt.Sprintf("%d of %d", n, len(w.themes))
	}
	lines := []setupLine{
		{text: "prefix " + prefixName(w.draft.Prefix) + ", shortcut " + shortcutName(w.draft.Shortcut)},
		{text: "theme " + w.draft.Appearance.Theme + " (" + w.draft.Appearance.Mode + "), themes offered: " + themes},
		{text: "animations " + w.animation},
		{text: "demo " + map[bool]string{true: "installed", false: "not installed"}[w.demo]},
	}
	for i, s := range w.mcp {
		if w.mcpMarked[i] {
			lines = append(lines, setupLine{text: "add noct to " + s.client})
		}
	}
	for _, b := range w.draft.CustomBinds {
		if !slices.Contains(a.config.CustomBinds, b) {
			lines = append(lines, setupLine{text: "shortcut " + prettyKey(b.Key) + " runs " + customBindName(b)})
		}
	}
	return append(lines, setupLine{}, setupLine{text: "enter saves all of it.", muted: true})
}

func countMarked(names []string, marked map[string]bool) int {
	n := 0
	for _, name := range names {
		if marked[name] {
			n++
		}
	}
	return n
}

func mark(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func (a *App) setupHints() string {
	switch a.setup.step {
	case setupStart:
		return "↑↓ select  enter choose  esc close"
	case setupKeys:
		return "↑↓ select  ←→ change  enter next  s skip  esc back"
	case setupThemes:
		return "↑↓ select  space offer  a all or none  enter use it  m mode  b background  s skip  esc back"
	case setupDemo:
		return "space on or off  enter next  s skip  esc back"
	case setupMCP:
		return "↑↓ select  space mark  enter next  s skip  esc back"
	case setupBinds:
		if a.configPanel.bindCapture.custom != nil {
			return "esc cancel"
		}
		return "↑↓ select  enter choose  s skip  esc back"
	case setupSummary:
		return "enter save  esc back"
	}
	return "↑↓ select  enter next  s skip  esc back"
}
