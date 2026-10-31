package app

import (
	"cmp"
	"fmt"
	"image"
	"math"
	"slices"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/script"
)

// The hints of each part of the logs panel: only how to move through it and where to go next;
// ? lists every key.
const (
	logsCalendarHints = "{left}{right}{up}{down} day  {[}{]} month  {enter} the day's logs  {f} filter  {?} keys  {ctrl+d} close"
	logsListHints     = "{up}{down} log  {enter} the log  {left} calendar  {f} filter  {s} sort  {z} the log alone  {?} keys"
)

// logsFilterHints explain the filter line: words and comparisons, all of them at once.
const logsFilterHints = "{enter} filter  esc cancel  name, date, time, ok/failed, >1MB, <100 lines"

// Sizes of the logs panel's parts.
const (
	calendarWidth = 30
	dayCellWidth  = 4
	barWidth      = 10
	// logsReload is how old the listing may get while the panel is on screen.
	logsReload = 2 * time.Second
	// doubleClick is the longest time between the clicks of a double click.
	doubleClick = 400 * time.Millisecond
)

// Orders of the logs list.
const (
	logsByDate = "date"
	logsByName = "name"
	logsBySize = "size"
)

var logsOrders = []string{logsByDate, logsByName, logsBySize}

// What the logs panel shows: the calendar and the list with the log beside them, or one of them.
type logsShown int

const (
	logsBoth logsShown = iota
	logsLogOnly
	logsListOnly
)

// previewPercentStep is how much + and - change the room the log takes.
const previewPercentStep = 10

// logsPanel shows the logs by day: a calendar of the month, each day marked by what ran on it,
// and the selected day's logs with their size, lines and how their run ended.
type logsPanel struct {
	app   *App
	close func()

	day       time.Time
	focusList bool
	allDays   bool
	// filter lists the logs of every day that match it; see logFilter.
	filter   string
	order    string
	selected int
	shown    logsShown

	// preview is the selected log, beside the calendar and the list.
	preview logPreview

	entries  []logstore.Entry
	runs     []script.Run
	runOfLog map[string]script.Run
	// terminalSessions names the session of each terminal's log.
	terminalSessions map[string]string
	loaded           time.Time
	// lines caches the line count of each log, for the size it had when counted.
	lines map[string]countedLines

	// Where things were drawn, relative to the panel, for clicks.
	monthRow  image.Rectangle
	grid      image.Point
	list      image.Rectangle
	first     int
	lastClick time.Time
}

type countedLines struct {
	size  int64
	lines int
}

// logRow is a line of the list: a log and the run that wrote it, when a script or flow did.
type logRow struct {
	entry logstore.Entry
	run   *script.Run
}

// startLogs opens the logs panel, pruning the logs past the configured limits first.
func (a *App) startLogs(id layout.PaneID, _ image.Rectangle) (view, error) {
	a.pruneLogs()
	return newLogsPanel(a, func() { a.detachView(id) }), nil
}

// pruneLogs removes the logs older than the configured days and, past the configured size, the
// oldest ones, forgetting them in the runs that wrote them.
func (a *App) pruneLogs() {
	limits := a.config.Logs
	removed, err := logstore.Prune(logstore.LogsDir(), limits.KeepDays, limits.MaxSizeBytes(), time.Now())
	if len(removed) > 0 {
		gone := map[string]bool{}
		for _, e := range removed {
			gone[e.Path] = true
		}
		if forgetErr := script.ForgetLogs(runsPath(), gone); err == nil {
			err = forgetErr
		}
	}
	if err != nil {
		a.statusMessage = err.Error()
	}
}

func newLogsPanel(a *App, close func()) *logsPanel {
	p := &logsPanel{app: a, close: close, order: logsByDate, lines: map[string]countedLines{}}
	p.day = dateOf(time.Now())
	p.reload()
	return p
}

func dateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func (p *logsPanel) reload() {
	p.entries, _ = logstore.List(logstore.LogsDir()) // unreadable logs show as none
	p.runs, _ = script.Runs(runsPath())
	p.runOfLog = map[string]script.Run{}
	for _, run := range p.runs {
		if run.Log != "" {
			p.runOfLog[run.Log] = run
		}
	}
	p.terminalSessions = p.app.terminalLogSessions()
	p.loaded = time.Now()
}

// onDay tells whether a log shows on a day: the day it started or the day it was last written
// to, so a terminal left open since yesterday shows today too.
func onDay(e logstore.Entry, day time.Time) bool {
	return dateOf(e.At).Equal(day) || dateOf(lastWrite(e)).Equal(day)
}

// logDays are the days a log shows on, once each.
func logDays(e logstore.Entry) []time.Time {
	started, written := dateOf(e.At), dateOf(lastWrite(e))
	if started.Equal(written) {
		return []time.Time{started}
	}
	return []time.Time{started, written}
}

// rows are the logs listed: the selected day's, or every day's, or those of any day that match
// the filter, in the chosen order.
func (p *logsPanel) rows() []logRow {
	filter := parseLogFilter(p.filter)
	var rows []logRow
	for _, e := range p.entries {
		if !p.allDays && filter.empty() && !onDay(e, p.day) {
			continue
		}
		row := logRow{entry: e}
		if run, ok := p.runOfLog[e.Path]; ok {
			row.run = &run
		}
		if filter.matches(row, func() int { return p.linesOf(e) }) {
			rows = append(rows, row)
		}
	}
	slices.SortStableFunc(rows, func(a, b logRow) int {
		switch p.order {
		case logsByName:
			return cmp.Compare(strings.ToLower(a.name()), strings.ToLower(b.name()))
		case logsBySize:
			return cmp.Compare(b.entry.Size, a.entry.Size)
		}
		return lastWrite(b.entry).Compare(lastWrite(a.entry))
	})
	return rows
}

func (r logRow) name() string {
	if r.run != nil && r.run.Trigger != "" {
		return r.run.Name() + " (" + r.run.Trigger + ")"
	}
	if r.run != nil {
		return r.run.Name()
	}
	return r.entry.Name
}

// dayStats sums up a day: its logs, their size, and the runs started on it.
type dayStats struct {
	logs   int
	size   int64
	runs   int
	failed int
}

func (p *logsPanel) statsByDay() map[time.Time]dayStats {
	stats := map[time.Time]dayStats{}
	for _, e := range p.entries {
		for _, day := range logDays(e) {
			s := stats[day]
			s.logs++
			s.size += e.Size
			stats[day] = s
		}
	}
	for _, run := range p.runs {
		day := dateOf(run.At)
		s := stats[day]
		s.runs++
		if run.Failed() {
			s.failed++
		}
		stats[day] = s
	}
	return stats
}

func (p *logsPanel) linesOf(e logstore.Entry) int {
	if counted, ok := p.lines[e.Path]; ok && counted.size == e.Size {
		return counted.lines
	}
	lines := logstore.CountLines(e)
	p.lines[e.Path] = countedLines{size: e.Size, lines: lines}
	return lines
}

func (p *logsPanel) SendKey(key uv.KeyPressEvent) {
	name := key.String()
	switch name {
	case "ctrl+d":
		p.close()
		p.Close()
		return
	}
	if name == "?" {
		p.showKeys()
		return
	}
	if p.layoutKey(name) {
		return
	}
	if p.preview.focused {
		p.previewKey(name)
		return
	}
	switch name {
	case "tab", "shift+tab":
		p.focusList = !p.focusList
		return
	case "u":
		p.focusPreview(true)
		return
	case "s":
		p.order = cycle(logsOrders, p.order, 1)
		return
	case "a":
		p.allDays = !p.allDays
		p.selected = 0
		return
	case "f":
		p.app.openPrompt("filter logs", p.filter, logsFilterHints, func(text string) {
			p.filter, p.selected, p.focusList = text, 0, text != ""
		})
		return
	case "delete":
		p.askToClear()
		return
	case "[":
		p.moveDay(0, -1)
		return
	case "]":
		p.moveDay(0, 1)
		return
	case "pgup", "pgdown":
		if !p.focusList { // on the calendar they change the month; in the list, the page
			p.moveDay(0, map[string]int{"pgup": -1, "pgdown": 1}[name])
			return
		}
	}
	if p.focusList {
		p.listKey(name)
	} else {
		p.calendarKey(name)
	}
}

