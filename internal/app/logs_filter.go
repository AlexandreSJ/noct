package app

import (
	"regexp"
	"strconv"
	"strings"
)

// logFilter is what the logs panel's filter line asks for: every word must match a log. A word
// is text found in its name, date, time or how its run ended, or a comparison of its size
// (">1MB", "size<500k") or lines (">100", "lines>=20").
type logFilter struct {
	words       []string
	comparisons []logComparison
}

// logComparison compares a log's size in bytes, or its lines, with a value.
type logComparison struct {
	lines bool
	op    string
	value int64
}

var comparisonWord = regexp.MustCompile(`^(size|lines)?(<=|>=|<|>|=)(\d+(?:\.\d+)?)([kmg]?b?)?$`)

var sizeUnits = map[string]float64{"": 1, "b": 1, "k": 1 << 10, "kb": 1 << 10, "m": 1 << 20, "mb": 1 << 20, "g": 1 << 30, "gb": 1 << 30}

func parseLogFilter(text string) logFilter {
	var f logFilter
	for _, word := range strings.Fields(strings.ToLower(text)) {
		if c, ok := parseComparison(word); ok {
			f.comparisons = append(f.comparisons, c)
			continue
		}
		f.words = append(f.words, word)
	}
	return f
}

// parseComparison reads a size or lines comparison: a unit or "size" makes it a size, and a bare
// number counts lines.
func parseComparison(word string) (logComparison, bool) {
	m := comparisonWord.FindStringSubmatch(word)
	if m == nil {
		return logComparison{}, false
	}
	number, err := strconv.ParseFloat(m[3], 64)
	if err != nil {
		return logComparison{}, false
	}
	isSize := m[1] == "size" || m[4] != ""
	if m[1] == "lines" && m[4] != "" {
		return logComparison{}, false
	}
	c := logComparison{lines: !isSize, op: m[2], value: int64(number)}
	if isSize {
		c.value = int64(number * sizeUnits[m[4]])
	}
	return c, true
}

func (f logFilter) empty() bool {
	return len(f.words) == 0 && len(f.comparisons) == 0
}

// matches tells whether a row passes the filter; lines counts the log's lines, only when asked.
func (f logFilter) matches(row logRow, lines func() int) bool {
	text := logRowText(row)
	for _, word := range f.words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	for _, c := range f.comparisons {
		value := row.entry.Size
		if c.lines {
			value = int64(lines())
		}
		if !c.holds(value) {
			return false
		}
	}
	return true
}

func (c logComparison) holds(value int64) bool {
	switch c.op {
	case "<":
		return value < c.value
	case "<=":
		return value <= c.value
	case ">":
		return value > c.value
	case ">=":
		return value >= c.value
	}
	return value == c.value
}

// logRowText is what the filter's words are looked for in: the name, the date and time in a few
// ways of writing them, and how the run ended.
func logRowText(row logRow) string {
	at := row.entry.At
	parts := []string{row.name(), row.entry.Name, at.Format("2006-01-02 15:04:05"), at.Format("Jan 2 Monday")}
	if row.run != nil {
		switch {
		case row.run.Exit == nil:
			parts = append(parts, "running")
		case row.run.Failed():
			parts = append(parts, "failed exit "+strconv.Itoa(*row.run.Exit))
		default:
			parts = append(parts, "ok")
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}
