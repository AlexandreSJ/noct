package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/script"
)

// launches records the programs the workspace starts, each shown by a fakeView.
type launches struct {
	programs []program
	views    []*fakeView
}

func recordLaunches(a *App) *launches {
	l := &launches{}
	a.launch = func(p program, _ pty.Size) (view, error) {
		v := &fakeView{}
		l.programs = append(l.programs, p)
		l.views = append(l.views, v)
		return v, nil
	}
	return l
}

func (l *launches) last(t *testing.T) program {
	t.Helper()
	if len(l.programs) == 0 {
		t.Fatal("nothing was started")
	}
	return l.programs[len(l.programs)-1]
}

func saveScript(t *testing.T, name, content string) {
	t.Helper()
	saveScriptIn(t, script.PersonalDir(), name, content)
}

func saveScriptIn(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, script.FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const greetScript = `
run: echo {{.greeting}} {{.who}}
params:
  - name: greeting
    choices: [hello, bye]
  - name: who
    default: world
    remember: true
`

// openScriptsPanel opens a scripts panel in a new tab, reading the library.
func openScriptsPanel(t *testing.T, a *App) *scriptsPanel {
	t.Helper()
	press(a, prefixPress, letter('s'))
	panel, ok := a.currentTab().focusedView().(*scriptsPanel)
	if !ok {
		t.Fatal("prefix+s did not open a scripts panel")
	}
	return panel
}

func typeText(a *App, text string) {
	for _, r := range text {
		press(a, letter(r))
	}
}

func TestRunningAScriptAsksItsParametersAndRunsBeside(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveScript(t, "greet", greetScript)
	openScriptsPanel(t, a)

	press(a, enterPress)
	if a.mode != formMode {
		t.Fatalf("enter on a script with parameters: mode %v, want the form", a.mode)
	}
	press(a, rightPress, tabPress, uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl})
	typeText(a, "ana")
	press(a, enterPress)

	run := l.last(t)
	if want := []string{"/bin/sh", "-c", "echo bye ana"}; !slices.Equal(run.argv, want) {
		t.Fatalf("ran %q, want %q", run.argv, want)
	}
	tab := a.currentTab()
	if len(tab.views) != 2 {
		t.Fatalf("%d panes, want the run beside the panel", len(tab.views))
	}
	if _, ok := tab.focusedView().(*runView); !ok {
		t.Fatal("the run pane does not have the focus")
	}
	if got := a.memory.Values(script.Script{Name: "greet"})["who"]; got != "ana" {
		t.Errorf("remembered who=%q", got)
	}
}

// editPart writes what the user saves in the editor opened for a part, and closes the editor.
func editPart(t *testing.T, l *launches, content string) {
	t.Helper()
	editor := l.last(t)
	if err := os.WriteFile(editor.argv[len(editor.argv)-1], []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	editor.onExit(0)
}

func TestEditingAScriptRenamesItAndKeepsItsRememberedValues(t *testing.T) {
	a := testApp(t)
	a.config.Editor = "cat"
	l := recordLaunches(a)
	saveScript(t, "greet", greetScript)
	openScriptsPanel(t, a)
	press(a, enterPress, enterPress) // run once, so who=world is remembered
	press(a, prefixPress, leftPress, escPress)

	press(a, letter('e'))
	if a.mode != scriptFormMode || a.scriptForm.fields[nameField] != "greet" {
		t.Fatalf("e: mode %v, name %q", a.mode, a.scriptForm.fields[nameField])
	}
	press(a, uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl})
	typeText(a, "hello")
	press(a, downPress, downPress, rightPress, enterPress) // the description, in the editor
	if a.mode != normalMode {
		t.Fatalf("enter on the description: mode %v", a.mode)
	}
	editPart(t, l, "Says hello\n")
	if a.mode != scriptFormMode || a.scriptForm.fields[descriptionPart] != "Says hello" {
		t.Fatalf("after the editor: mode %v, description %q", a.mode, a.scriptForm.fields[descriptionPart])
	}
	press(a, uv.KeyPressEvent{Code: 's', Mod: uv.ModCtrl})

	renamed, ok := a.scripts.Find("hello")
	if !ok || a.mode != normalMode {
		t.Fatalf("no script under the new name; mode %v status %q", a.mode, a.statusMessage)
	}
	if renamed.Description != "Says hello" || renamed.Run != "echo {{.greeting}} {{.who}}" {
		t.Errorf("saved %+v", renamed)
	}
	if got := a.memory.Values(renamed)["who"]; got != "world" {
		t.Errorf("remembered who=%q after renaming", got)
	}
}

