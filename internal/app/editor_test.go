package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/script"
)

func TestChoicesOfferInstalledPrograms(t *testing.T) {
	installed := func(program string) bool {
		return program == "hx" || program == "nvim" || program == "yazi" || program == "dolphin"
	}
	if got, want := editorChoices(installed), []string{"vim", "nano", "nvim", "hx", customChoice}; !slices.Equal(got, want) {
		t.Errorf("editors %q, want %q", got, want)
	}
	if got, want := fileManagerChoices(installed), []string{"terminal", "yazi", "dolphin", customChoice}; !slices.Equal(got, want) {
		t.Errorf("file managers %q, want %q", got, want)
	}
}

func TestCustomEditorIsTypedBelowTheChoice(t *testing.T) {
	a := openConfigAt(t, editorRow)
	a.configPanel.editor = newProgramSetting([]string{"vim", "nano", customChoice}, a.config.Editor)

	press(a, rightPress, rightPress)
	if a.mode != configMode || a.config.Editor != "nano" || !a.configPanel.editor.custom {
		t.Fatalf("on custom: mode %v editor %q custom %v", a.mode, a.config.Editor, a.configPanel.editor.custom)
	}
	press(a, downPress)
	if a.configPanel.selected != editorCommandRow {
		t.Fatalf("down from custom selected row %d, want the command row", a.configPanel.selected)
	}
	typeText(a, "noct-no-such-editor")
	if note := commandNote(a.configPanel.editor.draft.String()); note != "command not found" {
		t.Errorf("note %q for a missing command", note)
	}
	press(a, uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl})
	typeText(a, "cat -n")
	press(a, downPress) // leaving the row saves it
	if a.config.Editor != "cat -n" {
		t.Fatalf("editor %q after typing cat -n", a.config.Editor)
	}

	a.openConfig()
	if !a.configPanel.editor.custom || a.configPanel.editor.draft.String() != "cat -n" {
		t.Error("reopening the config lost the custom command")
	}
}

func TestCustomWithoutACommandKeepsTheEditor(t *testing.T) {
	a := openConfigAt(t, editorRow)
	a.configPanel.editor = newProgramSetting([]string{"vim", "nano", customChoice}, a.config.Editor)
	press(a, leftPress, escPress)
	if a.config.Editor != "vim" {
		t.Errorf("editor %q after leaving custom empty", a.config.Editor)
	}
}

func TestScriptsLocationComesFromTheConfig(t *testing.T) {
	a := openConfigAt(t, scriptsDirRow)
	if a.configPanel.scriptsDir.String() != script.PersonalDir() {
		t.Fatalf("location starts as %q, want the folder in use", a.configPanel.scriptsDir.String())
	}
	press(a, uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl})
	if !a.configPanel.copied || !slices.ContainsFunc(a.configLines(), func(l configLine) bool { return l.label == "copied to clipboard" }) {
		t.Error("ctrl+c did not copy the location")
	}

	elsewhere := t.TempDir()
	saveScriptIn(t, elsewhere, "remote", "run: true\n")
	press(a, uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl})
	typeText(a, elsewhere)
	press(a, enterPress)
	if a.config.ScriptsDir != elsewhere || a.scriptsDir() != elsewhere {
		t.Fatalf("scripts dir %q", a.config.ScriptsDir)
	}
	if _, ok := a.scripts.Find("remote"); !ok {
		t.Error("the library was not read from the new location")
	}
}

func TestEditingWithAMissingEditorSaysSo(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	a.config.Editor = "noct-no-such-editor --wait"
	saveScript(t, "s", "run: true\n")
	openScriptsPanel(t, a)
	press(a, letter('e'), downPress, downPress, enterPress)
	if len(l.programs) != 0 || a.statusMessage == "" {
		t.Errorf("started %d programs, status %q", len(l.programs), a.statusMessage)
	}
}

func TestOpenFolderInATerminal(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	dir := filepath.Join(t.TempDir(), "scripts")
	a.openFolder(dir)
	if run := l.last(t); run.dir != dir || run.argv[0] != a.shellArgv[0] {
		t.Errorf("opened %q in %s", run.argv, run.dir)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the folder was not created: %v", err)
	}

	a.config.FileManager = "cat"
	a.openFolder(dir)
	if run := l.last(t); run.argv[1] != "-c" || !slices.Contains(run.argv, dir) {
		t.Errorf("custom file manager ran %q", run.argv)
	}
}

func TestTailKeepsTheEnd(t *testing.T) {
	if got := tail("/home/someone/.local/share/noct/scripts", 12); got != "…oct/scripts" {
		t.Errorf("tail %q, want the last 11 cells after …", got)
	}
	if got := tail("short", 12); got != "short" {
		t.Errorf("short text changed: %q", got)
	}
}
