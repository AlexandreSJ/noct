package app

import (
	"path/filepath"
	"testing"
)

// titledDirView is a terminal in dir that set title, "" for none of its own.
type titledDirView struct {
	dirView
	title string
}

func (v *titledDirView) Title() string { return v.title }

func TestAShellsTitleTellsItsFolder(t *testing.T) {
	view := &titledDirView{}
	a := testApp(t, newTab(1, view))
	a.shells[1] = &shell{}
	home := homeFolder(t)
	shell := filepath.Base(a.shellArgv[0])
	cases := []struct {
		name, dir, title, paneName, want string
	}{
		{"in home", home, "", "", shell + " · ~"},
		{"under home", filepath.Join(home, "proj"), "", "", shell + " · ~/proj"},
		{"elsewhere", "/srv/data", "", "", shell + " · /srv/data"},
		{"beside home", home + "x", "", "", shell + " · " + home + "x"},
		{"not told", "", "", "", shell},
		{"its own title", home, "vim notes.md", "", "vim notes.md"},
		{"named by the user", home, "", "build", "build"},
	}
	for _, c := range cases {
		view.dir, view.title = c.dir, c.title
		a.currentTab().paneNames[1] = c.paneName
		if got := a.paneTitle(a.currentTab(), 1); got != c.want {
			t.Errorf("%s: title %q, want %q", c.name, got, c.want)
		}
	}
	view.dir, view.title = home, ""
	a.currentTab().paneNames[1] = ""
	if got := a.viewTitle(a.currentTab(), 1); got != shell {
		t.Errorf("the tab's name is %q, want the shell's alone", got)
	}
}

func TestOnlyAShellsTitleTellsItsFolder(t *testing.T) {
	a := testApp(t, newTab(1, &titledDirView{dirView: dirView{dir: "/srv"}}))
	if got, want := a.paneTitle(a.currentTab(), 1), filepath.Base(a.shellArgv[0]); got != want {
		t.Errorf("a program that is not a shell has the title %q, want %q", got, want)
	}
}
