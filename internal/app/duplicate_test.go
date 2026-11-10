package app

import "testing"

// busyShell is a terminal whose shell is in dir running command.
type busyShell struct {
	fakeView
	dir, command string
}

func (v *busyShell) Dir() string               { return v.dir }
func (v *busyShell) ForegroundCommand() string { return v.command }

func TestDuplicateOpensATerminalBesideOfferingWhatTheFocusedOneRuns(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t, newTab(1, &busyShell{dir: dir, command: "htop -d 5"}))
	a.shells[1] = &shell{}
	homeFolder(t)
	l := recordLaunches(a)
	press(a, prefixPress, letter('D'))
	if len(l.programs) != 1 || l.last(t).dir != dir {
		t.Fatalf("started %+v, want one terminal in %q", l.programs, dir)
	}
	id := focusedPaneID(a)
	if id == 1 || a.pendingRerun(id) != "htop -d 5" {
		t.Errorf("focus on %d offering %q, want the new terminal offering htop", id, a.pendingRerun(id))
	}
}

func TestDuplicatingAnIdleShellOffersNothing(t *testing.T) {
	a := testApp(t, newTab(1, &busyShell{dir: t.TempDir()}))
	a.shells[1] = &shell{}
	press(a, prefixPress, letter('D'))
	if id := focusedPaneID(a); id == 1 || a.pendingRerun(id) != "" {
		t.Errorf("focus on %d offering %q, want a new terminal offering nothing", id, a.pendingRerun(id))
	}
}

func TestOnlyATerminalDuplicates(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	l := recordLaunches(a)
	press(a, prefixPress, letter('D'))
	if len(l.programs) != 0 || a.statusMessage == "" {
		t.Errorf("started %d programs with message %q, want none and a reason", len(l.programs), a.statusMessage)
	}
}
