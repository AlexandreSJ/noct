package app

import (
	"fmt"
	"image"
	"slices"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// The log shown can be searched and filtered. / searches up from the end of what shows, as ? does
// in less, since the newest lines are at the bottom; n goes on up and N back down. f keeps only
// the lines with a text. Text in lower case matches any case; with a capital, only its own.

const (
	logSearchHints = "{enter} search up the log  esc cancel  lower case matches any case"
	logFilterHints = "{enter} keep the lines with this text  esc cancel  empty shows every line"
)

// lineReader is a terminal whose history and screen read as text, oldest line first.
type lineReader interface {
	LineCount() int
	LineText(line int) string
}

func (p *logsPanel) searchLog() {
	v := &p.preview
	p.app.openPrompt("search the log", v.query, logSearchHints, func(text string) {
		v.query, v.matched = text, false
		p.nextMatch(true)
	})
}

func (p *logsPanel) filterLog() {
	v := &p.preview
	p.app.openPrompt("filter the log", v.filter, logFilterHints, func(text string) {
		v.setFilter(text)
	})
}

// setFilter starts the log over through the filter, at its live end.
func (v *logPreview) setFilter(text string) {
	v.close()
	v.filter, v.offset, v.back, v.matched = text, 0, 0, false
}

// nextMatch scrolls to the next line up (or down) with the query, from the last match or from
// the edge of what shows, and tells when there is none.
func (p *logsPanel) nextMatch(up bool) {
	v := &p.preview
	lines, ok := v.terminal.(lineReader)
	if v.query == "" || !ok {
		return
	}
	history := v.scrollback()
	first := history - min(v.offset, history)
	from := first
	switch {
	case v.matched && up:
		from = v.match - 1
	case v.matched:
		from = v.match + 1
	case up:
		from = first + v.size.Rows - 1
	}
	step := map[bool]int{true: -1, false: 1}[up]
	query := []rune(v.query)
	for line := from; line >= 0 && line < lines.LineCount(); line += step {
		if len(matchesIn([]rune(lines.LineText(line)), query)) > 0 {
			v.match, v.matched = line, true
			v.offset = min(max(history-(line-v.size.Rows/3), 0), history)
			return
		}
	}
	where := map[bool]string{true: "above", false: "below"}[up]
	if v.parts() > 1 {
		where += " in this part of the log"
	}
	p.app.notice = fmt.Sprintf("no match for %q %s", v.query, where)
}

// matchesIn are where query starts in text, in any case when query has no capital.
func matchesIn(text, query []rune) []int {
	if len(query) == 0 {
		return nil
	}
	anyCase := !slices.ContainsFunc(query, unicode.IsUpper)
	same := func(a, b rune) bool { return a == b || anyCase && unicode.ToLower(a) == b }
	var starts []int
	for i := 0; i+len(query) <= len(text); i++ {
		if !slices.EqualFunc(text[i:i+len(query)], query, same) {
			continue
		}
		starts = append(starts, i)
	}
	return starts
}

// lightMatches shows in reverse every match of query in area of the screen.
func lightMatches(screen uv.Screen, area image.Rectangle, query string) {
	for y := area.Min.Y; y < area.Max.Y; y++ {
		var text []rune
		var at []int // the x of each rune
		for x := area.Min.X; x < area.Max.X; x++ {
			c := screen.CellAt(x, y)
			if c != nil && c.Content == "" {
				continue // the second cell of a wide character
			}
			content := " "
			if c != nil {
				content = c.Content
			}
			for _, r := range content {
				text, at = append(text, r), append(at, x)
			}
		}
		query := []rune(query)
		for _, start := range matchesIn(text, query) {
			for _, x := range at[start : start+len(query)] {
				if c := screen.CellAt(x, y); c != nil {
					cell := *c
					cell.Style.Attrs |= uv.AttrReverse
					screen.SetCell(x, y, &cell)
				}
			}
		}
	}
}
