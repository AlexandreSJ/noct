package app

import (
	"fmt"
	"image"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/pty"
)

// logsPreviewHints are the keys while the preview has the focus, scrolling the log's history.
const logsPreviewHints = "{up}{down} line  {pgup}{pgdown} page  {g} top  {G} live  {esc} back to the list"

// logPreview shows the selected log under the calendar and the list, following the selection and
// what the log still gets: a terminal prints the log and keeps following its end.
type logPreview struct {
	// path is the log shown, so moving to another log starts it over.
	path     string
	title    string
	terminal view
	size     pty.Size
	// offset is how many lines up the history it is scrolled; 0 is the live end.
	offset int
	// focused sends the keys to the preview, to scroll it, instead of to the list.
	focused bool
	// pageUp scrolls a page up once the terminal has its size.
	pageUp bool
	// area is where it was drawn, relative to the panel, for the mouse.
	area image.Rectangle
}

// focusPreview gives the log the keys, to scroll it, at the live end or a page up.
func (p *logsPanel) focusPreview(pageUp bool) {
	p.focusList = true
	p.preview.focused, p.preview.offset, p.preview.pageUp = true, 0, pageUp
}

func (v *logPreview) close() {
	if v.terminal != nil {
		v.terminal.Close()
		v.terminal = nil
	}
}

// previewKey scrolls the log's history; leaving goes back to the list, at the live end.
func (p *logsPanel) previewKey(name string) {
	v := &p.preview
	page := max(v.size.Rows-1, 1)
	switch name {
	case "up", "k":
		v.offset++
	case "down", "j":
		v.offset--
	case "pgup", "ctrl+b":
		v.offset += page
	case "pgdown", "ctrl+f":
		v.offset -= page
	case "home", "g":
		v.offset = v.scrollback()
	case "end", "G", "shift+g":
		v.offset = 0
	case "esc", "q", "left", "h":
		v.focused, v.offset = false, 0
	}
	v.offset = min(max(v.offset, 0), v.scrollback())
}

func (v *logPreview) scrollback() int {
	if s, ok := v.terminal.(scrollable); ok {
		return s.ScrollbackLen()
	}
	return 0
}

// follow starts the terminal over when the selection moved to another log. The log's older,
// rotated part prints first; the current one is followed, so a run still writing shows live.
func (p *logsPanel) follow(row logRow, ok bool, size pty.Size) {
	v := &p.preview
	if !ok {
		row.entry.Path = ""
	}
	if v.terminal != nil && v.path == row.entry.Path {
		if v.size != size {
			v.size = size
			v.terminal.Resize(size)
		}
		return
	}
	v.close()
	v.path, v.size, v.offset = row.entry.Path, size, 0
	if !ok {
		v.title = "no log selected"
		return
	}
	v.title = "log " + row.entry.At.Format("15:04:05") + " " + row.name()
	files := row.entry.LogFiles() // the current part first
	older := slices.Clone(files[1:])
	slices.Reverse(older)
	argv := append([]string{"/bin/sh", "-c", `current=$1; shift; [ $# -eq 0 ] || cat -- "$@"; exec tail -n +1 -f -- "$current"`, "log", files[0]}, older...)
	terminal, err := p.app.launch(program{argv: argv, dir: p.app.workDir, onExit: func(int) {}}, size)
	if err != nil {
		p.app.statusMessage = err.Error()
		return
	}
	v.terminal = terminal
}

// drawPreview draws the preview's title line and the log under it.
func (p *logsPanel) drawPreview(screen uv.Screen, area image.Rectangle, row logRow, ok bool) {
	pal := &p.app.palette
	v := &p.preview
	output := image.Rect(area.Min.X, area.Min.Y+1, area.Max.X, area.Max.Y)
	p.follow(row, ok, pty.Size{Cols: max(output.Dx(), 1), Rows: max(output.Dy(), 1)})
	if v.pageUp {
		v.offset, v.pageUp = max(v.size.Rows-1, 1), false
	}
	// the offset is kept past the history a new terminal has printed so far, which grows to it
	offset := min(v.offset, v.scrollback())
	style := pal.panelHeading
	if v.focused {
		style = pal.focusedTitle
	}
	title := v.title
	if offset > 0 {
		title += fmt.Sprintf("  ↑ %d/%d", offset, v.scrollback())
	}
	x := drawText(screen, area.Min.X, area.Min.Y, area.Max.X, "─ ", pal.unfocusedFrame)
	x = drawText(screen, x, area.Min.Y, area.Max.X, title+" ", style)
	drawText(screen, x, area.Min.Y, area.Max.X, strings.Repeat("─", max(area.Max.X-x, 0)), pal.unfocusedFrame)
	v.area = area
	if v.terminal == nil {
		return
	}
	if s, ok := v.terminal.(scrollable); ok {
		s.DrawScrolled(screen, output, offset)
	} else {
		v.terminal.Draw(screen, output)
	}
}

// previewMouse scrolls the log with the wheel; a click gives it the keys.
func (p *logsPanel) previewMouse(event uv.MouseEvent) {
	v := &p.preview
	switch event.(type) {
	case uv.MouseClickEvent:
		if event.Mouse().Button == uv.MouseLeft {
			p.focusList, v.focused = true, true
		}
	case uv.MouseWheelEvent:
		switch event.Mouse().Button {
		case uv.MouseWheelUp:
			v.offset += wheelLines
		case uv.MouseWheelDown:
			v.offset -= wheelLines
		}
		v.offset = min(max(v.offset, 0), v.scrollback())
	}
}
