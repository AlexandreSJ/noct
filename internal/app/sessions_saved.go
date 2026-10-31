package app

import (
	"image"
	"os"
	"reflect"
	"strings"
	"unicode"

	"github.com/AlexandreSJ/noct/internal/layout"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

// Sessions on disk: the layout of each session is written down as it changes and built again,
// with new programs, when a closed session opens.

// saveSessions writes the sessions down when they changed. A failure shows in the tab bar and
// the next save tries again.
func (a *App) saveSessions() {
	if a.sessionsPath == "" {
		return
	}
	var f saved.File
	for _, s := range a.sessions {
		f.Sessions = append(f.Sessions, a.savedSession(s))
	}
	if reflect.DeepEqual(f, a.sessionsSaved) {
		return
	}
	if err := saved.Save(a.sessionsPath, f); err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.sessionsSaved = f
}

// savedSession is the session as it is kept on disk.
func (a *App) savedSession(s *session) saved.Session {
	if s.closed != nil {
		kept := *s.closed
		kept.Name, kept.LastAccess, kept.Theme = s.name, s.lastAccess, s.theme
		return kept
	}
	out := saved.Session{Name: s.name, Created: s.created, LastAccess: s.lastAccess, Theme: s.theme}
	for index, t := range s.tabs {
		st, ok := a.savedTab(t)
		if !ok {
			continue
		}
		if index == s.activeTab {
			out.ActiveTab = len(out.Tabs)
		}
		out.Tabs = append(out.Tabs, st)
	}
	return out
}

// savedTab is the tab's layout with the panes that can open again; the others are left out, and
// the layout closes over them when it is restored.
func (a *App) savedTab(t *tab) (saved.Tab, bool) {
	if t.isEmpty() {
		return saved.Tab{}, false
	}
	st := saved.Tab{Name: t.name, Layout: t.tree.Shape(), Focused: t.focused, Fullscreen: t.fullscreen, Colors: t.colors.saved()}
	for _, id := range t.tree.Panes() {
		if p, ok := a.savedPane(t, id); ok {
			st.Panes = append(st.Panes, p)
		}
	}
	for _, f := range t.floating {
		if p, ok := a.savedPane(t, f.id); ok {
			p.Floating = &saved.Frame{X: f.frame.Min.X, Y: f.frame.Min.Y, Width: f.frame.Dx(), Height: f.frame.Dy()}
			st.Panes = append(st.Panes, p)
		}
	}
	return st, len(st.Panes) > 0
}

// directoryTeller is a view that knows the directory its program is in: a terminal.
type directoryTeller interface {
	Dir() string
}

// commandTeller is a view that knows the command its shell runs, such as a terminal running claude.
type commandTeller interface {
	ForegroundCommand() string
}

func (a *App) savedPane(t *tab, id layout.PaneID) (saved.Pane, bool) {
	p := saved.Pane{ID: id, Name: t.paneNames[id], Colors: t.paneColors[id].saved()}
	switch v := t.views[id].(type) {
	case *scriptsPanel:
		p.Kind = saved.Scripts
	case *inspectPanel:
		p.Kind = saved.Inspect
	case *logsPanel:
		p.Kind = saved.Logs
	default:
		sh := a.shells[id]
		if sh == nil {
			return p, false // runs, editors and help end with what they were for
		}
		p.Kind, p.Dir, p.Log, p.Command = saved.Terminal, a.workDir, sh.log, sh.rerun
		if teller, ok := v.(directoryTeller); ok && teller.Dir() != "" {
			p.Dir = teller.Dir()
		}
		if teller, ok := v.(commandTeller); ok {
			if command := teller.ForegroundCommand(); command != "" {
				p.Command = command
			}
		}
	}
	return p, true
}

// restoreTab builds a saved tab again with new panes. Panes that fail to start leave the layout;
// a tab left with none is dropped.
func (a *App) restoreTab(st saved.Tab) *tab {
	newIDs := make(map[layout.PaneID]layout.PaneID)
	tree, err := layout.FromShape(st.Layout, func(savedID layout.PaneID) layout.PaneID {
		newIDs[savedID] = a.newPaneID()
		return newIDs[savedID]
	})
	if err != nil {
		a.statusMessage = err.Error()
		return nil
	}
	t := newEmptyTab()
	t.tree, t.name, t.fullscreen, t.colors = tree, st.Name, st.Fullscreen, colorSetFrom(st.Colors)
	for _, p := range st.Panes {
		a.restorePane(t, p, newIDs)
	}
	t.focused = newIDs[st.Focused]
	for _, id := range tree.Panes() {
		if _, ok := t.views[id]; !ok {
			t.removeView(id)
		}
		if t.isEmpty() {
			return nil
		}
	}
	if _, ok := t.views[t.focused]; !ok {
		t.focused = t.tree.Panes()[0]
	}
	t.resizeViews(a.workspaceArea())
	return t
}

// restorePane starts a saved pane again in the tab, under its new ID.
func (a *App) restorePane(t *tab, p saved.Pane, newIDs map[layout.PaneID]layout.PaneID) {
	if p.Floating != nil {
		newIDs[p.ID] = a.newPaneID()
	}
	id, ok := newIDs[p.ID]
	if !ok {
		return
	}
	v, err := a.restoreView(id, p)
	if err != nil || v == nil {
		if err != nil {
			a.statusMessage = err.Error()
		}
		return
	}
	t.views[id] = v
	if p.Name != "" {
		t.paneNames[id] = p.Name
	}
	if colors := colorSetFrom(p.Colors); len(colors) > 0 {
		t.paneColors[id] = colors
	}
	if f := p.Floating; f != nil {
		t.floating = append(t.floating, &floatingPane{id: id, frame: image.Rect(f.X, f.Y, f.X+f.Width, f.Y+f.Height)})
	}
}

func (a *App) restoreView(id layout.PaneID, p saved.Pane) (view, error) {
	area := a.workspaceArea()
	switch p.Kind {
	case saved.Terminal:
		v, err := a.startShellLogged(id, area, existingDir(p.Dir, a.workDir), p.Log)
		if err == nil && !strings.ContainsFunc(p.Command, unicode.IsControl) {
			a.shells[id].rerun = p.Command // typed as keys, a control character would be a key
		}
		return v, err
	case saved.Scripts:
		return a.startScripts(id, area)
	case saved.Inspect:
		return a.startInspect(id, area)
	case saved.Logs:
		return a.startLogs(id, area)
	}
	return nil, nil
}

// existingDir is dir while it exists, or fallback: a shell cannot start in a removed directory.
func existingDir(dir, fallback string) string {
	if info, err := os.Stat(dir); dir != "" && err == nil && info.IsDir() {
		return dir
	}
	return fallback
}

// terminalLogSessions names the session of each terminal's log: the open sessions' terminals and
// the closed sessions' saved ones.
func (a *App) terminalLogSessions() map[string]string {
	sessions := map[string]string{}
	for _, s := range a.sessions {
		if s.closed != nil {
			for _, t := range s.closed.Tabs {
				for _, p := range t.Panes {
					if p.Log != "" {
						sessions[p.Log] = s.name
					}
				}
			}
			continue
		}
		for _, t := range s.tabs {
			for id := range t.views {
				if sh := a.shells[id]; sh != nil && sh.log != "" {
					sessions[sh.log] = s.name
				}
			}
		}
	}
	return sessions
}
