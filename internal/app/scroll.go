package app

import (
	"fmt"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// scrollKey is the noct mode key that scrolls the focused terminal's history.
const scrollKey = "g"

// scrollable is a view with history to page through: terminals are, noct's own panels are not.
type scrollable interface {
	ScrollbackLen() int
	DrawScrolled(screen uv.Screen, area uv.Rectangle, offset int)
}

// fullScreener is a terminal that tells whether a full-screen program shows.
type fullScreener interface {
	FullScreen() bool
}

// scrollStep is where a key moves the offset up a history, as in vim; false for keys that do
// not scroll. page is how many lines a page has.
func scrollStep(name string, offset, page, history int) (int, bool) {
	switch name {
	case "up", "k", "ctrl+y":
		offset++
	case "down", "j", "ctrl+e":
		offset--
	case "ctrl+u":
		offset += max(page/2, 1)
	case "ctrl+d":
		offset -= max(page/2, 1)
	case "pgup", "ctrl+b":
		offset += page
	case "pgdown", "ctrl+f":
		offset -= page
	case "home", "g":
		offset = history
	case "end", "G", "shift+g":
		offset = 0
	default:
		return offset, false
	}
	return min(max(offset, 0), history), true
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

// handleScrollKey pages through the focused terminal's history with vim's keys and the arrows;
// any other key returns to live. The prefix and the direct shortcuts leave scrolling and work as
// usual, so moving to another pane never needs scrolling back first; the scroll shortcut itself
// only leaves.
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
		if name != scrollKey {
			a.runShortcut(name, move)
		}
		return
	}
	if isModifierKey(key) {
		return
	}
	page := max(a.workspaceArea().Dy()-3, 1) // the frame takes two rows; one line of overlap
	offset, ok := scrollStep(key.String(), a.scrollOffset, page, s.ScrollbackLen())
	if !ok {
		a.mode = normalMode
		return
	}
	a.scrollOffset = offset
}

// isModifierKey is a modifier pressed alone, which terminals with the kitty keyboard protocol
// report.
func isModifierKey(key uv.KeyPressEvent) bool {
	return key.Code >= uv.KeyLeftShift && key.Code <= uv.KeyIsoLevel5Shift
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
