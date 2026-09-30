package app

import (
	"image"
	"strconv"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
)

// mouseView is a terminal whose program asked for the mouse.
type mouseView struct {
	fakeView
	events []uv.MouseEvent
}

func (v *mouseView) WantsMouseAt(image.Point) bool { return true }
func (v *mouseView) SendMouse(event uv.MouseEvent) { v.events = append(v.events, event) }

func click(a *App, x, y int) {
	a.handleEvent(uv.MouseClickEvent{X: x, Y: y, Button: uv.MouseLeft})
}

func TestClickingATabSelectsIt(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}), newTab(2, &fakeView{}))
	a.session.tabs[0].name, a.session.tabs[1].name = "one", "two"
	click(a, len(" 1 one  2 "), 0)
	if a.session.activeTab != 1 {
		t.Errorf("active tab %d after clicking the second label", a.session.activeTab)
	}
}

func TestClickingAPaneFocusesItAndReachesItsProgram(t *testing.T) {
	right := &mouseView{}
	a := testApp(t, newTab(1, &fakeView{}))
	a.currentTab().tree.Split(1, 2, layout.SideBySide)
	a.currentTab().views[2] = right

	click(a, 70, 10)
	if focusedPaneID(a) != 2 {
		t.Fatalf("focus %d after clicking the right pane", focusedPaneID(a))
	}
	if len(right.events) != 1 || right.events[0].Mouse().X != 70-51 || right.events[0].Mouse().Y != 10-2 {
		t.Errorf("program got %v, want the click relative to its content", right.events)
	}
	click(a, 10, 10)
	if focusedPaneID(a) != 1 || len(right.events) != 1 {
		t.Errorf("focus %d, right pane events %d", focusedPaneID(a), len(right.events))
	}
}

func TestDraggingTheDividerResizesThePanes(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	left := a.currentTab().views[1].(*fakeView)

	click(a, 49, 10)
	a.handleEvent(uv.MouseMotionEvent{X: 30, Y: 10, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseReleaseEvent{X: 30, Y: 10, Button: uv.MouseLeft})
	if left.size != (pty.Size{Cols: 28, Rows: 27}) {
		t.Errorf("left pane is %+v after dragging the divider to column 30", left.size)
	}
	a.handleEvent(uv.MouseMotionEvent{X: 60, Y: 10, Button: uv.MouseLeft})
	if left.size.Cols != 28 {
		t.Error("the divider kept following the mouse after the release")
	}
}

func TestWheelScrollsTheHistoryAndReturnsToLive(t *testing.T) {
	a := testApp(t, newTab(1, &scrollableView{history: 20}))
	a.handleEvent(uv.MouseWheelEvent{X: 10, Y: 10, Button: uv.MouseWheelUp})
	if a.mode != scrollMode || a.scrollOffset != wheelLines {
		t.Fatalf("mode %v offset %d after wheel up", a.mode, a.scrollOffset)
	}
	a.handleEvent(uv.MouseWheelEvent{X: 10, Y: 10, Button: uv.MouseWheelDown})
	if a.mode != normalMode || a.scrollOffset != 0 {
		t.Errorf("mode %v offset %d after scrolling back down", a.mode, a.scrollOffset)
	}
}

func TestMouseOffIgnoresClicks(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}), newTab(2, &fakeView{}))
	a.config.Mouse = false
	click(a, 1, 0)
	if a.session.activeTab != 0 {
		t.Error("a click selected a tab with the mouse off")
	}
}

func TestClickingAScriptSelectsIt(t *testing.T) {
	a := testApp(t)
	saveScript(t, "alpha", "run: true\n")
	saveScript(t, "beta", "run: true\n")
	panel := openScriptsPanel(t, a)
	a.draw(uv.NewScreenBuffer(a.size.X, a.size.Y), image.Rect(0, 0, a.size.X, a.size.Y))

	click(a, 5, 3) // the list's second row: the frame, then alpha
	if panel.selected != 1 {
		t.Errorf("selected %d after clicking beta", panel.selected)
	}
}

