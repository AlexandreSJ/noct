package app

import (
	"image"
	"path/filepath"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// rerunHints are the keys of the box offering to run again what a terminal ran before.
const rerunHints = "{enter} run again  esc cancel"

// shell is a pane running the user's shell.
type shell struct {
	// log is the terminal's log, or "" when terminals are not logged.
	log string
	// rerun is what the terminal ran when its session closed, offered until run or cancelled.
	rerun string
}

// pendingRerun is the command a terminal offers to run again, or "".
func (a *App) pendingRerun(id layout.PaneID) string {
	if sh := a.shells[id]; sh != nil {
		return sh.rerun
	}
	return ""
}

// handleRerunKey answers the box over the focused terminal: enter types the command into the
// shell, so it lands in the shell's history and the shell stays when it ends; esc forgets it.
// Other keys wait for an answer.
func (a *App) handleRerunKey(t *tab, key uv.KeyPressEvent) {
	sh := a.shells[t.focused]
	key, ok := a.keymap.resolve("rerun", key)
	switch {
	case !ok:
	case key.MatchString("enter"):
		typeInView(t.focusedView(), sh.rerun, true)
		sh.rerun = ""
	case key.MatchString("esc"):
		sh.rerun = ""
	}
}

// drawRerun shows, centered over a terminal, what it ran before and how to run it again.
func (a *App) drawRerun(screen uv.Screen, content image.Rectangle, command string) {
	program := filepath.Base(strings.Fields(command)[0])
	hints := a.hint("rerun", rerunHints)
	lines := []string{"was running " + program, command, hints}
	width := 0
	for _, line := range lines {
		width = max(width, ansi.StringWidth(line))
	}
	box := a.drawPanel(screen, centeredRect(content, width+6, panelHeight(len(lines))), "")
	drawCentered(screen, box, box.Min.Y, lines[0], a.palette.panelText)
	drawCentered(screen, box, box.Min.Y+1, ansi.Truncate(command, box.Dx(), "…"), a.palette.panelMuted)
	drawCentered(screen, box, box.Min.Y+2, hints, a.palette.panelMuted)
}

// typeInView types text into a view as keys, a newline as enter, and enter after it when asked.
func typeInView(v view, text string, enter bool) {
	for _, r := range text {
		if r == '\n' {
			v.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
			continue
		}
		v.SendKey(uv.KeyPressEvent{Code: r, Text: string(r)})
	}
	if enter {
		v.SendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
	}
}
