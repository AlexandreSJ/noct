package expr

import (
	"slices"
	"strings"
	"unicode/utf8"
)

type Line struct {
	Depth    int
	Text     string
	Operator bool
}

var logicLevels = levels[1:notLevel]

// Outline lays an expression out by its logic, so a and b or c reads as [a and b] or [c]: the
// parts an operator joins go a level deeper than it, and parts joined by the same operator in a
// row stay side by side. A part in parentheses with logic inside has them on lines of their own,
// as operators, around its parts a level deeper. An expression with no logic operator outside
// parentheses, or one that does not read, is one line.
func Outline(text string) []Line {
	return outline(text, 0)
}

func outline(text string, depth int) []Line {
	text = strings.TrimSpace(text)
	if inner := unwrapped(text); inner != text && len(logicCuts(inner)) > 0 {
		lines := []Line{{Depth: depth, Text: "(", Operator: true}}
		lines = append(lines, outline(inner, depth+1)...)
		return append(lines, Line{Depth: depth, Text: ")", Operator: true})
	}
	cuts := logicCuts(text)
	if len(cuts) == 0 {
		return []Line{{Depth: depth, Text: text}}
	}
	var lines []Line
	from := 0
	for _, cut := range cuts {
		lines = append(lines, outline(text[from:cut.at], depth+1)...)
		lines = append(lines, Line{Depth: depth, Text: cut.text, Operator: true})
		from = cut.end
	}
	return append(lines, outline(text[from:], depth+1)...)
}

// logicCuts are the operators of the loosest logic level found outside parentheses, brackets and
// braces, where the text splits into its parts. A ?? or a lambda's => there binds looser than
// any of them, and a text that does not read cannot be split: then there is none.
func logicCuts(text string) []token {
	outside := outsideTokens(text)
	for _, tok := range outside {
		if tok.kind == opToken && (tok.text == "??" || tok.text == "=>") {
			return nil
		}
	}
	for _, ops := range logicLevels {
		var cuts []token
		for _, tok := range outside {
			if (tok.kind == opToken || tok.kind == nameToken) && slices.Contains(ops, tok.text) {
				cuts = append(cuts, tok)
			}
		}
		if len(cuts) > 0 {
			return cuts
		}
	}
	return nil
}

func outsideTokens(text string) []token {
	var outside []token
	nesting := 0
	l := lexer{text: text}
	for tok := l.next(); tok.kind != endToken; tok = l.next() {
		switch {
		case tok.kind == badToken:
			return nil
		case tok.kind == opToken && strings.Contains("([{", tok.text):
			nesting++
		case tok.kind == opToken && strings.Contains(")]}", tok.text):
			nesting--
		case nesting == 0:
			outside = append(outside, tok)
		}
	}
	return outside
}

func unwrapped(text string) string {
	if strings.HasPrefix(text, "(") && closes(text) == len(text)-1 {
		return strings.TrimSpace(text[1 : len(text)-1])
	}
	return text
}

func closes(text string) int {
	nesting := 0
	l := lexer{text: text}
	for tok := l.next(); tok.kind != endToken && tok.kind != badToken; tok = l.next() {
		switch {
		case tok.kind != opToken:
		case strings.Contains("([{", tok.text):
			nesting++
		case strings.Contains(")]}", tok.text):
			if nesting--; nesting == 0 {
				return tok.at
			}
		}
	}
	return -1
}

var logicWords = map[string]bool{"and": true, "or": true, "not": true, "nand": true, "nor": true, "xor": true, "&&": true, "||": true, "!": true, "(": true, ")": true}

// LogicMarks are where the logic operators and parentheses of text are, as byte ranges, for an
// editor to color them as they are typed; what does not read yet is passed over.
func LogicMarks(text string) [][2]int {
	var marks [][2]int
	l := lexer{text: text}
	for {
		tok := l.next()
		switch {
		case tok.kind == endToken:
			return marks
		case tok.kind == badToken:
			_, size := utf8.DecodeRuneInString(text[tok.at:])
			l.at = tok.at + max(size, 1)
		case (tok.kind == opToken || tok.kind == nameToken) && logicWords[tok.text]:
			marks = append(marks, [2]int{tok.at, tok.end})
		}
	}
}
