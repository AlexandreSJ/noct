package app

import (
	"encoding/json"
	"net"
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// linesView is a pane that shows a line of text, for reading panes.
type linesView struct {
	fakeView
	text string
}

func (v *linesView) Draw(screen uv.Screen, area uv.Rectangle) {
	drawText(screen, area.Min.X, area.Min.Y, area.Max.X, v.text, uv.Style{})
}

func (v *linesView) Title() string { return "server" }

// ask sends a request to the workspace as noct mcp does, and returns where the answer comes.
func ask(t *testing.T, a *App, action string, args any) <-chan ipc.Exit {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	answers := make(chan ipc.Exit, 1)
	go func() {
		m, err := ipc.NewConn(client).Receive()
		var exit ipc.Exit
		if err == nil {
			err = json.Unmarshal(m.Payload, &exit)
		}
		if err != nil {
			exit.Reason = err.Error()
		}
		answers <- exit
	}()
	hello := ipc.Hello{Version: ipc.Version, Purpose: ipc.Ask, Request: &ipc.Request{Action: action, Args: raw}}
	a.handleConnection(connection{conn: ipc.NewConn(server), hello: hello})
	return answers
}

func answerOf(t *testing.T, answers <-chan ipc.Exit, into any) {
	t.Helper()
	exit := <-answers
	if exit.Reason != ipc.RequestDone {
		t.Fatalf("answer: %s", exit.Reason)
	}
	if into != nil {
		if err := json.Unmarshal(exit.Data, into); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRequestsListAndReadPanes(t *testing.T) {
	tab := newTab(1, &linesView{text: "listening on :8080"})
	a := testApp(t, tab)
	var panes []PaneInfo
	answerOf(t, ask(t, a, "list_panes", nil), &panes)
	if len(panes) != 1 || panes[0].ID != 1 || panes[0].Title != "server" || !panes[0].Focused {
		t.Fatalf("panes = %+v", panes)
	}
	var screen struct{ Text string }
	answerOf(t, ask(t, a, "read_pane", map[string]int{"pane": 1}), &screen)
	if !strings.Contains(screen.Text, "listening on :8080") {
		t.Errorf("screen = %q", screen.Text)
	}
	if exit := <-ask(t, a, "read_pane", map[string]int{"pane": 9}); exit.Reason == ipc.RequestDone {
		t.Error("read a pane that does not exist")
	}
}

func TestRequestsSendInputAndOpenCommands(t *testing.T) {
	target := &fakeView{}
	a := testApp(t, newTab(1, target))
	answerOf(t, ask(t, a, "send_input", map[string]any{"pane": 1, "text": "ls", "enter": true}), nil)
	if len(target.keys) != 3 || target.keys[2].Code != uv.KeyEnter {
		t.Errorf("keys = %+v", target.keys)
	}
	l := recordLaunches(a)
	var opened struct{ Pane int }
	answerOf(t, ask(t, a, "open_command", map[string]string{"command": "make test", "dir": "/tmp"}), &opened)
	if program := l.last(t); !slices.Contains(program.argv, "make test") || program.dir != "/tmp" || opened.Pane == 0 {
		t.Errorf("opened %d with %+v", opened.Pane, program)
	}
}

// A run asked for opens where the user sees it, like r in the TUI, and tells where its log goes.
func TestRequestsRunAFlowInThePanel(t *testing.T) {
	a := testApp(t)
	l := recordLaunches(a)
	saveFlow(t, "trio", threeSteps)
	var started struct {
		RunID string `json:"run_id"`
	}
	answerOf(t, ask(t, a, "run", map[string]any{"flow": "trio", "params": map[string]string{"x": "1"}, "steps": "0..1"}), &started)
	argv := l.last(t).argv[1:]
	if !slices.Equal(argv, []string{"flow", "run", "trio", "--steps", "0..1", "x=1"}) || !strings.HasSuffix(started.RunID, "-flow-trio.log") {
		t.Errorf("ran %v, run id %q", argv, started.RunID)
	}
	saveFlow(t, "drafted", "draft: true\nsteps: [{run: 'true'}]\n")
	if exit := <-ask(t, a, "run", map[string]any{"flow": "drafted"}); !strings.Contains(exit.Reason, "draft") {
		t.Errorf("a draft ran: %+v", exit)
	}
}

// An action the policy asks about waits for the user: prefix+y shows it in the MCP popup, above
// the setups, and y approves, n denies.
func TestApprovalsWaitForTheUser(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	first := ask(t, a, "approve", map[string]string{"tool": "run_command", "summary": "rm -rf build"})
	second := ask(t, a, "approve", map[string]string{"tool": "send_input", "summary": "type ls in pane 1"})
	if !strings.Contains(a.notice, "prefix+y") {
		t.Errorf("notice = %q", a.notice)
	}
	press(a, prefixPress, letter('y'))
	if a.mode != mcpMode || !strings.Contains(screenText(a), "rm -rf build") {
		t.Fatalf("mode %v, screen:\n%s", a.mode, screenText(a))
	}
	press(a, letter('y'))
	var decision struct{ Approved bool }
	answerOf(t, first, &decision)
	if !decision.Approved {
		t.Error("y did not approve")
	}
	press(a, letter('n'))
	answerOf(t, second, &decision)
	if decision.Approved || len(a.approvals) != 0 {
		t.Errorf("n: approved %v, %d waiting", decision.Approved, len(a.approvals))
	}
	if screen := screenText(a); a.mode != mcpMode || strings.Contains(screen, "Waiting for approval") {
		t.Errorf("after deciding every approval, mode %v, screen:\n%s", a.mode, screen)
	}
}
