package app

import (
	"fmt"
	"image"
	"math"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

func (p *logsPanel) Draw(screen uv.Screen, area uv.Rectangle) {
	if time.Since(p.loaded) > logsReload {
		p.reload()
	}
	pal := &p.app.palette
	drawText(screen, area.Min.X+1, area.Max.Y-1, area.Max.X, p.app.hint("logs", logsHints), pal.panelMuted)
	body := image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Max.Y-2)
	stats := p.statsByDay()
	calendar, list := logsAreas(body)
	p.drawCalendar(screen, calendar, stats)
	p.drawDay(screen, list, stats[p.day])
	p.monthRow = p.monthRow.Sub(area.Min)
	p.grid = p.grid.Sub(area.Min)
	p.list = p.list.Sub(area.Min)
}

// logsAreas places the calendar beside the list when the body is wide enough, above it otherwise.
func logsAreas(body image.Rectangle) (calendar, list image.Rectangle) {
	if body.Dx() >= calendarWidth+34 {
		calendar = image.Rect(body.Min.X+1, body.Min.Y, body.Min.X+1+calendarWidth, body.Max.Y)
		list = image.Rect(calendar.Max.X+3, body.Min.Y, body.Max.X-1, body.Max.Y)
		return calendar, list
	}
	height := 9 // the month's title, the weekdays, six weeks and the colors' legend
	if body.Dy() >= 30 {
		height = 18 // with the totals
	}
	calendar = image.Rect(body.Min.X+1, body.Min.Y, body.Max.X-1, min(body.Min.Y+height, body.Max.Y))
	list = image.Rect(body.Min.X+1, calendar.Max.Y+1, body.Max.X-1, body.Max.Y)
	return calendar, list
}

// monthStart is the first day of day's month and its column in a week starting on Monday.
func monthStart(day time.Time) (time.Time, int) {
	first := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.Local)
	return first, (int(first.Weekday()) + 6) % 7
}

// drawCalendar draws the month of the selected day, the legend of its colors, then the totals of
// the month and of all logs. A day with logs shows over a color that grows with them (ADR-038).
func (p *logsPanel) drawCalendar(screen uv.Screen, area image.Rectangle, stats map[time.Time]dayStats) {
	p.drawMonthHeader(screen, area)
	p.grid = image.Pt(area.Min.X, area.Min.Y+2)
	first, offset := monthStart(p.day)
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

// drawMonthHeader draws the month's title between its arrows, and the weekdays' names under it.
func (p *logsPanel) drawMonthHeader(screen uv.Screen, area image.Rectangle) {
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

func (s dayStats) plus(other dayStats) dayStats {
	return dayStats{logs: s.logs + other.logs, size: s.size + other.size, runs: s.runs + other.runs, failed: s.failed + other.failed}
}

// drawTotals shows the month's logs and runs, and how much of the configured limit all logs take.
func (p *logsPanel) drawTotals(screen uv.Screen, area image.Rectangle, stats map[time.Time]dayStats) {
	pal := &p.app.palette
	var month dayStats
	var all int64
	for day, s := range stats {
		if day.Year() == p.day.Year() && day.Month() == p.day.Month() {
			month = month.plus(s)
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
	title, summary := p.listHeading(rows, s)
	drawText(screen, area.Min.X, area.Min.Y, area.Max.X, title, titleStyle)
	drawText(screen, area.Min.X, area.Min.Y+1, area.Max.X, summary, pal.panelText)
	drawText(screen, area.Min.X, area.Min.Y+2, area.Max.X, "sorted by "+p.order+"; the bar goes from the lightest log to the heaviest", pal.panelMuted)
	list := image.Rect(area.Min.X, area.Min.Y+4, area.Max.X, area.Max.Y)
	first := p.first
	p.list, p.first = list, 0
	if len(rows) == 0 {
		drawText(screen, list.Min.X, list.Min.Y, list.Max.X, "no logs", pal.panelMuted)
		return
	}
	p.current(rows)
	lightest, heaviest := sizeRange(rows)
	p.first = listFirst(first, p.selected, len(rows), list.Dy())
	for i := p.first; i < len(rows) && i-p.first < list.Dy(); i++ {
		p.drawLogRow(screen, list, list.Min.Y+i-p.first, rows[i], i == p.selected, fraction(rows[i].entry.Size, lightest, heaviest))
	}
}

// listHeading is the title and summary over the list: of the filter, of all days or of the day.
func (p *logsPanel) listHeading(rows []logRow, s dayStats) (title, summary string) {
	switch {
	case p.filter != "":
		return "filter: " + p.filter, plural(len(rows), "log") + " of every day · f changes it, empty clears it"
	case p.allDays:
		return "all days", plural(len(rows), "log") + " · " + totalSize(p.entries)
	}
	title = p.day.Format("Monday, January 2 2006")
	summary = fmt.Sprintf("%s · %s · %s lines · %s, %d failed", plural(s.logs, "log"), logstore.PrettySize(s.size), p.dayLines(), plural(s.runs, "run"), s.failed)
	return title, summary
}

func (p *logsPanel) dayLines() string {
	total := 0
	for _, e := range p.entriesOn(p.day) {
		total += p.linesOf(e)
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
	style := p.logRowStyle(selected)
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

func (p *logsPanel) logRowStyle(selected bool) uv.Style {
	pal := &p.app.palette
	switch {
	case !selected:
		return pal.panelText
	case p.focusList:
		return pal.selectedRow
	}
	return pal.focusedTitle
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
