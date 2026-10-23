package app

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

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
	emptyName := ""
	if allowTheme {
		emptyName = "the theme's"
	}
	p := newColorPicker(current, emptyName, func(c theme.Color, ok bool) {
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
	}{
		{"h", 0}, {"k", 0}, {"l", 1}, {"j", 9}, {"h", 8}, {"k", 0}, {"right", 1}, {"down", 9},
		{"j", 196}, // from the bright colors to the triangle's red corner
		{"j", 166}, // the line under it, half a cell to the left
		{"l", 161},
		{"k", 196},
	}
	for _, s := range steps {
		pressPicker(p, s.key)
		if p.cell != s.want {
			t.Fatalf("after %s the picker is on %d, want %d", s.key, p.cell, s.want)
		}
	}
	p.cell = 255
	pressPicker(p, "l", "j")
	if p.cell != 255 {
		t.Errorf("the picker left the grays' last cell for %d", p.cell)
	}
}

func TestUpAndDownGoStraightThroughTheTriangle(t *testing.T) {
	p, _ := testPicker("196", false)
	var path []uint8
	for range 6 {
		pressPicker(p, "j")
		path = append(path, p.cell)
	}
	want := []uint8{166, 131, 101, 66, 36, 240} // the grays under the column
	if !slices.Equal(path, want) {
		t.Errorf("down from red went through %v, want %v", path, want)
	}
	cells := swatchCells(p.slice)
	for _, n := range path[:5] {
		if x := cells[n].X; x != 15 && x != 16 {
			t.Errorf("%d is at column %d, off the straight line", n, x)
		}
	}
}

func TestTheSlicesHoldTheCubeOnce(t *testing.T) {
	seen := map[uint8]int{}
	for slice := range cubeSlices {
		cells := swatchCells(slice)
		taken := map[image.Point]uint8{}
		for n, at := range cells {
			if at.Y >= swatchLines || at.X+2 > 39 {
				t.Errorf("%d is at %v in slice %d, out of the picker", n, at, slice)
			}
			if other, ok := taken[at]; ok {
				t.Errorf("%d and %d share %v in slice %d", other, n, at, slice)
			}
			taken[at] = n
			if inCube(n) {
				seen[n]++
			}
		}
	}
	for n := 16; n < 232; n++ {
		if seen[uint8(n)] != 1 {
			t.Errorf("%d is in %d slices", n, seen[uint8(n)])
		}
	}
	corners := swatchCells(5)
	if corners[196].Y != cubeTop || corners[46].Y != cubeTop+5 || corners[21].Y != cubeTop+5 || corners[46].X > corners[21].X {
		t.Errorf("slice 5 is not red on top, green and blue at the bottom: %v %v %v", corners[196], corners[46], corners[21])
	}
}

func TestCommaAndPeriodChangeTheSlice(t *testing.T) {
	p, _ := testPicker("196", false)
	if p.slice != 5 {
		t.Fatalf("the picker opened on 196 shows slice %d", p.slice)
	}
	pressPicker(p, ".")
	if red, _, _ := cubeLevels(p.cell); p.slice != 6 || red != 5 {
		t.Errorf("a lighter slice went to %d in slice %d, want a red in 6", p.cell, p.slice)
	}
	if !p.moved {
		t.Error("a new slice did not move the selection")
	}
	p, _ = testPicker("0", false)
	pressPicker(p, ",")
	if p.cell != 0 || p.slice != 4 {
		t.Errorf("a new slice moved the system color to %d, slice %d", p.cell, p.slice)
	}
	p, _ = testPicker("16", false)
	pressPicker(p, ",", ".")
	if p.slice != 1 || p.cell == 16 {
		t.Errorf("from black, the next slice is %d with %d", p.slice, p.cell)
	}
}

func TestThePickerDrawsTheSelectionWhereItsCellIs(t *testing.T) {
	pal := newPalette(theme.Builtins()[0], false)
	for _, n := range []uint8{0, 15, 16, 21, 52, 124, 231, 232, 255} {
		p, _ := testPicker(theme.Color(strconv.Itoa(int(n))), false)
		buf := uv.NewScreenBuffer(40, swatchLines+pickerRows)
		p.draw(buf, image.Rect(0, 0, 40, swatchLines+pickerRows), &pal)
		at := swatchCells(p.slice)[n]
		if c := buf.CellAt(at.X, at.Y); c == nil || c.Content != "[" {
			t.Errorf("the selection of %d is not at %v", n, at)
		}
		if c := buf.CellAt(at.X, at.Y); c != nil && !colorsEqual(c.Style.Bg, theme.Indexed(n).Resolve()) {
			t.Errorf("the cell of %d has the background %v", n, c.Style.Bg)
		}
	}
}

