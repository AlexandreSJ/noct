package app

import (
	"fmt"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// listFilter narrows a long list as it is typed, such as the settings, the binds, the scripts or
// the flows: f (or /) opens it, the keys type in it and the list keeps the rows that hold every
// word typed, in any case. enter keeps it and gives the list its keys back; esc clears it, and
// esc on the list clears it too before it closes anything.
type listFilter struct {
	field  textField
	typing bool
}

// filterHint is the hint of the key that opens a list's filter.
const filterHint = "{f} filter"

// filterCount says how many of a list's total rows the filter keeps.
func filterCount(kept, total int) string {
	if kept == 0 {
		return "no match"
	}
	return fmt.Sprintf("%d of %d", kept, total)
}

// isFilterKey tells whether a key opens a list's filter.
func isFilterKey(name string) bool { return name == "f" || name == "/" }

// on tells whether the filter narrows the list, or is being typed.
func (f *listFilter) on() bool { return f.typing || f.query() != "" }

// query is the text typed, trimmed.
func (f *listFilter) query() string { return strings.TrimSpace(f.field.String()) }

func (f *listFilter) start() { f.typing = true }

func (f *listFilter) clear() { *f = listFilter{} }

// keeps tells whether texts, taken together, hold every word typed, in any case.
func (f *listFilter) keeps(texts ...string) bool {
	words := strings.Fields(strings.ToLower(f.query()))
	if len(words) == 0 {
		return true
	}
	text := strings.ToLower(strings.Join(texts, " "))
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

// handleKey types a key in the filter while it is being typed. The keys that move in a list
// pass to it, so the selection moves among the rows kept. It tells whether it used the key and
// whether the text changed.
func (f *listFilter) handleKey(key uv.KeyPressEvent) (used, changed bool) {
	switch key.String() {
	case "esc":
		changed = f.query() != ""
		f.clear()
		return true, changed
	case "enter", "tab":
		f.typing = false
		return true, false
	case "up", "down", "pgup", "pgdown", "ctrl+p", "ctrl+n":
		return false, false
	}
	before := f.field.String()
	f.field.handleKey(key)
	return true, f.field.String() != before
}

// draw writes the filter on row y from x up to maxX: what is typed, with the cursor while it is
// being typed, then how many rows it keeps and, while typed, enter, which keeps it.
func (f *listFilter) draw(screen uv.Screen, x, y, maxX int, kept string, pal *palette) {
	x = drawText(screen, x, y, maxX, "filter ", pal.panelMuted)
	style := pal.panelText
	if f.typing {
		style = pal.typing()
		kept += "  enter"
	}
	// a narrow list keeps room for what is typed, giving up enter, then the count
	for _, short := range []string{strings.TrimSuffix(kept, "  enter"), ""} {
		if maxX-x-ansi.StringWidth(kept)-2 > len(f.field.String()) {
			break
		}
		kept = short
	}
	width := max(maxX-x-ansi.StringWidth(kept)-2, 1)
	if f.typing {
		x = f.field.draw(screen, x, y, width, false, style, pal.selectedRow)
	} else {
		x = drawText(screen, x, y, x+width, f.query(), style)
	}
	drawText(screen, x+2, y, maxX, kept, pal.panelMuted)
}
