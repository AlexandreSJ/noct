package app

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/AlexandreSJ/noct/internal/ipc"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

// Sessions are saved periodically so the directories shells moved to survive a crash.
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
	// closed is the layout kept while the session is closed; nil while it is open.
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
		a.sessions = append(a.sessions, &session{name: s.Name, created: s.Created, lastAccess: s.LastAccess, closed: &s})
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