func TestEnterTakesTheColorAsTheThemeStoresIt(t *testing.T) {
	p, r := testPicker("3", false)
	pressPicker(p, "enter")
	if r.color != "3" || !r.ok {
		t.Errorf("enter on 3 gave %q, %v", r.color, r.ok)
	}
	p, r = testPicker("59", false)
	pressPicker(p, "l", "enter")
	if r.color != "#5f0087" || !r.ok {
		t.Errorf("enter on 54 gave %q, %v", r.color, r.ok)
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
	a.configPanel.edit.picker.cell, a.configPanel.edit.picker.moved = 196, true
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
	a.configPanel.edit.picker.cell, a.configPanel.edit.picker.moved = 196, true
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
	a.configPanel.edit.picker.cell, a.configPanel.edit.picker.moved = 250, true // the grid's last rows
	text := screenText(a)
	if !strings.Contains(text, "hex") || !strings.Contains(text, "250") {
		t.Errorf("a short picker hides the hex field or the selected color:\n%s", text)
	}
}

func TestThePickerFitsInItsPanels(t *testing.T) {
	a, _ := editingMine(t)
	a.size.Y = 40
	a.selectConfigRow(themeRowBase + configRow(theme.FocusedBorder))
	press(a, enterPress)
	b, _ := openColorsOn(t)
	b.size.Y = 40
	selectColorRow(t, b, theme.FocusedBorder, false)
	press(b, enterPress)
	for name, open := range map[string]struct {
		app    *App
		picker *colorPicker
	}{"theme editor": {a, a.configPanel.edit.picker}, "panel colors": {b, b.colors.picker}} {
		open.picker.cell, open.picker.moved = 255, true // the last line of cells
		text := screenText(open.app)
		if open.picker.first != 0 {
			t.Errorf("the %s scrolls the picker by %d lines", name, open.picker.first)
		}
		for _, want := range []string{"system", "slice", "232-255", "hex"} {
			if !strings.Contains(text, want) {
				t.Errorf("the %s hides %q of the picker:\n%s", name, want, text)
			}
		}
	}
}

func TestEnterWithoutMovingKeepsTheColor(t *testing.T) {
	p, r := testPicker("#9d7cd8", false)
	pressPicker(p, "enter")
	if r.color != "#9d7cd8" || !r.ok {
		t.Errorf("enter at once gave %q, want #9d7cd8 kept", r.color)
	}
	if p, _ := testPicker("#9d7cd8", false); p.candidate() != "#9d7cd8" {
		t.Errorf("the stage would show %q before any move", p.candidate())
	}
}

func TestClicksOnTheThemePageStayInTheList(t *testing.T) {
	a, _ := editingMine(t)
	a.size.Y = 40
	a.selectConfigRow(themeRowBase + configRow(theme.FocusedBorder))
	press(a, enterPress)
	l := a.configLayout()
	inner := l.inner()
	click(a, inner.Min.X+3, inner.Min.Y+2)
	if a.configPanel.edit.selected != theme.FocusedBorder || a.configPanel.edit.picker == nil {
		t.Fatalf("a click with the picker open moved to %s", theme.Components[a.configPanel.edit.selected].Label)
	}
	press(a, escPress)
	click(a, inner.Max.X-3, inner.Min.Y+2) // on the stage, at a row's height
	if a.configPanel.edit.picker != nil || a.configPanel.edit.selected != theme.FocusedBorder {
		t.Errorf("a click on the stage picked %s", theme.Components[a.configPanel.edit.selected].Label)
	}
}

func TestANewThemeDoesNotOverwriteABrokenFile(t *testing.T) {
	a, dir := editorApp(t, newThemeRow)
	writeUserTheme(t, dir, "bad", "[panels]\nfocused_border = \"#12345\"\n")
	a.loadThemes()
	press(a, enterPress)
	typeInto(a, "bad")
	press(a, enterPress)
	if a.mode != promptMode {
		t.Errorf("a name whose file is there went on (mode %v)", a.mode)
	}
}
