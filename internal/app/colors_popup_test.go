package app

import (
	"math"
	"reflect"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/theme"
)

var colorsKey = uv.KeyPressEvent{Code: 'p', Mod: uv.ModShift, Text: "P"}

func openColorsOn(t *testing.T) (*App, *tab) {
	t.Helper()
	a, tb := colorsApp(t)
	press(a, prefixPress, colorsKey)
	if a.mode != colorsMode {
		t.Fatalf("prefix + P left mode %v, want the colors popup", a.mode)
	}
	return a, tb
}

// selectColorRow moves the popup's selection to the row of c.
func selectColorRow(t *testing.T, a *App, c theme.Component, forTab bool) {
	t.Helper()
	for i, row := range a.colors.rows {
		if !row.heading && row.component == c && row.forTab == forTab {
			a.colors.selected = i
			return
		}
	}
	t.Fatalf("no row for %s", theme.Components[c].Label)
}

func TestColorsOpenForTheFocusedPaneAndItsTab(t *testing.T) {
	a, tb := openColorsOn(t)
	if a.colors.tab != tb || a.colors.pane != tb.focused {
		t.Errorf("the popup is on pane %d of %p, want %d of %p", a.colors.pane, a.colors.tab, tb.focused, tb)
	}
	text := screenText(a)
	for _, want := range []string{"focused border", "unfocused title", "background", "inactive tab background"} {
		if !strings.Contains(text, want) {
			t.Errorf("the popup does not list %q", want)
		}
	}
}

func TestColorsNeedATab(t *testing.T) {
	a := testApp(t)
	press(a, prefixPress, colorsKey)
	if a.mode == colorsMode {
		t.Error("the colors popup opened on the home screen")
	}
}

func TestPickingAPaneColorKeepsIt(t *testing.T) {
	a, tb := openColorsOn(t)
	selectColorRow(t, a, theme.FocusedBorder, false)
	press(a, enterPress)
	if a.colors.picker == nil {
		t.Fatal("enter did not open the picker")
	}
	a.colors.picker.cell, a.colors.picker.moved = 196, true
	press(a, enterPress)
	if got := tb.paneColors[tb.focused][theme.FocusedBorder]; got != "#ff0000" {
		t.Errorf("the pane's focused border is %q, want #ff0000", got)
	}
	if a.colors.picker != nil || a.mode != colorsMode {
		t.Error("the popup did not go back to its list")
	}
}

func TestPickingATabColorKeepsIt(t *testing.T) {
	a, tb := openColorsOn(t)
	selectColorRow(t, a, theme.ActiveTabText, true)
	press(a, enterPress)
	a.colors.picker.cell, a.colors.picker.moved = 46, true
	press(a, enterPress)
	if got := tb.colors[theme.ActiveTabText]; got != "#00ff00" {
		t.Errorf("the tab's active name is %q, want #00ff00", got)
	}
}

func TestThemeInThePickerTakesTheColorAway(t *testing.T) {
	a, tb := openColorsOn(t)
	tb.paneColors[tb.focused] = colorSet{theme.FocusedBorder: "#ff0000"}
	selectColorRow(t, a, theme.FocusedBorder, false)
	press(a, enterPress, letter('t'))
	if _, ok := tb.paneColors[tb.focused][theme.FocusedBorder]; ok {
		t.Error("t kept the pane's own color")
	}
}

func TestLowContrastIsWarnedButAllowed(t *testing.T) {
	a, tb := openColorsOn(t)
	tb.paneColors[tb.focused] = colorSet{theme.Background: "#000000", theme.Text: "#111111"}
	if w := a.colorsWarning(tb, tb.focused); !strings.HasPrefix(w, "low contrast") {
		t.Errorf("warning %q, want low contrast", w)
	}
	if !strings.Contains(screenText(a), "low contrast") {
		t.Error("the popup does not show the warning")
	}
	tb.paneColors[tb.focused] = colorSet{theme.Background: "#000000", theme.Text: "#ffffff"}
	if w := a.colorsWarning(tb, tb.focused); w != "" {
		t.Errorf("white on black warns %q", w)
	}
}

func TestContrastOfBlackOnWhiteIs21(t *testing.T) {
	if c := contrast(colorOf("#ffffff"), colorOf("#000000")); math.Abs(c-21) > 0.01 {
		t.Errorf("contrast %v, want 21", c)
	}
	if c := contrast(colorOf("#777777"), colorOf("#777777")); math.Abs(c-1) > 0.01 {
		t.Errorf("contrast of a color with itself %v, want 1", c)
	}
}

func TestAltShiftLetterRunsTheUppercaseCommand(t *testing.T) {
	a, _ := colorsApp(t)
	a.shortcut = shortcutFromConfig(config.Shortcut{Alt: true})
	press(a, uv.KeyPressEvent{Code: 'p', Mod: uv.ModAlt | uv.ModShift})
	if a.mode != colorsMode {
		t.Errorf("alt+shift+p left mode %v, want the colors popup", a.mode)
	}
}

func TestAltShiftHjklStillMoveThePane(t *testing.T) {
	a, tb := colorsApp(t)
	a.shortcut = shortcutFromConfig(config.Shortcut{Alt: true})
	before := tb.tree.Shape()
	press(a, uv.KeyPressEvent{Code: 'l', Mod: uv.ModAlt | uv.ModShift})
	if reflect.DeepEqual(before, a.currentTab().tree.Shape()) || a.mode == colorsMode {
		t.Error("alt+shift+l did not move the focused pane")
	}
}

func TestColorsPopupShowsTheStageWithTheCandidate(t *testing.T) {
	a, _ := openColorsOn(t)
	selectColorRow(t, a, theme.FocusedBorder, false)
	press(a, enterPress)
	a.colors.picker.cell, a.colors.picker.moved = 196, true
	pane, _ := a.colorsPreview()
	if pane[theme.FocusedBorder] != "#ff0000" {
		t.Errorf("the stage shows %v, want the color under the selection", pane)
	}
	screenText(a) // draws with the candidate
	press(a, escPress)
	if _, ok := a.colors.tab.paneColors[a.colors.pane][theme.FocusedBorder]; ok {
		t.Error("esc in the picker kept the color")
	}
}

func TestOnlyThePanesOwnColorsAreWarnedAbout(t *testing.T) {
	a, tb := openColorsOn(t)
	if w := a.colorsWarning(tb, tb.focused); w != "" {
		t.Errorf("the theme alone warns %q", w)
	}
	tb.paneColors[tb.focused] = colorSet{theme.FocusedTitle: "#ffffff"}
	if w := a.colorsWarning(tb, tb.focused); w != "" {
		t.Errorf("a readable title warns %q, about the theme's own border", w)
	}
	tb.paneColors[tb.focused] = colorSet{theme.UnfocusedBorder: "#1b1c27"}
	if w := a.colorsWarning(tb, tb.focused); !strings.HasPrefix(w, "low contrast") {
		t.Errorf("a border almost the background warns %q", w)
	}
}

func TestAltShiftLettersThatAreNoCommandReachTheProgram(t *testing.T) {
	a, tb := colorsApp(t)
	a.shortcut = shortcutFromConfig(config.Shortcut{Alt: true})
	key := uv.KeyPressEvent{Code: 'z', Mod: uv.ModAlt | uv.ModShift}
	press(a, key)
	got := tb.focusedView().(*cellView).keys
	if a.mode != normalMode || len(got) != 1 || got[0].Code != 'z' || got[0].Mod != key.Mod {
		t.Errorf("alt+shift+z gave mode %v and the program %v", a.mode, got)
	}
}
