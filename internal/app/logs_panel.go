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

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/script"
)

const logsHints = "tab calendar/list  ←→↑↓ day  [ ] month  f filter  s sort  a all days  enter open  i flow run  del clear  ctrl+d close"

// logsFilterHints explain the filter line: words and comparisons, all of them at once.
const logsFilterHints = "enter filter  esc cancel  name, date, time, ok/failed, >1MB, <100 lines"

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

	entries  []logstore.Entry
	runs     []script.Run
	runOfLog map[string]script.Run
	loaded   time.Time
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
	removed, err := logstore.Prune(logstore.LogsDir(), limits.KeepDays, int64(limits.MaxSizeMB)<<20, time.Now())
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
	p.loaded = time.Now()
}

// rows are the logs listed: the selected day's, or every day's, or those of any day that match
// the filter, in the chosen order.
func (p *logsPanel) rows() []logRow {
	filter := parseLogFilter(p.filter)
	var rows []logRow
	for _, e := range p.entries {
		if !p.allDays && filter.empty() && !dateOf(e.At).Equal(p.day) {
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
		return b.entry.At.Compare(a.entry.At)
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
		day := dateOf(e.At)
		s := stats[day]
		s.logs++
		s.size += e.Size
		stats[day] = s
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
		return
	case "tab", "shift+tab":
		p.focusList = !p.focusList
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
	case "[", "pgup":
		p.moveDay(0, -1)
		return
	case "]", "pgdown":
		p.moveDay(0, 1)
		return
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
	case "enter":
		p.focusList = true
	}
}

func (p *logsPanel) moveDay(days, months int) {
	p.day, p.selected = p.day.AddDate(0, months, days), 0
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
	case "esc", "left", "h":
		p.focusList = false
	case "enter":
		if row, ok := p.current(rows); ok {
			p.app.openLog(row)
		}
	case "i":
		if row, ok := p.current(rows); ok {
			p.inspectRun(row)
		}
	}
	p.current(rows)
}