func TestDirectShortcutsRunCommandsWithoutThePrefix(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	a.config.Shortcut = config.Shortcut{Ctrl: true, Alt: true}
	a.shortcut = shortcutFromConfig(a.config.Shortcut)
	ctrlAlt := uv.ModCtrl | uv.ModAlt

	press(a, uv.KeyPressEvent{Code: uv.KeyRight, Mod: ctrlAlt})
	if a.mode != normalMode || focusedPaneID(a) != 2 {
		t.Fatalf("ctrl+alt+right: mode %v focus %d", a.mode, focusedPaneID(a))
	}
	press(a, uv.KeyPressEvent{Code: uv.KeyLeft, Mod: ctrlAlt | uv.ModShift})
	if panes := a.currentTab().tree.Panes(); panes[0] != 2 {
		t.Errorf("ctrl+alt+shift+left did not move the pane: %v", panes)
	}
	press(a, uv.KeyPressEvent{Code: 't', Mod: ctrlAlt})
	if len(a.session.tabs) != 2 {
		t.Errorf("ctrl+alt+t: %d tabs", len(a.session.tabs))
	}

	focused := a.currentTab().focusedView().(*fakeView)
	enterAlt := uv.KeyPressEvent{Code: uv.KeyEnter, Mod: uv.ModAlt} // what ctrl+alt+m arrives as
	press(a, enterAlt, uv.KeyPressEvent{Code: 'z', Mod: ctrlAlt})
	if len(focused.keys) != 2 {
		t.Errorf("keys that are not shortcuts did not reach the program: %v", focused.keys)
	}

	a.config.Shortcut.Ctrl, a.config.Shortcut.Alt = false, false
	a.shortcut = shortcutFromConfig(a.config.Shortcut)
	press(a, uv.KeyPressEvent{Code: 't', Mod: ctrlAlt})
	if len(a.session.tabs) != 2 {
		t.Error("a shortcut ran while they are off")
	}
}

// textView shows fixed lines, like a terminal after a command printed them.
type textView struct {
	fakeView
	lines []string
}

func (v *textView) Draw(screen uv.Screen, area uv.Rectangle) {
	for i, line := range v.lines {
		drawText(screen, area.Min.X, area.Min.Y+i, area.Max.X, line, uv.Style{})
	}
}

func TestDraggingOverTextSelectsAndCopiesIt(t *testing.T) {
	a := testApp(t, newTab(1, &textView{lines: []string{"hello world", "second line"}}))
	// The content starts at column 1, row 2: the tab bar and the frame come first.
	click(a, 1+6, 2)
	a.handleEvent(uv.MouseMotionEvent{X: 1 + 5, Y: 3, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseReleaseEvent{X: 1 + 5, Y: 3, Button: uv.MouseLeft})

	if got := a.selectedText(); got != "world\nsecond" {
		t.Errorf("selected %q", got)
	}
	if a.notice != "copied to the clipboard" {
		t.Errorf("notice %q", a.notice)
	}
	press(a, letter('x'))
	if a.selection != nil {
		t.Error("a key did not clear the selection")
	}
}

func TestClickWithoutDraggingSelectsNothing(t *testing.T) {
	a := testApp(t, newTab(1, &textView{lines: []string{"hello"}}))
	click(a, 3, 2)
	a.handleEvent(uv.MouseReleaseEvent{X: 3, Y: 2, Button: uv.MouseLeft})
	if a.selection != nil {
		t.Error("a plain click left a selection")
	}
}

func TestProgramsWithTheMouseGetDragsInsteadOfSelection(t *testing.T) {
	program := &mouseView{}
	a := testApp(t, newTab(1, program))
	click(a, 5, 5)
	a.handleEvent(uv.MouseMotionEvent{X: 8, Y: 5, Button: uv.MouseLeft})
	if a.selection != nil || len(program.events) != 2 {
		t.Errorf("selection %v, program got %d events", a.selection, len(program.events))
	}
}

func TestAltIsTheDefaultShortcut(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, uv.KeyPressEvent{Code: 'a', Mod: uv.ModAlt})
	if a.mode != moveMode {
		t.Fatalf("alt+a: mode %v, want move", a.mode)
	}
	press(a, escPress, uv.KeyPressEvent{Code: uv.KeyRight, Mod: uv.ModAlt | uv.ModShift})
	if panes := a.currentTab().tree.Panes(); panes[1] != 1 {
		t.Errorf("alt+shift+right did not move the pane: %v", panes)
	}
}

