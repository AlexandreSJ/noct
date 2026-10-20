package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/theme"
)

// editorApp is an app with the config open on a row and the user's themes in a folder of its own.
func editorApp(t *testing.T, row configRow) (*App, string) {
	t.Helper()
	dir := themesFolder(t)
	a := openConfigAt(t, row)
	a.loadThemes()
	return a, dir
}

func pickInFinder(t *testing.T, a *App, label string) {
	t.Helper()
	for i, m := range a.finder.matches {
		if m.label == label {
			a.finder.selected = i
			press(a, enterPress)
			return
		}
	}
	t.Fatalf("%q is not in the finder: %v", label, a.finder.matches)
}

func TestNewThemeAsksTheNameThenTheBase(t *testing.T) {
	a, dir := editorApp(t, newThemeRow)
	press(a, enterPress)
	if a.mode != promptMode {
		t.Fatalf("mode %v, want the name prompt", a.mode)
	}
	typeInto(a, "mine")
	press(a, enterPress)
	if a.mode != finderMode {
		t.Fatalf("mode %v, want the base theme to choose", a.mode)
	}
	pickInFinder(t, a, "frost")
	content, err := os.ReadFile(filepath.Join(dir, "mine.toml"))
	if err != nil || !strings.Contains(string(content), `base = "frost"`) {
		t.Fatalf("mine.toml: %q, %v", content, err)
	}
	if a.mode != configMode || a.configPanel.page != themePage || a.configPanel.edit.theme.Name != "mine" {
		t.Fatalf("mode %v page %v, want the editor on mine", a.mode, a.configPanel.page)
	}
	if a.session.theme != "mine" {
		t.Errorf("the session is in %q, want mine", a.session.theme)
	}
}

func TestEditingABuiltinThemeMakesACopy(t *testing.T) {
	a, dir := editorApp(t, editThemeRow)
	a.session.theme = "blood"
	press(a, enterPress)
	if a.mode != promptMode {
		t.Fatalf("mode %v, want a name for the copy", a.mode)
	}
	typeInto(a, "myblood")
	press(a, enterPress)
	content, err := os.ReadFile(filepath.Join(dir, "myblood.toml"))
	if err != nil || !strings.Contains(string(content), `base = "blood"`) {
		t.Fatalf("myblood.toml: %q, %v", content, err)
	}
	if a.configPanel.page != themePage || a.configPanel.edit.theme.Name != "myblood" {
		t.Errorf("the editor is not on myblood")
	}
}

func TestEditingAUserThemeOpensItDirectly(t *testing.T) {
	a, dir := editorApp(t, editThemeRow)
	writeUserTheme(t, dir, "mine", "base = \"rose\"\n")
	a.loadThemes()
	a.session.theme = "mine"
	press(a, enterPress)
	if a.mode != configMode || a.configPanel.page != themePage || a.configPanel.edit.theme.Name != "mine" {
		t.Fatalf("mode %v page %v, want the editor on mine at once", a.mode, a.configPanel.page)
	}
	if a.configPanel.edit.base.Name != "rose" {
		t.Errorf("base %s, want rose", a.configPanel.edit.base.Name)
	}
}

func TestANameInUseIsRefused(t *testing.T) {
	a, dir := editorApp(t, newThemeRow)
	writeUserTheme(t, dir, "taken", "")
	a.loadThemes()
	for _, name := range []string{"noct", "taken", "a b"} {
		a.mode = configMode
		press(a, enterPress)
		typeInto(a, name)
		press(a, enterPress)
		if a.mode != promptMode || a.statusMessage == "" {
			t.Errorf("%q: mode %v, message %q; want the prompt again with the problem", name, a.mode, a.statusMessage)
		}
		a.statusMessage = ""
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("the folder has %d files, want only taken.toml", len(entries))
	}
}

func TestTheEditorListsTheComponentsByGroup(t *testing.T) {
	a, _ := editorApp(t, editThemeRow)
	a.openThemeEditor(a.themeNamed("noct"))
	seen := []theme.Component{a.configPanel.edit.selected}
	for range theme.Count - 1 {
		press(a, letter('j'))
		seen = append(seen, a.configPanel.edit.selected)
	}
	for i, c := range seen {
		if c != theme.Component(i) {
			t.Fatalf("the editor goes through %v, want every component in order", seen)
		}
	}
	press(a, escPress)
	if a.configPanel.page != settingsPage || a.configPanel.selected != editThemeRow || a.mode != configMode {
		t.Errorf("esc left page %v row %v mode %v, want the settings on edit theme", a.configPanel.page, a.configPanel.selected, a.mode)
	}
}