func (p *logsPanel) calendarKey(name string) {
	switch name {
	case "left", "h":
		p.moveDay(-1, 0)
	case "right", "l":
		p.moveDay(1, 0)
	case "up", "k":
		p.moveDay(-7, 0)
	case "down", "j":
		p.moveDay(7, 0)
	case "t", "home":
		p.day, p.selected = dateOf(time.Now()), 0
	case "enter", "space":
		p.focusList = true
	}
}

// moveDay moves the selected day; a month on from the 31st lands on the next month's last day,
// where AddDate would skip to the month after.
func (p *logsPanel) moveDay(days, months int) {
	day := p.day.AddDate(0, 0, days)
	if months != 0 {
		first := time.Date(day.Year(), day.Month()+time.Month(months), 1, 0, 0, 0, 0, time.Local)
		last := first.AddDate(0, 1, -1).Day()
		day = first.AddDate(0, 0, min(day.Day(), last)-1)
	}
	p.day, p.selected = day, 0
}

func (p *logsPanel) listKey(name string) {
	rows := p.rows()
	switch name {
	case "up", "k":
		p.selected--
	case "down", "j":
		p.selected++
	case "home", "g":
		p.selected = 0
	case "end", "G", "shift+g":
		p.selected = len(rows) - 1
	case "pgup", "pgdown":
		p.selected, _ = listJump(name, p.selected, len(rows), p.list.Dy())
	case "esc", "left", "h":
		p.focusList = false
	case "enter", "space":
		p.focusPreview(false)
	case "i":
		if row, ok := p.current(rows); ok {
			p.inspectRun(row)
		}
	}
	p.current(rows)
}

// layoutKey shows the log alone or hides it, moves it to another side of the view and changes
// its size; the side and the size are kept in the config. It tells whether it used the key.
func (p *logsPanel) layoutKey(name string) bool {
	switch name {
	case "z":
		if p.shown == logsLogOnly {
			p.leavePreview()
		} else {
			p.shown = logsLogOnly
			p.focusPreview(false)
		}
	case "v":
		if p.shown == logsListOnly {
			p.shown = logsBoth
		} else {
			p.shown, p.preview.focused = logsListOnly, false
		}
	case "p":
		next := p.app.config
		next.Logs.Preview = cycle(config.LogPreviewSides, next.Logs.Preview, 1)
		p.app.applyConfig(next)
		p.showBoth()
	case "+", "=", "-":
		step := previewPercentStep
		if name == "-" {
			step = -step
		}
		next := p.app.config
		next.Logs.PreviewPercent = min(max(next.Logs.PreviewPercent+step, config.MinPreviewPercent), config.MaxPreviewPercent)
		p.app.applyConfig(next)
		p.showBoth()
	default:
		return false
	}
	return true
}

// showBoth brings back what was hidden, so a change to the log's place or size shows.
func (p *logsPanel) showBoth() {
	if p.shown == logsLogOnly {
		p.leavePreview()
	}
	p.shown = logsBoth
}

