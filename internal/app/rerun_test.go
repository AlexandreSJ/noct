package app

import (
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

// commandView is a terminal whose shell runs a program.
type commandView struct {
	fakeView
	command string
}

func (v *commandView) ForegroundCommand() string { return v.command }

// typed is what a view received as text, with enter as a newline.
func typed(v *fakeView) string {
	var out strings.Builder
	for _, key := range v.keys {
		if key.MatchString("enter") {
			out.WriteByte('\n')
			continue
		}
		out.WriteString(key.Text)
	}
	return out.String()
}

// restoredTerminal opens a session's terminal again, saved while its shell ran command.
func restoredTerminal(t *testing.T, a *App, command string) (*fakeView, layout.PaneID) {
	t.Helper()
	l := recordLaunches(a)
	tree := layout.New(1)
	a.session.tabs = append(a.session.tabs, a.restoreTab(saved.Tab{
		Layout:  tree.Shape(),
		Focused: 1,
		Panes:   []saved.Pane{{ID: 1, Kind: saved.Terminal, Command: command}},
	}))
	tab := a.currentTab()
	if tab == nil || len(l.views) != 1 {
		t.Fatal("the terminal did not open again")
	}
	return l.views[0], tab.focused
}

func TestClosingASessionKeepsWhatItsTerminalsRan(t *testing.T) {
	a := sessionsApp(t)
	a.launch = func(program, pty.Size) (view, error) { return &commandView{command: "claude --resume"}, nil }
	a.openPanel(a.startTerminal)
	first := a.session
	second, _ := a.createSession("other")
	a.switchSession(second)
	if err := a.closeSession(first); err != nil {
		t.Fatal(err)
	}
	if got := first.closed.Tabs[0].Panes[0].Command; got != "claude --resume" {
		t.Errorf("kept command %q, want claude --resume", got)
	}

	recordLaunches(a)
	a.switchSession(first)
	id := first.tabs[0].focused
	if got := a.pendingRerun(id); got != "claude --resume" {
		t.Errorf("offered %q, want claude --resume", got)
	}
	// Until it is answered, the offer survives closing again.
	if got := a.savedSession(first).Tabs[0].Panes[0].Command; got != "claude --resume" {
		t.Errorf("saved again as %q", got)
	}
}

func TestEnterRunsTheCommandAgainInTheShell(t *testing.T) {
	a := sessionsApp(t)
	v, id := restoredTerminal(t, a, "htop -d 5")
	press(a, letter('x'))
	if len(v.keys) != 0 {
		t.Fatalf("a key reached the shell before the box was answered: %q", typed(v))
	}
	press(a, enterPress)
	if got := typed(v); got != "htop -d 5\n" {
		t.Errorf("typed %q, want the command and enter", got)
	}
	if a.pendingRerun(id) != "" {
		t.Error("the box stayed after running")
	}
	press(a, letter('x'))
	if got := typed(v); !strings.HasSuffix(got, "x") {
		t.Error("keys do not reach the shell after the box")
	}
}

func TestEscForgetsTheCommand(t *testing.T) {
	a := sessionsApp(t)
	v, id := restoredTerminal(t, a, "htop")
	press(a, escPress)
	if len(v.keys) != 0 || a.pendingRerun(id) != "" {
		t.Errorf("after esc: typed %q, still offering %q", typed(v), a.pendingRerun(id))
	}
	if got := a.savedSession(a.session).Tabs[0].Panes[0].Command; got != "" {
		t.Errorf("a cancelled command was saved: %q", got)
	}
}

func TestCommandsWithControlCharactersAreNotOffered(t *testing.T) {
	a := sessionsApp(t)
	_, id := restoredTerminal(t, a, "echo 'a\x15b'")
	if a.pendingRerun(id) != "" {
		t.Error("offered a command holding ctrl+u")
	}
}

func TestTerminalGoesOnWithItsLog(t *testing.T) {
	a := sessionsApp(t)
	a.config.Logs.Terminals = true
	l := recordLaunches(a)
	a.openPanel(a.startTerminal)
	started := l.last(t)
	if started.logPath == "" || started.continueLog {
		t.Fatalf("a new terminal logs to %q, continuing %v", started.logPath, started.continueLog)
	}
	kept := a.savedSession(a.session).Tabs[0]

	a.restoreTab(kept)
	reopened := l.last(t)
	if reopened.logPath != started.logPath || !reopened.continueLog {
		t.Errorf("reopened logging to %q, continuing %v; want %q, continuing", reopened.logPath, reopened.continueLog, started.logPath)
	}

	a.config.Logs.Terminals = false
	a.restoreTab(kept)
	if l.last(t).logPath != "" {
		t.Error("a terminal logged with terminal logs off")
	}
}
