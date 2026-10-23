package app

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// themesFolder points the user's themes at a new folder and returns it.
func themesFolder(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := theme.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeUserTheme(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func colorOf(c theme.Color) color.Color { return c.Resolve() }

func TestPaletteFollowsEachComponent(t *testing.T) {
	var th theme.Theme
	for c := range theme.Count {
		th.Colors[c] = theme.Indexed(uint8(20 + c))
	}
	p := newPalette(th, true)
	of := func(c theme.Component) color.Color { return colorOf(th.Colors[c]) }
	checks := []struct {
		name      string
		got, want color.Color
	}{
		{"active tab text", p.activeTab.Fg, of(theme.ActiveTabText)},
		{"active tab background", p.activeTab.Bg, of(theme.ActiveTabBackground)},
		{"inactive tab text", p.tab.Fg, of(theme.InactiveTabText)},
		{"inactive tab background", p.tab.Bg, of(theme.InactiveTabBackground)},
		{"focused border", p.focusedFrame.Fg, of(theme.FocusedBorder)},
		{"unfocused border", p.unfocusedFrame.Fg, of(theme.UnfocusedBorder)},
		{"focused title", p.focusedTitle.Fg, of(theme.FocusedTitle)},
		{"unfocused title", p.unfocusedTitle.Fg, of(theme.UnfocusedTitle)},
		{"panel text", p.panelText.Fg, of(theme.PanelText)},
		{"muted", p.panelMuted.Fg, of(theme.PanelMuted)},
		{"heading", p.panelHeading.Fg, of(theme.PanelHeading)},
		{"selected text", p.selectedRow.Fg, of(theme.SelectedText)},
		{"selected background", p.selectedRow.Bg, of(theme.SelectedBackground)},
		{"popup background", p.popupBackground, of(theme.PopupBackground)},
		{"message", p.status.Fg, of(theme.Attention)},
		{"success", p.success.Fg, of(theme.Success)},
		{"error", p.failure.Fg, of(theme.Error)},
		{"raven", p.logo.Fg, of(theme.Logo)},
		{"calendar", p.heat[heatLevels-1], of(theme.Calendar)},
		{"background", p.background, of(theme.Background)},
		{"text", p.text, of(theme.Text)},
	}
	for _, c := range checks {
		if !colorsEqual(c.got, c.want) {
			t.Errorf("%s is %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestEmptyComponentsKeepTheTerminalsBackground(t *testing.T) {
	noct := theme.Builtins()[0]
	p := newPalette(noct, false)
	if p.tab.Bg != nil || p.popupBackground != nil {
		t.Errorf("with the terminal's background, the inactive tab has %v and popups %v", p.tab.Bg, p.popupBackground)
	}
	p = newPalette(noct, true)
	want := colorOf("#1a1b26")
	if !colorsEqual(p.tab.Bg, want) || !colorsEqual(p.popupBackground, want) {
		t.Errorf("with the theme's background, the inactive tab has %v and popups %v", p.tab.Bg, p.popupBackground)
	}
}

func TestUserThemesJoinTheConfigCycle(t *testing.T) {
	dir := themesFolder(t)
	writeUserTheme(t, dir, "mine", "base = \"noct\"\n[panels]\nfocused_border = \"#ff0000\"\n")
	a := openConfigAt(t, themeRow)
	a.loadThemes()
	a.applyConfig(withTheme(a.config, "sakura"))
	press(a, letter('l'))
	if a.config.Appearance.Theme != "mine" {
		t.Fatalf("after sakura comes %s, want mine", a.config.Appearance.Theme)
	}
	if !colorsEqual(a.palette.focusedFrame.Fg, colorOf("#ff0000")) {
		t.Errorf("focused border %v, want mine's red", a.palette.focusedFrame.Fg)
	}
}

func withTheme(c config.Config, name string) config.Config {
	c.Appearance.Theme = name
	return c
}

func TestABrokenThemeFileIsReportedAndSkipped(t *testing.T) {
	dir := themesFolder(t)
	writeUserTheme(t, dir, "bad", "[panels]\nfocused_border = \"#12345\"\n")
	writeUserTheme(t, dir, "mine", "")
	a := testApp(t)
	a.loadThemes()
	if !strings.Contains(a.statusMessage, "bad.toml") {
		t.Errorf("message %q does not name bad.toml", a.statusMessage)
	}
	if a.themeNamed("mine").Name != "mine" || a.themeNamed("bad").Name != "noct" {
		t.Errorf("mine is %s and bad is %s", a.themeNamed("mine").Name, a.themeNamed("bad").Name)
	}
}

func TestEditingAThemeFileShowsAtOnce(t *testing.T) {
	dir := themesFolder(t)
	writeUserTheme(t, dir, "mine", "[panels]\nfocused_border = \"#ff0000\"\n")
	a := testApp(t)
	a.loadThemes()
	a.config.Appearance.Theme = "mine"
	a.applyPalette()
	if a.checkThemes() {
		t.Error("checkThemes changed with nothing new")
	}
	writeUserTheme(t, dir, "mine", "[panels]\nfocused_border = \"#00ff00\"\n")
	if !a.checkThemes() {
		t.Fatal("checkThemes missed the change")
	}
	if !colorsEqual(a.palette.focusedFrame.Fg, colorOf("#00ff00")) {
		t.Errorf("focused border %v, want the new green", a.palette.focusedFrame.Fg)
	}
}

func TestDeletingTheThemeInUseFallsBackToTheDefault(t *testing.T) {
	dir := themesFolder(t)
	writeUserTheme(t, dir, "mine", "[panels]\nfocused_border = \"#ff0000\"\n")
	a := testApp(t)
	a.loadThemes()
	a.config.Appearance.Theme = "mine"
	a.applyPalette()
	if err := os.Remove(filepath.Join(dir, "mine.toml")); err != nil {
		t.Fatal(err)
	}
	a.checkThemes()
	if !colorsEqual(a.palette.focusedFrame.Fg, colorOf("#9d7cd8")) {
		t.Errorf("focused border %v, want noct's", a.palette.focusedFrame.Fg)
	}
}

func TestRunsShowSuccessAndErrorInTheirColors(t *testing.T) {
	a := testApp(t)
	a.palette = newPalette(theme.Builtins()[0], false)
	start := func(pty.Size, func(int)) (view, error) { return &fakeView{}, nil }
	v, err := newRunView("run", &a.palette, start, func() {}, pty.Size{Cols: 80, Rows: 24}, "")
	if err != nil {
		t.Fatal(err)
	}
	v.finished(v.run, 0)
	if _, style := v.status(); style != a.palette.success {
		t.Errorf("a run that ended well shows in %v", style)
	}
	v.finished(v.run, 1)
	if _, style := v.status(); style != a.palette.failure {
		t.Errorf("a failed run shows in %v", style)
	}
}
