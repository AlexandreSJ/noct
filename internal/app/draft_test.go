package app

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// A draft shows as one, does not run, and P promotes it after asking.
func TestInspectPromotesDrafts(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveFlow(t, "drafted", "draft: true\nsteps: [{run: 'true'}]\n")
	pressDrawn(a, prefixPress, letter('i'))
	if !regexp.MustCompile(`✎ +drafted`).MatchString(screenText(a)) {
		t.Errorf("the draft is not marked:\n%s", screenText(a))
	}
	pressDrawn(a, letter('r'))
	if len(l.programs) != 0 || !strings.Contains(a.statusMessage, "draft") {
		t.Fatalf("r on a draft: %d started, status %q", len(l.programs), a.statusMessage)
	}
	pressDrawn(a, letter('P'), enterPress)
	if f, _ := flow.Load(filepath.Join(flow.Dir(), "drafted"+flow.Ext)); f.Draft {
		t.Error("P did not promote the draft")
	}
}

// A script drafted by an AI does not run from the scripts panel until promoted.
func TestScriptsPanelPromotesDrafts(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveScript(t, "fresh", "run: echo hi\ndraft: true\n")
	pressDrawn(a, prefixPress, letter('s'), enterPress)
	if len(l.programs) != 0 || !strings.Contains(a.statusMessage, "draft") {
		t.Fatalf("enter on a draft: %d started, status %q", len(l.programs), a.statusMessage)
	}
	pressDrawn(a, letter('P'), enterPress)
	a.reloadScripts()
	if s, _ := a.scripts.Find("fresh"); s.Draft {
		t.Error("P did not promote the script")
	}
}
