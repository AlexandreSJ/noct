package app

import (
	"encoding/json"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// requestPane sends a pane request to the workspace as noct flow run does, and returns where the
// answer comes.
func requestPane(t *testing.T, a *App, req ipc.PaneRequest) <-chan ipc.Exit {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	answers := make(chan ipc.Exit, 1)
	go func() {
		m, err := ipc.NewConn(client).Receive()
		var exit ipc.Exit
		if err == nil && m.Kind == ipc.KindExit {
			err = json.Unmarshal(m.Payload, &exit)
		}
		if err != nil {
			exit.Reason = err.Error()
		}
		answers <- exit
	}()
	a.handleConnection(connection{conn: ipc.NewConn(server), hello: ipc.Hello{Version: ipc.Version, Purpose: ipc.OpenPane, Pane: &req}})
	return answers
}

// A step's pane opens beside the run that asked, and its exit code goes back when it ends.
func TestPaneRequestWaitsForTheExitCode(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2), newTab(3, &fakeView{}))
	a.activeTab = 1
	l := recordLaunches(a)
	answers := requestPane(t, a, ipc.PaneRequest{Command: "make test", Dir: "/tmp", Title: "build", Near: 2, Wait: true})
	program := l.last(t)
	if !slices.Contains(program.argv, "make test") || program.dir != "/tmp" {
		t.Fatalf("started %+v", program)
	}
	// It opens in the asking run's tab, focused there, without changing the tab on screen.
	if asking := a.tabs[0]; len(asking.views) != 3 || asking.focused == 1 || asking.focused == 2 || a.activeTab != 1 {
		t.Errorf("tab of the run: %d panes, focus %d; tab on screen %d", len(asking.views), asking.focused, a.activeTab)
	}
	program.onExit(5)
	if exit := <-answers; exit.Reason != ipc.PaneExited || exit.Code != 5 {
		t.Errorf("answer = %+v", exit)
	}
}

// A pane in the background answers once open and leaves the focus on the run.
func TestPaneRequestInTheBackgroundAnswersAtOnce(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	a.currentTab().focus(2)
	recordLaunches(a)
	answers := requestPane(t, a, ipc.PaneRequest{Command: "tail -f log", Title: "logs", Near: 2})
	if exit := <-answers; exit.Reason != ipc.PaneOpened {
		t.Errorf("answer = %+v", exit)
	}
	if len(a.currentTab().views) != 3 || focusedPaneID(a) != 2 {
		t.Errorf("%d panes, focus %d", len(a.currentTab().views), focusedPaneID(a))
	}
}

// A flow's prompt waits for the user: its run's tab comes to the screen, with the run focused.
func TestAttentionShowsTheRunThatAsks(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2), newTab(3, &fakeView{}))
	a.activeTab = 1
	answerOf(t, ask(t, a, "attention", paneArgs{Pane: 2}), nil)
	if a.activeTab != 0 || a.tabs[0].focused != 2 {
		t.Errorf("tab on screen %d, focus %d", a.activeTab, a.tabs[0].focused)
	}
	if !strings.Contains(a.notice, "waits for your answer") {
		t.Errorf("notice = %q", a.notice)
	}
	if exit := <-ask(t, a, "attention", paneArgs{Pane: 9}); exit.Reason == ipc.RequestDone {
		t.Error("a pane that is gone got attention")
	}
}
