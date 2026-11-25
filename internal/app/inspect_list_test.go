package app

import (
	"image"
	"strings"
	"testing"

	saved "github.com/AlexandreSJ/noct/internal/session"
)

func inspectRowKeys(p *inspectPanel) []string {
	var keys []string
	for _, r := range p.rows() {
		keys = append(keys, strings.Repeat(" ", r.depth)+r.key())
	}
	return keys
}

// The inspect lists flows in their folders, closed until opened, and functions under ƒ functions.
func TestTheInspectListsFlowsInTheirFolders(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "ops/db/backup", "steps: []\n")
	saveFlow(t, "ops/deploy", "steps: []\n")
	saveFlow(t, "hello", "steps: []\n")
	saveFunction(t, "utils/greet", "steps: []\n")
	pressDrawn(a, prefixPress, letter('i'))
	p := a.currentTab().focusedView().(*inspectPanel)
	if f, _, _ := p.current(); f.Name != "hello" {
		t.Fatalf("it opened on %q, want the first flow the list shows", f.Name)
	}
	want := "flows:ops,hello,functions:, functions:utils"
	if got := strings.Join(inspectRowKeys(p), ","); got != want {
		t.Fatalf("rows %s, want %s", got, want)
	}
	pressDrawn(a, letter('k'), enterPress, letter('j'), letter('l'), letter('j'))
	if f, _, _ := p.current(); f.Name != "ops/db/backup" {
		t.Errorf("down in ops/db the flow is %q, want ops/db/backup", f.Name)
	}
	if screen := screenText(a); !strings.Contains(screen, "- ops") || !strings.Contains(screen, "│ │") || !strings.Contains(screen, "backup") {
		t.Errorf("the open folders do not show:\n%s", screen)
	}
	pressDrawn(a, letter('h'))
	if p.rowKey != "flows:ops/db" {
		t.Errorf("h on a flow selected %q, want its folder", p.rowKey)
	}
	pressDrawn(a, letter('h'), letter('h'))
	if p.rowKey != "flows:ops" || a.folderOpen("flows:ops/db") {
		t.Errorf("h on a closed folder selected %q, want the folder it is in", p.rowKey)
	}
	if f, _, _ := p.current(); f.Name != "ops/db/backup" {
		t.Errorf("selecting folders changed the chart's flow to %q", f.Name)
	}
}

// A restored inspect opens on the flow it had, its folders open; one that is gone leaves it on
// the first flow.
func TestTheInspectOpensAgainOnItsFlow(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "ops/deploy", "steps: []\n")
	saveFlow(t, "hello", "steps: []\n")
	pressDrawn(a, prefixPress, letter('i'))
	tab := a.currentTab()
	id := tab.focused
	tab.views[id].(*inspectPanel).flowName = "ops/deploy"
	pane, _ := a.savedPane(tab, id)
	if pane.Item != "ops/deploy" {
		t.Fatalf("the session keeps %q", pane.Item)
	}
	v, err := a.restoreView(id, pane)
	if err != nil {
		t.Fatal(err)
	}
	p := v.(*inspectPanel)
	if row, _, ok := p.selectedRow(); !ok || row.entry == nil || row.entry.Name != "ops/deploy" {
		t.Errorf("restored on %+v, want ops/deploy shown in its open folder", row)
	}
	a.setExpanded("flows:ops", false) // the list's top is then hello
	v, _ = a.restoreView(id, saved.Pane{Kind: saved.Inspect, Item: "gone"})
	if f, _, _ := v.(*inspectPanel).current(); f.Name != "hello" {
		t.Errorf("a flow that is gone opened %q, want the list's first flow", f.Name)
	}
}

// The scripts panel lists folders inside folders, each a step further in.
func TestTheScriptsPanelListsFoldersInFolders(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveScript(t, "ops/db/backup", "run: echo backup\n")
	saveScript(t, "ops/deploy", "run: echo deploy\n")
	pressDrawn(a, prefixPress, letter('s'))
	pressDrawn(a, enterPress, letter('j'), enterPress, letter('j'))
	p := a.currentTab().focusedView().(*scriptsPanel)
	row, ok := p.current()
	if !ok || row.script == nil || row.script.ID() != "ops/db/backup" || row.depth != 2 {
		t.Fatalf("selected %+v, want ops/db/backup two folders in", row)
	}
	if screen := screenText(a); !strings.Contains(screen, "- db") || !strings.Contains(screen, "backup") {
		t.Errorf("the folder in a folder does not show:\n%s", screen)
	}
}
