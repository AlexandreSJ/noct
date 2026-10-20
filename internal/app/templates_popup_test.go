package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/templates"
	"github.com/AlexandreSJ/noct/internal/theme"
)

var templatesKey = uv.KeyPressEvent{Code: 't', Mod: uv.ModShift, Text: "T"}

const devTemplate = `name: dev
colors: { tabs.active_text: "#111111" }
layout: columns
panes:
  - terminal: { name: server, run: fastfetch, start: run }
  - rows:
      - terminal: { name: claude, run: claude, start: ask }
      - terminal: { run: htop, start: none }
        colors: { panels.focused_border: "#ff0000" }
`

func writeTemplate(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTemplatesListTheFiles(t *testing.T) {
	dir := templatesFolder(t)
	writeTemplate(t, dir, "dev", devTemplate)
	writeTemplate(t, dir, "api", "panes:\n  - logs: {}\n")
	a := testApp(t)
	press(a, prefixPress, templatesKey)
	if a.mode != templatesMode {
		t.Fatalf("prefix + T on the home screen left mode %v", a.mode)
	}
	if len(a.templates.list) != 2 || a.templates.list[0].Name != "api" || a.templates.selected != 0 {
		t.Errorf("listed %v", a.templates.list)
	}
}

func TestTheListShowsTheLayoutOfTheSelected(t *testing.T) {
	dir := templatesFolder(t)
	writeTemplate(t, dir, "dev", devTemplate)
	a := testApp(t)
	a.size.Y = 40
	a.openTemplates()
	text := screenText(a)
	for _, want := range []string{"dev", "server", "claude"} {
		if !strings.Contains(text, want) {
			t.Errorf("the list does not show %q:\n%s", want, text)
		}
	}
}

func TestOpeningATemplateBuildsTheTab(t *testing.T) {
	dir := templatesFolder(t)
	writeTemplate(t, dir, "dev", devTemplate)
	shellDir := t.TempDir()
	a, _ := besideShell(t, shellDir)
	l := recordLaunches(a)
	a.openTemplates()
	press(a, enterPress)
	tb := a.currentTab()
	if len(a.session.tabs) != 2 || tb.name != "dev" || len(tb.views) != 3 || a.mode != normalMode {
		t.Fatalf("opened %d tabs, the new one %q with %d panes, mode %v", len(a.session.tabs), tb.name, len(tb.views), a.mode)
	}
	panes := tb.tree.Panes()
	if tb.focused != panes[0] || tb.paneNames[panes[0]] != "server" || tb.paneColors[panes[2]][theme.FocusedBorder] != "#ff0000" || tb.colors[theme.ActiveTabText] != "#111111" {
		t.Errorf("focus %d, names %v, colors %v and %v", tb.focused, tb.paneNames, tb.paneColors, tb.colors)
	}
	for _, p := range l.programs {
		if p.dir != shellDir {
			t.Errorf("a terminal opened in %q, want the focused shell's %q", p.dir, shellDir)
		}
	}
}

func TestRunTypesTheCommandAndAskOffersIt(t *testing.T) {
	dir := templatesFolder(t)
	writeTemplate(t, dir, "dev", devTemplate)
	a := testApp(t)
	l := recordLaunches(a)
	a.openTemplates()
	press(a, enterPress)
	panes := a.currentTab().tree.Panes()
	if !hasTyped(l, "fastfetch\n") {
		t.Error("the run pane did not get fastfetch")
	}
	if got := a.pendingRerun(panes[1]); got != "claude" {
		t.Errorf("the ask pane offers %q, want claude", got)
	}
	if a.pendingRerun(panes[2]) != "" || hasTyped(l, "htop\n") {
		t.Error("the none pane runs or offers htop")
	}
}

func hasTyped(l *launches, text string) bool {
	for _, v := range l.views {
		if typed(v) == text {
			return true
		}
	}
	return false
}

func TestAPaneThatCannotOpenIsLeftOut(t *testing.T) {
	dir := templatesFolder(t)
	writeTemplate(t, dir, "x", "layout: columns\npanes:\n  - terminal: { run: \"ls\\nrm\", start: run }\n  - logs: {}\n")
	a := testApp(t)
	l := recordLaunches(a)
	a.openTemplates()
	press(a, enterPress)
	if tb := a.currentTab(); tb == nil || len(tb.views) != 2 {
		t.Fatalf("the tab did not open with both panes")
	}
	if hasTyped(l, "ls\nrm\n") || hasTyped(l, "ls\n") {
		t.Error("a command with a newline was typed")
	}
}

func TestTemplatesEditRenameDelete(t *testing.T) {
	dir := templatesFolder(t)
	writeTemplate(t, dir, "dev", devTemplate)
	a := testApp(t)
	a.config.Editor = "sh"
	l := recordLaunches(a)
	a.openTemplates()
	press(a, letter('e'))
	if argv := l.last(t).argv; argv[len(argv)-1] != filepath.Join(dir, "dev.yaml") {
		t.Errorf("the editor got %v", argv)
	}
	a.openTemplates()
	press(a, letter('r'))
	for range len("dev") {
		press(a, backspacePress)
	}
	typeInto(a, "web")
	press(a, enterPress)
	if _, err := templates.Load(dir, "web"); err != nil || a.mode != templatesMode {
		t.Fatalf("rename: %v, mode %v", err, a.mode)
	}
	press(a, letter('d'), enterPress)
	if list, _ := templates.LoadAll(dir); len(list) != 0 {
		t.Errorf("delete left %v", list)
	}
}

func TestAGoneTemplateSaysSoOnEnter(t *testing.T) {
	dir := templatesFolder(t)
	writeTemplate(t, dir, "dev", devTemplate)
	a := testApp(t)
	a.openTemplates()
	_ = os.Remove(filepath.Join(dir, "dev.yaml"))
	press(a, enterPress)
	if len(a.session.tabs) != 0 || a.statusMessage == "" {
		t.Errorf("%d tabs and message %q after opening a deleted template", len(a.session.tabs), a.statusMessage)
	}
}

func TestAnUnknownPaneIsLeftOutAndTold(t *testing.T) {
	dir := templatesFolder(t)
	writeTemplate(t, dir, "x", "layout: columns\npanes:\n  - logs: {}\n  - editor: {}\n")
	a := testApp(t)
	a.openTemplates()
	press(a, enterPress)
	if tb := a.currentTab(); tb == nil || len(tb.views) != 1 || !strings.Contains(a.statusMessage, "editor") {
		t.Errorf("tab %v, message %q", tb, a.statusMessage)
	}
}

func TestALongListOfTemplatesScrolls(t *testing.T) {
	dir := templatesFolder(t)
	for i := range 40 {
		writeTemplate(t, dir, fmt.Sprintf("t%02d", i), "panes:\n  - logs: {}\n")
	}
	a := testApp(t)
	a.openTemplates()
	for range 39 {
		press(a, downPress)
	}
	if text := screenText(a); !strings.Contains(text, "t39") {
		t.Errorf("the selected template is not shown:\n%s", text)
	}
}
