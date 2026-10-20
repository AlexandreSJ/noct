package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/theme"
)

// pickerResult records what a picker handed back.
type pickerResult struct {
	color theme.Color
	ok    bool
	calls int
}

func testPicker(current theme.Color, allowTheme bool) (*colorPicker, *pickerResult) {
	r := &pickerResult{}
	p := newColorPicker(current, allowTheme, func(c theme.Color, ok bool) {
		r.color, r.ok = c, ok
		r.calls++
	})
	return p, r
}

func pressPicker(p *colorPicker, keys ...string) {
	for _, k := range keys {
		event, _ := keyEvent(k)
		p.handleKey(event)
	}
}

func TestThePickerStartsOnTheCurrentColor(t *testing.T) {
	for current, want := range map[theme.Color]uint8{"60": 60, "#ffffff": 231, "": 0, "#5f5f87": 60} {
		if p, _ := testPicker(current, false); p.cell != want {
			t.Errorf("picker for %q starts on %d, want %d", current, p.cell, want)
		}
	}
}

func TestThePickerMovesOnTheGrid(t *testing.T) {
	p, _ := testPicker("0", false)
	steps := []struct {
		key  string
		want uint8
	}{{"h", 0}, {"k", 0}, {"l", 1}, {"j", 17}, {"h", 16}, {"k", 0}, {"right", 1}, {"down", 17}}
	for _, s := range steps {
		pressPicker(p, s.key)
		if p.cell != s.want {
			t.Fatalf("after %s the picker is on %d, want %d", s.key, p.cell, s.want)
		}
	}
	p.cell = 255
	pressPicker(p, "l", "j")
	if p.cell != 255 {
		t.Errorf("the picker left the grid's last cell for %d", p.cell)
	}
}

func TestEnterTakesTheColorAsTheThemeStoresIt(t *testing.T) {
	p, r := testPicker("3", false)
	pressPicker(p, "enter")
	if r.color != "3" || !r.ok {
		t.Errorf("enter on 3 gave %q, %v", r.color, r.ok)
	}
	p, r = testPicker("60", false)
	pressPicker(p, "enter")
	if r.color != "#5f5f87" || !r.ok {
		t.Errorf("enter on 60 gave %q, %v", r.color, r.ok)
	}
}

func TestHexIsTypedAfterHash(t *testing.T) {
	p, r := testPicker("0", false)
	pressPicker(p, "#", "f", "f", "8", "8", "0", "0", "enter")
	if r.color != "#ff8800" || !r.ok {
		t.Errorf("typed #ff8800, got %q, %v", r.color, r.ok)
	}
	p, r = testPicker("0", false)
	pressPicker(p, "#", "1", "2", "enter")
	if r.calls != 0 || p.problem == "" {
		t.Errorf("#12 was taken (%d calls) or not told (%q)", r.calls, p.problem)
	}
}

func TestEscLeavesTheColorAsItWas(t *testing.T) {
	p, r := testPicker("0", false)
	pressPicker(p, "l", "esc")
	if r.ok || r.calls != 1 {
		t.Errorf("esc gave %q, ok %v, %d calls", r.color, r.ok, r.calls)
	}
}

func TestThemeIsOnlyThereWhenAllowed(t *testing.T) {
	p, r := testPicker("5", false)
	pressPicker(p, "t")
	if r.calls != 0 {
		t.Error("t took the theme's color where it is not allowed")
	}
	p, r = testPicker("5", true)
	pressPicker(p, "t")
	if r.color != "" || !r.ok {
		t.Errorf("t gave %q, %v; want the theme's", r.color, r.ok)
	}
}

// editingMine opens the editor on a user theme named mine.
func editingMine(t *testing.T) (*App, string) {
	t.Helper()
	a, dir := editorApp(t, editThemeRow)
	writeUserTheme(t, dir, "mine", "base = \"noct\"\n")
	a.loadThemes()
	a.useTheme("mine")
	a.openThemeEditor(a.themeNamed("mine"))
	return a, dir
}

func TestPickingSavesTheThemeFile(t *testing.T) {
	a, dir := editingMine(t)
	a.selectConfigRow(themeRowBase + configRow(theme.FocusedBorder))
	press(a, enterPress)
	if a.configPanel.edit.picker == nil {
		t.Fatal("enter did not open the picker")
	}
	a.configPanel.edit.picker.cell = 196
	press(a, enterPress)
	content, _ := os.ReadFile(filepath.Join(dir, "mine.toml"))
	if !strings.Contains(string(content), `focused_border = "#ff0000"`) {
		t.Errorf("mine.toml:\n%s", content)
	}
	if a.configPanel.edit.picker != nil || !colorsEqual(a.palette.focusedFrame.Fg, colorOf("#ff0000")) {
		t.Errorf("after picking, the picker is %v and the border %v", a.configPanel.edit.picker, a.palette.focusedFrame.Fg)
	}
}

func TestThePreviewFollowsTheSelection(t *testing.T) {
	a, dir := editingMine(t)
	a.size.Y = 40
	a.selectConfigRow(themeRowBase + configRow(theme.FocusedBorder))
	press(a, enterPress)
	a.configPanel.edit.picker.cell = 196
	if got := a.configPanel.edit.previewed().Colors[theme.FocusedBorder]; got != "#ff0000" {
		t.Errorf("the stage shows %q, want the color under the selection", got)
	}
	screenText(a)
	press(a, escPress)
	content, _ := os.ReadFile(filepath.Join(dir, "mine.toml"))
	if strings.Contains(string(content), "focused_border") {
		t.Errorf("esc wrote the color:\n%s", content)
	}
}

func TestAShortTerminalStillShowsTheHexField(t *testing.T) {
	a, _ := editingMine(t)
	a.size.Y = 20
	a.selectConfigRow(themeRowBase + configRow(theme.FocusedBorder))
	press(a, enterPress)
	a.configPanel.edit.picker.cell = 250 // the grid's last rows
	text := screenText(a)
	if !strings.Contains(text, "hex") || !strings.Contains(text, "250") {
		t.Errorf("a short picker hides the hex field or the selected color:\n%s", text)
	}
}
