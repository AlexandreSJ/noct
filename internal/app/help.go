package app

import (
	"image"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// helpScript pages through what explains a program: the notes of a script that uses it, then
// the program's --help, then its manual. stdin is closed so a program that ignores --help and
// waits for input cannot steal the keyboard from the pager.
const helpScript = `{ [ -n "$2" ] && cat "$2" && echo; "$1" --help </dev/null 2>&1; echo; MANPAGER=cat man "$1" 2>/dev/null; } | ${PAGER:-less}`

// foregrounder is a view that runs programs, such as a terminal with a shell.
type foregrounder interface {
	ForegroundProgram() (string, bool)
}

// showHelp opens the help of the program running in the focused terminal beside it. With only
// the shell running there is no program to explain, so the popup of noct's keys stays open.
func (a *App) showHelp() {
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		return
	}
	terminal, ok := t.focusedView().(foregrounder)
	if !ok {
		a.statusMessage = "help is for programs running in a terminal"
		return
	}
	name, running := terminal.ForegroundProgram()
	if !running {
		a.statusMessage = "no program is running in this terminal"
		return
	}
	notes := ""
	for _, s := range a.scripts.All() {
		if s.Program == name {
			notes = s.NotesFile()
			break
		}
	}
	a.mode = normalMode
	argv := []string{"/bin/sh", "-c", helpScript, "help", name, notes}
	a.openPanel(func(id layout.PaneID, frame image.Rectangle) (view, error) {
		return a.startInPane(id, frame, argv, a.workDir, "help-"+name)
	})
}
