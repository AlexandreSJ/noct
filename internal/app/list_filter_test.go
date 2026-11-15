package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// f on the settings opens the filter: what is typed keeps the rows that hold it, with their
// sections; enter keeps it while the arrows move among them, and esc clears it before closing.
func TestTheConfigFiltersItsSettings(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	a.openConfig()
	press(a, letter('f'))
	typeText(a, "fullscreen")
	rows := a.selectableConfigRows()
	if !slices.Contains(rows, fullscreenHintRow) || slices.Contains(rows, themeRow) {
		t.Fatalf("fullscreen keeps rows %v", rows)
	}
	if a.configPanel.selected != rows[0] {
		t.Errorf("selected %d, not the first row kept %d", a.configPanel.selected, rows[0])
	}
	text := screenText(a)
	for _, want := range []string{"filter fullscreen", "Appearance", "fullscreen hint"} {
		if !strings.Contains(text, want) {
			t.Errorf("the screen lacks %q:\n%s", want, text)
		}
	}
	press(a, enterPress)
	a.selectConfigRow(fullscreenHintRow)
	press(a, letter('l')) // the filter no longer types: l changes the setting
	if a.config.Appearance.FullscreenHint {
		t.Error("l after enter did not turn the hint off")
	}
	press(a, escPress)
	if a.mode != configMode || a.configPanel.filter.on() {
		t.Fatalf("esc cleared the filter? mode %v, filter %+v", a.mode, a.configPanel.filter)
	}
	if !slices.Contains(a.selectableConfigRows(), themeRow) {
		t.Error("the cleared filter still hides the theme")
	}
	press(a, escPress)
	if a.mode != normalMode {
		t.Error("a second esc did not close the config")
	}
}

// Words match anywhere in a row, its notes or its section, in any case; nothing matching selects
// nothing to change.
func TestTheConfigFilterMatchesEveryWord(t *testing.T) {
	a := testApp(t)
	a.openConfig()
	press(a, letter('/'))
	typeText(a, "LOGS size")
	rows := a.selectableConfigRows()
	if !slices.Contains(rows, logsSizeRow) || slices.Contains(rows, logsTerminalsRow) {
		t.Errorf("logs size keeps %v", rows)
	}
	typeText(a, " nothing-has-this")
	if rows := a.selectableConfigRows(); len(rows) != 0 {
		t.Fatalf("kept %v", rows)
	}
	before := a.config
	press(a, enterPress, rightPress, downPress)
	if a.config.Logs.MaxSizeKB != before.Logs.MaxSizeKB || a.config.Logs.KeepDays != before.Logs.KeepDays {
		t.Error("a key changed a setting the filter hides")
	}
	if !strings.Contains(screenText(a), "nothing matches") {
		t.Error("the filter does not say nothing matches")
	}
}

// The binds page filters the actions by name, screen or key; f while a key is taken is the key.
func TestTheBindsPageFiltersActions(t *testing.T) {
	a := openBindsPage(t)
	press(a, letter('f'))
	typeText(a, "zoom")
	for _, row := range a.selectableConfigRows() {
		if b, ok := bindRowAt(row); ok && !strings.Contains(b.action.label, "zoom") {
			t.Errorf("zoom kept %q", b.action.label)
		}
	}
	if len(a.selectableConfigRows()) == 0 {
		t.Fatal("zoom kept nothing")
	}
	press(a, enterPress, enterPress, letter('f'))
	if c := a.configPanel.bindCapture; c.pending != "f" && !c.choosing {
		t.Errorf("f while taking a key was not taken: %+v", c)
	}
}

// ? on the binds page lists its keys, and closing the list goes back to the page.
func TestTheBindsPageListsItsKeys(t *testing.T) {
	a := openBindsPage(t)
	press(a, letter('?'))
	if a.mode != infoMode || !strings.Contains(screenText(a), "add a key to the action") {
		t.Fatalf("? shows mode %v:\n%s", a.mode, screenText(a))
	}
	press(a, letter('q'))
	if a.mode != configMode || a.configPanel.page != bindsPage {
		t.Errorf("closing the keys went to mode %v, page %v", a.mode, a.configPanel.page)
	}
}

