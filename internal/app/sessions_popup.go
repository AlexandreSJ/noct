package app

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// The sessions popup (prefix+o) lists the sessions, the most recently used first, with what each
// holds and when it was last used; its keys open, create, rename, close and delete them.

const (
	sessionHints       = "{enter} open  {n} new  {r} rename  {x} close  {d} delete  {f} filter  esc"
	sessionPanelWidth  = 66
	sessionNameColumns = 20
)

// openSessions shows the sessions popup on the session on screen, with the rows the filter keeps.
func (a *App) openSessions() {
	filter := strings.ToLower(a.sessionFilter)
	a.sessionRows = slices.DeleteFunc(slices.Clone(a.sessions), func(s *session) bool {
		return !strings.Contains(strings.ToLower(s.name), filter)
	})
	slices.SortStableFunc(a.sessionRows, func(x, y *session) int { return y.lastAccess.Compare(x.lastAccess) })
	a.sessionSelected = max(slices.Index(a.sessionRows, a.session), 0)
	a.mode = sessionsMode
}

// handleSessionsKey shows the selected session in its theme while the popup is open, and the
// session on screen in its own once it closes.
func (a *App) handleSessionsKey(key uv.KeyPressEvent) {
	a.moveThroughSessions(key)
	a.showScreenTheme()
}

func (a *App) moveThroughSessions(key uv.KeyPressEvent) {
	switch key.String() {
	case "esc":
		if a.sessionFilter != "" {
			a.sessionFilter = ""
			a.openSessions()
			return
		}
		a.mode = normalMode
		return
	case "q":
		a.mode = normalMode
		return
	case "f":
		a.promptSessionFilter()
		return
	case "n":
		a.promptNewSession()
		return
	}
	if len(a.sessionRows) == 0 {
		return // the filter keeps none
	}
	selected := a.sessionRows[a.sessionSelected]
	if !slices.Contains(a.sessions, selected) {
		a.openSessions() // deleted meanwhile, as by noct kill: the rows start over
		return
	}
	switch key.String() {
	case "up", "k":
		a.sessionSelected = max(a.sessionSelected-1, 0)
	case "down", "j":
		a.sessionSelected = min(a.sessionSelected+1, len(a.sessionRows)-1)
	case "pgup", "pgdown", "home", "end":
		a.sessionSelected, _ = listJump(key.String(), a.sessionSelected, len(a.sessionRows), len(a.sessionRows))
	case "enter", "o":
		a.switchSession(selected)
	case "r":
		a.promptSessionRename(selected)
	case "x":
		a.askToCloseSession(selected)
	case "d", "delete":
		a.askToConfirm("delete session "+selected.name+"? its programs stop and its layout is forgotten", func() {
			a.deleteSession(selected)
			a.openSessions()
		})
	}
}

// promptSessionFilter asks for the text the names must hold; empty shows every session again.
func (a *App) promptSessionFilter() {
	a.openPrompt("filter sessions", a.sessionFilter, "{enter} filter  esc cancel  part of the name", func(text string) {
		a.sessionFilter = strings.TrimSpace(text)
		a.openSessions()
	})
	a.prompt.cancel = a.openSessions
}

func (a *App) promptNewSession() {
	a.openPrompt("new session", "", "{enter} create  esc cancel  empty picks a number", func(name string) {
		s, err := a.createSession(name)
		if err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.switchSession(s)
	})
	a.prompt.cancel = a.openSessions
}

func (a *App) promptSessionRename(s *session) {
	a.openPrompt("rename session", s.name, "{enter} save  esc cancel", func(name string) {
		if err := a.renameSession(s, name); err != nil {
			a.statusMessage = err.Error()
		}
		a.openSessions()
	})
	a.prompt.cancel = a.openSessions
}

// askToCloseSession asks before closing a session whose programs would stop.
func (a *App) askToCloseSession(s *session) {
	closeIt := func() {
		if err := a.closeSession(s); err != nil {
			a.statusMessage = err.Error()
		}
		a.openSessions()
	}
	if s.closed != nil {
		return
	}
	a.askToConfirm("close session "+s.name+"? its programs stop; its layout opens again later", closeIt)
}

// drawSessions draws the sessions popup, one session per row.
func (a *App) drawSessions(screen uv.Screen) {
	pal := &a.palette
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	title, rows := "sessions", max(len(a.sessionRows), 1)
	if a.sessionFilter != "" {
		title += " · filter: " + a.sessionFilter
	}
	inner := a.drawPanel(screen, centeredRect(area, sessionPanelWidth, panelHeight(rows+2)), title)
	if len(a.sessionRows) == 0 {
		drawText(screen, inner.Min.X+1, inner.Min.Y, inner.Max.X, "no session matches; esc shows them all", pal.panelMuted)
	}
	now := time.Now()
	for i, s := range a.sessionRows {
		y := inner.Min.Y + i
		if y >= inner.Max.Y-2 {
			break
		}
		style := pal.panelText
		if i == a.sessionSelected {
			style = pal.selectedRow
		}
		drawRow(screen, inner, y, a.sessionLabel(s), lastUsed(s.lastAccess, now, s == a.session && a.client != nil), style)
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("sessions", sessionHints), pal.panelMuted)
}

// sessionLabel is a session's row: open (●) or closed (○), its name and its tabs.
func (a *App) sessionLabel(s *session) string {
	mark, tabs := "●", len(s.tabs)
	if s.closed != nil {
		mark, tabs = "○", len(s.closed.Tabs)
	}
	name := s.name
	if len([]rune(name)) > sessionNameColumns {
		name = string([]rune(name)[:sessionNameColumns-1]) + "…"
	}
	return fmt.Sprintf("%s %-*s %s", mark, sessionNameColumns, name, tabCount(tabs))
}

func tabCount(tabs int) string {
	switch tabs {
	case 0:
		return "empty"
	case 1:
		return "1 tab"
	}
	return fmt.Sprintf("%d tabs", tabs)
}

// lastUsed tells when a session was last used, in words for the last day and as a date before.
func lastUsed(when, now time.Time, attached bool) string {
	elapsed := now.Sub(when)
	switch {
	case attached:
		return "attached"
	case when.IsZero():
		return "never"
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%d min ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(elapsed.Hours()))
	case when.Year() == now.Year():
		return when.Format("Jan 2 15:04")
	}
	return when.Format("2006-01-02 15:04")
}
