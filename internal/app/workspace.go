package app

import (
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"slices"
	"time"

	"github.com/AlexandreSJ/noct/internal/kittygfx"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/pane"
	"github.com/AlexandreSJ/noct/internal/pty"
)

// maxPaneLogSize caps each terminal's log; older output rotates into one backup file.
const maxPaneLogSize = 10 << 20

// viewStarter creates the view of a new pane, sized for its frame.
type viewStarter func(id layout.PaneID, frame image.Rectangle) (view, error)

func (a *App) newPaneID() layout.PaneID {
	a.nextPane++
	return a.nextPane
}

// program is what a new pane runs.
type program struct {
	argv []string
	dir  string
	// logName tells the pane's log apart from others started the same second; without it, and
	// without logPath, nothing is logged.
	logName string
	// logPath, when set, is the log's path, chosen by whoever needs to find the log again.
	logPath string
	// env is added to the environment programs get.
	env []string
	// onExit runs on the event loop after the program exits.
	onExit func(exitCode int)
}

// startProgram runs p in a terminal of the given size, logging its output.
func (a *App) startProgram(p program, size pty.Size) (view, error) {
	opts := pane.Options{
		Argv:           p.argv,
		Env:            append(slices.Clone(a.shellEnv), p.env...),
		Dir:            p.dir,
		Size:           size,
		OnScreenChange: a.notifyScreenChanged,
		OnExit:         func(code int) { a.onLoop(func() { p.onExit(code) }) },
	}
	if a.imagesEnabled {
		opts.Images = kittygfx.NewSession(a.newImageID, a.cellSize)
		opts.OnTerminalGraphics = a.forwardGraphics
	}
	logPath := p.logPath
	if logPath == "" && p.logName != "" {
		logPath = logstore.Path(p.logName, time.Now())
	}
	if logPath != "" {
		if log, err := logstore.Create(logPath, maxPaneLogSize); err == nil {
			opts.Log = log
		} else {
			a.statusMessage = err.Error() // the terminal still works, only without a log
		}
	}
	v, err := pane.Start(opts)
	if err != nil {
		return nil, fmt.Errorf("starting %s: %w", filepath.Base(p.argv[0]), err)
	}
	return v, nil
}

// startTerminal opens the user's shell; the pane leaves its tab when the shell exits.
func (a *App) startTerminal(id layout.PaneID, frame image.Rectangle) (view, error) {
	return a.startInPane(id, frame, a.shellArgv, a.workDir, fmt.Sprintf("pane%d", id))
}

// startInPane runs argv in dir in the pane id, which leaves its tab when the program exits.
func (a *App) startInPane(id layout.PaneID, frame image.Rectangle, argv []string, dir, logName string) (view, error) {
	return a.launch(program{
		argv:    argv,
		dir:     dir,
		logName: logName,
		onExit:  func(int) { a.detachView(id) },
	}, contentSize(frame))
}

// startScripts opens a scripts panel, reading the library first to catch changes made elsewhere.
func (a *App) startScripts(id layout.PaneID, _ image.Rectangle) (view, error) {
	a.reloadScripts()
	return &scriptsPanel{app: a, close: func() { a.detachView(id) }}, nil
}

func (a *App) openEmptyTab() {
	a.tabs = append(a.tabs, newEmptyTab())
	a.activeTab = len(a.tabs) - 1
}

func (a *App) selectTab(index int) {
	if len(a.tabs) == 0 {
		return
	}
	a.activeTab = (index%len(a.tabs) + len(a.tabs)) % len(a.tabs)
}

// openPanel adds a pane to the current tab, splitting the focused pane. On the home screen it
// opens a tab first. The view starts at its final size, so a program is not resized at birth.
func (a *App) openPanel(start viewStarter) {
	if a.currentTab() == nil {
		a.openEmptyTab()
	}
	t := a.currentTab()
	if t.isEmpty() {
		id := a.newPaneID()
		v, err := start(id, a.workspaceArea())
		if err != nil {
			a.statusMessage = err.Error()
			a.removeTabIfEmpty(t)
			return
		}
		t.placeFirst(id, v)
		return
	}
	if err := a.openPanelBeside(t, t.tiledFocus(), start); err != nil {
		a.statusMessage = err.Error()
	}
}

// openPanelBeside splits target, a tile of t, for a new pane that gets the focus.
func (a *App) openPanelBeside(t *tab, target layout.PaneID, start viewStarter) error {
	id := a.newPaneID()
	t.tree.Split(target, id, splitDirectionFor(t.tree.Rects(a.workspaceArea())[target]))
	if !t.fits(a.workspaceArea()) {
		t.tree.Remove(id)
		return errors.New("no room for another panel")
	}
	v, err := start(id, t.tree.Rects(a.workspaceArea())[id])
	if err != nil {
		t.tree.Remove(id)
		return err
	}
	t.views[id] = v
	t.focus(id)
	t.resizeViews(a.workspaceArea())
	return nil
}

// toggleStacked turns the focused tile's container into a stack or back into a split.
func (a *App) toggleStacked() {
	t := a.currentTab()
	if t.isEmpty() || t.floatingPane(t.focused) != nil {
		return
	}
	if !t.tree.ToggleStacked(t.focused) {
		a.statusMessage = "a lone panel has nothing to stack with"
		return
	}
	t.resizeViews(a.workspaceArea())
}

