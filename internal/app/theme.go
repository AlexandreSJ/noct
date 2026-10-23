package app

import (
	"errors"
	"image/color"
	"slices"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// themeList is every theme the config offers: the ones noct comes with, then the user's.
func (a *App) themeList() []theme.Theme {
	if len(a.themes) == 0 {
		return theme.Builtins()
	}
	return a.themes
}

// themeNamed is the theme with that name, or the default one.
func (a *App) themeNamed(name string) theme.Theme {
	if t, ok := theme.Find(a.themeList(), name); ok {
		return t
	}
	return a.themeList()[0]
}

// shownTheme is the theme with that name in the colors of the config's mode, as the screen
// shows it.
func (a *App) shownTheme(name string) theme.Theme {
	return a.themeNamed(name).In(a.config.Appearance.Mode == config.ModeLight)
}

func (a *App) nextThemeName(current string, step int) string {
	list := a.themeList()
	i := max(slices.IndexFunc(list, func(t theme.Theme) bool { return t.Name == current }), 0)
	return list[(i+step+len(list))%len(list)].Name
}

// loadThemes reads the user's themes again. A theme file that cannot be read is left out and
// told in the tab bar, once until the problem changes.
func (a *App) loadThemes() {
	dir, err := theme.Dir()
	if err != nil {
		a.themes = theme.Builtins()
		return
	}
	user, errs := theme.LoadDir(dir, theme.Builtins())
	a.themes = append(theme.Builtins(), user...)
	a.themesSignature = dirSignature([]string{dir}, 1)
	problem := ""
	if err := errors.Join(errs...); err != nil {
		problem = errs[0].Error()
	}
	if problem != "" && problem != a.themeProblem {
		a.statusMessage = problem
	}
	a.themeProblem = problem
}

// checkThemes reads the user's themes again when their folder changed, telling whether it did.
func (a *App) checkThemes() bool {
	dir, err := theme.Dir()
	if err != nil || dirSignature([]string{dir}, 1) == a.themesSignature {
		return false
	}
	a.loadThemes()
	a.showScreenTheme()
	return true
}

// showScreenTheme draws with the theme the screen should show now: the selected session's
// while the sessions popup or the search is on one, else the session on screen's.
func (a *App) showScreenTheme() {
	f := &a.finder
	switch {
	case a.mode == sessionsMode && len(a.sessionRows) > 0:
		a.previewTheme(a.sessionTheme(a.sessionRows[a.sessionSelected]))
	case a.mode == finderMode && len(f.matches) > 0 && f.matches[f.selected].theme != "":
		a.previewTheme(f.matches[f.selected].theme)
	default:
		a.applyPalette()
	}
}

// applyPalette draws with the theme of the session on screen from now on.
func (a *App) applyPalette() {
	a.previewTheme(a.sessionTheme(a.session))
}

// previewTheme draws with the theme named, saving nothing. The palette is replaced in place,
// since some views keep a pointer to it.
func (a *App) previewTheme(name string) {
	a.palette = newPalette(a.shownTheme(name), a.config.Appearance.Background == config.BackgroundTheme)
}

// sessionTheme is the theme s draws with: its own while it exists, else the config's.
func (a *App) sessionTheme(s *session) string {
	if s != nil && s.theme != "" {
		if _, ok := theme.Find(a.themeList(), s.theme); ok {
			return s.theme
		}
	}
	return a.config.Appearance.Theme
}

// palette is every style the workspace draws with, for one theme and background choice.
type palette struct {
	// background and text are nil when the terminal's own colors are kept.
	background color.Color
	text       color.Color
	// popupBackground fills the panels noct draws over the workspace.
	popupBackground color.Color

	activeTab      uv.Style
	tab            uv.Style
	focusedFrame   uv.Style
	unfocusedFrame uv.Style
	focusedTitle   uv.Style
	unfocusedTitle uv.Style
	status         uv.Style
	success        uv.Style
	failure        uv.Style
	panelText      uv.Style
	panelMuted     uv.Style
	panelHeading   uv.Style
	logo           uv.Style
	selectedRow    uv.Style

	// heat colors the calendar's days by their logs, from a few to the most.
	heat [heatLevels]color.Color
}

// heatLevels are the steps of the calendar's colors, up to the busiest day.
const heatLevels = 4

func newPalette(t theme.Theme, paintBackground bool) palette {
	var background, text color.Color
	if paintBackground {
		background, text = t.Colors[theme.Background].Resolve(), t.Colors[theme.Text].Resolve()
	}
	// orBackground is a component's color, or the background when the theme leaves it to it.
	orBackground := func(c theme.Component) color.Color {
		if t.Colors[c] == "" {
			return background
		}
		return t.Colors[c].Resolve()
	}
	plain := func(c theme.Component, attrs uint8) uv.Style {
		return uv.Style{Fg: t.Colors[c].Resolve(), Bg: background, Attrs: attrs}
	}
	themeBackground, calendar := t.Colors[theme.Background].Resolve(), t.Colors[theme.Calendar].Resolve()
	return palette{
		background:      background,
		text:            text,
		popupBackground: orBackground(theme.PopupBackground),
		activeTab:       uv.Style{Fg: t.Colors[theme.ActiveTabText].Resolve(), Bg: orBackground(theme.ActiveTabBackground), Attrs: uv.AttrBold},
		tab:             uv.Style{Fg: t.Colors[theme.InactiveTabText].Resolve(), Bg: orBackground(theme.InactiveTabBackground)},
		focusedFrame:    plain(theme.FocusedBorder, 0),
		unfocusedFrame:  plain(theme.UnfocusedBorder, 0),
		focusedTitle:    plain(theme.FocusedTitle, uv.AttrBold),
		unfocusedTitle:  plain(theme.UnfocusedTitle, 0),
		status:          plain(theme.Attention, 0),
		success:         plain(theme.Success, 0),
		failure:         plain(theme.Error, 0),
		panelText:       plain(theme.PanelText, 0),
		panelMuted:      plain(theme.PanelMuted, 0),
		panelHeading:    plain(theme.PanelHeading, uv.AttrBold),
		logo:            plain(theme.Logo, 0),
		selectedRow:     uv.Style{Fg: t.Colors[theme.SelectedText].Resolve(), Bg: orBackground(theme.SelectedBackground), Attrs: uv.AttrBold},
		heat: [heatLevels]color.Color{
			mix(themeBackground, calendar, 0.3),
			mix(themeBackground, calendar, 0.55),
			mix(themeBackground, calendar, 0.8),
			calendar,
		},
	}
}

// mix is the color at fraction of the way from a to b.
func mix(a, b color.Color, fraction float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	channel := func(x, y uint32) uint8 {
		return uint8((float64(x) + (float64(y)-float64(x))*fraction) / 0x101)
	}
	return color.RGBA{R: channel(ar, br), G: channel(ag, bg), B: channel(ab, bb), A: 0xff}
}