func TestMovingInTheEditorBlinksTheComponentOnTheStage(t *testing.T) {
	a, _ := editorApp(t, editThemeRow)
	a.openThemeEditor(a.themeNamed("noct"))
	press(a, letter('j'))
	if a.configPanel.edit.blinkAt.IsZero() || a.stageTick == nil {
		t.Error("moving did not blink the component on the stage")
	}
}

func TestTheEditorDrawsTheListAndTheStage(t *testing.T) {
	a, _ := editorApp(t, editThemeRow)
	a.size.Y = 40
	a.openThemeEditor(a.themeNamed("noct"))
	text := screenText(a)
	for _, want := range []string{"config › theme › noct", "focused border", "zsh", "logs"} {
		if !strings.Contains(text, want) {
			t.Errorf("the editor does not show %q:\n%s", want, text)
		}
	}
}

// pickNextColor opens the picker on the selected component and picks the cell after its color.
func pickNextColor(a *App) {
	press(a, enterPress, letter('l'), enterPress)
}

func TestPickingAColorDoesNotTellABrokenThemeAgain(t *testing.T) {
	a, dir := editorApp(t, editThemeRow)
	writeUserTheme(t, dir, "broken", "[tabs]\nactive_text = \"nope\"\n")
	writeUserTheme(t, dir, "mine", "base = \"rose\"\n")
	a.loadThemes()
	if a.statusMessage == "" {
		t.Fatal("the broken theme was not told")
	}
	a.statusMessage = ""
	a.openThemeEditor(a.themeNamed("mine"))
	pickNextColor(a)
	if a.statusMessage != "" {
		t.Errorf("picking a color told %q, want nothing", a.statusMessage)
	}
}

func TestAColorThatCannotBeSavedIsNotKept(t *testing.T) {
	a, dir := editorApp(t, editThemeRow)
	writeUserTheme(t, dir, "mine", "base = \"rose\"\n")
	a.loadThemes()
	a.useTheme("mine")
	a.openThemeEditor(a.themeNamed("mine"))
	before := a.palette.background
	beforeColor := a.configPanel.edit.theme.Colors[theme.Background]
	readOnly(t, dir)
	pickNextColor(a)
	if a.statusMessage == "" {
		t.Fatal("the failed save was not told")
	}
	if got := a.configPanel.edit.theme.Colors[theme.Background]; got != beforeColor {
		t.Errorf("the editor keeps %q, want %q, which is what the file has", got, beforeColor)
	}
	if !colorsEqual(a.palette.background, before) {
		t.Errorf("the screen shows %v, want %v", a.palette.background, before)
	}
}

func TestANewThemeThatCannotBeSavedLeavesNoPreview(t *testing.T) {
	a, dir := editorApp(t, newThemeRow)
	readOnly(t, dir)
	press(a, enterPress)
	typeInto(a, "mine")
	press(a, enterPress)
	typeInto(a, "frost")
	showsTheme(t, a, "frost")
	press(a, enterPress)
	if a.statusMessage == "" || a.mode != configMode {
		t.Fatalf("mode %v, message %q; want the config with the problem", a.mode, a.statusMessage)
	}
	showsTheme(t, a, "noct")
}

// readOnly keeps files from being written in dir until the test ends.
func readOnly(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root writes in read-only folders")
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestABackgroundCanFollowTheThemesBackgroundAgain(t *testing.T) {
	a, dir := editorApp(t, editThemeRow)
	writeUserTheme(t, dir, "mine", "base = \"rose\"\n[tabs]\nactive_background = \"#112233\"\n")
	a.loadThemes()
	a.openThemeEditor(a.themeNamed("mine"))
	a.selectConfigRow(themeRowBase + configRow(theme.ActiveTabBackground))
	press(a, enterPress)
	if hints := a.configPanel.edit.picker.hints(); !strings.Contains(hints, "t background") {
		t.Errorf("the picker hints %q, want t background", hints)
	}
	press(a, letter('t'))
	if got := a.themeNamed("mine").Colors[theme.ActiveTabBackground]; got != "" {
		t.Errorf("mine's active tab background is %q, want the background", got)
	}
	a.selectConfigRow(themeRowBase + configRow(theme.ActiveTabText))
	press(a, enterPress, letter('t'))
	if a.configPanel.edit.picker == nil {
		t.Error("t closed the picker of a text, which cannot follow the background")
	}
}
