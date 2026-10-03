package app

import (
	"cmp"
	"fmt"
	"image"
	"slices"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/script"
)

const logsHints = "{tab} calendar/list  {left}{right}{up}{down} day  {[} {]} month  {f} filter  {s} sort  {a} all days  {enter} open  {i} flow run  {delete} clear  {ctrl+d} close"

// logsFilterHints explain the filter line: words and comparisons, all of them at once.
const logsFilterHints = "{enter} filter  esc cancel  name, date, time, ok/failed, >1MB, <100 lines"

// Sizes of the logs panel's parts.
const (
	calendarWidth = 30
	dayCellWidth  = 4
	barWidth      = 10
	// logsReload is how old the listing may get while the panel is on screen.
	logsReload  = 2 * time.Second
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

func (a *App) startLogs(id layout.PaneID, _ image.Rectangle) (view, error) {
	a.pruneLogs()
	return newLogsPanel(a, func() { a.detachView(id) }), nil
}

// pruneLogs applies the configured age and size limits, forgetting the removed logs in their runs.
func (a *App) pruneLogs() {
	limits := a.config.Logs
	removed, err := logstore.Prune(logstore.LogsDir(), limits.KeepDays, int64(limits.MaxSizeMB)<<20, time.Now())
	if len(removed) > 0 {
		if forgetErr := script.ForgetLogs(runsPath(), logPaths(removed)); err == nil {
			err = forgetErr
		}
	}
	if err != nil {
		a.statusMessage = err.Error()
	}
}

// logPaths is the set script.ForgetLogs takes.
func logPaths(entries []logstore.Entry) map[string]bool {
	paths := map[string]bool{}
	for _, e := range entries {
		paths[e.Path] = true
	}
	return paths
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

// rows are the selected day's logs, every day's, or any day's that match the filter, sorted.
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
	slices.SortStableFunc(rows, compareLogRows(p.order))
	return rows
}

// compareLogRows orders rows by name, by size from the heaviest, or by date from the newest.
func compareLogRows(order string) func(a, b logRow) int {
	return func(a, b logRow) int {
		switch order {
		case logsByName:
			return cmp.Compare(strings.ToLower(a.name()), strings.ToLower(b.name()))
		case logsBySize:
			return cmp.Compare(b.entry.Size, a.entry.Size)
		}
		return b.entry.At.Compare(a.entry.At)
	}
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
	case "pgup", "pgdown":
		p.selected, _ = listJump(name, p.selected, len(rows), p.list.Dy())
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
	ofDay := p.entriesOn(day)
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

func (p *logsPanel) entriesOn(day time.Time) []logstore.Entry {
	var ofDay []logstore.Entry
	for _, e := range p.entries {
		if dateOf(e.At).Equal(day) {
			ofDay = append(ofDay, e)
		}
	}
	return ofDay
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
	err := logstore.Remove(entries...)
	if forgetErr := script.ForgetLogs(runsPath(), logPaths(entries)); err == nil {
		err = forgetErr
	}
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.notice = fmt.Sprintf("deleted %d logs (%s)", len(entries), totalSize(entries))
}

// openLog prints a log in a terminal beside the panel, so it draws as the program did; this
// writes no log of its own.
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

// WantsMouseAt takes clicks on the month's title, its days and the list.
func (p *logsPanel) WantsMouseAt(point image.Point) bool {
	grid := image.Rectangle{Min: p.grid, Max: p.grid.Add(image.Pt(7*dayCellWidth, 6))}
	return point.In(p.monthRow) || point.In(grid) || point.In(p.list)
}

// SendMouse: a double click on a log opens it; the month title's arrows change the month.
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
	first, offset := monthStart(p.day)
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
