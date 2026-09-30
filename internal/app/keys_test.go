package app

import (
	"image"
	"path/filepath"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/script"
)

var (
	prefixPress = uv.KeyPressEvent{Code: uv.KeySpace, Mod: uv.ModCtrl}
	escPress    = uv.KeyPressEvent{Code: uv.KeyEscape}
	enterPress  = uv.KeyPressEvent{Code: uv.KeyEnter}
	leftPress   = uv.KeyPressEvent{Code: uv.KeyLeft}
	rightPress  = uv.KeyPressEvent{Code: uv.KeyRight}
	upPress     = uv.KeyPressEvent{Code: uv.KeyUp}
	downPress   = uv.KeyPressEvent{Code: uv.KeyDown}
	tabPress    = uv.KeyPressEvent{Code: uv.KeyTab}
	backTab     = uv.KeyPressEvent{Code: uv.KeyTab, Mod: uv.ModShift}
)

// fakeView stands in for a terminal so key handling runs without processes.
type fakeView struct {
	keys   []uv.KeyPressEvent
	size   pty.Size
	closed bool
}

func (v *fakeView) Draw(uv.Screen, uv.Rectangle) {}
func (v *fakeView) Resize(size pty.Size)         { v.size = size }
func (v *fakeView) SendKey(key uv.KeyPressEvent) { v.keys = append(v.keys, key) }
func (v *fakeView) Paste(string)                 {}
func (v *fakeView) Cursor() (uv.Position, bool)  { return image.Point{}, false }
func (v *fakeView) Title() string                { return "" }
func (v *fakeView) Close()                       { v.closed = true }

func letter(r rune) uv.KeyPressEvent {
	return uv.KeyPressEvent{Code: r, Text: string(r)}
}

func testApp(t *testing.T, tabs ...*tab) *App {
	t.Helper()
	current := &session{name: "1", tabs: tabs}
	a := &App{session: current, sessions: []*session{current}, shells: make(map[layout.PaneID]bool), size: image.Pt(100, 30), configPath: filepath.Join(t.TempDir(), "config.toml")}
	a.config = config.Default()
	a.prefix, _ = prefixFromConfig(a.config.Prefix) // the default prefix is always valid
	a.shortcut = shortcutFromConfig(a.config.Shortcut)
	a.nextPane = 100
	t.Setenv("XDG_DATA_HOME", t.TempDir())  // personal scripts
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // runs and logs
	a.workDir = t.TempDir()
	a.shellArgv = []string{"/bin/sh"}
	a.memory, _ = script.LoadMemory(filepath.Join(t.TempDir(), "params.json")) // a missing file is empty
	a.versions = script.NewVersions(nil)
	a.launch = func(program, pty.Size) (view, error) { return &fakeView{}, nil }
	return a
}

func sideBySideTab(left, right layout.PaneID) *tab {
	t := newTab(left, &fakeView{})
	t.tree.Split(left, right, layout.SideBySide)
	t.views[right] = &fakeView{}
	return t
}

func press(a *App, keys ...uv.KeyPressEvent) {
	for _, key := range keys {
		a.handleKey(key)
	}
}

func focusedPaneID(a *App) layout.PaneID {
	return a.currentTab().focused
}

func TestNavigationKeepsCommandModeOn(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))

	press(a, prefixPress, rightPress)
	if a.mode != commandMode || focusedPaneID(a) != 2 {
		t.Fatalf("mode=%v focus=%d after prefix+right", a.mode, focusedPaneID(a))
	}
	press(a, escPress)
	if a.mode != normalMode {
		t.Error("esc did not cancel command mode")
	}
}

func TestPrefixAgainCancelsCommandMode(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, prefixPress)
	if a.mode != normalMode {
		t.Error("second prefix did not cancel command mode")
	}
}

func TestArrowsCrossFromPanesIntoTabs(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2), sideBySideTab(3, 4))
	press(a, prefixPress)

	steps := []struct {
		key       uv.KeyPressEvent
		wantTab   int
		wantFocus layout.PaneID
	}{
		{rightPress, 0, 2},
		{rightPress, 1, 3}, // past the right edge: next tab, its leftmost pane
		{rightPress, 1, 4},
		{rightPress, 1, 4}, // last tab: nowhere further
		{leftPress, 1, 3},
		{leftPress, 0, 2}, // past the left edge: previous tab, its rightmost pane
	}
	for i, step := range steps {
		press(a, step.key)
		if a.session.activeTab != step.wantTab || focusedPaneID(a) != step.wantFocus {
			t.Fatalf("step %d: tab %d focus %d, want tab %d focus %d",
				i, a.session.activeTab, focusedPaneID(a), step.wantTab, step.wantFocus)
		}
	}
}

