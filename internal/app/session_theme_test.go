package app

import (
	"image/color"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/theme"
)

var backspacePress = uv.KeyPressEvent{Code: uv.KeyBackspace}

// themeColor is the focused border of the theme with that name, which tells themes apart.
func themeColor(name string) color.Color {
	t, _ := theme.Find(theme.Builtins(), name)
	return t.Colors[theme.FocusedBorder].Resolve()
}

func showsTheme(t *testing.T, a *App, name string) {
	t.Helper()
	if !colorsEqual(a.palette.focusedFrame.Fg, themeColor(name)) {
		t.Errorf("the screen shows %v, want %s's %v", a.palette.focusedFrame.Fg, name, themeColor(name))
	}
}

// twoSessions is an app with session 1 in blood on screen and session 2 in frost behind it.
func twoSessions(t *testing.T) (*App, *session, *session) {
	t.Helper()
	a := sessionsApp(t)
	first := a.session
	first.theme = "blood"
	second, err := a.createSession("2")
	if err != nil {
		t.Fatal(err)
	}
	second.theme = "frost"
	a.switchSession(first)
	return a, first, second
}

func TestEachSessionKeepsItsTheme(t *testing.T) {
	a, first, second := twoSessions(t)
	showsTheme(t, a, "blood")
	a.switchSession(second)
	showsTheme(t, a, "frost")
	a.openConfig()
	a.configPanel.selected = themeRow
	press(a, rightPress)
	if second.theme != "mono" || first.theme != "blood" {
		t.Errorf("themes %s and %s after changing the second's, want blood and mono", first.theme, second.theme)
	}
	a.switchSession(first)
	showsTheme(t, a, "blood")
	a.switchSession(second)
	showsTheme(t, a, "mono")
}

func TestANewSessionStartsWithTheThemeOfTheOneItCameFrom(t *testing.T) {
	a := sessionsApp(t)
	a.session.theme = "ember"
	s, err := a.createSession("x")
	if err != nil {
		t.Fatal(err)
	}
	if s.theme != "ember" {
		t.Errorf("new session in %q, want ember", s.theme)
	}
}

func TestSessionThemesAreSavedAndLoaded(t *testing.T) {
	a := sessionsApp(t)
	a.session.theme = "rose"
	if got := a.savedSession(a.session).Theme; got != "rose" {
		t.Fatalf("saved theme %q, want rose", got)
	}
	a.saveSessions()
	b := sessionsApp(t)
	b.sessionsPath = a.sessionsPath
	b.sessions, b.session = nil, nil
	b.loadSessions()
	if b.session.theme != "rose" {
		t.Errorf("loaded theme %q, want rose", b.session.theme)
	}
}

func TestASessionWhoseThemeIsGoneUsesTheConfigsTheme(t *testing.T) {
	a := sessionsApp(t)
	a.config.Appearance.Theme = "aurora"
	a.session.theme = "deleted"
	if got := a.sessionTheme(a.session); got != "aurora" {
		t.Errorf("theme %q, want the config's aurora", got)
	}
	a.applyPalette()
	showsTheme(t, a, "aurora")
}

func TestMovingThroughTheSessionsShowsEachOnesTheme(t *testing.T) {
	a, _, _ := twoSessions(t)
	a.openSessions()
	press(a, letter('j'))
	showsTheme(t, a, "frost")
	press(a, letter('k'))
	showsTheme(t, a, "blood")
	press(a, letter('j'), escPress)
	showsTheme(t, a, "blood")
}

func TestTheSearchShowsASessionsThemeOnIt(t *testing.T) {
	a, _, _ := twoSessions(t)
	a.openFinder()
	typeInto(a, "session 2")
	showsTheme(t, a, "frost")
	press(a, escPress)
	showsTheme(t, a, "blood")
	a.openFinder()
	typeInto(a, "session 2")
	for range 3 {
		press(a, backspacePress)
	}
	typeInto(a, "zzz")
	showsTheme(t, a, "blood")
}

func TestTheConfigThemeChangesTheSessionOnScreen(t *testing.T) {
	a := sessionsApp(t)
	a.openConfig()
	a.configPanel.selected = themeRow
	press(a, rightPress)
	if a.session.theme != "aurora" || a.config.Appearance.Theme != "aurora" {
		t.Errorf("session theme %q and config theme %q, want both aurora", a.session.theme, a.config.Appearance.Theme)
	}
}

func TestPastingInTheSearchShowsTheSessionsTheme(t *testing.T) {
	a, _, _ := twoSessions(t)
	a.openFinder()
	if err := a.handleEvent(uv.PasteEvent{Content: "session 2"}); err != nil {
		t.Fatal(err)
	}
	showsTheme(t, a, "frost")
}

func TestReadingTheThemesAgainKeepsTheSessionPreview(t *testing.T) {
	dir := themesFolder(t)
	a, _, _ := twoSessions(t)
	a.loadThemes()
	a.openSessions()
	press(a, letter('j'))
	writeUserTheme(t, dir, "mine", "base = \"rose\"\n")
	a.checkThemes()
	showsTheme(t, a, "frost")
	a.openFinder()
	typeInto(a, "session 2")
	writeUserTheme(t, dir, "other", "base = \"rose\"\n")
	a.checkThemes()
	showsTheme(t, a, "frost")
}

func TestModeRowShowsTheLightColorsOfTheTheme(t *testing.T) {
	a := openConfigAt(t, modeRow)
	a.applyConfig(withTheme(a.config, "blood"))
	a.session.theme = "blood"
	a.applyPalette()
	press(a, letter('l'))
	if a.config.Appearance.Mode != config.ModeLight {
		t.Fatalf("mode %q after the row, want light", a.config.Appearance.Mode)
	}
	blood, _ := theme.Find(theme.Builtins(), "blood")
	if want := blood.Light[theme.FocusedBorder].Resolve(); !colorsEqual(a.palette.focusedFrame.Fg, want) {
		t.Errorf("the screen shows %v, want blood's light %v", a.palette.focusedFrame.Fg, want)
	}
	press(a, letter('l'))
	if a.config.Appearance.Mode != config.ModeDark {
		t.Errorf("mode %q after the row again, want dark", a.config.Appearance.Mode)
	}
	showsTheme(t, a, "blood")
}
