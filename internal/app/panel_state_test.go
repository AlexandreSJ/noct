package app

import (
	"encoding/json"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/logstore"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

// closeAndReopen closes the session on screen and opens it again, drawn, as alt+q and noct do.
func closeAndReopen(t *testing.T, a *App) *inspectPanel {
	t.Helper()
	first := a.session
	other, _ := a.createSession("other")
	a.switchSession(other)
	if err := a.closeSession(first); err != nil {
		t.Fatal(err)
	}
	a.switchSession(first)
	drawApp(a)
	p, ok := a.currentTab().focusedView().(*inspectPanel)
	if !ok {
		t.Fatalf("the restored pane is %T, want the inspect", a.currentTab().focusedView())
	}
	return p
}

func TestTheInspectOpensAgainAsItWasLeft(t *testing.T) {
	a := sessionsApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "deploy", "steps: [{wait: 5s}, {wait: 6s}]\n")
	p := openInspectOn(t, a)
	pressDrawn(a, tabPress, letter('s'), letter('v'))
	p.selected = stepKey(flow.Path("", "", 1))
	selected := p.selected
	pressDrawn(a, letter('e'))
	form, ok := p.editor.(*stepForm)
	if !ok {
		t.Fatalf("e opened %T, want the step's form", p.editor)
	}
	value := form.grid.at(0, stepFormValueColumn)
	value.field = newTextField("7s")

	p = closeAndReopen(t, a)
	if !p.listHidden || !p.focusChart || !p.showScope || !p.hideDetails || p.selected != selected {
		t.Errorf("restored list hidden %v, chart %v, scope %v, details hidden %v on %q; want all on %q",
			p.listHidden, p.focusChart, p.showScope, p.hideDetails, p.selected, selected)
	}
	form, ok = p.editor.(*stepForm)
	if !ok {
		t.Fatalf("restored with %T open, want the step's form", p.editor)
	}
	if form.title() != "wait" || form.grid.at(0, stepFormValueColumn).value() != "7s" {
		got := form.grid.at(0, stepFormValueColumn).value()
		t.Errorf("the form holds %q, want what was typed and not saved", got)
	}
	if !form.changed() {
		t.Error("the restored form tells nothing changed, so esc would not ask to save")
	}
}

func TestAStepGoneLeavesTheRestoredChartWithoutAForm(t *testing.T) {
	a := sessionsApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "deploy", "steps: [{run: 'echo one'}]\n")
	p := openInspectOn(t, a)
	p.restoreState(&saved.InspectState{Selected: stepKey("9"), Editing: editingStep, Zoom: 1}, "")
	drawApp(a)
	if p.editor != nil || p.reopen != nil {
		t.Errorf("a step that is gone opened %T", p.editor)
	}
}

func TestADraftKeepsNoSecret(t *testing.T) {
	a := testApp(t)
	saveFlow(t, "login", "variables: {token: {type: text, default: hunter2, secret: true}}\nsteps: [{run: 'true'}]\n")
	a.reloadFlows()
	table := a.newParamsTable(a.flows[0])
	content, _ := json.Marshal(table.grid.draft())
	if strings.Contains(string(content), "hunter2") {
		t.Errorf("the draft keeps the secret: %s", content)
	}
	table.grid.at(0, paramNameColumn).field = newTextField("renamed")
	d := table.grid.draft()
	fresh := a.newParamsTable(a.flows[0])
	applyDraft(fresh, d)
	if name, secret := fresh.grid.at(0, paramNameColumn).value(), fresh.grid.at(0, paramDefaultColumn).value(); name != "renamed" || secret != "hunter2" {
		t.Errorf("restored %q with default %q, want renamed keeping hunter2", name, secret)
	}
}

func TestTheLogsPanelOpensAgainAsItWasLeft(t *testing.T) {
	a := testApp(t)
	p := newLogsPanel(a, func() {})
	day := time.Date(2026, 3, 4, 0, 0, 0, 0, time.Local)
	p.day, p.allDays, p.order, p.shown, p.preview.times = day, true, logsBySize, logsListOnly, true
	rows := []logRow{{entry: logstore.Entry{Path: "a.log"}}, {entry: logstore.Entry{Path: "b.log"}}}
	p.selected = 1
	p.current(rows)
	state := p.savedState()

	q := newLogsPanel(a, func() {})
	q.restoreState(state, "exit:1")
	q.current(rows)
	if !q.day.Equal(day) || !q.allDays || q.order != logsBySize || q.shown != logsListOnly || !q.preview.times || q.filter != "exit:1" {
		t.Errorf("restored %+v, want what was saved", q.savedState())
	}
	if q.selected != 1 {
		t.Errorf("restored on row %d, want b.log's", q.selected)
	}
}
