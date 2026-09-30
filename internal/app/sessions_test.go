package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/layout"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

// sessionsApp is a workspace that keeps its sessions in a file of its own.
func sessionsApp(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	a.sessionsPath = filepath.Join(t.TempDir(), "sessions.json")
	return a
}

func typeInto(a *App, text string) {
	for _, r := range text {
		a.handleKey(letter(r))
	}
}

// twoShells opens a tab with two shells side by side, the second named.
func twoShells(a *App) {
	a.openPanel(a.startTerminal)
	a.openPanel(a.startTerminal)
	t := a.currentTab()
	t.paneNames[t.focused] = "server"
	t.name = "work"
}

func TestClosedSessionOpensAgainAsItWas(t *testing.T) {
	a := sessionsApp(t)
	twoShells(a)
	a.runAction("s") // a scripts panel beside them
	first := a.session
	before := len(first.tabs[0].views)

	second, _ := a.createSession("other")
	a.switchSession(second)
	if err := a.closeSession(first); err != nil {
		t.Fatal(err)
	}
	if first.closed == nil || len(first.tabs) != 0 {
		t.Fatal("closing kept the session's tabs")
	}

	a.switchSession(first)
	if len(first.tabs) != 1 {
		t.Fatalf("%d tabs after opening again", len(first.tabs))
	}
	restored := first.tabs[0]
	if len(restored.views) != before || restored.name != "work" {
		t.Errorf("restored %d panes named %q, want %d named work", len(restored.views), restored.name, before)
	}
	names := 0
	for _, name := range restored.paneNames {
		if name == "server" {
			names++
		}
	}
	if names != 1 {
		t.Errorf("pane names %v, want the server pane", restored.paneNames)
	}
}

func TestSessionsSurviveTheWorkspace(t *testing.T) {
	a := sessionsApp(t)
	twoShells(a)
	dev, _ := a.createSession("dev")
	a.switchSession(dev)
	a.leaveSession()

	b := sessionsApp(t)
	b.sessionsPath = a.sessionsPath
	b.sessions, b.session = nil, nil
	b.loadSessions()
	if b.session.name != "dev" || len(b.sessions) != 2 {
		t.Fatalf("loaded %d sessions, on %q; want 2, on dev", len(b.sessions), b.session.name)
	}
	first := b.findSession("1")
	if first.closed == nil || len(first.closed.Tabs) != 1 || len(first.closed.Tabs[0].Panes) != 2 {
		t.Errorf("session 1 was not kept with its tab: %+v", first.closed)
	}
}

func TestRestoringDropsPanesThatCannotOpenAgain(t *testing.T) {
	a := sessionsApp(t)
	tree := layout.New(1)
	tree.Split(1, 2, layout.SideBySide)
	restored := a.restoreTab(saved.Tab{
		Layout:  tree.Shape(),
		Focused: 2,
		Panes:   []saved.Pane{{ID: 1, Kind: saved.Terminal, Dir: "/does/not/exist"}},
	})
	if restored == nil || len(restored.views) != 1 || len(restored.tree.Panes()) != 1 {
		t.Fatalf("restored %+v, want the terminal alone", restored)
	}
	if restored.focused != restored.tree.Panes()[0] {
		t.Error("the focus stayed on a pane that did not open")
	}
}

func TestDeletingTheSessionOnScreenMovesToTheLastUsed(t *testing.T) {
	a := sessionsApp(t)
	first := a.session
	first.lastAccess = time.Now().Add(-time.Hour)
	dev, _ := a.createSession("dev")
	a.switchSession(dev)
	a.openPanel(a.startTerminal)
	shell := a.currentTab().focusedView().(*fakeView)

	a.deleteSession(dev)
	if a.session != first || a.findSession("dev") != nil {
		t.Errorf("on %q with dev still there: %v", a.session.name, a.findSession("dev") != nil)
	}
	if !shell.closed {
		t.Error("the deleted session's shell kept running")
	}
}

func TestTheOnlySessionCannotBeClosed(t *testing.T) {
	a := sessionsApp(t)
	if err := a.closeSession(a.session); err == nil {
		t.Error("closed the only session")
	}
}

func TestSessionsPopupCreatesAndRenames(t *testing.T) {
	a := sessionsApp(t)
	press(a, prefixPress, letter('o'), letter('n'))
	typeInto(a, "dev")
	press(a, enterPress)
	if a.session.name != "dev" || a.mode != normalMode {
		t.Fatalf("on %q in mode %v after creating dev", a.session.name, a.mode)
	}

	press(a, prefixPress, letter('o'), letter('r'), uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl})
	typeInto(a, "has space")
	press(a, enterPress)
	if a.session.name != "dev" || !strings.Contains(a.statusMessage, "spaces") {
		t.Errorf("renamed to %q, message %q", a.session.name, a.statusMessage)
	}
	press(a, letter('r'), uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl})
	typeInto(a, "api")
	press(a, enterPress)
	if a.session.name != "api" || a.mode != sessionsMode {
		t.Errorf("name %q in mode %v, want api back in the popup", a.session.name, a.mode)
	}
}

func TestAttachingAskedSessions(t *testing.T) {
	a := sessionsApp(t)
	if _, err := a.sessionToAttach(helloFor("nowhere", false)); err == nil {
		t.Error("attached to a session that does not exist")
	}
	s, err := a.sessionToAttach(helloFor("dev", true))
	if err != nil || s.name != "dev" {
		t.Fatalf("new dev: %v, %v", s, err)
	}
	if _, err := a.sessionToAttach(helloFor("dev", true)); err == nil {
		t.Error("created dev twice")
	}
	if s, _ := a.sessionToAttach(helloFor("", false)); s != a.session {
		t.Error("without a name, attached elsewhere than the last session")
	}
}

func TestLastUsed(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.Local)
	for when, want := range map[time.Time]string{
		now.Add(-10 * time.Second): "just now",
		now.Add(-5 * time.Minute):  "5 min ago",
		now.Add(-3 * time.Hour):    "3 h ago",
		now.AddDate(0, -1, 0):      "Aug 29 12:00",
		now.AddDate(-1, 0, 0):      "2025-09-29 12:00",
		{}:                         "never",
	} {
		if got := lastUsed(when, now, false); got != want {
			t.Errorf("lastUsed(%v) = %q, want %q", when, got, want)
		}
	}
}

func helloFor(name string, create bool) ipc.Hello {
	return ipc.Hello{Session: name, NewSession: create}
}
