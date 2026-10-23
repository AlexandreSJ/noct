package app

import (
	"image"
	"os"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
)

func TestClosingARunGivesTheFocusBackToThePaneBeforeIt(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.currentTab().tree.Split(1, 2, layout.SideBySide)
	a.currentTab().views[2] = &fakeView{}
	a.currentTab().focus(2) // the panel the script runs from, on the right of pane 1
	press(a, prefixPress, letter('p'))
	opened := focusedPaneID(a)

	press(a, prefixPress, letter('x'))
	if focusedPaneID(a) != 2 {
		t.Errorf("closing pane %d focused %d, want 2, the pane used before it", opened, focusedPaneID(a))
	}
}

func openFloatingPane(t *testing.T, a *App) *floatingPane {
	t.Helper()
	press(a, prefixPress, letter('u'))
	f := a.currentTab().floatingPane(focusedPaneID(a))
	if f == nil {
		t.Fatal("prefix+u did not open a floating pane")
	}
	return f
}

func TestFloatingPaneOpensOverTheTiles(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	f := openFloatingPane(t, a)
	area := a.workspaceArea()
	if !f.frame.In(area) || len(a.currentTab().tree.Panes()) != 2 {
		t.Fatalf("frame %v, tiles %v", f.frame, a.currentTab().tree.Panes())
	}
	if id, _ := a.currentTab().paneAt(center(f.frame), area); id != f.id {
		t.Error("the floating pane is not on top")
	}

	press(a, prefixPress, leftPress, escPress)
	if a.currentTab().floatingPane(focusedPaneID(a)) != nil {
		t.Fatal("left from the floating pane stayed floating")
	}
	press(a, prefixPress, letter('x'))
	if len(a.currentTab().views) != 2 {
		t.Errorf("closing a tile took %d views with it", 3-len(a.currentTab().views))
	}
}

func TestLastTileClosingLeavesTheFloatingPaneTiled(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	f := openFloatingPane(t, a)
	a.detachView(1)
	tab := a.currentTab()
	if len(a.session.tabs) != 1 || tab.tree == nil || !tab.tree.Contains(f.id) || len(tab.floating) != 0 {
		t.Errorf("tabs %d, floating %d", len(a.session.tabs), len(tab.floating))
	}
}

func TestDraggingAFloatingPane(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	f := openFloatingPane(t, a)
	start := f.frame

	a.handleEvent(uv.MouseClickEvent{X: start.Min.X + 3, Y: start.Min.Y, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseMotionEvent{X: start.Min.X + 1, Y: start.Min.Y + 2, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseReleaseEvent{X: start.Min.X + 1, Y: start.Min.Y + 2, Button: uv.MouseLeft})
	if want := start.Add(image.Pt(-2, 2)); f.frame != want {
		t.Fatalf("moved to %v, want %v", f.frame, want)
	}

	right := f.frame.Max.X - 1
	a.handleEvent(uv.MouseClickEvent{X: right, Y: f.frame.Min.Y + 3, Button: uv.MouseRight})
	a.handleEvent(uv.MouseMotionEvent{X: right + 6, Y: f.frame.Min.Y + 3, Button: uv.MouseRight})
	view := a.currentTab().views[f.id].(*fakeView)
	if f.frame.Dx() != start.Dx()+6 || view.size.Cols != start.Dx()+6-2 {
		t.Errorf("resized to %v (view %v), want 6 columns wider", f.frame, view.size)
	}
}

func TestResizeModeGrowsTheFocusedTile(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	left := a.currentTab().views[1].(*fakeView)
	a.currentTab().resizeViews(a.workspaceArea())
	before := left.size.Cols
	press(a, prefixPress, letter('n'), rightPress, rightPress)
	if a.mode != resizeMode || left.size.Cols != before+2*resizeStepX {
		t.Fatalf("mode %v, width %d, want %d", a.mode, left.size.Cols, before+2*resizeStepX)
	}
	press(a, escPress)
	if a.mode != normalMode {
		t.Error("esc did not leave resize mode")
	}
}

func TestFullscreenShowsOnlyTheFocusedPane(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	right := a.currentTab().views[2].(*fakeView)
	a.currentTab().focus(2)
	press(a, prefixPress, letter('f'))
	area := a.workspaceArea()
	if visible := a.currentTab().visible(area); len(visible) != 1 || visible[0].frame != area {
		t.Fatalf("visible %v", visible)
	}
	if right.size != contentSize(area) {
		t.Errorf("the fullscreen pane is %v", right.size)
	}
	press(a, prefixPress, leftPress, escPress)
	left := a.currentTab().views[1].(*fakeView)
	if focusedPaneID(a) != 1 || left.size != contentSize(area) || right.size == contentSize(area) {
		t.Errorf("focus %d, left %v, right %v", focusedPaneID(a), left.size, right.size)
	}
	press(a, prefixPress, letter('f'))
	if len(a.currentTab().visible(area)) != 2 || left.size == contentSize(area) {
		t.Error("leaving fullscreen did not bring the layout back")
	}
}

func TestSelectionStaysInItsPane(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	a.currentTab().views[1] = &textView{lines: []string{"left side text"}}
	a.currentTab().views[2] = &textView{lines: []string{"right side text"}}
	click(a, 3, 2)
	a.handleEvent(uv.MouseMotionEvent{X: 80, Y: 4, Button: uv.MouseLeft}) // into the right pane
	a.handleEvent(uv.MouseReleaseEvent{X: 80, Y: 4, Button: uv.MouseLeft})
	if text := a.selectedText(); strings.Contains(text, "right") || !strings.HasPrefix(text, "ft side text") {
		t.Errorf("selected %q across panes", text)
	}
}

func TestBrokenEditIsReportedAndListed(t *testing.T) {
	a := testApp(t)
	a.config.Editor = "cat"
	l := recordLaunches(a)
	saveScript(t, "fragile", "run: echo fine\n")
	openScriptsPanel(t, a)
	press(a, letter('e'), uv.KeyPressEvent{Code: 's', Mod: uv.ModCtrl}) // saving keeps a backup
	s, _ := a.scripts.Find("fragile")
	if err := os.WriteFile(s.File(), []byte("run: echo {{.oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.reloadScripts()

	broken, ok := a.scripts.Find("fragile")
	if !ok || broken.Problem == nil {
		t.Fatal("the broken script left the list")
	}
	press(a, enterPress)
	if len(l.programs) != 0 {
		t.Fatal("a broken script ran")
	}
	press(a, letter('e')) // a broken script opens whole in the editor
	if editor := l.last(t); editor.argv[len(editor.argv)-2] != broken.File() {
		t.Fatalf("editor %q", editor.argv)
	}
	l.last(t).onExit(0)
	if !strings.Contains(a.statusMessage, broken.BackupFile()) {
		t.Errorf("status %q does not point to the backup", a.statusMessage)
	}
	if content, _ := os.ReadFile(broken.BackupFile()); string(content) != "run: echo fine\n" {
		t.Errorf("backup %q", content)
	}
}