func (a *App) moveFocus(heading layout.Heading) {
	t := a.currentTab()
	if t.isEmpty() {
		return
	}
	if neighbor, ok := a.neighbor(t, heading); ok {
		a.focusPane(t, neighbor)
	}
}

// focusPane focuses a pane and sizes the views again: in fullscreen the pane arriving takes the
// whole tab, and in a stack it unfolds. Views whose size stays the same are not resized.
func (a *App) focusPane(t *tab, id layout.PaneID) {
	t.focus(id)
	t.resizeViews(a.workspaceArea())
}

// toggleFullscreen shows the focused pane alone over the tab, or the layout again.
func (a *App) toggleFullscreen() {
	t := a.currentTab()
	t.fullscreen = !t.fullscreen
	t.resizeViews(a.workspaceArea())
}

// moveFocusAcrossTabs moves left or right to the neighbor pane, and past the edge of the tab
// into the adjacent tab, entering it from the matching side.
func (a *App) moveFocusAcrossTabs(heading layout.Heading) {
	t := a.currentTab()
	if !t.isEmpty() {
		if neighbor, ok := a.neighbor(t, heading); ok {
			a.focusPane(t, neighbor)
			return
		}
	}
	next := a.activeTab + tabStep(heading)
	if next < 0 || next >= len(a.tabs) {
		return
	}
	a.activeTab = next
	if entered := a.tabs[next]; !entered.isEmpty() {
		entered.focus(entered.edgePane(heading))
	}
}

// moveFocusedPane moves the focused pane the way i3 does (see layout.Tree.Move). Past the left or
// right edge of the tab, it carries the pane into the adjacent tab, or into a new tab. A floating
// pane moves by a step instead.
func (a *App) moveFocusedPane(heading layout.Heading) {
	t := a.currentTab()
	if t.isEmpty() {
		return
	}
	if f := t.floatingPane(t.focused); f != nil {
		a.moveFloating(t, f, heading)
		return
	}
	if t.tree.Move(t.focused, heading) {
		t.resizeViews(a.workspaceArea())
		return
	}
	if heading == layout.Left || heading == layout.Right {
		a.movePaneToAdjacentTab(heading)
	}
}

func (a *App) movePaneToAdjacentTab(heading layout.Heading) {
	source := a.currentTab()
	id, moved, name := source.focused, source.focusedView(), source.paneNames[source.focused]
	var target *tab
	if next := a.activeTab + tabStep(heading); next >= 0 && next < len(a.tabs) {
		target = a.tabs[next]
		if !target.isEmpty() && !canSplit(target.rects(a.workspaceArea())[target.edgePane(heading)], layout.SideBySide) {
			a.statusMessage = "no room in that tab"
			return
		}
	} else if len(source.views) == 1 {
		return // a lone pane moving into a new tab would change nothing
	}

	source.removeView(id)
	source.resizeViews(a.workspaceArea())
	a.removeTabIfEmpty(source)
	if target == nil {
		target = newEmptyTab()
		if heading == layout.Right {
			a.tabs = append(a.tabs, target)
		} else {
			a.tabs = slices.Insert(a.tabs, 0, target)
		}
	}
	a.placeArriving(target, id, moved, heading)
	if name != "" {
		target.paneNames[id] = name
	}
	a.activeTab = slices.Index(a.tabs, target)
	target.resizeViews(a.workspaceArea())
}

// placeArriving puts a pane into a tab it enters from the given heading, beside the edge pane.
func (a *App) placeArriving(target *tab, id layout.PaneID, v view, heading layout.Heading) {
	if target.isEmpty() {
		target.placeFirst(id, v)
		return
	}
	edge := target.edgePane(heading)
	target.tree.Split(edge, id, layout.SideBySide)
	if heading == layout.Right {
		target.tree.Swap(edge, id) // entering from the left, the pane goes first
	}
	target.views[id] = v
	target.focus(id)
}

func tabStep(heading layout.Heading) int {
	if heading == layout.Left {
		return -1
	}
	return 1
}

// closeFocused closes the focused pane, and its tab with it if it was the last.
func (a *App) closeFocused() {
	t := a.currentTab()
	v, id := t.focusedView(), t.focused
	a.detachView(id)
	v.Close()
}

// detachView takes a view out of whichever tab holds it; a tab left without panes closes.
// Unknown ids are ignored: a closed terminal still reports its exit later.
func (a *App) detachView(id layout.PaneID) {
	for _, t := range a.tabs {
		if _, ok := t.views[id]; ok {
			t.removeView(id)
			t.resizeViews(a.workspaceArea())
			a.removeTabIfEmpty(t)
			return
		}
	}
}

// removeTabIfEmpty closes a tab without panes. The selection stays on the same tab, or moves to
// the one that takes the closed tab's place; with no tabs left, the home screen shows.
func (a *App) removeTabIfEmpty(t *tab) {
	index := slices.Index(a.tabs, t)
	if !t.isEmpty() || index < 0 {
		return
	}
	a.tabs = slices.Delete(a.tabs, index, index+1)
	if index < a.activeTab {
		a.activeTab--
	}
	a.activeTab = max(min(a.activeTab, len(a.tabs)-1), 0)
}
