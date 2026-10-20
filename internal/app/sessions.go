package app

import (
	"errors"
	"fmt"
	"image"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/layout"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

// sessionSaveInterval is how often the sessions are written down while noct runs, so the
// directories shells moved to survive a crash.
const sessionSaveInterval = 30 * time.Second

// session is a set of tabs the user attaches to by name. An open session runs its programs, in
// the background while another is on screen; a closed one keeps only its layout, and opening it
// starts its shells and panels again.
type session struct {
	name        string
	tabs        []*tab
	activeTab   int
	tabBarFirst int
	created     time.Time
	lastAccess  time.Time
	// theme is the session's theme, "" for the config's.
	theme string
	// closed is the layout of a closed session, nil while it is open.
	closed *saved.Session
}

// loadSessions reads the saved sessions, all closed, and picks the most recently used to attach
// to. Without any, there is one new session.
func (a *App) loadSessions() {
	f, err := saved.Load(a.sessionsPath)
	if err != nil {
		a.statusMessage = err.Error()
	}
	for _, s := range f.Sessions {
		if saved.CheckName(s.Name) != nil || a.findSession(s.Name) != nil {
			continue
		}
		a.sessions = append(a.sessions, &session{name: s.Name, created: s.Created, lastAccess: s.LastAccess, theme: s.Theme, closed: &s})
	}
	a.session = a.mostRecentSession(nil)
	if a.session == nil {
		a.session, _ = a.createSession("") // a free name is always valid
	}
}

func (a *App) findSession(name string) *session {
	for _, s := range a.sessions {
		if s.name == name {
			return s
		}
	}
	return nil
}

// mostRecentSession is the session used last, other than except.
func (a *App) mostRecentSession(except *session) *session {
	var recent *session
	for _, s := range a.sessions {
		if s != except && (recent == nil || s.lastAccess.After(recent.lastAccess)) {
			recent = s
		}
	}
	return recent
}

// createSession adds an open, empty session; without a name it gets the first free number.
func (a *App) createSession(name string) (*session, error) {
	if name == "" {
		name = saved.FreeName(func(name string) bool { return a.findSession(name) != nil })
	}
	if err := a.checkNewSessionName(name); err != nil {
		return nil, err
	}
	now := time.Now()
	s := &session{name: name, created: now, lastAccess: now}
	if a.session != nil {
		s.theme = a.sessionTheme(a.session) // a new session starts in the colors of the one it came from
	}
	a.sessions = append(a.sessions, s)
	return s, nil
}

func (a *App) checkNewSessionName(name string) error {
	if err := saved.CheckName(name); err != nil {
		return err
	}
	if a.findSession(name) != nil {
		return fmt.Errorf("there is already a session named %s", name)
	}
	return nil
}

func (a *App) renameSession(s *session, name string) error {
	if name == s.name {
		return nil
	}
	if err := a.checkNewSessionName(name); err != nil {
		return err
	}
	s.name = name
	a.saveSessions()
	return nil
}

// switchSession puts s on screen, opening it when closed. What was going on in the session left,
// such as a selection or a panel, does not follow.
func (a *App) switchSession(s *session) {
	if a.client != nil {
		a.touchSession() // the user leaves it now; without a terminal, it was left before
	}
	a.session = s
	a.applyPalette()
	a.openSession(s)
	a.touchSession()
	a.mode, a.drag, a.selection, a.scrollOffset = normalMode, nil, nil, 0
	for _, t := range s.tabs {
		t.resizeViews(a.workspaceArea()) // the screen may have changed while it was away
	}
	a.saveSessions()
}

// sessionToAttach is the session a terminal asks for: a new one, one by name, or the one used last.
func (a *App) sessionToAttach(hello ipc.Hello) (*session, error) {
	switch {
	case hello.NewSession:
		return a.createSession(hello.Session)
	case hello.Session == "":
		return a.session, nil
	}
	if s := a.findSession(hello.Session); s != nil {
		return s, nil
	}
	return nil, fmt.Errorf("there is no session named %s; noct new %s creates it", hello.Session, hello.Session)
}

// leaveSession saves the sessions as the terminal leaves the one on screen, now its last use.
func (a *App) leaveSession() {
	a.touchSession()
	a.saveSessions()
}

// touchSession marks the session on screen as used now.
func (a *App) touchSession() {
	a.session.lastAccess = time.Now()
}

// openSession starts again the shells and panels of a closed session, in the layout they had.
func (a *App) openSession(s *session) {
	if s.closed == nil {
		return
	}
	for _, st := range s.closed.Tabs {
		if t := a.restoreTab(st); t != nil {
			s.tabs = append(s.tabs, t)
		}
	}
	s.activeTab = max(min(s.closed.ActiveTab, len(s.tabs)-1), 0)
	s.closed = nil
}

// closeSession stops the programs of a session and keeps its layout, to open it again later. The
// session on screen gives its place to the one used before it; the only session stays open.
func (a *App) closeSession(s *session) error {
	if s.closed != nil {
		return nil
	}
	if s == a.session {
		previous := a.mostRecentSession(s)
		if previous == nil {
			return errors.New("the only session stays open; noct quit closes it")
		}
		a.switchSession(previous)
	}
	layout := a.savedSession(s)
	a.closeViews(s)
	s.closed = &layout
	a.saveSessions()
	return nil
}

// deleteSession stops the programs of a session and forgets it. Deleting the session on screen
// moves to the one used before it, or to a new one.
func (a *App) deleteSession(s *session) {
	if s == a.session {
		next := a.mostRecentSession(s)
		if next == nil {
			next, _ = a.createSession("") // a free name is always valid
		}
		a.switchSession(next)
	}
	a.closeViews(s)
	a.sessions = slices.DeleteFunc(a.sessions, func(other *session) bool { return other == s })
	a.saveSessions()
}

// closeViews stops every program of the session. Their exits come later and find no pane.
func (a *App) closeViews(s *session) {
	for _, t := range s.tabs {
		for id, v := range t.views {
			delete(a.shells, id)
			v.Close()
		}
	}
	s.tabs, s.activeTab, s.tabBarFirst = nil, 0, 0
}

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

type sessionArgs struct {
	Name string `json:"name"`
}

// sessionInfos lists the sessions for the command line, the most recently used first.
func (a *App) sessionInfos() []saved.Info {
	infos := []saved.Info{}
	for _, s := range a.sessions {
		info := saved.Info{Name: s.name, Tabs: len(s.tabs), Open: s.closed == nil, LastAccess: s.lastAccess}
		if s.closed != nil {
			info.Tabs = len(s.closed.Tabs)
		}
		info.Attached = s == a.session && a.client != nil
		infos = append(infos, info)
	}
	slices.SortStableFunc(infos, func(x, y saved.Info) int { return y.LastAccess.Compare(x.LastAccess) })
	return infos
}

func (a *App) deleteSessionNamed(name string) error {
	s := a.findSession(name)
	if s == nil {
		return fmt.Errorf("there is no session named %s", name)
	}
	a.deleteSession(s)
	return nil
}