// historyView is a terminal whose lines scrolled into a history, the last ones on screen.
type historyView struct {
	fakeView
	lines []string
}

func numberedLines(count int) []string {
	lines := make([]string, count)
	for i := range lines {
		lines[i] = "line " + strconv.Itoa(i+1)
	}
	return lines
}

func (v *historyView) ScrollbackLen() int { return max(len(v.lines)-v.size.Rows, 0) }
func (v *historyView) Draw(screen uv.Screen, area uv.Rectangle) {
	v.DrawScrolled(screen, area, 0)
}

func (v *historyView) DrawScrolled(screen uv.Screen, area uv.Rectangle, offset int) {
	history := v.ScrollbackLen()
	first := history - min(max(offset, 0), history)
	for y := 0; y < min(area.Dy(), v.size.Rows) && first+y < len(v.lines); y++ {
		drawText(screen, area.Min.X, area.Min.Y+y, area.Max.X, v.lines[first+y], uv.Style{})
	}
}

func TestSelectionKeepsGoingWhileTheWheelScrolls(t *testing.T) {
	v := &historyView{lines: numberedLines(100)}
	a := testApp(t, newTab(1, v))
	a.currentTab().resizeViews(a.workspaceArea()) // 27 rows on screen: lines 74 to 100

	click(a, 1, 2+26) // line 100, the last row
	for range 10 {
		a.handleEvent(uv.MouseWheelEvent{X: 1, Y: 2, Button: uv.MouseWheelUp})
	}
	if a.mode != scrollMode || a.scrollOffset != 10*wheelLines {
		t.Fatalf("mode %v offset %d while selecting", a.mode, a.scrollOffset)
	}
	a.handleEvent(uv.MouseReleaseEvent{X: 1, Y: 2, Button: uv.MouseLeft})

	text := a.selectedText()
	lines := strings.Split(text, "\n")
	if lines[0] != "line 44" || lines[len(lines)-1] != "l" || len(lines) != 57 {
		t.Errorf("selected %d lines, from %q to %q", len(lines), lines[0], lines[len(lines)-1])
	}
}

func TestDraggingPastTheTopScrollsTheHistory(t *testing.T) {
	v := &historyView{lines: numberedLines(100)}
	a := testApp(t, newTab(1, v))
	a.currentTab().resizeViews(a.workspaceArea())

	click(a, 1, 2+5)
	for range 4 {
		a.handleEvent(uv.MouseMotionEvent{X: 1, Y: 0, Button: uv.MouseLeft})
	}
	a.handleEvent(uv.MouseReleaseEvent{X: 1, Y: 0, Button: uv.MouseLeft})
	if first := strings.Split(a.selectedText(), "\n")[0]; first != "line 70" {
		t.Errorf("the selection starts at %q, want 4 lines above the screen", first)
	}
}

func TestRunStatusLineIsNotSelected(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveScript(t, "hi", "run: echo hi\n")
	openScriptsPanel(t, a)
	press(a, enterPress)
	run := a.currentTab().focusedView().(*runView)
	run.terminal = &textView{lines: []string{"hi"}}
	l.last(t).onExit(0)

	frame, _ := a.currentTab().frameOf(focusedPaneID(a), a.workspaceArea())
	content := contentRect(frame)
	click(a, content.Min.X, content.Min.Y)
	a.handleEvent(uv.MouseMotionEvent{X: content.Max.X - 1, Y: content.Max.Y - 1, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseReleaseEvent{X: content.Max.X - 1, Y: content.Max.Y - 1, Button: uv.MouseLeft})
	if text := a.selectedText(); strings.Contains(text, "done") || !strings.HasPrefix(text, "hi") {
		t.Errorf("selected %q", text)
	}
}

func TestPrefixLeavesScrollingToChangePane(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	a.currentTab().views[1] = &historyView{lines: numberedLines(100)}
	a.currentTab().resizeViews(a.workspaceArea())
	press(a, prefixPress, letter('['), upPress)
	if a.mode != scrollMode {
		t.Fatalf("mode %v, want scrolling", a.mode)
	}
	press(a, prefixPress, rightPress, escPress)
	if focusedPaneID(a) != 2 || a.mode != normalMode {
		t.Errorf("focus %d mode %v after prefix+right while scrolling", focusedPaneID(a), a.mode)
	}
}
