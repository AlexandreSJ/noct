package app

import (
	"fmt"
	"image"
	"slices"
	"strconv"

	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
)

// paneEnv names, for a flow's run, its pane: where the panes its steps ask for open.
const paneEnv = "NOCT_PANE"

// openRequestedPane opens the pane a flow's step asks for beside the run that asks, in its tab,
// held open with its exit code like a script's run. A step that waits gets the focus and its
// exit code when it ends; one in the background is answered at once and leaves the focus where
// it was.
func (a *App) openRequestedPane(c connection) {
	req := c.hello.Pane
	answer := func(exit ipc.Exit) {
		_ = c.conn.SendJSON(ipc.KindExit, exit) // a run that stopped listening has nothing to learn
		_ = c.conn.Close()
	}
	if req == nil {
		answer(ipc.Exit{Reason: "no pane was asked for"})
		return
	}
	t, near := a.tabOfPane(layout.PaneID(req.Near))
	if t == nil {
		a.openEmptyTab()
		t, near = a.currentTab(), 0
	}
	argv := append(slices.Clone(a.shellArgv), "-c", req.Command)
	title := orDefault(req.Title, "step")
	start := func(id layout.PaneID, frame image.Rectangle) (view, error) {
		run := func(size pty.Size, onExit func(int)) (view, error) {
			p := program{argv: argv, dir: orDefault(req.Dir, a.workDir), logName: "step-" + title}
			p.onExit = func(code int) {
				if req.Wait {
					answer(ipc.Exit{Reason: ipc.PaneExited, Code: code})
				}
				onExit(code)
			}
			return a.launch(p, size)
		}
		return newRunView(title, &a.palette, run, func() { a.detachView(id) }, contentSize(frame))
	}
	var err error
	if t.isEmpty() {
		a.openPanel(start)
	} else {
		err = a.openPanelBeside(t, orTile(t, near), start)
	}
	switch {
	case err != nil:
		answer(ipc.Exit{Reason: err.Error()})
	case !req.Wait:
		if near != 0 {
			a.focusPane(t, near)
		}
		answer(ipc.Exit{Reason: ipc.PaneOpened})
	}
}

// showPane brings a flow's run to the screen and focuses it, for a prompt that waits for the
// user's answer there.
func (a *App) showPane(args paneArgs) error {
	t, id := a.tabOfPane(layout.PaneID(args.Pane))
	if t == nil {
		return fmt.Errorf("there is no pane %d", args.Pane)
	}
	a.activeTab = slices.Index(a.tabs, t)
	a.focusPane(t, id)
	a.notice = a.viewTitle(t, id) + " waits for your answer"
	return nil
}

// tabOfPane finds the tab a pane is in.
func (a *App) tabOfPane(id layout.PaneID) (*tab, layout.PaneID) {
	for _, t := range a.tabs {
		if _, ok := t.views[id]; ok {
			return t, id
		}
	}
	return nil, 0
}

// orTile is the pane when it is a tile of the tab, or the tile a new pane would split.
func orTile(t *tab, id layout.PaneID) layout.PaneID {
	if t.tree.Contains(id) {
		return id
	}
	return t.tiledFocus()
}

func paneEnvFor(id layout.PaneID) string {
	return paneEnv + "=" + strconv.Itoa(int(id))
}