// showKeys lists every key of the panel, as the binds have them, in a popup.
func (p *logsPanel) showKeys() {
	c, _ := keyContextNamed("logs")
	pal := &p.app.palette
	type keyLine struct{ keys, label string }
	var keyLines []keyLine
	width := 0
	for _, action := range c.actions {
		var keys []string
		for _, key := range p.app.pressedKeys(c.id, action) {
			keys = append(keys, prettyKey(key))
		}
		if len(keys) > 0 { // an action the binds left without keys has nothing to press
			keyLines = append(keyLines, keyLine{strings.Join(keys, " "), action.label})
			width = max(width, ansi.StringWidth(keyLines[len(keyLines)-1].keys))
		}
	}
	var lines []styledLine
	for _, l := range keyLines {
		lines = append(lines, styledLine{l.keys + strings.Repeat(" ", width-ansi.StringWidth(l.keys)+2) + l.label, pal.panelText})
	}
	lines = append(lines, styledLine{"", pal.panelText}, styledLine{"change them in the config: binds of every screen, logs", pal.panelMuted})
	p.app.openInfo("logs keys", lines)
}

// inspectRun opens a flow's run in the inspect, with how each of its blocks went.
func (p *logsPanel) inspectRun(row logRow) {
	result, ok := flow.ReadResult(logstore.ResultPath(row.entry.Path))
	if row.run == nil || row.run.Flow == "" || !ok {
		p.app.notice = p.app.hint("logs", "{i} opens a flow's run; this log has none to show")
		return
	}
	p.app.openFlowRun(row.run.Flow, result)
}

// current is the selected row, kept in range; in the list of all days, the calendar follows it.
func (p *logsPanel) current(rows []logRow) (logRow, bool) {
	if len(rows) == 0 {
		p.selected = 0
		return logRow{}, false
	}
	p.selected = min(max(p.selected, 0), len(rows)-1)
	row := rows[p.selected]
	if (p.allDays || p.filter != "") && p.focusList {
		p.day = dateOf(lastWrite(row.entry))
	}
	return row, true
}

// askToClear asks what to delete: the selected log, the day's logs or all of them.
func (p *logsPanel) askToClear() {
	var options []choiceOption
	if row, ok := p.current(p.rows()); ok && p.focusList {
		options = append(options, choiceOption{key: "l", label: "this log", pick: func() {
			p.clear("delete the log of "+row.name()+"?", []logstore.Entry{row.entry})
		}})
	}
	day := p.day
	var ofDay []logstore.Entry
	for _, e := range p.entries {
		if onDay(e, day) {
			ofDay = append(ofDay, e)
		}
	}
	entries := slices.Clone(p.entries)
	options = append(options,
		choiceOption{key: "d", label: "logs of " + day.Format("Jan 2"), pick: func() {
			p.clear(fmt.Sprintf("delete the %d logs of %s (%s)?", len(ofDay), day.Format("Jan 2"), totalSize(ofDay)), ofDay)
		}},
		choiceOption{key: "a", label: "all logs", pick: func() {
			p.clear(fmt.Sprintf("delete all %d logs (%s)?", len(entries), totalSize(entries)), entries)
		}},
	)
	p.app.openChoice("clear", options...)
}

func totalSize(entries []logstore.Entry) string {
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	return logstore.PrettySize(total)
}

func (p *logsPanel) clear(question string, entries []logstore.Entry) {
	if len(entries) == 0 {
		p.app.notice = "no logs to delete"
		return
	}
	p.app.askToConfirm(question, func() {
		p.app.removeLogs(entries)
		p.reload()
	})
}

// removeLogs deletes logs and forgets them in the runs that wrote them.
func (a *App) removeLogs(entries []logstore.Entry) {
	gone := map[string]bool{}
	for _, e := range entries {
		gone[e.Path] = true
	}
	err := logstore.Remove(entries...)
	if forgetErr := script.ForgetLogs(runsPath(), gone); err == nil {
		err = forgetErr
	}
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.notice = fmt.Sprintf("deleted %d logs (%s)", len(entries), totalSize(entries))
}