func TestScriptWithoutParametersRunsRightAway(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveScript(t, "build", "run: make build\nworkdir: /srv\n")
	openScriptsPanel(t, a)
	press(a, enterPress)

	if run := l.last(t); run.argv[2] != "make build" || run.dir != "/srv" {
		t.Errorf("ran %q in %s", run.argv, run.dir)
	}
}

func TestEnvParameterMustBeSet(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveScript(t, "login", "run: login {{.pw}}\nparams: [{name: pw, from: 'env:NOCT_TEST_PW'}]\n")
	openScriptsPanel(t, a)
	press(a, enterPress)
	if len(l.programs) != 0 || !strings.Contains(a.statusMessage, "NOCT_TEST_PW") {
		t.Fatalf("ran %d programs, status %q", len(l.programs), a.statusMessage)
	}

	a.shellEnv = []string{"NOCT_TEST_PW=hunter2"}
	press(a, enterPress)
	if run := l.last(t); run.argv[2] != "login hunter2" {
		t.Errorf("ran %q", run.argv)
	}
}

func TestRunViewHoldsTheOutputUntilClosed(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveScript(t, "fail", "run: exit 3\n")
	openScriptsPanel(t, a)
	press(a, enterPress)
	run := a.currentTab().focusedView().(*runView)

	l.last(t).onExit(3)
	if run.running || run.Title() != "fail ✗ 3" {
		t.Fatalf("after exit: running=%v title %q", run.running, run.Title())
	}
	press(a, letter('x')) // not a run view key: ignored
	press(a, letter('r'))
	if len(l.programs) != 2 || !run.running || !l.views[0].closed {
		t.Fatalf("r: %d runs, running=%v, old closed=%v", len(l.programs), run.running, l.views[0].closed)
	}
	l.programs[0].onExit(0) // the replaced run's exit arrives late
	if !run.running {
		t.Fatal("the old run's exit ended the new one")
	}

	l.last(t).onExit(0)
	press(a, enterPress)
	if len(a.currentTab().views) != 1 || !l.views[1].closed {
		t.Fatal("enter did not close the finished run")
	}
}

func TestNewScriptFromTheForm(t *testing.T) {
	a := testApp(t)
	a.config.Editor = "cat -n" // installed everywhere, with an argument
	a.config.ScriptHints = true
	l := recordLaunches(a)
	openScriptsPanel(t, a)
	save := uv.KeyPressEvent{Code: 's', Mod: uv.ModCtrl}

	press(a, letter('n'), letter('s'))
	typeText(a, "smoke")
	press(a, save)
	if a.mode != scriptFormMode || a.statusMessage == "" {
		t.Fatalf("saved a script without a command: mode %v", a.mode)
	}
	press(a, tabPress, tabPress, enterPress)
	if editor := l.last(t); !strings.HasPrefix(editor.argv[2], "cat -n ") || !strings.HasSuffix(editor.argv[len(editor.argv)-1], "script.sh") {
		t.Fatalf("editor %q", editor.argv)
	}
	editPart(t, l, "make smoke\n")
	press(a, downPress, enterPress) // params
	editPart(t, l, "- name: target\n  default: all\n")
	press(a, save)

	s, ok := a.scripts.Find("smoke")
	if !ok || s.Run != "make smoke" || len(s.Params) != 1 {
		t.Fatalf("created %+v; status %q", s, a.statusMessage)
	}
	if content, _ := os.ReadFile(s.File()); !strings.HasPrefix(string(content), "# description:") {
		t.Errorf("no hints on top:\n%s", content)
	}
	if _, err := os.Stat(a.scriptForm.tempDir); a.scriptForm.tempDir != "" || err == nil {
		t.Error("the form's temporary files were left behind")
	}
}

