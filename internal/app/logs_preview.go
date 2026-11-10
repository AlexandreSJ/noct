package app

import (
	"fmt"
	"image"
	"math"
	"slices"
	"strconv"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/pty"
)

// logsPreviewHints are the keys while the preview has the focus, scrolling the log's history.
const logsPreviewHints = "{up}{down} line  {pgup}{pgdown} page  {g} top  {G} live  {/} search  {n}{N} matches  {f} filter  {enter} the list  {p} move  {+}{-} size  {?} keys"

// logPartLines is how many lines of a log the preview holds at a time. A terminal keeps a
// bounded history, and a log can have millions of lines: scrolling past either end of the part
// shown goes on into the part before or after it, so every line of the log can be reached.
const logPartLines = 5000

// atPartTop is an offset past any history, which the drawing brings down to the oldest line.
const atPartTop = math.MaxInt32

// logPreview shows the selected log beside the calendar and the list, following the selection
// and what the log still gets: a terminal prints the log and keeps following its end.
type logPreview struct {
	// path is the log shown, so moving to another log starts it over.
	path     string
	title    string
	terminal view
	size     pty.Size
	// offset is how many lines up the history it is scrolled; 0 is the end of the part.
	offset int
	// lines is how many lines the log had when it was selected, which places the parts; back is
	// how many parts before the last one the part shown is, 0 for the last, which follows the log.
	lines int
	back  int
	// focused sends the keys to the preview, to scroll it, instead of to the list.
	focused bool
	// pageUp scrolls a page up once the terminal has its size.
	pageUp bool
	// area is where it was drawn, relative to the panel, for the mouse.
	area image.Rectangle
	// query is what the log is searched for, and match the line of history and screen the last
	// match is on, while matched. filter keeps only the lines with it.
	query   string
	match   int
	matched bool
	filter  string
}

// focusPreview gives the log the keys, to scroll it, at the live end or a page up.
func (p *logsPanel) focusPreview(pageUp bool) {
	if p.shown == logsListOnly {
		p.shown = logsBoth
	}
	p.focusList = true
	p.preview.focused, p.preview.offset, p.preview.pageUp = true, 0, pageUp
	p.preview.showPart(0, false)
}

// leavePreview gives the keys back to the list, with the log at its live end again.
func (p *logsPanel) leavePreview() {
	v := &p.preview
	v.focused, v.offset = false, 0
	v.showPart(0, false)
	if p.shown == logsLogOnly {
		p.shown = logsBoth
	}
}

// clearSearch ends the search and the filter, telling whether there was one.
func (v *logPreview) clearSearch() bool {
	searched := v.query != "" || v.filter != ""
	if v.filter != "" {
		v.setFilter("")
	}
	v.query, v.matched = "", false
	return searched
}

func (v *logPreview) close() {
	if v.terminal != nil {
		v.terminal.Close()
		v.terminal = nil
	}
}

// parts is how many parts of logPartLines the log is shown in; filtered, it shows whole.
func (v *logPreview) parts() int {
	if v.filter != "" {
		return 1
	}
	return max((v.lines+logPartLines-1)/logPartLines, 1)
}

// partLines are the first and last lines of the part shown, counted from 1.
func (v *logPreview) partLines() (int, int) {
	last := v.lines - v.back*logPartLines
	return max(last-logPartLines, 0) + 1, last
}

// showPart moves to another part, at its oldest line or at its end, starting the terminal over.
// It tells whether there is such a part.
func (v *logPreview) showPart(back int, top bool) bool {
	if back == v.back || back < 0 || back >= v.parts() {
		return false
	}
	v.close()
	v.back, v.offset = back, 0
	if top {
		v.offset = atPartTop
	}
	return true
}

// previewKey scrolls the log's history, into the parts before and after the one shown at its
// ends; leaving goes back to the list, at the live end.
func (p *logsPanel) previewKey(name string) {
	v := &p.preview
	switch name {
	case "esc", "q":
		if !v.clearSearch() {
			p.leavePreview()
		}
		return
	case "left", "h", "enter", "space":
		p.leavePreview()
		return
	case "/":
		p.searchLog()
		return
	case "n", "N", "shift+n":
		p.nextMatch(name == "n")
		return
	case "f":
		p.filterLog()
		return
	}
	if v.changePart(name) {
		return
	}
	v.offset = min(v.offset, v.scrollback()) // a part opened at its top has the offset past it
	if offset, ok := scrollStep(name, v.offset, max(v.size.Rows-1, 1), v.scrollback()); ok {
		v.offset = offset
	}
}

// changePart moves to the part a key leads to: the first for the top, the last for live, and
// the next one past either end of the part shown.
func (v *logPreview) changePart(name string) bool {
	switch name {
	case "home", "g":
		return v.showPart(v.parts()-1, true)
	case "end", "G", "shift+g":
		return v.showPart(0, false)
	case "up", "k", "ctrl+y", "ctrl+u", "pgup", "ctrl+b":
		return v.offset >= v.scrollback() && v.showPart(v.back+1, false)
	case "down", "j", "ctrl+e", "pgdown", "ctrl+f":
		return v.offset == 0 && v.showPart(v.back-1, true)
	}
	return false
}

func (v *logPreview) scrollback() int {
	if s, ok := v.terminal.(scrollable); ok {
		return s.ScrollbackLen()
	}
	return 0
}