func (p *logsPanel) Draw(screen uv.Screen, area uv.Rectangle) {
	if time.Since(p.loaded) > logsReload {
		p.reload()
	}
	pal := &p.app.palette
	hints := logsCalendarHints
	switch {
	case p.preview.focused:
		hints = logsPreviewHints
	case p.focusList:
		hints = logsListHints
	}
	hintLines := wrapHints(p.app.hint("logs", hints), area.Dx()-2, 2)
	for i, line := range hintLines {
		drawText(screen, area.Min.X+1, area.Max.Y-len(hintLines)+i, area.Max.X, line, pal.panelMuted)
	}
	body := image.Rect(area.Min.X, area.Min.Y, area.Max.X, max(area.Max.Y-len(hintLines)-1, area.Min.Y))
	lists, preview := p.splitView(body)
	// drawn last, the log shows the selection the list settled on
	defer func() {
		if preview.Empty() {
			p.preview.close()
			p.preview.path, p.preview.area = "", image.Rectangle{}
			return
		}
		row, ok := p.current(p.rows())
		p.drawPreview(screen, preview, row, ok)
		p.preview.area = p.preview.area.Sub(area.Min)
	}()
	p.monthRow, p.grid, p.list = image.Rectangle{}, image.Point{}, image.Rectangle{}
	if lists.Empty() {
		return
	}
	stats := p.statsByDay()
	var calendar, list image.Rectangle
	if lists.Dx() >= calendarWidth+34 {
		calendar = image.Rect(lists.Min.X+1, lists.Min.Y, lists.Min.X+1+calendarWidth, lists.Max.Y)
		list = image.Rect(calendar.Max.X+3, lists.Min.Y, lists.Max.X-1, lists.Max.Y)
	} else {
		height := 9 // the month's title, the weekdays, six weeks and the colors' legend
		if lists.Dy() >= 30 {
			height = 18 // with the totals
		}
		calendar = image.Rect(lists.Min.X+1, lists.Min.Y, lists.Max.X-1, min(lists.Min.Y+height, lists.Max.Y))
		list = image.Rect(lists.Min.X+1, min(calendar.Max.Y+1, lists.Max.Y), lists.Max.X-1, lists.Max.Y)
	}
	p.drawCalendar(screen, calendar, stats)
	p.drawDay(screen, list, stats[p.day])
	p.monthRow = p.monthRow.Sub(area.Min)
	p.grid = p.grid.Sub(area.Min)
	p.list = p.list.Sub(area.Min)
}

// splitView divides the view between the calendar with the list and the log, on the side and
// with the share of the config, or gives it all to the one shown alone. Either may be empty.
func (p *logsPanel) splitView(body image.Rectangle) (lists, preview image.Rectangle) {
	inset := image.Rect(body.Min.X+1, body.Min.Y, body.Max.X-1, body.Max.Y)
	switch p.shown {
	case logsLogOnly:
		return image.Rectangle{}, inset
	case logsListOnly:
		return body, image.Rectangle{}
	}
	percent := p.app.config.Logs.PreviewPercent
	height, width := body.Dy()*percent/100, body.Dx()*percent/100
	side := p.app.config.Logs.Preview
	if (side == "right" || side == "left") && body.Dx()-width-1 < calendarWidth+2 {
		side = "below" // beside the log, the calendar would not fit
	}
	lists, preview = body, inset
	switch side {
	case "above":
		preview.Max.Y = body.Min.Y + height
		lists.Min.Y = min(preview.Max.Y+1, body.Max.Y)
	case "right":
		preview.Min.X = body.Max.X - 1 - width
		lists.Max.X = max(preview.Min.X-1, body.Min.X)
	case "left":
		preview.Max.X = body.Min.X + 1 + width
		lists.Min.X = min(preview.Max.X+1, body.Max.X)
	default:
		preview.Min.Y = body.Max.Y - height
		lists.Max.Y = max(preview.Min.Y-1, body.Min.Y)
	}
	return lists, preview
}

