package app

import (
	"strings"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// textField is one line of text being typed, with a cursor that moves and erases as in a shell.
type textField struct {
	text   []rune
	cursor int
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
	runes := []rune(singleLine(text))
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
		f.cursor = 0
	case "end", "ctrl+e":
		f.cursor = len(f.text)
	case "backspace", "shift+backspace":
		f.erase(max(f.cursor-1, 0), f.cursor)
	case "delete", "ctrl+d":
		f.erase(f.cursor, min(f.cursor+1, len(f.text)))
	case "ctrl+w", "ctrl+backspace", "ctrl+h", "alt+backspace":
		f.erase(f.wordStartBefore(), f.cursor)
	case "alt+d", "ctrl+delete":
		f.erase(f.cursor, f.wordEndAfter())
	case "ctrl+u":
		f.erase(0, f.cursor)
	case "ctrl+k":
		f.erase(f.cursor, len(f.text))
	default:
		if !isPlainText(key) {
			return false
		}
		f.insert(key.Text)
	}
	return true
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
func (f *textField) view(width int, secret bool) string {
	text := f.text
	if secret {
		text = []rune(strings.Repeat("•", len(text)))
	}
	before, after := string(text[:f.cursor]), string(text[f.cursor:])
	if width < 2 {
		return before + "▏" + after
	}
	return ansi.Truncate(tail(before, width-1)+"▏"+after, width, "…")
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
	return a.editTypedText(func(field *textField) { field.insert(text) })
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
