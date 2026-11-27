package app

import (
	"github.com/AlexandreSJ/noct/internal/secret"

	"image"
	"image/color"
	"strings"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// textField is one line of text being typed, with a cursor that moves and erases as in a shell.
type textField struct {
	text   []rune
	cursor int
	// selecting marks the text from anchor to the cursor, as dragging the mouse over it does;
	// typing replaces it and backspace erases it.
	selecting bool
	anchor    int
	// shown is the first character the last draw showed, to tell which one a click is on.
	shown int
	// multiline keeps line breaks, which enter types; top is the first line the last draw showed.
	multiline bool
	top       int
	// paint, when set, colors characters of a line as it is drawn, by their index in the line.
	paint func(line string) map[int]color.Color
}

func newTextField(text string) textField {
	runes := []rune(text)
	return textField{text: runes, cursor: len(runes)}
}

func (f *textField) String() string {
	return string(f.text)
}

// insert types text at the cursor; a paste of several lines becomes one.
func (f *textField) insert(text string) {
	if from, to, ok := f.selection(); ok {
		f.erase(from, to)
	}
	f.selecting = false
	runes := []rune(singleLine(text))
	if f.multiline {
		runes = []rune(keptLines(text))
	}
	f.text = append(f.text[:f.cursor], append(runes, f.text[f.cursor:]...)...)
	f.cursor += len(runes)
}

// singleLine turns line breaks and tabs into spaces and drops other control characters.
func singleLine(text string) string {
	text = strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, text)
}

// erase removes the text between two positions, in either order, and leaves the cursor there.
func (f *textField) erase(from, to int) {
	from, to = min(from, to), max(from, to)
	f.text = append(f.text[:from], f.text[to:]...)
	f.cursor = from
}

// handleKey edits or moves through the text and tells whether the key was one of its keys.
func (f *textField) handleKey(key uv.KeyPressEvent) bool {
	if from, to, ok := f.selection(); ok && isEraseKey(key.String()) {
		f.erase(from, to)
		f.selecting = false
		return true
	}
	if !isPlainText(key) {
		f.selecting = false // a key that moves leaves the text unmarked
	}
	switch key.String() {
	case "left", "ctrl+b":
		f.cursor = max(f.cursor-1, 0)
	case "right", "ctrl+f":
		f.cursor = min(f.cursor+1, len(f.text))
	case "ctrl+left", "alt+left", "alt+b":
		f.cursor = f.wordStartBefore()
	case "ctrl+right", "alt+right", "alt+f":
		f.cursor = f.wordEndAfter()
	case "home", "ctrl+a":
		f.cursor = f.lineStart()
	case "end", "ctrl+e":
		f.cursor = f.lineEnd()
	case "backspace", "shift+backspace":
		f.erase(max(f.cursor-1, 0), f.cursor)
	case "delete", "ctrl+d":
		f.erase(f.cursor, min(f.cursor+1, len(f.text)))
	case "ctrl+w", "ctrl+backspace", "ctrl+h", "alt+backspace":
		f.erase(f.wordStartBefore(), f.cursor)
	case "alt+d", "ctrl+delete":
		f.erase(f.cursor, f.wordEndAfter())
	case "ctrl+u":
		f.erase(f.lineStart(), f.cursor)
	case "ctrl+k":
		f.erase(f.cursor, f.lineEnd())
	default:
		if !isPlainText(key) {
			return false
		}
		f.insert(key.Text)
	}
	return true
}

func isEraseKey(name string) bool {
	return name == "backspace" || name == "shift+backspace" || name == "delete" || name == "ctrl+d"
}

// selection is the marked text's start and end, when some is.
func (f *textField) selection() (from, to int, ok bool) {
	if !f.selecting || f.anchor == f.cursor {
		return 0, 0, false
	}
	return min(f.anchor, f.cursor), max(f.anchor, f.cursor), true
}

// selected is the marked text, or "".
func (f *textField) selected() string {
	from, to, ok := f.selection()
	if !ok {
		return ""
	}
	return string(f.text[from:to])
}

// mark starts marking the text at the cursor, which moves to mark more.
func (f *textField) mark() {
	f.selecting, f.anchor = true, f.cursor
}

