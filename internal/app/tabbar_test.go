package app

import (
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

func spanOf(spans []tabSpan, tab int) (tabSpan, bool) {
	for _, span := range spans {
		if span.tab == tab && !span.mark {
			return span, true
		}
	}
	return tabSpan{}, false
}

func TestTabBarKeepsNamesThatFit(t *testing.T) {
	spans, _ := layoutTabBar([]string{"one", "two"}, 0, 0, 80)
	if len(spans) != 2 || spans[0].text != " 1 one " || spans[1].text != " 2 two " {
		t.Errorf("spans %+v, want both labels whole", spans)
	}
}

func TestTabBarCutsLongNamesBeforeHidingTabs(t *testing.T) {
	names := []string{"a very long tab name", "short", "another very long name"}
	spans, first := layoutTabBar(names, 2, 0, 40)
	if first != 0 || len(spans) != 3 {
		t.Fatalf("first %d, spans %+v: want all three tabs", first, spans)
	}
	if spans[1].text != " 2 short " {
		t.Errorf("short name changed to %q", spans[1].text)
	}
	if !strings.Contains(spans[0].text, "…") || !strings.Contains(spans[2].text, "…") {
		t.Errorf("long names not cut: %q, %q", spans[0].text, spans[2].text)
	}
	if end := spans[len(spans)-1].end; end > 40 {
		t.Errorf("bar ends at %d, past 40", end)
	}
}

func TestTabBarScrollsToTheActiveTab(t *testing.T) {
	names := make([]string, 30)
	for index := range names {
		names[index] = "terminal name"
	}
	spans, first := layoutTabBar(names, 29, 0, 60)
	span, ok := spanOf(spans, 29)
	if !ok || span.end > 60 {
		t.Fatalf("active tab not in view: %+v", spans)
	}
	if first == 0 || !spans[0].mark || spans[0].tab != first-1 {
		t.Errorf("no mark for the hidden tabs on the left: first %d, %+v", first, spans[0])
	}
	if last := spans[len(spans)-1]; last.mark {
		t.Errorf("mark on the right with the last tab shown: %+v", last)
	}

	// Going back one tab keeps the bar where it is; going past its start moves it.
	if _, again := layoutTabBar(names, 28, first, 60); again != first {
		t.Errorf("bar moved from %d to %d for a tab already in view", first, again)
	}
	spans, first = layoutTabBar(names, 0, first, 60)
	if _, ok := spanOf(spans, 0); !ok || first != 0 || !spans[len(spans)-1].mark {
		t.Errorf("first tab not shown with a mark on the right: first %d, %+v", first, spans)
	}
}

func manyTabs(t *testing.T, count int) *App {
	t.Helper()
	tabs := make([]*tab, count)
	for index := range tabs {
		tabs[index] = newTab(1, &fakeView{})
		tabs[index].name = "a long terminal name"
	}
	return testApp(t, tabs...)
}

func TestClickingTheMarkSelectsTheHiddenTab(t *testing.T) {
	a := manyTabs(t, 20)
	a.session.activeTab = 19
	spans := a.tabBar()
	if !spans[0].mark {
		t.Fatalf("no left mark: %+v", spans[0])
	}
	click(a, 0, 0)
	if a.session.activeTab != spans[0].tab {
		t.Errorf("active tab %d after clicking the mark for %d", a.session.activeTab, spans[0].tab)
	}
	if _, ok := spanOf(a.tabBar(), a.session.activeTab); !ok {
		t.Errorf("tab %d selected by the mark is not shown", a.session.activeTab)
	}
}

func TestWheelOverTheTabBarChangesTab(t *testing.T) {
	a := manyTabs(t, 20)
	wheel := func(button uv.MouseButton) {
		a.handleEvent(uv.MouseWheelEvent{X: 5, Y: 0, Button: button})
	}
	wheel(uv.MouseWheelUp)
	if a.session.activeTab != 0 {
		t.Errorf("wheel up on the first tab went to %d", a.session.activeTab)
	}
	for range 19 {
		wheel(uv.MouseWheelDown)
		if _, ok := spanOf(a.tabBar(), a.session.activeTab); !ok {
			t.Fatalf("active tab %d not shown", a.session.activeTab)
		}
	}
	wheel(uv.MouseWheelDown)
	if a.session.activeTab != 19 {
		t.Errorf("wheel down past the last tab went to %d", a.session.activeTab)
	}
}

func TestTabBarLeavesRoomForTheMode(t *testing.T) {
	a := manyTabs(t, 20)
	a.session.activeTab = 19
	a.mode = commandMode
	spans := a.tabBar()
	room := a.size.X - len(modeBadges[commandMode]) - 2
	if end := spans[len(spans)-1].end; end > room {
		t.Errorf("tabs end at %d, over the mode badge from %d", end, room)
	}
}
