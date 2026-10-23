package app

import (
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
)

var ctrlAltG = uv.KeyPressEvent{Code: 'g', Mod: uv.ModCtrl | uv.ModAlt}

// A bind's command asks each {{.name}} as a path, runs where the focused terminal is, and its
// pane closes once it ends well; a failure stays to be read.
func TestACommandBindAsksItsValuesAndClosesWhenItEndsWell(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.config.CustomBinds = []config.CustomBind{{Key: "ctrl+alt+g", Name: "less", Command: "printf {{.file}}"}}
	l := recordLaunches(a)
	press(a, ctrlAltG)
	if a.mode != formMode || a.form.field().Name != "file" || !a.form.field().Path {
		t.Fatalf("mode %v, want the form asking for file as a path", a.mode)
	}
	typeText(a, "notes.md")
	press(a, enterPress)
	run := l.last(t)
	if !slices.Equal(run.argv, []string{"/bin/sh", "-c", "printf notes.md"}) {
		t.Fatalf("ran %q", run.argv)
	}
	if len(a.currentTab().views) != 2 {
		t.Fatalf("%d panes, want the run beside", len(a.currentTab().views))
	}
	run.onExit(0)
	if len(a.currentTab().views) != 1 {
		t.Errorf("%d panes after the run ended with 0, want its pane closed", len(a.currentTab().views))
	}

	press(a, ctrlAltG)
	typeText(a, "missing.md")
	press(a, enterPress)
	l.last(t).onExit(1)
	if len(a.currentTab().views) != 2 {
		t.Errorf("%d panes after the run failed, want it kept", len(a.currentTab().views))
	}
}

// A bind after the prefix runs from noct mode; one on a key a command has is left out.
func TestAPrefixBindRunsAfterThePrefix(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.config.CustomBinds = []config.CustomBind{
		{Key: "prefix m", Name: "hello", Command: "printf hello"},
		{Key: "prefix x", Name: "taken", Command: "printf no"},
	}
	l := recordLaunches(a)
	press(a, prefixPress, letter('m'))
	if run := l.last(t); run.argv[2] != "printf hello" {
		t.Fatalf("ran %q", run.argv)
	}
	if a.mode != normalMode {
		t.Errorf("mode %v after the bind, want normal", a.mode)
	}
	if problems := a.customBindProblems(); len(problems) != 1 || !strings.Contains(problems[0], "prefix x") {
		t.Errorf("problems %q, want one about prefix x", problems)
	}
}

// Keys programs need, and keys typed as text, cannot be binds.
func TestBindsLeaveTheKeysProgramsNeed(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.config.CustomBinds = []config.CustomBind{
		{Key: "ctrl+c", Name: "a", Command: "printf a"},
		{Key: "g", Name: "b", Command: "printf b"},
		{Key: "ctrl+alt+nope", Name: "c", Command: "printf c"},
		{Key: "ctrl+alt+g", Name: "d"},
	}
	if problems := a.customBindProblems(); len(problems) != 4 {
		t.Errorf("problems %q, want each bind's", problems)
	}
	if _, ok := a.customBindFor(uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl}, false); ok {
		t.Error("ctrl+c was taken from the program")
	}
}

// A script bind runs the script; the search lists the binds.
func TestAScriptBindRunsItAndTheSearchFindsIt(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	saveScript(t, "hi", "program: printf\nrun: printf hi\n")
	a.reloadScripts()
	a.config.CustomBinds = []config.CustomBind{{Key: "ctrl+alt+g", Name: "say hi", Script: "hi"}}
	l := recordLaunches(a)
	press(a, ctrlAltG)
	if run := l.last(t); run.argv[2] != "printf hi" {
		t.Fatalf("ran %q", run.argv)
	}
	found := false
	for _, item := range a.finderItems() {
		found = found || item.kind == "bind" && item.label == "say hi"
	}
	if !found {
		t.Error("the search does not list the bind")
	}
}
