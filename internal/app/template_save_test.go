package app

import (
	"os"
	"path/filepath"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/templates"
	"github.com/AlexandreSJ/noct/internal/theme"
)

var saveTemplateKey = uv.KeyPressEvent{Code: 'r', Mod: uv.ModShift, Text: "R"}

// templatesFolder points the user's templates at a new folder and returns it.
func templatesFolder(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := templates.Dir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// devTab is a tab with a shell running claude, a scripts panel and a run, which templates leave out.
func devTab(t *testing.T) (*App, *tab) {
	t.Helper()
	a := testApp(t)
	a.launch = func(program, pty.Size) (view, error) { return &commandView{command: "claude"}, nil }
	a.openPanel(a.startTerminal)
	a.openPanel(a.startScripts)
	tb := a.currentTab()
	start := func(pty.Size, func(int)) (view, error) { return &fakeView{}, nil }
	run, err := newRunView("run", &a.palette, start, func() {}, pty.Size{Cols: 80, Rows: 24}, "")
	if err != nil {
		t.Fatal(err)
	}
	tb.tree.Split(tb.focused, 99, layout.SideBySide)
	tb.views[99] = run
	tb.name = "dev"
	return a, tb
}

func TestSaveTemplateListsThePanesWithTheirCommands(t *testing.T) {
	templatesFolder(t)
	a, _ := devTab(t)
	press(a, prefixPress, saveTemplateKey)
	if a.mode != templateSaveMode {
		t.Fatalf("prefix + R left mode %v", a.mode)
	}
	s := a.templateSave
	if len(s.panes) != 2 || s.panes[0].kind != "terminal" || s.panes[0].command != "claude" || s.panes[0].start != templates.Ask || s.panes[1].kind != "scripts" {
		t.Errorf("panes %+v, want the shell asking for claude and the scripts panel", s.panes)
	}
	if s.name.String() != "dev" {
		t.Errorf("name %q, want the tab's", s.name.String())
	}
}

func TestStartCyclesRunAskNone(t *testing.T) {
	templatesFolder(t)
	a, _ := devTab(t)
	a.openTemplateSave()
	press(a, downPress)
	for _, want := range []templates.Start{templates.None, templates.Run, templates.Ask} {
		press(a, rightPress)
		if got := a.templateSave.panes[0].start; got != want {
			t.Fatalf("start %q, want %q", got, want)
		}
	}
	press(a, leftPress)
	if got := a.templateSave.panes[0].start; got != templates.Run {
		t.Errorf("left gave %q, want run", got)
	}
	press(a, downPress, rightPress)
	if got := a.templateSave.panes[1].start; got != "" {
		t.Errorf("the scripts panel has start %q", got)
	}
}

func TestEnterSavesTheTemplate(t *testing.T) {
	dir := templatesFolder(t)
	a, tb := devTab(t)
	tb.colors = colorSet{theme.ActiveTabText: "#111111"}
	tb.paneColors[tb.tree.Panes()[0]] = colorSet{theme.FocusedBorder: "#ff0000"}
	a.openTemplateSave()
	press(a, enterPress)
	tpl, err := templates.Load(dir, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(tpl.Root.Columns) != 2 || tpl.Tab != "dev" || tpl.Colors["tabs.active_text"] != "#111111" {
		t.Fatalf("saved %+v", tpl)
	}
	shell := tpl.Root.Columns[0].Pane
	if shell.Kind != "terminal" || shell.Command != "claude" || shell.Start != templates.Ask || shell.Colors["panels.focused_border"] != "#ff0000" {
		t.Errorf("the shell was saved as %+v", shell)
	}
	if a.mode != normalMode || a.notice != "saved template dev" {
		t.Errorf("mode %v, notice %q", a.mode, a.notice)
	}
}

func TestSavingOverATemplateAsksFirst(t *testing.T) {
	dir := templatesFolder(t)
	_ = templates.Save(dir, templates.Template{Name: "dev", Root: templates.Node{Pane: &templates.Pane{Kind: "logs"}}})
	a, _ := devTab(t)
	a.openTemplateSave()
	press(a, enterPress)
	if a.mode != confirmMode {
		t.Fatalf("mode %v, want the question", a.mode)
	}
	press(a, escPress)
	if tpl, _ := templates.Load(dir, "dev"); tpl.Root.Pane == nil || tpl.Root.Pane.Kind != "logs" {
		t.Errorf("esc replaced the template: %+v", tpl.Root)
	}
	press(a, enterPress, enterPress)
	if tpl, _ := templates.Load(dir, "dev"); len(tpl.Root.Columns) != 2 {
		t.Errorf("enter kept the old template: %+v", tpl.Root)
	}
}

func TestABadNameStaysInThePopup(t *testing.T) {
	dir := templatesFolder(t)
	a, _ := devTab(t)
	a.openTemplateSave()
	typeInto(a, " x")
	press(a, enterPress)
	if a.mode != templateSaveMode || a.statusMessage == "" {
		t.Errorf("mode %v, message %q", a.mode, a.statusMessage)
	}
	if _, err := os.Stat(filepath.Join(dir, "dev x.yaml")); err == nil {
		t.Error("a bad name was saved")
	}
}

func TestSaveTemplateNeedsATab(t *testing.T) {
	a := testApp(t)
	press(a, prefixPress, saveTemplateKey)
	if a.mode == templateSaveMode {
		t.Error("prefix + R opened on the home screen")
	}
}