func TestTabAndShiftTabChangeTabs(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2), sideBySideTab(3, 4), sideBySideTab(5, 6))
	press(a, prefixPress, backTab)
	if a.session.activeTab != 2 {
		t.Fatalf("shift+tab from the first tab went to %d, want the last", a.session.activeTab)
	}
	press(a, tabPress)
	if a.session.activeTab != 0 {
		t.Errorf("tab from the last tab went to %d, want the first", a.session.activeTab)
	}
}

func TestUnknownKeyInCommandModeIsIgnored(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('m'))
	if a.mode != commandMode {
		t.Error("an unknown key left command mode")
	}
}

func TestKeysReachTheFocusedView(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, letter('a'))
	if got := a.currentTab().focusedView().(*fakeView).keys; len(got) != 1 {
		t.Errorf("focused view got %v", got)
	}
}

func TestClosingAPaneClosesItsView(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	closing := a.currentTab().focusedView().(*fakeView)
	press(a, prefixPress, letter('x'))
	if !closing.closed || a.currentTab().focused != 2 || a.mode != normalMode {
		t.Errorf("closed=%v focus=%d", closing.closed, a.currentTab().focused)
	}
}

func TestMoveSwapsPanesInsideTheTab(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('a'), rightPress)

	if got := a.currentTab().tree.Panes(); got[0] != 2 || got[1] != 1 {
		t.Errorf("panes %v after moving 1 right, want [2 1]", got)
	}
	if a.mode != moveMode || focusedPaneID(a) != 1 {
		t.Errorf("mode=%v focus=%d, want move mode following pane 1", a.mode, focusedPaneID(a))
	}
}

func TestMovePastTheEdgeCarriesThePaneToTheNextTab(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2), newTab(3, &fakeView{}))
	a.currentTab().focused = 2
	press(a, prefixPress, letter('a'), rightPress)

	if a.session.activeTab != 1 || focusedPaneID(a) != 2 {
		t.Fatalf("tab %d focus %d, want pane 2 in the second tab", a.session.activeTab, focusedPaneID(a))
	}
	if got := a.currentTab().tree.Panes(); got[0] != 2 || got[1] != 3 {
		t.Errorf("panes %v, want the moved pane entering from the left", got)
	}
	if got := a.session.tabs[0].tree.Panes(); len(got) != 1 || got[0] != 1 {
		t.Errorf("source tab kept %v", got)
	}
}

func TestMovePastTheLastTabOpensANewOne(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	a.currentTab().focused = 2
	press(a, prefixPress, letter('a'), rightPress)
	if len(a.session.tabs) != 2 || a.session.activeTab != 1 || focusedPaneID(a) != 2 {
		t.Errorf("tabs=%d active=%d focus=%d", len(a.session.tabs), a.session.activeTab, focusedPaneID(a))
	}
}

func TestRenameTab(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('r'), letter('a'), letter('p'), letter('x'))
	press(a, uv.KeyPressEvent{Code: uv.KeyBackspace}, letter('i'), enterPress)
	if a.currentTab().name != "api" || a.mode != normalMode {
		t.Errorf("name=%q mode=%v", a.currentTab().name, a.mode)
	}

	press(a, prefixPress, letter('r'), letter('z'), escPress)
	if a.currentTab().name != "api" {
		t.Errorf("esc still renamed the tab to %q", a.currentTab().name)
	}
}

func TestScriptsAndTerminalPanelsSplitTheTab(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('s'))
	if a.mode != normalMode || len(a.currentTab().views) != 3 || a.currentTab().focusedView().Title() != "scripts" {
		t.Errorf("mode=%v views=%d after prefix+s", a.mode, len(a.currentTab().views))
	}
	press(a, prefixPress, letter('c'))
	if a.mode != configMode {
		t.Errorf("prefix+c opened mode %v, want the config", a.mode)
	}
}

func TestNewTabOpensWithATerminal(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('t'))
	if len(a.session.tabs) != 2 || a.session.activeTab != 1 || len(a.currentTab().views) != 1 {
		t.Fatalf("tabs=%d active=%d views=%d after prefix+t", len(a.session.tabs), a.session.activeTab, len(a.currentTab().views))
	}
	if _, ok := a.currentTab().focusedView().(*fakeView); !ok {
		t.Error("the new tab's panel is not a terminal")
	}
}

