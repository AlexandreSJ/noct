package app

import (
	"slices"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
)

// A kind of panel needs its key wired like the others: listed in the commands popup and taken as
// a command, with or without the prefix.
func TestEveryPanelKindHasItsKey(t *testing.T) {
	for _, kind := range panelKinds {
		listed := slices.ContainsFunc(commandShortcuts, func(s shortcut) bool { return s.keys == kind.key })
		if !listed || !slices.Contains(actionKeys, kind.key) {
			t.Errorf("%s: key %q listed %v, in actionKeys %v", kind.name, kind.key, listed, slices.Contains(actionKeys, kind.key))
		}
	}
}

func TestNewTabOpensThePanelTheConfigChooses(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.config.NewTab = "scripts"
	press(a, prefixPress, letter('t'))
	if _, ok := a.currentTab().focusedView().(*scriptsPanel); !ok || a.session.activeTab != 1 {
		t.Fatalf("active tab %d with %T, want a new tab with the scripts panel", a.session.activeTab, a.currentTab().focusedView())
	}
	a.config.NewTab = "gone"
	press(a, prefixPress, letter('t'))
	if _, ok := a.currentTab().focusedView().(*fakeView); !ok {
		t.Errorf("a panel noct does not have opened %T, want a terminal", a.currentTab().focusedView())
	}
}

// An empty new tab shows the home; a panel fills it, and x or ctrl+d close it without quitting.
func TestEmptyNewTab(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.config.NewTab = config.NewTabEmpty
	press(a, prefixPress, letter('t'))
	if len(a.session.tabs) != 2 || !a.currentTab().isEmpty() {
		t.Fatalf("tabs=%d, empty=%v after an empty new tab", len(a.session.tabs), a.currentTab().isEmpty())
	}
	press(a, prefixPress, letter('p'))
	if len(a.session.tabs) != 2 || len(a.currentTab().views) != 1 {
		t.Fatalf("tabs=%d views=%d, want the terminal in the empty tab", len(a.session.tabs), len(a.currentTab().views))
	}

	press(a, prefixPress, letter('t'), prefixPress, letter('x'))
	if len(a.session.tabs) != 2 || a.mode != normalMode {
		t.Errorf("tabs=%d mode %v after x in an empty tab", len(a.session.tabs), a.mode)
	}
	press(a, prefixPress, letter('t'), prefixPress, letter('w'))
	if !a.currentTab().isEmpty() {
		t.Error("stacking an empty tab gave it panes")
	}
	ctrlD := uv.KeyPressEvent{Code: 'd', Mod: uv.ModCtrl}
	press(a, ctrlD, ctrlD)
	if len(a.session.tabs) != 2 || a.quitting {
		t.Errorf("tabs=%d quitting=%v after ctrl+d in an empty tab", len(a.session.tabs), a.quitting)
	}
}

func TestConfigCyclesEveryKindOfPanelForANewTab(t *testing.T) {
	a := openConfigAt(t, newTabRow)
	a.config.NewTab = config.NewTabTerminal
	var seen []string
	for range len(panelKinds) + 1 {
		seen = append(seen, a.config.NewTab)
		press(a, letter('l'))
	}
	var want []string
	for _, k := range panelKinds {
		want = append(want, k.name)
	}
	want = append(want, config.NewTabEmpty)
	if !slices.Equal(seen, want) {
		t.Errorf("new tab choices %v, want %v", seen, want)
	}
	if label := a.newTabLabel("flows"); label != "flows ("+a.keysFor("i")+")" {
		t.Errorf("flows is labeled %q", label)
	}
}

// With close when empty off, closing a tab's last pane leaves the tab, showing the home, where a
// new panel fills it and ctrl+d closes it.
func TestEmptyTabsStayWhenTheConfigKeepsThem(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}), newTab(2, &fakeView{}))
	a.config.CloseEmptyTabs = false
	a.session.activeTab = 1
	press(a, prefixPress, letter('x'))
	if len(a.session.tabs) != 2 || a.session.activeTab != 1 || !a.currentTab().isEmpty() {
		t.Fatalf("tabs=%d active=%d after closing the last pane, want the empty tab kept", len(a.session.tabs), a.session.activeTab)
	}
	press(a, prefixPress, letter('s'))
	if _, ok := a.currentTab().focusedView().(*scriptsPanel); !ok || len(a.session.tabs) != 2 {
		t.Fatalf("tabs=%d with %T, want the scripts panel in the kept tab", len(a.session.tabs), a.currentTab().focusedView())
	}
	a.detachView(a.currentTab().focused) // the panel closing by itself, as ctrl+d in it does
	press(a, uv.KeyPressEvent{Code: 'd', Mod: uv.ModCtrl})
	if len(a.session.tabs) != 1 || a.quitting {
		t.Errorf("tabs=%d quitting=%v after ctrl+d in the kept tab", len(a.session.tabs), a.quitting)
	}
}