// The default agent's row steps through auto and the agents, saving each; on custom agent, enter
// asks for its name and command, adds it to [agents] and makes it the default.
func TestTheConfigSetsTheDefaultAgent(t *testing.T) {
	a := testApp(t)
	a.openConfig()
	a.selectConfigRow(agentRow)
	press(a, rightPress)
	if a.config.Agent == flow.AutoAgent {
		t.Fatalf("right left the agent at auto")
	}
	for range len(flow.AgentNames(nil)) {
		press(a, rightPress)
	}
	if a.shownAgent() != customAgent {
		t.Fatalf("after the agents, the row shows %q", a.shownAgent())
	}
	saved := a.config.Agent
	press(a, enterPress)
	typeText(a, "mine")
	press(a, enterPress)
	typeText(a, "echo {prompt}")
	press(a, enterPress)
	if a.mode != configMode || a.config.Agent != "mine" || a.config.Agents["mine"] != "echo {prompt}" {
		t.Fatalf("after adding: mode %v, agent %q, agents %v (was %q)", a.mode, a.config.Agent, a.config.Agents, saved)
	}
	if !strings.Contains(screenText(a), "< mine >") {
		t.Errorf("the row does not show mine:\n%s", screenText(a))
	}
}

// esc while naming a custom agent goes back to the config, with the default as it was.
func TestCancellingACustomAgentGoesBackToTheConfig(t *testing.T) {
	a := testApp(t)
	a.openConfig()
	a.selectConfigRow(agentRow)
	press(a, leftPress, enterPress, escPress)
	if a.mode != configMode || a.config.Agent != flow.AutoAgent {
		t.Errorf("mode %v, agent %q after esc", a.mode, a.config.Agent)
	}
}

// f in the scripts panel keeps the scripts whose name, description or folder hold what is
// typed, their folders open; esc gives the whole list back.
func TestTheScriptsPanelFilters(t *testing.T) {
	a := testApp(t)
	saveScript(t, "greet", greetScript)
	saveScript(t, "deploy", "run: echo deploy\ndescription: ship it\n")
	panel := openScriptsPanel(t, a)
	press(a, letter('f'))
	typeText(a, "ship")
	rows := panel.rows()
	if len(rows) != 1 || rows[0].script == nil || rows[0].script.Name != "deploy" {
		t.Fatalf("ship kept %d rows", len(rows))
	}
	press(a, enterPress, escPress)
	if len(panel.rows()) != 2 || panel.filter.on() {
		t.Errorf("esc left %d rows, filter %+v", len(panel.rows()), panel.filter)
	}
}

// f in the inspect keeps the flows whose name holds what is typed; the keys type in the filter
// rather than act, so r types an r.
func TestTheFlowsListFilters(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveFlow(t, "build", threeSteps)
	saveFlow(t, "release", threeSteps)
	pressDrawn(a, prefixPress, letter('i'))
	p := a.currentTab().focusedView().(*inspectPanel)
	pressDrawn(a, letter('f'), letter('r'), letter('e'))
	if f, _, _ := p.current(); len(p.shown()) != 1 || f.Name != "release" {
		t.Fatalf("re kept %d, selected %q", len(p.shown()), f.Name)
	}
	if len(l.programs) != 0 {
		t.Errorf("typing r ran %v", l.programs)
	}
	if !strings.Contains(screenText(a), "filter re") {
		t.Errorf("the list does not show the filter:\n%s", screenText(a))
	}
	pressDrawn(a, escPress)
	if len(p.shown()) != 2 {
		t.Errorf("esc left %d flows", len(p.shown()))
	}
}

// A word in braces that is no key, as {prompt}, stays as written in hints.
func TestHintsKeepBracesThatAreNoKey(t *testing.T) {
	a := testApp(t)
	if got := a.hint("prompt", "{enter} add  in place of {prompt}"); got != "enter add  in place of {prompt}" {
		t.Errorf("hint %q", got)
	}
}

// A changelog's paragraph and list items, broken in the file, read as one line each.
func TestChangelogSoftBreaksJoin(t *testing.T) {
	got := joinSoftBreaks([]string{"Flows grow up. A step", "can ask.", "", "### Added", "", "- one", "  two", "- three"})
	want := []string{"Flows grow up. A step can ask.", "", "### Added", "", "- one two", "- three"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