func TestClosingTheLastPaneOfTheLastTabShowsHome(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.detachView(1)
	if len(a.session.tabs) != 0 || a.quitting {
		t.Fatalf("tabs=%d quitting=%v, want the home screen", len(a.session.tabs), a.quitting)
	}
	press(a, letter('a'))
	if !a.homeHintShown {
		t.Error("a key on the home screen did not reveal the prefix")
	}
}

func TestPanelFromHomeOpensATab(t *testing.T) {
	a := testApp(t)
	press(a, prefixPress, letter('s'))
	if len(a.session.tabs) != 1 || len(a.currentTab().views) != 1 {
		t.Errorf("tabs=%d after prefix+s at home", len(a.session.tabs))
	}
}

func TestClosingTheLastPaneClosesTheTab(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2), newTab(3, &fakeView{}))
	a.session.activeTab = 1
	press(a, prefixPress, letter('x'))
	if len(a.session.tabs) != 1 || a.session.activeTab != 0 || len(a.currentTab().views) != 2 {
		t.Errorf("tabs=%d active=%d", len(a.session.tabs), a.session.activeTab)
	}
}

func TestMovingTheLastPaneOutClosesItsTab(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}), sideBySideTab(2, 3))
	press(a, prefixPress, letter('a'), rightPress)
	if len(a.session.tabs) != 1 || focusedPaneID(a) != 1 || len(a.currentTab().views) != 3 {
		t.Errorf("tabs=%d focus=%d views=%d", len(a.session.tabs), focusedPaneID(a), len(a.currentTab().views))
	}
}

func TestTabCommandsNeedATab(t *testing.T) {
	a := testApp(t)
	for _, s := range a.availableShortcuts(commandShortcuts) {
		if s.needsTab {
			t.Errorf("%q offered on the home screen", s.keys)
		}
	}
	press(a, prefixPress, letter('x'))
	if a.mode != commandMode || len(a.session.tabs) != 0 {
		t.Errorf("mode %v tabs %d after a tab command at home", a.mode, len(a.session.tabs))
	}
}

func TestKeysOnHomeRevealThePrefix(t *testing.T) {
	a := testApp(t)
	press(a, letter('a'))
	if !a.homeHintShown || a.homeHintExpired == nil {
		t.Error("a key on the home screen did not reveal the prefix for a while")
	}
}

func TestQuitAsksFirst(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('q'))
	if a.quitting || a.mode != confirmMode {
		t.Fatalf("quitting=%v mode=%v, want a question", a.quitting, a.mode)
	}
	press(a, escPress)
	if a.quitting || a.mode != normalMode {
		t.Fatalf("esc: quitting=%v mode=%v", a.quitting, a.mode)
	}
	press(a, prefixPress, letter('q'), enterPress)
	if !a.quitting {
		t.Error("enter did not quit")
	}
}

func TestCtrlDTwiceOnHomeQuits(t *testing.T) {
	ctrlD := uv.KeyPressEvent{Code: 'd', Mod: uv.ModCtrl}
	a := testApp(t)
	press(a, ctrlD)
	if a.quitting || a.homeHint() != "ctrl+d again to quit" {
		t.Fatalf("one ctrl+d: quitting=%v hint=%q", a.quitting, a.homeHint())
	}
	press(a, letter('a'), ctrlD)
	if a.quitting {
		t.Fatal("ctrl+d, another key, ctrl+d quit")
	}
	press(a, ctrlD)
	if !a.quitting {
		t.Error("ctrl+d twice did not quit")
	}
}

func TestCtrlDClosesTheScriptsPanel(t *testing.T) {
	a := testApp(t)
	press(a, prefixPress, letter('s'))
	press(a, uv.KeyPressEvent{Code: 'd', Mod: uv.ModCtrl})
	if len(a.session.tabs) != 0 {
		t.Errorf("scripts panel still open: %d tabs", len(a.session.tabs))
	}
}

// scrollableView is a terminal stand-in with history.
type scrollableView struct {
	fakeView
	history int
}

func (v *scrollableView) ScrollbackLen() int                        { return v.history }
func (v *scrollableView) DrawScrolled(uv.Screen, uv.Rectangle, int) {}