// drawCalendar draws the month of the selected day, the legend of its colors, then the totals of
// the month and of all logs. A day with logs shows over a color that grows with them (ADR-038).
func (p *logsPanel) drawCalendar(screen uv.Screen, area image.Rectangle, stats map[time.Time]dayStats) {
	pal := &p.app.palette
	titleStyle := pal.panelHeading
	if !p.focusList {
		titleStyle = pal.focusedTitle
	}
	width := 7 * dayCellWidth
	p.monthRow = image.Rect(area.Min.X, area.Min.Y, area.Min.X+width, area.Min.Y+1)
	drawText(screen, area.Min.X, area.Min.Y, area.Max.X, "◂", pal.panelMuted)
	drawCentered(screen, p.monthRow, area.Min.Y, p.day.Format("January 2006"), titleStyle)
	drawText(screen, area.Min.X+width-1, area.Min.Y, area.Max.X, "▸", pal.panelMuted)
	for i, name := range []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"} {
		drawText(screen, area.Min.X+i*dayCellWidth+1, area.Min.Y+1, area.Max.X, name, pal.panelMuted)
	}
	p.grid = image.Pt(area.Min.X, area.Min.Y+2)
	first := time.Date(p.day.Year(), p.day.Month(), 1, 0, 0, 0, 0, time.Local)
	offset := (int(first.Weekday()) + 6) % 7
	today := dateOf(time.Now())
	busiest := busiestDay(stats)
	for day := first; day.Month() == first.Month(); day = day.AddDate(0, 0, 1) {
		cell := offset + day.Day() - 1
		x, y := p.grid.X+cell%7*dayCellWidth, p.grid.Y+cell/7
		if y >= area.Max.Y {
			break
		}
		// the color spans the whole cell, the number in the middle, under its weekday's name;
		// today takes brackets in the cell's margins
		format := " %2d "
		if day.Equal(today) {
			format = "[%2d]"
		}
		drawText(screen, x, y, area.Max.X, fmt.Sprintf(format, day.Day()), p.dayStyle(day, stats[day], busiest))
	}
	if y := p.grid.Y + 6; y < area.Max.Y {
		p.drawHeatLegend(screen, area.Min.X, y, area.Max.X)
	}
	if y := p.grid.Y + 8; y < area.Max.Y { // image.Rect would swap the ends of a calendar too short
		p.drawTotals(screen, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y))
	}
}

// dayStyle is how a calendar day shows: muted without logs or runs, over the color of its logs
// when it has them, and underlined when selected, which shows over any color.
func (p *logsPanel) dayStyle(day time.Time, s dayStats, busiest int) uv.Style {
	pal := &p.app.palette
	style := pal.panelMuted
	if s.logs > 0 || s.runs > 0 {
		style = pal.panelText
	}
	style = p.heatStyle(style, heatLevel(s.logs, busiest))
	if day.Equal(p.day) {
		style.Underline = uv.UnderlineSingle
	}
	return style
}

// drawTotals shows the month's logs and runs, and how much of the configured limit all logs take.
func (p *logsPanel) drawTotals(screen uv.Screen, area image.Rectangle) {
	pal := &p.app.palette
	month := p.monthStats()
	all := int64(0)
	for _, e := range p.entries {
		all += e.Size
	}
	limits := p.app.config.Logs
	limit := limits.MaxSizeBytes()
	used := float64(all) / float64(max(limit, 1))
	drawLines(screen, area, area.Min.Y, []styledLine{
		{"this month", pal.panelHeading},
		{plural(month.logs, "log") + " · " + logstore.PrettySize(month.size), pal.panelText},
		{fmt.Sprintf("%s · %d failed", plural(month.runs, "run"), month.failed), pal.panelText},
		{"", pal.panelText},
		{"all logs", pal.panelHeading},
		{fmt.Sprintf("%s · %s of %s", plural(len(p.entries), "log"), logstore.PrettySize(all), logstore.PrettySize(limit)), pal.panelText},
	})
	y := area.Min.Y + 6
	if y < area.Max.Y {
		x := drawBar(screen, area.Min.X, y, barWidth*2, used, pal)
		drawText(screen, x+1, y, area.Max.X, fmt.Sprintf("%.0f%%", math.Min(used, 1)*100), pal.panelMuted)
	}
	if y+1 < area.Max.Y {
		drawText(screen, area.Min.X, y+1, area.Max.X, fmt.Sprintf("kept %d days (config)", limits.KeepDays), pal.panelMuted)
	}
}

