package app

import (
	"os"
	"testing"

	"github.com/AlexandreSJ/noct/internal/pty"
)

// dirView is a terminal that tells the folder its shell is in.
type dirView struct {
	fakeView
	dir string
}

func (v *dirView) Dir() string { return v.dir }

// besideShell is an app whose only pane is a shell in dir, with home as the home folder.
func besideShell(t *testing.T, dir string) (*App, string) {
	t.Helper()
	a := testApp(t, newTab(1, &dirView{dir: dir}))
	a.shells[1] = &shell{}
	return a, homeFolder(t)
}

func homeFolder(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func openedIn(t *testing.T, a *App, open func()) string {
	t.Helper()
	l := recordLaunches(a)
	open()
	return l.last(t).dir
}

func TestNewTerminalOpensInTheFocusedShellsFolder(t *testing.T) {
	dir := t.TempDir()
	a, _ := besideShell(t, dir)
	if got := openedIn(t, a, func() { a.openPanel(a.startTerminal) }); got != dir {
		t.Errorf("opened in %q, want the shell's %q", got, dir)
	}
}

func TestNewTerminalBesideARunOpensWhereTheRunStarted(t *testing.T) {
	dir := t.TempDir()
	start := func(pty.Size, func(int)) (view, error) { return &fakeView{}, nil }
	a := testApp(t)
	run, err := newRunView("run", &a.palette, start, func() {}, pty.Size{Cols: 80, Rows: 24}, dir)
	if err != nil {
		t.Fatal(err)
	}
	a.session.tabs = append(a.session.tabs, newTab(1, run))
	homeFolder(t)
	if got := openedIn(t, a, func() { a.openPanel(a.startTerminal) }); got != dir {
		t.Errorf("opened in %q, want the run's %q", got, dir)
	}
}

func TestNewTerminalBesideAPanelWithoutAFolderOpensAtHome(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	home := homeFolder(t)
	if got := openedIn(t, a, func() { a.openPanel(a.startTerminal) }); got != home {
		t.Errorf("opened in %q, want home %q", got, home)
	}
}

func TestNewTerminalOnTheHomeScreenOpensAtHome(t *testing.T) {
	a := testApp(t)
	home := homeFolder(t)
	if got := openedIn(t, a, func() { a.openPanel(a.startTerminal) }); got != home {
		t.Errorf("opened in %q, want home %q", got, home)
	}
}

func TestNewTerminalBesideAShellWhoseFolderCannotBeReadOpensInTheAttachFolder(t *testing.T) {
	a, _ := besideShell(t, "")
	if got := openedIn(t, a, func() { a.openPanel(a.startTerminal) }); got != a.workDir {
		t.Errorf("opened in %q, want the attach folder %q", got, a.workDir)
	}
}

func TestNewTerminalBesideAShellInADeletedFolderOpensAtHome(t *testing.T) {
	gone := t.TempDir()
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	// Linux tells a deleted folder this way.
	a, home := besideShell(t, gone+" (deleted)")
	if got := openedIn(t, a, func() { a.openPanel(a.startTerminal) }); got != home {
		t.Errorf("opened in %q, want home %q", got, home)
	}
}

func TestFloatingTerminalOpensInTheFocusedShellsFolder(t *testing.T) {
	dir := t.TempDir()
	a, _ := besideShell(t, dir)
	if got := openedIn(t, a, a.openFloating); got != dir {
		t.Errorf("opened in %q, want the shell's %q", got, dir)
	}
}

func TestNewTerminalBesideAFocusedFloatingPaneUsesItsFolder(t *testing.T) {
	dir := t.TempDir()
	a := testApp(t, newTab(1, &fakeView{}))
	homeFolder(t)
	a.launch = func(program, pty.Size) (view, error) { return &dirView{dir: dir}, nil }
	a.openFloating()
	if got := openedIn(t, a, func() { a.openPanel(a.startTerminal) }); got != dir {
		t.Errorf("opened in %q, want the floating shell's %q", got, dir)
	}
}

func TestNewTerminalWithoutAHomeFolderOpensInTheAttachFolder(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	t.Setenv("HOME", "")
	if got := openedIn(t, a, func() { a.openPanel(a.startTerminal) }); got != a.workDir {
		t.Errorf("opened in %q, want the attach folder %q", got, a.workDir)
	}
}

func TestNewTabOpensInTheFolderOfThePanelLeftBehind(t *testing.T) {
	dir := t.TempDir()
	a, _ := besideShell(t, dir)
	if got := openedIn(t, a, a.openNewTab); got != dir {
		t.Errorf("opened in %q, want the shell's %q", got, dir)
	}
	if len(a.session.tabs) != 2 {
		t.Errorf("%d tabs, want 2", len(a.session.tabs))
	}
}

func TestNewTabFromTheHomeScreenOpensAtHome(t *testing.T) {
	a := testApp(t)
	home := homeFolder(t)
	if got := openedIn(t, a, a.openNewTab); got != home {
		t.Errorf("opened in %q, want home %q", got, home)
	}
}
