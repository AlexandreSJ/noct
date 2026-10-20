package app

import (
	"image"
	"slices"
	"strings"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

const (
	finderHints   = "type to search  {up}{down} select  {enter} open  esc close"
	finderResults = 12
)

// finder searches, by fuzzy match, noct's actions, the scripts and the open tabs.
type finder struct {
	title    string
	query    textField
	items    []finderItem
	matches  []finderItem
	selected int
	first    int // the first match shown
	// cancel, when set, runs after esc, such as going back to the prompt that opened the search.
	cancel func()
}

type finderItem struct {
	label string
	kind  string
	// theme is the theme the screen shows while the item is selected, as a session's; "" for none.
	theme string
	open  func()
}

func (a *App) openFinder() {
	a.openFinderWith("search", a.finderItems())
}

// openFinderWith searches the items given, such as the scripts a flow's step can run.
func (a *App) openFinderWith(title string, items []finderItem) {
	a.finder = finder{title: title, items: items}
	a.finder.filter()
	a.mode = finderMode
}

func (a *App) finderItems() []finderItem {
	var items []finderItem
	for _, s := range a.scripts.All() {
		if s.Problem != nil {
			continue
		}
		items = append(items, finderItem{label: s.ID(), kind: "script", open: func() { a.runScript(s) }})
	}
	for _, s := range a.availableShortcuts(commandShortcuts) {
		if s.keys != "space" && slices.Contains(actionKeys, s.keys) {
			items = append(items, finderItem{label: s.label, kind: "prefix " + a.hint("commands", shortcutTemplate(s.keys)), open: func() { a.runAction(s.keys) }})
		}
	}
	for i, t := range a.session.tabs {
		items = append(items, finderItem{label: "tab " + a.tabLabel(t), kind: "tab", open: func() { a.selectTab(i) }})
	}
	for _, s := range a.sessions {
		if s != a.session {
			items = append(items, finderItem{label: "session " + s.name, kind: "session", theme: a.sessionTheme(s), open: func() { a.switchSession(s) }})
		}
	}
	return items
}

// filter keeps the items that match the query, best first.
func (f *finder) filter() {
	type scored struct {
		item  finderItem
		score int
	}
	var found []scored
	for _, item := range f.items {
		if score, ok := fuzzyScore(f.query.String(), item.label); ok {
			found = append(found, scored{item, score})
		}
	}
	slices.SortStableFunc(found, func(x, y scored) int { return y.score - x.score })
	f.matches = f.matches[:0]
	for _, s := range found {
		f.matches = append(f.matches, s.item)
	}
	f.selected = 0
}

// handleFinderKey shows a session in its theme while the selection is on it.
func (a *App) handleFinderKey(key uv.KeyPressEvent) {
	a.searchKey(key)
	if a.mode == finderMode || key.String() == "esc" {
		a.showScreenTheme()
	}
}

func (a *App) searchKey(key uv.KeyPressEvent) {
	f := &a.finder
	switch key.String() {
	case "esc":
		a.mode = normalMode
		if f.cancel != nil {
			f.cancel()
		}
	case "enter":
		a.mode = normalMode
		if len(f.matches) > 0 {
			f.matches[f.selected].open()
		}
	case "up", "ctrl+p":
		f.selected = max(f.selected-1, 0)
	case "down", "ctrl+n":
		f.selected = min(f.selected+1, max(len(f.matches)-1, 0))
	case "pgup", "pgdown", "home", "end":
		f.selected, _ = listJump(key.String(), f.selected, len(f.matches), finderResults)
	default:
		a.typeKey(key)
	}
}

// fuzzyScore tells whether the query's letters appear in text in order, ignoring case, and how
// well: letters in a row and letters starting a word count more, so "np" ranks "new panel" high.
func fuzzyScore(query, text string) (int, bool) {
	score, last := 0, -2
	runes := []rune(strings.ToLower(text))
	i := 0
	for _, q := range strings.ToLower(query) {
		for i < len(runes) && runes[i] != q {
			i++
		}
		if i == len(runes) {
			return 0, false
		}
		score++
		if i == last+1 {
			score += 4
		}
		if i == 0 || !unicode.IsLetter(runes[i-1]) && !unicode.IsDigit(runes[i-1]) {
			score += 3
		}
		last = i
		i++
	}
	return score, true
}

func (a *App) drawFinder(screen uv.Screen) {
	f := &a.finder
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, 60, panelHeight(finderResults+3)), f.title)
	drawRow(screen, inner, inner.Min.Y, "› "+f.query.view(inner.Dx()-4, false), "", a.palette.panelHeading)
	first := listFirst(f.first, f.selected, len(f.matches), finderResults)
	f.first = first
	for i := first; i < len(f.matches) && i-first < finderResults; i++ {
		style := a.palette.panelText
		if i == f.selected {
			style = a.palette.selectedRow
		}
		drawRow(screen, inner, inner.Min.Y+2+i-first, f.matches[i].label, f.matches[i].kind, style)
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("search", finderHints), a.palette.panelMuted)
}