func TestDeletingAScriptAsksFirst(t *testing.T) {
	a := testApp(t)
	saveScript(t, "old", "run: true\n")
	openScriptsPanel(t, a)
	del := uv.KeyPressEvent{Code: uv.KeyDelete}

	press(a, del, escPress)
	if _, ok := a.scripts.Find("old"); !ok {
		t.Fatal("deleted without confirmation")
	}
	press(a, del, enterPress)
	if _, ok := a.scripts.Find("old"); ok {
		t.Fatal("still there after confirming")
	}
}

func TestFuzzyScoreRanksWordStarts(t *testing.T) {
	newPanel, _ := fuzzyScore("np", "new panel")
	openHelp, _ := fuzzyScore("np", "program help")
	if newPanel <= openHelp {
		t.Errorf("new panel %d, program help %d", newPanel, openHelp)
	}
	if _, ok := fuzzyScore("xyz", "new panel"); ok {
		t.Error("matched letters that are not there")
	}
	if _, ok := fuzzyScore("", "anything"); !ok {
		t.Error("an empty query should match everything")
	}
}

func TestFinderOpensScriptsAndActions(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveScript(t, "deploy", "run: ./deploy.sh\n")
	a.reloadScripts()

	press(a, prefixPress, letter(' '))
	if a.mode != finderMode {
		t.Fatalf("prefix+space: mode %v", a.mode)
	}
	typeText(a, "depl")
	press(a, enterPress)
	if run := l.last(t); run.argv[2] != "./deploy.sh" {
		t.Fatalf("ran %q", run.argv)
	}

	press(a, prefixPress, letter(' '))
	typeText(a, "new tab")
	press(a, enterPress)
	if len(a.session.tabs) != 2 {
		t.Errorf("%d tabs after choosing new tab", len(a.session.tabs))
	}
}

// busyView is a terminal whose shell runs a program in the foreground.
type busyView struct {
	fakeView
	program string
}

func (v *busyView) ForegroundProgram() (string, bool) {
	return v.program, v.program != ""
}

func TestHelpOpensForTheForegroundProgram(t *testing.T) {
	a := testApp(t, newTab(1, &busyView{program: "htop"}))
	l := recordLaunches(a)
	press(a, prefixPress, letter('?'))
	if help := l.last(t); !slices.Contains(help.argv, "htop") {
		t.Fatalf("help ran %q", help.argv)
	}

	idle := testApp(t, newTab(1, &busyView{}))
	press(idle, prefixPress, letter('?'))
	if idle.mode != commandMode || idle.statusMessage == "" {
		t.Errorf("with only the shell: mode %v, status %q", idle.mode, idle.statusMessage)
	}
}

func TestSortByLastRunKeepsTheSelection(t *testing.T) {
	a := testApp(t)
	recordLaunches(a)
	a.config.ScriptsSort = config.SortByRun
	saveScript(t, "alpha", "run: true\n")
	saveScript(t, "beta", "run: true\n")
	panel := openScriptsPanel(t, a)

	press(a, downPress, enterPress) // run beta
	press(a, prefixPress, leftPress, escPress)
	if a.scripts.Scripts[0].Name != "beta" {
		t.Fatalf("beta ran last but %s comes first", a.scripts.Scripts[0].Name)
	}
	if row, _ := panel.current(); row.script.Name != "beta" {
		t.Errorf("the selection moved to %s when the list was sorted", row.script.Name)
	}
	runs, _ := script.Runs(runsPath())
	if len(runs) != 1 || runs[0].Script != "beta" {
		t.Errorf("runs %v", runs)
	}
}

func TestSortOrderFromTheConfig(t *testing.T) {
	a := openConfigAt(t, scriptsSortRow)
	saveScript(t, "alpha", "run: true\n")
	saveScript(t, "beta", "run: true\n")
	later := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(script.PersonalDir(), "beta", script.FileName), later, later); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(script.PersonalDir(), "alpha", script.FileName), later.Add(-time.Hour), later.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	a.reloadScripts()

	press(a, rightPress)
	if a.config.ScriptsSort != config.SortByEdited || a.scripts.Scripts[0].Name != "beta" {
		t.Errorf("sort %q, first %s", a.config.ScriptsSort, a.scripts.Scripts[0].Name)
	}
	press(a, rightPress, rightPress)
	if a.config.ScriptsSort != config.SortByName || a.scripts.Scripts[0].Name != "alpha" {
		t.Errorf("sort %q, first %s", a.config.ScriptsSort, a.scripts.Scripts[0].Name)
	}
}
