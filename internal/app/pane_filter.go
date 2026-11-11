package app

import (
	"fmt"
	"image"
	"regexp"
	"slices"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// A terminal can show only the lines that match a pattern, as its program goes on: f while
// scrolling its history sets it, and f with nothing typed shows every line again. Scrolling shows
// the whole history, to read a match in its place.

const paneFilterHints = "{enter} show only the lines matching  esc cancel  a regular expression; lower case matches any case; empty shows every line"

type paneFilter struct {
	text    string
	pattern *regexp.Regexp
}

// lineFilterer is a terminal whose lines can be picked by their text and drawn alone.
type lineFilterer interface {
	LastLinesMatching(match func(text string) bool, n int) []int
	DrawLines(screen uv.Screen, area uv.Rectangle, lines []int)
}

// filterPane asks for the focused terminal's filter.
func (a *App) filterPane() {
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		return
	}
	id := t.focused
	if _, ok := t.views[id].(lineFilterer); !ok {
		a.statusMessage = "only terminals can be filtered"
		return
	}
	a.openPrompt("filter the pane", a.paneFilters[id].text, paneFilterHints, func(text string) {
		if err := a.setPaneFilter(id, text); err != nil {
			a.statusMessage = err.Error()
		}
	})
}

func (a *App) setPaneFilter(id layout.PaneID, text string) error {
	if text == "" {
		delete(a.paneFilters, id)
		return nil
	}
	source := text
	if !slices.ContainsFunc([]rune(text), unicode.IsUpper) {
		source = "(?i)" + text
	}
	pattern, err := regexp.Compile(source)
	if err != nil {
		return fmt.Errorf("filter %q: %w", text, err)
	}
	if a.paneFilters == nil {
		a.paneFilters = map[layout.PaneID]paneFilter{}
	}
	a.paneFilters[id] = paneFilter{text: text, pattern: pattern}
	return nil
}

// drawFiltered draws only the last lines of pane id that match its filter, telling whether it
// has one.
func (a *App) drawFiltered(screen uv.Screen, v view, id layout.PaneID, area image.Rectangle) bool {
	f, filtered := a.paneFilters[id]
	lines, ok := v.(lineFilterer)
	if !filtered || !ok {
		return false
	}
	lines.DrawLines(screen, area, lines.LastLinesMatching(f.pattern.MatchString, area.Dy()))
	return true
}
