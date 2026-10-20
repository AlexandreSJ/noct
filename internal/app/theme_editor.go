package app

import (
	"fmt"
	"image"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/theme"
)

// themeRowBase is the first row of the theme editor: one row per component, in their order.
const themeRowBase configRow = 2000

// stageBlink is how long a component stays lit on the stage after the editor lands on it.
const stageBlink = 400 * time.Millisecond

// themeListWidth is the width of the editor's list of components; the stage takes the rest.
const themeListWidth = 34

const themeEditorHints = "{up}{down} select  {enter} pick a color  esc back"

// groupTitles name the groups of the theme file in the editor.
var groupTitles = map[string]string{
	"background": "Background", "tabs": "Tabs", "panels": "Panels",
	"noct": "noct's panels", "state": "State", "other": "Other",
}

// themeEdit is the theme being edited and the base its file is written against.
type themeEdit struct {
	theme, base theme.Theme
	selected    theme.Component
	// blinkAt is when the selected component was lit on the stage.
	blinkAt time.Time
}

func themeRowAt(row configRow) (theme.Component, bool) {
	c := theme.Component(row - themeRowBase)
	return c, row >= themeRowBase && c < theme.Count
}

// startNewTheme asks for a new theme's name, then the theme it starts from.
func (a *App) startNewTheme() {
	a.askThemeName("new theme", "", func(name string) {
		var items []finderItem
		for _, t := range a.themeList() {
			kind := "yours"
			if t.Builtin {
				kind = "built in"
			}
			items = append(items, finderItem{label: t.Name, kind: kind, theme: t.Name, open: func() { a.createTheme(name, t) }})
		}
		a.openFinderWith("start "+name+" from", items)
		a.finder.cancel = func() { a.mode = configMode }
	})
}

// editCurrentTheme edits the session's theme, or a copy of it when noct comes with it.
func (a *App) editCurrentTheme() {
	current := a.themeNamed(a.sessionTheme(a.session))
	if !current.Builtin {
		a.openThemeEditor(current)
		return
	}
	a.askThemeName("copy "+current.Name+" as", "", func(name string) { a.createTheme(name, current) })
}

// askThemeName asks for a theme name until it is one a new theme can take.
func (a *App) askThemeName(title, initial string, then func(name string)) {
	a.openPrompt(title, initial, "{enter} next  esc cancel", func(name string) {
		if err := a.checkNewThemeName(name); err != nil {
			a.statusMessage = err.Error()
			a.askThemeName(title, name, then)
			return
		}
		then(name)
	})
	a.prompt.cancel = func() { a.mode = configMode }
}

func (a *App) checkNewThemeName(name string) error {
	if err := theme.CheckName(name); err != nil {
		return err
	}
	if _, ok := theme.Find(a.themeList(), name); ok {
		return fmt.Errorf("there is a theme %s already", name)
	}
	return nil
}

// createTheme writes a theme named name with base's colors, puts it on the session on screen
// and opens it in the editor.
func (a *App) createTheme(name string, base theme.Theme) {
	t := base
	t.Name, t.Base, t.Builtin = name, base.Name, false
	if err := a.saveTheme(t, base); err != nil {
		a.statusMessage = err.Error()
		a.mode = configMode
		return
	}
	a.useTheme(name)
	a.openThemeEditor(a.themeNamed(name))
}

func (a *App) saveTheme(t, base theme.Theme) error {
	dir, err := theme.Dir()
	if err == nil {
		err = theme.Save(dir, t, base)
	}
	if err != nil {
		return err
	}
	a.loadThemes()
	return nil
}

// useTheme puts the theme on the session on screen, as the config's theme row does.
func (a *App) useTheme(name string) {
	next := a.config
	next.Appearance.Theme = name
	a.session.theme = name
	a.applyConfig(next)
}

// openThemeEditor shows the editor's page on t's first component.
func (a *App) openThemeEditor(t theme.Theme) {
	a.commitTyped()
	a.configPanel.edit = &themeEdit{theme: t, base: a.themeNamed(t.Base)}
	a.configPanel.page, a.configPanel.selected = themePage, themeRowBase
	a.mode = configMode
}

func (a *App) closeThemeEditor() {
	a.configPanel.page, a.configPanel.selected, a.configPanel.edit = settingsPage, editThemeRow, nil
}

func (a *App) handleThemeEditorKey(key uv.KeyPressEvent) {
	switch key.String() {
	case "esc":
		a.closeThemeEditor()
	case "up", "k":
		a.moveConfigSelection(-1)
	case "down", "j":
		a.moveConfigSelection(1)
	case "pgup", "pgdown", "home", "end":
		a.jumpConfigSelection(key.String())
	}
}

// lightComponent shows the component selected on the stage for a moment.
func (a *App) lightComponent(c theme.Component) {
	edit := a.configPanel.edit
	edit.selected, edit.blinkAt = c, time.Now()
	a.stageTick = time.After(stageBlink)
}

// themeEditorLines are the components by group, each with its color.
func (a *App) themeEditorLines() []configLine {
	var lines []configLine
	group := ""
	for c, info := range theme.Components {
		if info.Group != group {
			group = info.Group
			lines = append(lines, configLine{label: groupTitles[group], heading: true})
		}
		lines = append(lines, configLine{label: info.Label, value: swatchValue(a.configPanel.edit.theme.Colors[c]), row: themeRowBase + configRow(c)})
	}
	return lines
}

// swatchValue is a color as the editor lists it: a swatch and the color, or the background.
func swatchValue(c theme.Color) string {
	if c == "" {
		return "background"
	}
	return "██ " + string(c)
}

// drawThemeEditor draws the list of components on the left and the stage, in the theme being
// edited, on the right.
func (a *App) drawThemeEditor(screen uv.Screen, l configLayout, inner image.Rectangle) {
	edit := a.configPanel.edit
	list := inner
	list.Max.X = min(inner.Min.X+themeListWidth, inner.Max.X)
	for i := l.first; i < len(l.rows) && i-l.first < l.visible(); i++ {
		y := inner.Min.Y + i - l.first
		line := l.rows[i]
		a.drawConfigLine(screen, list, y, line)
		if c, ok := themeRowAt(line.row); ok && edit.theme.Colors[c] != "" {
			x := list.Max.X - 1 - ansi.StringWidth(line.value)
			style := a.configLineStyle(line)
			style.Fg = edit.theme.Colors[c].Resolve()
			drawText(screen, x, y, list.Max.X, "██", style)
		}
	}
	for y := inner.Min.Y; y < inner.Max.Y-2; y++ {
		setRune(screen, list.Max.X, y, "│", a.palette.unfocusedFrame)
	}
	stage := image.Rect(list.Max.X+2, inner.Min.Y, inner.Max.X, inner.Max.Y-2)
	p := newPalette(edit.theme, true) // the stage shows the background whatever the screen keeps
	drawStage(screen, stage, &p, edit.selected, time.Since(edit.blinkAt) < stageBlink)
}