// monthStats sums up the selected day's month, each log once even when it shows on two of its days.
func (p *logsPanel) monthStats() dayStats {
	inMonth := func(day time.Time) bool { return day.Year() == p.day.Year() && day.Month() == p.day.Month() }
	var month dayStats
	for _, e := range p.entries {
		if slices.ContainsFunc(logDays(e), inMonth) {
			month.logs++
			month.size += e.Size
		}
	}
	for _, run := range p.runs {
		if inMonth(dateOf(run.At)) {
			month.runs++
			if run.Failed() {
				month.failed++
			}
		}
	}
	return month
}

// drawBar draws a bar width cells wide, filled for the fraction given, and returns the column
// after it.
func drawBar(screen uv.Screen, x, y, width int, fraction float64, pal *palette) int {
	filled := int(math.Round(math.Min(math.Max(fraction, 0), 1) * float64(width)))
	for i := range width {
		cell, style := "░", pal.unfocusedFrame
		if i < filled {
			cell, style = "█", pal.focusedFrame
		}
		setRune(screen, x+i, y, cell, style)
	}
	return x + width
}

// drawDay shows the day's summary and its logs, or every day's logs.
func (p *logsPanel) drawDay(screen uv.Screen, area image.Rectangle, s dayStats) {
	pal := &p.app.palette
	rows := p.rows()
	titleStyle := pal.panelHeading
	if p.focusList {
		titleStyle = pal.focusedTitle
	}
	title := p.day.Format("Monday, January 2 2006")
	summary := fmt.Sprintf("%s · %s · %s lines · %s, %d failed", plural(s.logs, "log"), logstore.PrettySize(s.size), p.dayLines(), plural(s.runs, "run"), s.failed)
	switch {
	case p.filter != "":
		title, summary = "filter: "+p.filter, plural(len(rows), "log")+" of every day · f changes it, empty clears it"
	case p.allDays:
		title, summary = "all days", plural(len(rows), "log")+" · "+totalSize(p.entries)
	}
	drawText(screen, area.Min.X, area.Min.Y, area.Max.X, title, titleStyle)
	drawText(screen, area.Min.X, area.Min.Y+1, area.Max.X, summary, pal.panelText)
	drawText(screen, area.Min.X, area.Min.Y+2, area.Max.X, "sorted by "+p.order+"; the bar goes from the lightest log to the heaviest", pal.panelMuted)
	list := image.Rect(area.Min.X, min(area.Min.Y+4, area.Max.Y), area.Max.X, area.Max.Y) // image.Rect would swap the ends of an area too short
	first := p.first
	p.list, p.first = list, 0
	if len(rows) == 0 {
		drawText(screen, list.Min.X, list.Min.Y, list.Max.X, "no logs", pal.panelMuted)
		return
	}
	p.current(rows)
	columns := p.logColumns(list, rows)
	if list.Min.Y == area.Min.Y+4 {
		p.drawLogColumnHeaders(screen, list.Min.Y-1, columns)
	}
	lightest, heaviest := sizeRange(rows)
	p.first = listFirst(first, p.selected, len(rows), list.Dy())
	for i := p.first; i < len(rows) && i-p.first < list.Dy(); i++ {
		p.drawLogRow(screen, list, list.Min.Y+i-p.first, columns, rows[i], i == p.selected, fraction(rows[i].entry.Size, lightest, heaviest))
	}
}

func (p *logsPanel) dayLines() string {
	total := 0
	for _, e := range p.entries {
		if onDay(e, p.day) {
			total += p.linesOf(e)
		}
	}
	return groupDigits(total)
}

