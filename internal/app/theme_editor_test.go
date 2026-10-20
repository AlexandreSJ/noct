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
