package app

import (
	"slices"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// The settings and the binds pages filter their rows as f (or /) and then text are typed: a row
// stays when the words are in its label, its value, the notes under it or its section's heading,
// and a section whose heading matches stays whole.

// filterConfigLines keeps the lines of the rows the filter keeps, each with its section's
// heading and the notes under it.
func (a *App) filterConfigLines(lines []configLine) []configLine {
	f := &a.configPanel.filter
	if f.query() == "" {
		return lines
	}
	var kept []configLine
	var heading *configLine
	headingKept, headingMatches := false, false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line.heading {
			heading, headingKept = &lines[i], false
			headingMatches = f.keeps(line.label, line.value)
			if headingMatches {
				kept, headingKept = append(kept, line), true
			}
			continue
		}
		group := []configLine{line}
		for i+1 < len(lines) && lines[i+1].note && !lines[i+1].heading {
			i++
			group = append(group, lines[i])
		}
		if !headingMatches && !f.keeps(groupText(group)...) {
			continue
		}
		if heading != nil && !headingKept {
			kept, headingKept = append(kept, *heading), true
		}
		kept = append(kept, group...)
	}
	return kept
}

// groupText is the text of a row and the notes under it.
func groupText(group []configLine) []string {
	var texts []string
	for _, line := range group {
		texts = append(texts, line.label, line.value)
	}
	return texts
}

// configFilterChanged keeps the selection on a row the filter shows, the first one when the
// selected row is left out.
func (a *App) configFilterChanged() {
	rows := a.selectableConfigRows()
	if len(rows) > 0 && !slices.Contains(rows, a.configPanel.selected) {
		a.selectConfigRow(rows[0])
	}
	a.configPanel.first = 0
}

// configFilterKey runs the filter's keys on the settings and binds pages: typing in it while it is
// open, f or / to open it, and esc to clear it. It tells whether it used the key.
func (a *App) configFilterKey(key uv.KeyPressEvent) bool {
	p := &a.configPanel
	c := p.bindCapture
	if p.page == themePage || p.typing || p.number != nil || p.capture.active || c.active || c.choosing || c.custom != nil {
		return false // a key being taken, or text typed in a row, is no filter's
	}
	if p.filter.typing {
		used, changed := p.filter.handleKey(key)
		if changed {
			a.configFilterChanged()
		}
		return used
	}
	switch name := key.String(); {
	case isFilterKey(name):
		p.filter.start()
		return true
	case name == "esc" && p.filter.on():
		p.filter.clear()
		return true
	}
	return false
}

// configFilterCount says how many of the page's rows the filter keeps.
func (a *App) configFilterCount() string {
	n := len(a.selectableConfigRows())
	if n == 0 {
		return "nothing matches"
	}
	return plural(n, "row")
}
