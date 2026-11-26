package app

import (
	"strings"
	"unicode"
)

var jsonPairs = map[rune]rune{'{': '}', '[': ']', '"': '"'}

func (f *textField) inJSON() bool {
	text := strings.TrimSpace(string(f.text))
	return text == "" || strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[")
}

func (f *textField) inJSONString() bool {
	quoted := false
	for i, r := range f.text[:f.cursor] {
		if r == '"' && (i == 0 || f.text[i-1] != '\\') {
			quoted = !quoted
		}
	}
	return quoted
}

// typeJSON types r as JSON is written: an opening brace, bracket or quote with its closing one
// after the cursor, a closing one already there stepped over. It tells whether r was taken.
func (f *textField) typeJSON(r rune) bool {
	if !f.inJSON() || f.selecting {
		return false
	}
	next := rune(0)
	if f.cursor < len(f.text) {
		next = f.text[f.cursor]
	}
	switch {
	case (r == '}' || r == ']' || r == '"') && next == r:
		f.cursor++
	case r == '"' && f.inJSONString():
		return false
	case jsonPairs[r] != 0 && !f.inJSONString():
		f.insert(string(r) + string(jsonPairs[r]))
		f.cursor--
	default:
		return false
	}
	return true
}

// newJSONLine starts a line in a JSON value, a comma first after a value, indented as deep as
// the cursor is in braces and brackets.
func (f *textField) newJSONLine() {
	before := strings.TrimRightFunc(string(f.text[:f.cursor]), unicode.IsSpace)
	if before != "" && !strings.ContainsAny(before[len(before)-1:], "{[,:") {
		f.insert(",")
	}
	depth := 0
	for _, r := range before {
		switch r {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}
	f.insert("\n" + strings.Repeat("  ", max(depth, 0)))
}

// jsonLines lay a JSON value out a member a line, indented by its depth, as conditionLines lays
// out logic: braces and brackets in the primary color, each depth in its shade. A $ in it, which
// makes it no JSON yet, stays as written.
func jsonLines(text string, pal *palette) []styledLine {
	var lines []styledLine
	depth, line, quoted := 0, "", false
	flush := func() {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, styledLine{strings.Repeat("  ", depth) + strings.TrimSpace(line), shadeStyle(pal, depth)})
		}
		line = ""
	}
	mark := func(text string) {
		lines = append(lines, styledLine{strings.Repeat("  ", depth) + text, primaryStyle(pal)})
	}
	flat := strings.ReplaceAll(text, "\n", " ")
	for i, r := range flat {
		switch {
		case r == '"' && (i == 0 || flat[i-1] != '\\'):
			quoted = !quoted
			line += string(r)
		case quoted:
			line += string(r)
		case r == '{' || r == '[':
			flush()
			mark(string(r))
			depth++
		case r == '}' || r == ']':
			flush()
			depth = max(depth-1, 0)
			mark(string(r))
		case r == ',':
			line += ","
			flush()
		default:
			line += string(r)
		}
	}
	flush()
	return lines
}
