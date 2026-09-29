package flow

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Cron is a five-field schedule, as crontab writes it: minute, hour, day of month, month and day
// of week, each a *, numbers, ranges (1-5), lists (1,3), steps (*/15, 5-30/5) or names (jan, mon).
type Cron struct {
	fields [5]uint64
	// Days of month and of week both set means either of them, as in crontab.
	anyDayOfMonth, anyDayOfWeek bool
}

type cronField struct {
	min, max int
	names    []string
}

var cronFields = [5]cronField{
	{0, 59, nil},
	{0, 23, nil},
	{1, 31, nil},
	{1, 12, []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}},
	{0, 7, []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}},
}

// ParseCron reads a five-field schedule.
func ParseCron(text string) (Cron, error) {
	parts := strings.Fields(text)
	if len(parts) != 5 {
		return Cron{}, fmt.Errorf("cron %q needs 5 fields: minute hour day month weekday", text)
	}
	var c Cron
	for i, part := range parts {
		bits, err := cronFields[i].parse(strings.ToLower(part))
		if err != nil {
			return Cron{}, fmt.Errorf("cron %q: %w", text, err)
		}
		c.fields[i] = bits
	}
	if c.fields[4]&(1<<7) != 0 {
		c.fields[4] |= 1 // 7 is Sunday too
	}
	c.anyDayOfMonth, c.anyDayOfWeek = parts[2] == "*", parts[4] == "*"
	return c, nil
}

// parse turns a field into a bit per value it takes.
func (f cronField) parse(text string) (uint64, error) {
	var bits uint64
	for _, item := range strings.Split(text, ",") {
		span, stepText, stepped := strings.Cut(item, "/")
		step := 1
		if stepped {
			var err error
			if step, err = strconv.Atoi(stepText); err != nil || step < 1 {
				return 0, fmt.Errorf("step %q is not a positive number", stepText)
			}
		}
		from, to, err := f.span(span)
		if err != nil {
			return 0, err
		}
		for v := from; v <= to; v += step {
			bits |= 1 << v
		}
	}
	return bits, nil
}

// span is the range an item covers: * is the whole field, a single value only itself.
func (f cronField) span(text string) (int, int, error) {
	if text == "*" {
		return f.min, f.max, nil
	}
	fromText, toText, isRange := strings.Cut(text, "-")
	from, err := f.value(fromText)
	if err != nil {
		return 0, 0, err
	}
	to := from
	if isRange {
		if to, err = f.value(toText); err != nil {
			return 0, 0, err
		}
	}
	if from > to {
		return 0, 0, fmt.Errorf("range %q goes backwards", text)
	}
	return from, to, nil
}

func (f cronField) value(text string) (int, error) {
	for i, name := range f.names {
		if text == name {
			return i + f.min, nil
		}
	}
	v, err := strconv.Atoi(text)
	if err != nil || v < f.min || v > f.max {
		return 0, fmt.Errorf("%q is not between %d and %d", text, f.min, f.max)
	}
	return v, nil
}

// Matches tells whether the schedule runs in the minute of t.
func (c Cron) Matches(t time.Time) bool {
	has := func(field, value int) bool { return c.fields[field]&(1<<value) != 0 }
	if !has(0, t.Minute()) || !has(1, t.Hour()) || !has(3, int(t.Month())) {
		return false
	}
	dayOfMonth, dayOfWeek := has(2, t.Day()), has(4, int(t.Weekday()))
	switch {
	case c.anyDayOfMonth:
		return dayOfWeek
	case c.anyDayOfWeek:
		return dayOfMonth
	}
	return dayOfMonth || dayOfWeek
}
