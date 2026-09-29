package app

import (
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
)

func flowNamed(a *App, name string) (flow.Flow, bool) {
	for _, f := range a.flows {
		if f.Name == name {
			return f, true
		}
	}
	return flow.Flow{}, false
}

// A draft the AI writes through the MCP shows in the open inspect panel without reopening it,
// and an unchanged library is not read again.
func TestOpenInspectFollowsFlowsChangedOnDisk(t *testing.T) {
	a := testApp(t)
	pressDrawn(a, prefixPress, letter('i'))
	if a.checkLibrary() {
		t.Error("read the library again though nothing changed")
	}
	f, err := flow.Create(flow.Dir(), "drafted")
	if err != nil {
		t.Fatal(err)
	}
	if !a.checkLibrary() {
		t.Fatal("a new flow did not reload the flows")
	}
	if got, ok := flowNamed(a, "drafted"); !ok || got.Draft {
		t.Fatalf("flows = %v, want drafted, not a draft", a.flows)
	}
	f.Draft = true
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	a.checkLibrary()
	if got, _ := flowNamed(a, "drafted"); !got.Draft {
		t.Error("the flow turned draft on disk but not in the panel")
	}
}

func TestOpenScriptsPanelFollowsScriptsChangedOnDisk(t *testing.T) {
	a := testApp(t)
	pressDrawn(a, prefixPress, letter('s'))
	loaded := a.scriptsLoaded
	a.checkLibrary()
	if a.scriptsLoaded != loaded {
		t.Error("read the scripts again though nothing changed")
	}
	saveScript(t, "fresh", "run: echo hi\n")
	a.checkLibrary()
	if _, ok := a.scripts.Find("fresh"); !ok {
		t.Error("a new script did not show in the scripts panel")
	}
}

// Without a panel showing them, scripts and flows are not looked at.
func TestLibraryIsNotCheckedWithoutAPanelShowingIt(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	if _, err := flow.Create(flow.Dir(), "unseen"); err != nil {
		t.Fatal(err)
	}
	if a.checkLibrary() {
		t.Error("checked the library with no panel showing it")
	}
}