// follow starts the terminal over when the selection moved to another log or to another part
// of it, and gives it the size it is drawn at.
func (p *logsPanel) follow(row logRow, ok bool, size pty.Size) {
	v := &p.preview
	if !ok {
		row.entry.Path = ""
	}
	if v.path != row.entry.Path {
		v.close()
		v.path, v.offset, v.back = row.entry.Path, 0, 0
		v.query, v.matched, v.filter = "", false, ""
		if ok {
			v.lines = p.linesOf(row.entry)
		}
	}
	if !ok {
		v.title = "no log selected"
		return
	}
	if v.terminal != nil {
		if v.size != size {
			v.size = size
			v.terminal.Resize(size)
		}
		return
	}
	if v.back == 0 {
		v.lines = p.linesOf(row.entry) // the part that follows the log starts from its end now
	}
	v.size = size
	v.title = "log " + lastWrite(row.entry).Format("15:04:05") + " " + row.name()
	// a clear near the end of the log would otherwise leave nothing above it to scroll to, and a
	// log ending inside a full-screen program no history at all
	terminal, err := p.app.launch(program{argv: v.partCommand(row.entry), dir: p.app.workDir, onExit: func(int) {}, keepScrollback: true, historyUnderFullScreen: true}, size)
	if err != nil {
		p.app.statusMessage = err.Error()
		return
	}
	v.terminal = terminal
}

// partCommand prints the part shown, out of the log's older, rotated file and then its current
// one, which the last part goes on following so a run still writing shows live. The current
// file's path always comes last.
func (v *logPreview) partCommand(e logstore.Entry) []string {
	files := slices.Clone(e.LogFiles()) // the current part first
	slices.Reverse(files)
	current := files[len(files)-1]
	if v.filter != "" {
		return append([]string{"/bin/sh", "-c", filteredLog, "log", v.filter, filterCase(v.filter)}, files...)
	}
	from, last := v.partLines()
	olderLines := 0
	if len(files) > 1 {
		olderLines = logstore.CountFileLines(files[0])
	}
	switch {
	case v.back > 0:
		script := `from=$1 last=$2; shift 2; cat -- "$@" | sed -n "${from},${last}p;${last}q"`
		return append([]string{"/bin/sh", "-c", script, "log", strconv.Itoa(from), strconv.Itoa(last)}, files...)
	case from > olderLines:
		return []string{"/bin/sh", "-c", `exec tail -n "+$1" -f -- "$2"`, "log", strconv.Itoa(from - olderLines), current}
	}
	return []string{"/bin/sh", "-c", `sed -n "$1,\$p" "$2"; exec tail -n +1 -f -- "$3"`, "log", strconv.Itoa(from), files[0], current}
}

// filteredLog prints the lines of the log's files with the text, then of its current file as it
// grows: sh -c filteredLog log text case older... current.
const filteredLog = `text=$1 case=$2; shift 2
{ while [ $# -gt 1 ]; do cat -- "$1"; shift; done; exec tail -n +1 -f -- "$1"; } | exec grep --line-buffered -F "$case" -e "$text"`

// filterCase is grep's flag for the case text matches in: any, unless text has a capital.
func filterCase(text string) string {
	if strings.ToLower(text) == text {
		return "-i"
	}
	return "-F"
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
	drawTitleLine(screen, area.Min.X, area.Max.X, area.Min.Y, v.title, strings.Join(p.previewStatus(row, offset), "  "), pal.unfocusedFrame, style)
	v.area = area
	if v.terminal == nil {
		return
	}
	if s, ok := v.terminal.(scrollable); ok {
		s.DrawScrolled(screen, output, offset)
	} else {
		v.terminal.Draw(screen, output)
	}
	if v.query != "" {
		lightMatches(screen, output, v.query)
	}
}

// previewStatus says where in the log the preview is: in a full-screen program, which lines of
// a log in parts, and how far up the part.
func (p *logsPanel) previewStatus(row logRow, offset int) []string {
	v := &p.preview
	var status []string
	if v.filter != "" {
		status = append(status, "filter: "+v.filter)
	}
	if s, ok := v.terminal.(fullScreener); ok && s.FullScreen() {
		status = append(status, "▣ ends in a full-screen program")
	}
	if v.parts() > 1 {
		from, last := v.partLines()
		total := v.lines
		if v.back == 0 {
			last = max(p.linesOf(row.entry), last) // the part that follows the log grows with it
			total = last
		}
		status = append(status, fmt.Sprintf("lines %s–%s of %s", groupDigits(from), groupDigits(last), groupDigits(total)))
	}
	if offset > 0 { // rows of the terminal it is drawn on, which the log's wrapped lines fill
		status = append(status, fmt.Sprintf("↑ %d/%d rows", offset, v.scrollback()))
	}
	return status
}

// previewMouse scrolls the log with the wheel, on into the next part past an end; a click gives
// it the keys.
func (p *logsPanel) previewMouse(event uv.MouseEvent) {
	v := &p.preview
	switch event.(type) {
	case uv.MouseClickEvent:
		if event.Mouse().Button == uv.MouseLeft {
			p.focusList, v.focused = true, true
		}
	case uv.MouseWheelEvent:
		v.offset = min(v.offset, v.scrollback())
		switch event.Mouse().Button {
		case uv.MouseWheelUp:
			if v.offset >= v.scrollback() && v.showPart(v.back+1, false) {
				return
			}
			v.offset += wheelLines
		case uv.MouseWheelDown:
			if v.offset == 0 && v.showPart(v.back-1, true) {
				return
			}
			v.offset -= wheelLines
		}
		v.offset = min(max(v.offset, 0), v.scrollback())
	}
}
