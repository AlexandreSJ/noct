package app

import (
	"fmt"
	"image"
	"slices"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/logstore"
)

// logColumn is a column of the logs list as drawn: which one, where it starts and how wide it is.
type logColumn struct {
	id    string
	x     int
	width int
}

// logColumnHeaders title the columns over the list; the run's mark needs none.
var logColumnHeaders = map[string]string{
	config.LogColumnEdited:  "edited",
	config.LogColumnStarted: "started",
	config.LogColumnName:    "name",
	config.LogColumnSession: "session",
	config.LogColumnBar:     "weight",
	config.LogColumnSize:    "size",
	config.LogColumnLines:   "lines",
}

// minNameWidth is the room the name keeps: on a narrow list, columns go in the order of
// logColumnsDropped until it has it.
const minNameWidth = 16

var logColumnsDropped = []string{config.LogColumnSession, config.LogColumnStarted, config.LogColumnLines, config.LogColumnBar, config.LogColumnEdited, config.LogColumnSize, config.LogColumnStatus}

// Widths of the columns that do not depend on the rows: the longest size (1023.9 KB) and line
// count (99,999 ln) there are, and a time with and without its date.
const (
	sizeColumnWidth  = 9
	linesColumnWidth = 9
	timeWidth        = 8
	timeAndDateWidth = 11
)

// logColumns places the configured columns across area, a space apart; the name takes the room
// left. Widths come from the whole list, not the rows on screen, so scrolling never moves them.
func (p *logsPanel) logColumns(area image.Rectangle, rows []logRow) []logColumn {
	ids := slices.Clone(p.app.config.Logs.Columns)
	widths := map[string]int{}
	for _, id := range ids {
		widths[id] = p.columnWidth(id, rows)
	}
	fixed := func() int {
		total := len(ids) - 1 // the spaces between
		for _, id := range ids {
			total += widths[id]
		}
		return total
	}
	room := func() int {
		if slices.Contains(ids, config.LogColumnName) {
			return minNameWidth
		}
		return 0
	}
	for _, dropped := range logColumnsDropped {
		if area.Dx()-fixed() >= room() {
			break
		}
		ids = slices.DeleteFunc(ids, func(id string) bool { return id == dropped })
	}
	widths[config.LogColumnName] = max(area.Dx()-fixed(), 0)
	columns := make([]logColumn, 0, len(ids))
	x := area.Min.X
	for _, id := range ids {
		columns = append(columns, logColumn{id: id, x: x, width: widths[id]})
		x += widths[id] + 1
	}
	return columns
}

// columnWidth is how wide a column is for the rows listed; the name's is what is left, set after.
func (p *logsPanel) columnWidth(id string, rows []logRow) int {
	switch id {
	case config.LogColumnName:
		return 0
	case config.LogColumnStatus:
		return 1
	case config.LogColumnBar:
		return barWidth
	case config.LogColumnSize:
		return sizeColumnWidth
	case config.LogColumnLines:
		return linesColumnWidth
	case config.LogColumnEdited, config.LogColumnStarted:
		if p.timesWithDate(id, rows) {
			return timeAndDateWidth
		}
		return timeWidth
	}
	width := ansi.StringWidth(logColumnHeaders[id])
	for _, row := range rows {
		width = max(width, ansi.StringWidth(p.sessionOf(row)))
	}
	return width
}

// timesWithDate tells whether a time column needs dates: some time in it falls on a day other
// than the one listed, or every day is.
func (p *logsPanel) timesWithDate(id string, rows []logRow) bool {
	if p.allDays || p.filter != "" {
		return true
	}
	return slices.ContainsFunc(rows, func(row logRow) bool { return !dateOf(columnTime(id, row)).Equal(p.day) })
}

func columnTime(id string, row logRow) time.Time {
	if id == config.LogColumnStarted {
		return row.entry.At
	}
	return lastWrite(row.entry)
}

// lastWrite is when a log was last written to; a log listed before it was ever read has its start.
func lastWrite(e logstore.Entry) time.Time {
	if e.Edited.IsZero() {
		return e.At
	}
	return e.Edited
}

// sessionOf is the session a log belongs to: a run's, or a terminal's; "" when none is known.
func (p *logsPanel) sessionOf(row logRow) string {
	if row.run != nil && row.run.Session != "" {
		return row.run.Session
	}
	return p.terminalSessions[row.entry.Path]
}

// drawLogColumnHeaders titles the columns, over the list.
func (p *logsPanel) drawLogColumnHeaders(screen uv.Screen, y int, columns []logColumn) {
	for _, c := range columns {
		header := logColumnHeaders[c.id]
		if c.id == config.LogColumnSize || c.id == config.LogColumnLines {
			header = fmt.Sprintf("%*s", c.width, header)
		}
		drawText(screen, c.x, y, c.x+c.width, header, p.app.palette.panelMuted)
	}
}

// drawLogRow draws one log in the list's columns, the bar weighing it against the others.
func (p *logsPanel) drawLogRow(screen uv.Screen, area image.Rectangle, y int, columns []logColumn, row logRow, selected bool, weight float64) {
	pal := &p.app.palette
	style := pal.panelText
	if selected {
		style = pal.selectedRow
		if !p.focusList {
			style = pal.focusedTitle
		}
	}
	fillRect(screen, image.Rect(area.Min.X, y, area.Max.X, y+1), style)
	for _, c := range columns {
		if c.id == config.LogColumnBar {
			drawBar(screen, c.x, y, c.width, weight, pal)
			continue
		}
		drawText(screen, c.x, y, c.x+c.width, p.cellText(c, row), style)
	}
}

// cellText is what a row shows in a column, right-aligned for numbers.
func (p *logsPanel) cellText(c logColumn, row logRow) string {
	switch c.id {
	case config.LogColumnEdited, config.LogColumnStarted:
		if c.width == timeAndDateWidth {
			return columnTime(c.id, row).Format("01-02 15:04")
		}
		return columnTime(c.id, row).Format("15:04:05")
	case config.LogColumnStatus:
		return runMark(row.run)
	case config.LogColumnName:
		return row.name()
	case config.LogColumnSession:
		return p.sessionOf(row)
	case config.LogColumnSize:
		return fmt.Sprintf("%*s", c.width, logstore.PrettySize(row.entry.Size))
	case config.LogColumnLines:
		return fmt.Sprintf("%*s", c.width, compactCount(p.linesOf(row.entry))+" ln")
	}
	return ""
}

// compactCount writes a count in at most six characters: 12,345, then 123k, 1.2M and 12M.
func compactCount(n int) string {
	switch {
	case n < 100_000:
		return groupDigits(n)
	case n < 1_000_000:
		return fmt.Sprintf("%dk", n/1000)
	case n < 10_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	}
	return fmt.Sprintf("%dM", n/1_000_000)
}