// posAt is the position x cells from the start of where the last draw put the field: before
// the character there, the end past the text, and one before what shows left of it.
func (f *textField) posAt(x int) int {
	if x < 0 {
		return max(f.shown-1, 0)
	}
	at := f.shown
	for width := 0; at < len(f.text); at++ {
		width += max(ansi.StringWidth(string(f.text[at])), 1)
		if width > x {
			break
		}
	}
	return at
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// wordStartBefore is where the word before the cursor starts, past any spaces or punctuation.
func (f *textField) wordStartBefore() int {
	i := f.cursor
	for i > 0 && !isWordRune(f.text[i-1]) {
		i--
	}
	for i > 0 && isWordRune(f.text[i-1]) {
		i--
	}
	return i
}

// wordEndAfter is where the word after the cursor ends, past any spaces or punctuation.
func (f *textField) wordEndAfter() int {
	i := f.cursor
	for i < len(f.text) && !isWordRune(f.text[i]) {
		i++
	}
	for i < len(f.text) && isWordRune(f.text[i]) {
		i++
	}
	return i
}

// view is the text with a bar at the cursor, cut to width so the cursor stays in sight. A
// secret shows a dot for each character.
// draw shows what of the field fits width cells at x, y in style, its text scrolled to keep the
// cursor in view and the cell under the cursor in cursor's colors, and returns where it ends. No
// mark goes in the text, so moving the cursor never shifts what is read.
func (f *textField) draw(screen uv.Screen, x, y, width int, hidden bool, style, cursor uv.Style) int {
	text := f.text
	if hidden {
		text = []rune(secret.Typed(string(text)))
	}
	if width <= 0 {
		return x
	}
	start := scrolledTo(text, f.shown, f.cursor, width)
	f.shown = start
	end := drawText(screen, x, y, x+width, shownFrom(text, start, width), style)
	if !hidden {
		f.paintLine(screen, image.Pt(x, y), x+width, string(text), start)
	}
	f.drawSelection(screen, x, y, width, text)
	under := " "
	if f.cursor < len(text) {
		under = string(text[f.cursor])
	}
	at := x + ansi.StringWidth(string(text[start:f.cursor]))
	cell := uv.Cell{Content: under, Width: max(ansi.StringWidth(under), 1), Style: cursor}
	screen.SetCell(at, y, &cell)
	return max(end, at+cell.Width)
}

// drawSelection shows the marked characters that show reversed, as a pane shows its selection.
func (f *textField) drawSelection(screen uv.Screen, x, y, width int, text []rune) {
	from, to, ok := f.selection()
	if !ok {
		return
	}
	at := x
	for i := f.shown; i < to && at < x+width; i++ {
		w := max(ansi.StringWidth(string(text[i])), 1)
		if i >= from {
			if cell := screen.CellAt(at, y); cell != nil {
				marked := *cell
				marked.Style.Attrs |= uv.AttrReverse
				screen.SetCell(at, y, &marked)
			}
		}
		at += w
	}
}

// typedField is the text the current mode types into, if any, and what to do after it changes.
func (a *App) typedField() (*textField, func()) {
	switch a.mode {
	case promptMode:
		return &a.prompt.input, nil
	case finderMode:
		return &a.finder.query, a.finder.filter
	case formMode:
		return a.form.typedField()
	case scriptFormMode:
		return a.scriptForm.typedField()
	case configMode:
		return a.configTypedField()
	case templateSaveMode:
		if a.templateSave.selected == 0 {
			return &a.templateSave.name, nil
		}
	case infoMode:
		if a.info.filter.typing {
			return &a.info.filter.field, func() { a.info.first = 0 }
		}
	}
	return nil, nil
}

// editTypedText applies edit to the text the current mode types into; it tells whether there is one.
func (a *App) editTypedText(edit func(field *textField)) bool {
	field, changed := a.typedField()
	if field == nil {
		return false
	}
	before := field.String()
	edit(field)
	if changed != nil && field.String() != before {
		changed()
	}
	return true
}

// typeKey hands a key to the text being typed and tells whether it used it; ctrl+c copies the text.
func (a *App) typeKey(key uv.KeyPressEvent) bool {
	used := false
	a.editTypedText(func(field *textField) {
		if name := key.String(); name == "ctrl+c" || name == "ctrl+shift+c" {
			a.copyToClipboard(field.String())
			a.notice, used = "copied to the clipboard", true
			return
		}
		used = field.handleKey(key)
	})
	return used
}

// pasteText types pasted text where text is being typed.
func (a *App) pasteText(text string) bool {
	if !a.editTypedText(func(field *textField) { field.insert(text) }) {
		return false
	}
	a.showScreenTheme() // a paste in the search can land on a session
	return true
}

// isPasteKey tells the paste keys apart; terminals that paste on them send the text instead.
func isPasteKey(key uv.KeyPressEvent) bool {
	name := key.String()
	return name == "ctrl+v" || name == "ctrl+shift+v" || name == "shift+insert"
}

// requestClipboard asks the attached terminal for its clipboard (OSC 52); the answer arrives as
// a ClipboardEvent and is typed like a paste.
func (a *App) requestClipboard() {
	if a.screen != nil {
		_, _ = a.screen.WriteString("\x1b]52;c;?\x07")
	}
}

// scrolledTo is the first character shown of text in width cells, from the one shown before,
// moved only as far as keeps the cursor in sight with a few characters around it, and never so
// far that room is left after the end.
func scrolledTo(text []rune, shown, cursor, width int) int {
	margin := max(1, min(4, width/4))
	start := min(max(shown, 0), len(text))
	if cursor-margin < start {
		start = max(cursor-margin, 0)
	}
	if start == 1 {
		start = 0 // the … would stand for the one character left of it
	}
	for start < cursor && ansi.StringWidth(string(text[start:min(cursor+margin, len(text))])) > width-1 {
		start++
	}
	for start > 0 && ansi.StringWidth(string(text[start-1:])) <= width-1 {
		start--
	}
	return start
}

// shownFrom is text from start as width cells show it: … in place of the first character when
// some are left of it, and … at the end when more follow.
func shownFrom(text []rune, start, width int) string {
	shown := string(text[start:])
	if start > 0 && start < len(text) {
		shown = "…" + string(text[start+1:])
	}
	return ansi.Truncate(shown, width, "…")
}

// paintLine gives the characters of line its paint colors, as drawn from at with start first.
func (f *textField) paintLine(screen uv.Screen, at image.Point, maxX int, line string, start int) {
	if f.paint == nil {
		return
	}
	colors, runes, x := f.paint(line), []rune(line), at.X
	for i := start; i < len(runes) && x < maxX; i++ {
		if c, ok := colors[i]; ok && (i > start || start == 0) {
			if cell := screen.CellAt(x, at.Y); cell != nil {
				painted := *cell
				painted.Style.Fg = c
				screen.SetCell(x, at.Y, &painted)
			}
		}
		x += max(ansi.StringWidth(string(runes[i])), 1)
	}
}