// inspectRun opens a flow's run in the inspect, with how each of its blocks went.
func (p *logsPanel) inspectRun(row logRow) {
	result, ok := flow.ReadResult(logstore.ResultPath(row.entry.Path))
	if row.run == nil || row.run.Flow == "" || !ok {
		p.app.notice = "i opens a flow's run; this log has none to show"
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
		p.day = dateOf(row.entry.At)
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
		if dateOf(e.At).Equal(day) {
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

// openLog shows a log beside the panel by printing it in a terminal, which draws it as the
// program did. Showing it writes no log of its own.
func (a *App) openLog(row logRow) {
	files := row.entry.LogFiles()
	slices.Reverse(files) // the rotated older part first
	argv := append([]string{"/bin/sh", "-c", `cat -- "$@"`, "cat"}, files...)
	title := "log " + row.entry.At.Format("15:04:05") + " " + row.name()
	a.openPanel(func(id layout.PaneID, frame image.Rectangle) (view, error) {
		start := func(size pty.Size, onExit func(int)) (view, error) {
			return a.launch(program{argv: argv, dir: a.workDir, onExit: onExit}, size)
		}
		return newRunView(title, &a.palette, start, func() { a.detachView(id) }, contentSize(frame))
	})
}

func (p *logsPanel) Draw(screen uv.Screen, area uv.Rectangle) {
	if time.Since(p.loaded) > logsReload {
		p.reload()
	}
	pal := &p.app.palette
	drawText(screen, area.Min.X+1, area.Max.Y-1, area.Max.X, logsHints, pal.panelMuted)
	body := image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Max.Y-2)
	stats := p.statsByDay()
	var calendar, list image.Rectangle
	if body.Dx() >= calendarWidth+34 {
		calendar = image.Rect(body.Min.X+1, body.Min.Y, body.Min.X+1+calendarWidth, body.Max.Y)
		list = image.Rect(calendar.Max.X+3, body.Min.Y, body.Max.X-1, body.Max.Y)
	} else {
		height := 9 // the month's title, the weekdays, six weeks and the colors' legend
		if body.Dy() >= 30 {
			height = 18 // with the totals
		}
		calendar = image.Rect(body.Min.X+1, body.Min.Y, body.Max.X-1, min(body.Min.Y+height, body.Max.Y))
		list = image.Rect(body.Min.X+1, calendar.Max.Y+1, body.Max.X-1, body.Max.Y)
	}
	p.drawCalendar(screen, calendar, stats)
	p.drawDay(screen, list, stats[p.day])
	p.monthRow = p.monthRow.Sub(area.Min)
	p.grid = p.grid.Sub(area.Min)
	p.list = p.list.Sub(area.Min)
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
		p.drawTotals(screen, image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), stats)
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
func (p *logsPanel) drawTotals(screen uv.Screen, area image.Rectangle, stats map[time.Time]dayStats) {
	pal := &p.app.palette
	var month dayStats
	var all int64
	for day, s := range stats {
		if day.Year() == p.day.Year() && day.Month() == p.day.Month() {
			month.logs, month.size, month.runs, month.failed = month.logs+s.logs, month.size+s.size, month.runs+s.runs, month.failed+s.failed
		}
		all += s.size
	}
	limits := p.app.config.Logs
	limit := int64(limits.MaxSizeMB) << 20
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
	list := image.Rect(area.Min.X, area.Min.Y+4, area.Max.X, area.Max.Y)
	p.list, p.first = list, 0
	if len(rows) == 0 {
		drawText(screen, list.Min.X, list.Min.Y, list.Max.X, "no logs", pal.panelMuted)
		return
	}
	p.current(rows)
	lightest, heaviest := sizeRange(rows)
	p.first = max(p.selected-list.Dy()+1, 0)
	for i := p.first; i < len(rows) && i-p.first < list.Dy(); i++ {
		p.drawLogRow(screen, list, list.Min.Y+i-p.first, rows[i], i == p.selected, fraction(rows[i].entry.Size, lightest, heaviest))
	}
}

func (p *logsPanel) dayLines() string {
	total := 0
	for _, e := range p.entries {
		if dateOf(e.At).Equal(p.day) {
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

// drawLogRow draws one log: when, how its run ended, its name, its weight bar, size and lines.
func (p *logsPanel) drawLogRow(screen uv.Screen, area image.Rectangle, y int, row logRow, selected bool, weight float64) {
	pal := &p.app.palette
	style := pal.panelText
	if selected {
		style = pal.selectedRow
		if !p.focusList {
			style = pal.focusedTitle
		}
	}
	when := row.entry.At.Format("15:04:05")
	if p.allDays {
		when = row.entry.At.Format("01-02 15:04")
	}
	right := fmt.Sprintf(" %9s", logstore.PrettySize(row.entry.Size))
	if area.Dx() >= 56 { // narrower, the name needs the room more than the lines
		right += fmt.Sprintf(" %9s", groupDigits(p.linesOf(row.entry))+" ln")
	}
	nameEnd := area.Max.X - ansi.StringWidth(right) - barWidth - 1
	fillRect(screen, image.Rect(area.Min.X, y, area.Max.X, y+1), style)
	x := drawText(screen, area.Min.X, y, nameEnd, when+" "+runMark(row.run)+" ", style)
	drawText(screen, x, y, nameEnd, row.name(), style)
	drawBar(screen, nameEnd+1, y, barWidth, weight, pal)
	drawText(screen, nameEnd+1+barWidth, y, area.Max.X, right, style)
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

// WantsMouseAt takes clicks on the month's title, its days and the list.
func (p *logsPanel) WantsMouseAt(point image.Point) bool {
	grid := image.Rectangle{Min: p.grid, Max: p.grid.Add(image.Pt(7*dayCellWidth, 6))}
	return point.In(p.monthRow) || point.In(grid) || point.In(p.list)
}

// SendMouse picks the day or the log clicked; a double click on a log opens it. The arrows of
// the month's title change the month, and the wheel moves through the list.
func (p *logsPanel) SendMouse(event uv.MouseEvent) {
	m := event.Mouse()
	point := image.Pt(m.X, m.Y)
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
		p.app.openLog(rows[index])
		p.lastClick = time.Time{}
	}
}

func (p *logsPanel) Resize(pty.Size) {}
func (p *logsPanel) Paste(string)    {}
func (p *logsPanel) Close()          {}

func (p *logsPanel) Cursor() (uv.Position, bool) {
	return image.Point{}, false
}

func (p *logsPanel) Title() string {
	return "logs"
}
