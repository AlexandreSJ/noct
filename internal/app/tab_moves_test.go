package app

import (
	"slices"
	"testing"

	"github.com/AlexandreSJ/noct/internal/config"
)

func TestANewTabOpensRightAfterTheCurrentOne(t *testing.T) {
	first, second, third := newTab(1, &fakeView{}), newTab(2, &fakeView{}), newTab(3, &fakeView{})
	a := testApp(t, first, second, third)
	a.config.NewTab = config.NewTabEmpty
	a.session.activeTab = 0

	press(a, prefixPress, letter('t'))
	tabs := a.session.tabs
	if len(tabs) != 4 || tabs[0] != first || tabs[2] != second || a.session.activeTab != 1 || !tabs[1].isEmpty() {
		t.Errorf("tabs %v, active %d: want the new tab second, and on it", tabs, a.session.activeTab)
	}
}

func TestMoveTabCarriesTheTabAlongTheBar(t *testing.T) {
	first, second, third := newTab(1, &fakeView{}), newTab(2, &fakeView{}), newTab(3, &fakeView{})
	a := testApp(t, first, second, third)

	press(a, prefixPress, letter('m'))
	if a.mode != tabMoveMode {
		t.Fatalf("mode %v after prefix+m", a.mode)
	}
	press(a, rightPress, letter('l'), rightPress) // the third press finds the end
	if want := []*tab{second, third, first}; !slices.Equal(a.session.tabs, want) || a.session.activeTab != 2 {
		t.Errorf("tabs %v, active %d, want the first tab last and on it", a.session.tabs, a.session.activeTab)
	}
	press(a, leftPress, enterPress)
	if want := []*tab{second, first, third}; !slices.Equal(a.session.tabs, want) || a.session.activeTab != 1 || a.mode != normalMode {
		t.Errorf("tabs %v, active %d, mode %v", a.session.tabs, a.session.activeTab, a.mode)
	}
}
