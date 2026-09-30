package app

import (
	"fmt"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// scrollable is a view with history to page through: terminals are, noct's own panels are not.
type scrollable interface {
	ScrollbackLen() int
	DrawScrolled(screen uv.Screen, area uv.Rectangle, offset int)
}

func (a *App) focusedScrollable() (scrollable, bool) {
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		return nil, false
	}
	s, ok := t.focusedView().(scrollable)
	return s, ok
}

func (a *App) startScroll() {
	s, ok := a.focusedScrollable()
	if !ok || s.ScrollbackLen() == 0 {
		a.statusMessage = "no history to scroll"
		a.mode = normalMode
		return
	}
	a.scrollOffset = 0
	a.mode = scrollMode
}

// handleScrollKey pages through the focused terminal's history; any exit key returns to live.
// The prefix and the direct shortcuts leave scrolling and work as usual, so moving to another
// pane never needs scrolling back first.
func (a *App) handleScrollKey(key uv.KeyPressEvent) {
	s, ok := a.focusedScrollable()
	if !ok {
		a.mode = normalMode
		return
	}
	if a.prefix.matches(key) {
		a.mode = commandMode
		return
	}
	if name, move, ok := a.shortcut.command(key, a.commandKey); ok {
		a.mode = normalMode
		a.runShortcut(name, move)
		return
	}
	page := max(a.workspaceArea().Dy()-3, 1) // the frame takes two rows; one line of overlap
	switch key.String() {
	case "up", "k":
		a.scrollTo(s, a.scrollOffset+1)
	case "down", "j":
		a.scrollTo(s, a.scrollOffset-1)
	case "pgup", "ctrl+b":
		a.scrollTo(s, a.scrollOffset+page)
	case "pgdown", "ctrl+f":
		a.scrollTo(s, a.scrollOffset-page)
	case "home", "g":
		a.scrollTo(s, s.ScrollbackLen())
	case "end", "G", "shift+g":
		a.scrollTo(s, 0)
	case "esc", "q", "enter":
		a.mode = normalMode
	}
}

func (a *App) scrollTo(s scrollable, offset int) {
	a.scrollOffset = min(max(offset, 0), s.ScrollbackLen())
}

// scrollPosition labels the focused pane while scrolling, e.g. "↑ 120/2000".
func (a *App) scrollPosition() string {
	s, ok := a.focusedScrollable()
	if !ok {
		return ""
	}
	return fmt.Sprintf("↑ %d/%d", a.scrollOffset, s.ScrollbackLen())
}
