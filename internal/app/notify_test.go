package app

import (
	"bytes"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/script"
)

// notified records what the system's notifications were asked to tell, and what reached the
// attached terminal.
type notified struct {
	system []string
	out    bytes.Buffer
}

func notifyingApp(t *testing.T, how string) (*App, *notified) {
	t.Helper()
	a := testApp(t)
	n := &notified{}
	a.config.Notify = how
	a.desktopNotify = func(title, body string) bool {
		n.system = append(n.system, title+": "+body)
		return true
	}
	a.screen = uv.NewTerminalScreen(&n.out, uv.Environ{"TERM=xterm-256color"})
	a.client = &client{hello: helloWith()}
	return a, n
}

func helloWith(env ...string) ipc.Hello { return ipc.Hello{Env: env} }

func (n *notified) terminal(t *testing.T, a *App) string {
	t.Helper()
	if err := a.screen.Flush(); err != nil {
		t.Fatal(err)
	}
	return n.out.String()
}

// runOutOfSight starts the script fail beside the focused pane and moves to a new tab, where its
// run is not on screen; end ends it with a code.
func runOutOfSight(t *testing.T, a *App) (end func(code int)) {
	t.Helper()
	l := recordLaunches(a)
	saveScript(t, "fail", "run: exit 3\n")
	openScriptsPanel(t, a)
	press(a, enterPress)
	a.openEmptyTab()
	return l.last(t).onExit
}

func TestARunEndingOutOfSightIsToldBySystemNotification(t *testing.T) {
	a, n := notifyingApp(t, config.NotifySystem)
	runOutOfSight(t, a)(3)
	if len(n.system) != 1 || !strings.Contains(n.system[0], "fail") || !strings.Contains(n.system[0], "exit 3") {
		t.Fatalf("notifications %q, want one telling fail failed with exit 3", n.system)
	}
	if out := n.terminal(t, a); strings.Contains(out, "\x1b]9;") {
		t.Errorf("told the terminal too: %q", out)
	}
}

func TestARunEndingInSightIsNotTold(t *testing.T) {
	a, n := notifyingApp(t, config.NotifySystem)
	l := recordLaunches(a)
	saveScript(t, "fail", "run: exit 3\n")
	openScriptsPanel(t, a)
	press(a, enterPress)
	l.last(t).onExit(3)
	if len(n.system) != 0 {
		t.Errorf("told %q about a run on screen", n.system)
	}
}

func TestWithoutSystemNotificationsTheTerminalTellsIt(t *testing.T) {
	a, n := notifyingApp(t, config.NotifySystem)
	a.desktopNotify = func(string, string) bool { return false }
	runOutOfSight(t, a)(0)
	if out := n.terminal(t, a); !strings.Contains(out, "\x1b]9;noct: fail finished") {
		t.Errorf("the terminal got %q, want an OSC 9 notification", out)
	}
}

func TestOverSSHTheTerminalTellsIt(t *testing.T) {
	a, n := notifyingApp(t, config.NotifySystem)
	a.client.hello = helloWith("SSH_CONNECTION=10.0.0.1 22 10.0.0.2 22")
	runOutOfSight(t, a)(0)
	if len(n.system) != 0 || !strings.Contains(n.terminal(t, a), "\x1b]9;") {
		t.Errorf("over ssh: notifications %q, terminal %q; want the terminal's", n.system, n.out.String())
	}
}

func TestTheBellRingsWhenChosen(t *testing.T) {
	a, n := notifyingApp(t, config.NotifyBell)
	runOutOfSight(t, a)(0)
	if out := n.terminal(t, a); len(n.system) != 0 || !strings.Contains(out, "\a") || strings.Contains(out, "\x1b]9;") {
		t.Errorf("bell: notifications %q, terminal %q; want only the bell", n.system, out)
	}
}

func TestNotificationsOff(t *testing.T) {
	a, n := notifyingApp(t, config.NotifyOff)
	runOutOfSight(t, a)(0)
	if out := n.terminal(t, a); len(n.system) != 0 || strings.ContainsAny(out, "\a") {
		t.Errorf("off: notifications %q, terminal %q", n.system, out)
	}
}

func TestATriggeredFlowEndingIsTold(t *testing.T) {
	a, n := notifyingApp(t, config.NotifySystem)
	a.finishTriggered(script.Run{Flow: "nightly", Trigger: "cron"}, 1)
	if len(n.system) != 1 || !strings.Contains(n.system[0], "flow nightly (cron) failed with exit 1") {
		t.Errorf("notifications %q, want the triggered flow's failure", n.system)
	}
}

func TestRunEndedText(t *testing.T) {
	for _, c := range []struct {
		code int
		want string
	}{{0, "deploy finished"}, {2, "deploy failed with exit 2"}} {
		if got := runEndedText("deploy", c.code); got != c.want {
			t.Errorf("runEndedText(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestTheConfigStepsHowRunsAreTold(t *testing.T) {
	a := openConfigAt(t, notifyRow)
	for _, want := range []string{config.NotifyBell, config.NotifyOff, config.NotifyOff} {
		press(a, rightPress)
		if a.config.Notify != want {
			t.Fatalf("notify %q, want %q", a.config.Notify, want)
		}
	}
	if !strings.Contains(screenText(a), "tell when runs end") {
		t.Errorf("no notify row:\n%s", screenText(a))
	}
}