func sizeRange(rows []logRow) (int64, int64) {
	lightest, heaviest := rows[0].entry.Size, rows[0].entry.Size
	for _, r := range rows {
		lightest, heaviest = min(lightest, r.entry.Size), max(heaviest, r.entry.Size)
	}
	return lightest, heaviest
}

// fraction places size between the lightest and the heaviest: 0 for the lightest, 1 for the
// heaviest, and 1 when all weigh the same.
func fraction(size, lightest, heaviest int64) float64 {
	if heaviest == lightest {
		return 1
	}
	return float64(size-lightest) / float64(heaviest-lightest)
}

// runMark tells how the run that wrote a log ended: ✓, ✗, ● while running or cut short, or
// nothing for a log that is not a run's, such as a terminal's.
func runMark(run *script.Run) string {
	switch {
	case run == nil:
		return " "
	case run.Exit == nil:
		return "●"
	case *run.Exit == 0:
		return "✓"
	}
	return "✗"
}

// plural writes a count with its noun: 1 log, 2 logs.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// groupDigits writes a number with thousands separated: 12,345.
func groupDigits(n int) string {
	text := fmt.Sprint(n)
	for i := len(text) - 3; i > 0; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return text
}

// WantsMouseAt takes clicks on the month's title, its days, the list and the log.
func (p *logsPanel) WantsMouseAt(point image.Point) bool {
	grid := image.Rectangle{Min: p.grid, Max: p.grid.Add(image.Pt(7*dayCellWidth, 6))}
	return point.In(p.monthRow) || point.In(grid) || point.In(p.list) || point.In(p.preview.area)
}

// SendMouse picks the day or the log clicked; a double click on a log, or a click on the log
// shown, scrolls it. The arrows of
// the month's title change the month, and the wheel moves through the list.
func (p *logsPanel) SendMouse(event uv.MouseEvent) {
	m := event.Mouse()
	point := image.Pt(m.X, m.Y)
	if point.In(p.preview.area) {
		p.previewMouse(event)
		return
	}
	p.leavePreview()
	switch event.(type) {
	case uv.MouseClickEvent:
		if m.Button != uv.MouseLeft {
			return
		}
		switch {
		case point.In(p.monthRow):
			p.clickMonth(point.X - p.monthRow.Min.X)
		case point.In(p.list):
			p.clickList(point.Y - p.list.Min.Y)
		default:
			p.clickDay(point.Sub(p.grid))
		}
	case uv.MouseWheelEvent:
		if m.Button == uv.MouseWheelUp {
			p.selected--
		} else if m.Button == uv.MouseWheelDown {
			p.selected++
		}
		p.current(p.rows())
	}
}

func (p *logsPanel) clickMonth(x int) {
	switch {
	case x < 2:
		p.moveDay(0, -1)
	case x >= p.monthRow.Dx()-2:
		p.moveDay(0, 1)
	}
}

func (p *logsPanel) clickDay(cell image.Point) {
	first := time.Date(p.day.Year(), p.day.Month(), 1, 0, 0, 0, 0, time.Local)
	offset := (int(first.Weekday()) + 6) % 7
	day := first.AddDate(0, 0, cell.Y*7+cell.X/dayCellWidth-offset)
	if day.Month() == first.Month() {
		p.day, p.selected, p.focusList = day, 0, false
	}
}

func (p *logsPanel) clickList(line int) {
	rows := p.rows()
	index := p.first + line
	if index >= len(rows) {
		return
	}
	again := index == p.selected && p.focusList && time.Since(p.lastClick) < doubleClick
	p.selected, p.focusList, p.lastClick = index, true, time.Now()
	if again {
		p.focusPreview(false)
		p.lastClick = time.Time{}
	}
}

func (p *logsPanel) Resize(pty.Size) {}
func (p *logsPanel) Paste(string)    {}
func (p *logsPanel) Close()          { p.preview.close() }

func (p *logsPanel) Cursor() (uv.Position, bool) {
	return image.Point{}, false
}

func (p *logsPanel) Title() string {
	return "logs"
}