func TestScrollModePagesThroughHistory(t *testing.T) {
	a := testApp(t, newTab(1, &scrollableView{history: 5}))
	press(a, prefixPress, letter('['))
	if a.mode != scrollMode {
		t.Fatalf("mode %v after prefix+[", a.mode)
	}
	press(a, upPress, upPress)
	if a.scrollOffset != 2 {
		t.Errorf("offset %d after two ups", a.scrollOffset)
	}
	press(a, letter('g'))
	if a.scrollOffset != 5 {
		t.Errorf("offset %d after g, want the top of the history", a.scrollOffset)
	}
	press(a, upPress, escPress)
	if a.scrollOffset != 5 || a.mode != normalMode {
		t.Errorf("offset %d mode %v, want clamped and back to normal", a.scrollOffset, a.mode)
	}
}

func TestScrollNeedsHistory(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('['))
	if a.mode != normalMode || a.statusMessage == "" {
		t.Errorf("mode %v message %q for a view without history", a.mode, a.statusMessage)
	}
}

func TestQuitConfirmsWithQToo(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('q'), letter('q'))
	if !a.quitting {
		t.Error("prefix+q, then q, did not quit")
	}

	other := testApp(t, sideBySideTab(1, 2))
	other.askToConfirm("delete script s?", func() { other.quitting = true })
	press(other, letter('q'))
	if other.quitting {
		t.Error("q confirmed a question it did not ask")
	}
}

func TestVimKeysWorkLikeArrows(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('l'))
	if focusedPaneID(a) != 2 || a.mode != commandMode {
		t.Fatalf("prefix+l: focus %d, mode %v", focusedPaneID(a), a.mode)
	}
	press(a, letter('h'), escPress, prefixPress, letter('a'), letter('l'))
	if got := a.currentTab().tree.Panes(); got[0] != 2 {
		t.Errorf("move with l: panes %v, want 1 moved right", got)
	}
}

// prefix+w folds the other panes of the container to a title row each; moving the focus
// unfolds the pane it reaches, and prefix+w again gives the split back.
func TestStackedLayoutFoldsTheOtherPanes(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	tree := a.currentTab().tree
	press(a, prefixPress, letter('w'))
	if tree.Folded(1) || !tree.Folded(2) || a.mode != normalMode {
		t.Fatalf("after prefix+w: 1 folded %v, 2 folded %v, mode %v", tree.Folded(1), tree.Folded(2), a.mode)
	}
	shown := contentSize(a.workspaceArea()).Rows - 1
	if rows := a.currentTab().views[1].(*fakeView).size.Rows; rows != shown {
		t.Errorf("shown pane has %d rows, want %d", rows, shown)
	}
	press(a, prefixPress, letter('j'))
	if focusedPaneID(a) != 2 || !tree.Folded(1) || a.currentTab().views[2].(*fakeView).size.Rows != shown {
		t.Errorf("focus down the stack: focus %d, 1 folded %v", focusedPaneID(a), tree.Folded(1))
	}
	press(a, escPress, prefixPress, letter('w'))
	if tree.Folded(1) {
		t.Error("prefix+w again left pane 1 folded")
	}
}

func TestRenamePanelFallsBackToTheProgram(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	program := a.viewTitle(a.currentTab(), 1)
	press(a, prefixPress, letter('e'))
	for _, r := range "logs" {
		press(a, letter(r))
	}
	press(a, enterPress)
	if got := a.viewTitle(a.currentTab(), 1); got != "logs" {
		t.Fatalf("renamed panel shows %q", got)
	}
	press(a, prefixPress, letter('e'), uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl}, enterPress)
	if got := a.viewTitle(a.currentTab(), 1); got != program {
		t.Errorf("empty name shows %q, want %q", got, program)
	}
}

// With ctrl as the shortcut, ctrl+h and ctrl+j stay with the program: terminals send them for
// backspace and newline.
func TestCtrlShortcutLeavesBackspaceAndNewline(t *testing.T) {
	d := directShortcut{mods: uv.ModCtrl}
	same := func(name string) (string, bool) { return name, true }
	for _, code := range []rune{'h', 'j'} {
		if _, _, ok := d.command(uv.KeyPressEvent{Code: code, Mod: uv.ModCtrl}, same); ok {
			t.Errorf("ctrl+%c was taken", code)
		}
	}
	if name, _, ok := d.command(uv.KeyPressEvent{Code: 'k', Mod: uv.ModCtrl}, same); !ok || name != "k" {
		t.Errorf("ctrl+k: %q %v", name, ok)
	}
}
