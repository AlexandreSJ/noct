package app

import (
	"image"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// rerunPrompt is the mark before the command the box offers to run again.
const rerunPrompt = "$ "

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

// drawRerun shows, centered over a terminal, what it ran before and how to run it again: the
// command stands out in bold behind a prompt mark, and the keys in the accent beside their muted
// labels, so neither hides among the rest.
func (a *App) drawRerun(screen uv.Screen, content image.Rectangle, command string) {
	pal := a.palette
	hints := []styledText{
		{a.hint("rerun", "{enter}"), pal.focusedTitle}, {" run again", pal.panelMuted},
		{"  ·  ", pal.panelMuted},
		{"esc", pal.focusedTitle}, {" cancel", pal.panelMuted},
	}
	if hints[0].text == "" {
		hints = hints[3:]
	}
	width := max(styledWidth(hints), ansi.StringWidth(rerunPrompt+command))
	box := a.drawPanel(screen, centeredRect(content, width+6, panelHeight(3)), "run again?")
	line := ansi.Truncate(command, box.Dx()-ansi.StringWidth(rerunPrompt), "…")
	drawStyledCentered(screen, box, box.Min.Y, []styledText{{rerunPrompt, pal.focusedTitle}, {line, pal.panelHeading}})
	drawStyledCentered(screen, box, box.Min.Y+2, hints)
}

// styledText is a piece of a line drawn in its own style.
type styledText struct {
	text  string
	style uv.Style
}

// styledWidth is how many cells the pieces of a line take.
func styledWidth(pieces []styledText) int {
	width := 0
	for _, piece := range pieces {
		width += ansi.StringWidth(piece.text)
	}
	return width
}

// drawStyledCentered writes the pieces of a line one after another, centered in area on row y.
func drawStyledCentered(screen uv.Screen, area image.Rectangle, y int, pieces []styledText) {
	x := area.Min.X + max(area.Dx()-styledWidth(pieces), 0)/2
	for _, piece := range pieces {
		x = drawText(screen, x, y, area.Max.X, piece.text, piece.style)
	}
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
