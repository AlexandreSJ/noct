package app

import (
	"fmt"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// minTabName is the fewest cells a tab's name shrinks to, the "…" included, before the tab bar
// scrolls instead of cutting names further.
const minTabName = 6

// Marks at the ends of the tab bar when tabs are hidden past them; clicking one selects the
// nearest hidden tab.
const (
	moreTabsLeft  = "‹"
	moreTabsRight = "›"
)

// tabSpan is a piece of the tab bar and the columns it covers: a tab's label, or a mark that
// selects the hidden tab next to it.
type tabSpan struct {
	start, end int
	text       string
	tab        int
	mark       bool
}

// layoutTabBar lays the tabs' names out in width cells. Long names are cut to one shared width,
// the widest that fits them all; when not even minTabName does, only a run of tabs shows, starting
// at first if the active tab stays in it, with a mark on each side that hides tabs. It gives the
// pieces to draw and the first tab shown, to start from next time so the bar does not jump.
func layoutTabBar(names []string, active, first, width int) ([]tabSpan, int) {
	if len(names) == 0 {
		return nil, 0
	}
	limit := longestName(names)
	for limit > minTabName && tabsWidth(names, limit, 0, len(names)-1) > width {
		limit--
	}
	labels := make([]string, len(names))
	for index, name := range names {
		labels[index] = tabBarLabel(index, ansi.Truncate(name, limit, "…"))
	}
	first = max(min(first, active), 0)
	for first < active && runWidth(labels, first, active) > width {
		first++
	}
	for first > 0 && runWidth(labels, first-1, lastShown(labels, first, width)) <= width {
		first-- // room freed by a wider screen or a closed tab: show the tabs before again
	}
	last := lastShown(labels, first, width)

	var spans []tabSpan
	x := 0
	add := func(text string, tab int, mark bool) {
		end := x + ansi.StringWidth(text)
		spans = append(spans, tabSpan{start: x, end: end, text: text, tab: tab, mark: mark})
		x = end
	}
	if first > 0 {
		add(moreTabsLeft, first-1, true)
	}
	for index := first; index <= last; index++ {
		add(labels[index], index, false)
	}
	if last < len(labels)-1 {
		add(moreTabsRight, last+1, true)
	}
	return spans, first
}

// tabBarLabel is how the tab bar shows a tab: its number and its name.
func tabBarLabel(index int, name string) string {
	return fmt.Sprintf(" %d %s ", index+1, name)
}

func longestName(names []string) int {
	longest := 0
	for _, name := range names {
		longest = max(longest, ansi.StringWidth(name))
	}
	return longest
}

// tabsWidth is how wide the labels of tabs first to last are with names cut to limit cells.
func tabsWidth(names []string, limit, first, last int) int {
	width := 0
	for index := first; index <= last; index++ {
		width += ansi.StringWidth(tabBarLabel(index, ansi.Truncate(names[index], limit, "…")))
	}
	return width
}

// runWidth is how wide showing labels first to last is, with the marks for tabs hidden around them.
func runWidth(labels []string, first, last int) int {
	width := 0
	if first > 0 {
		width += ansi.StringWidth(moreTabsLeft)
	}
	if last < len(labels)-1 {
		width += ansi.StringWidth(moreTabsRight)
	}
	for index := first; index <= last; index++ {
		width += ansi.StringWidth(labels[index])
	}
	return width
}

// lastShown is the last tab that fits in width when the bar starts at first; first itself always
// shows, even cut by the edge of a very narrow screen.
func lastShown(labels []string, first, width int) int {
	last := first
	for last+1 < len(labels) && runWidth(labels, first, last+1) <= width {
		last++
	}
	return last
}

// tabBar lays the tab bar out for the current tabs, keeping the active one in view.
func (a *App) tabBar() []tabSpan {
	names := make([]string, len(a.session.tabs))
	for index, t := range a.session.tabs {
		names[index] = a.tabLabel(t)
	}
	spans, first := layoutTabBar(names, a.session.activeTab, a.session.tabBarFirst, a.tabBarWidth())
	a.session.tabBarFirst = first
	return spans
}

// tabBarWidth is the room the tabs get: the screen's width, less what the status on the right
// takes, though never less than half of it.
func (a *App) tabBarWidth() int {
	status := ansi.StringWidth(a.tabBarStatus())
	if status == 0 {
		return a.size.X
	}
	return max(a.size.X-status-2, a.size.X/2)
}

func (a *App) drawTabBar(screen uv.Screen) {
	x := 0
	for _, span := range a.tabBar() {
		active, style := a.palette.tab, a.palette.tab
		if !span.mark {
			active, style = a.palette.forTab(a.session.tabs[span.tab].colors)
		}
		if span.tab == a.session.activeTab && !span.mark {
			style = active
		}
		x = drawText(screen, span.start, 0, a.size.X, span.text, style)
	}
	a.drawTabBarStatus(screen, x+2)
}

// tabAt is the tab that clicking column x of the tab bar selects: the tab under it, or the hidden
// tab next to the mark under it.
func (a *App) tabAt(x int) (int, bool) {
	for _, span := range a.tabBar() {
		if x >= span.start && x < span.end {
			return span.tab, true
		}
	}
	return 0, false
}

// wheelTabs moves to the tab before or after the active one as the wheel turns over the tab bar,
// stopping at the first and last tabs.
func (a *App) wheelTabs(button uv.MouseButton) {
	switch button {
	case uv.MouseWheelUp, uv.MouseWheelLeft:
		a.session.activeTab = max(a.session.activeTab-1, 0)
	case uv.MouseWheelDown, uv.MouseWheelRight:
		a.session.activeTab = min(a.session.activeTab+1, len(a.session.tabs)-1)
	}
}
