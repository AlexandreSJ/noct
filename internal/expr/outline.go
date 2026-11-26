package expr

import (
	"slices"
	"strings"
)

// Line is a line of an expression's outline: a part of it, or the logic operator that joins the
// parts around it, which sit a level deeper than the operator.
type Line struct {
	Depth    int
	Text     string
	Operator bool
}

// logicLevels are the logic operators from the loosest, as the parser binds them: or and nor,
// xor, then and and nand.
var logicLevels = levels[1:notLevel]

// Outline lays an expression out by its logic, so a and b or c reads as [a and b] or [c]: the
// parts an operator joins go a level deeper than it, and parts joined by the same operator in a
// row stay side by side. Parentheses around a whole part go. An expression with no logic operator
// outside parentheses, or one that does not read, is one line.
func Outline(text string) []Line {
	return outline(text, 0)
}

func outline(text string, depth int) []Line {
	text = unwrapped(strings.TrimSpace(text))
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

// outsideTokens are text's tokens outside parentheses, brackets and braces; none when the text
// does not read.
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

// unwrapped is text without the parentheses around the whole of it, as many as there are.
func unwrapped(text string) string {
	for strings.HasPrefix(text, "(") && closes(text) == len(text)-1 {
		text = strings.TrimSpace(text[1 : len(text)-1])
	}
	return text
}

// closes is where the parenthesis that opens text closes, or -1.
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
