package expr

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// tokenKind is what a token of an expression is.
type tokenKind int

const (
	endToken tokenKind = iota
	nameToken
	numberToken
	stringToken
	opToken
	// badToken is text no expression holds; its text is why. It is a problem only where the
	// expression needs a token, so a $ in a text can end before it.
	badToken
)

// token is a word of an expression: a name, a number, a string's text, or an operator, at its
// byte in the text.
type token struct {
	kind    tokenKind
	text    string
	at, end int
}

// operators are the expression's operators and marks, the longer first so == is read before =.
var operators = []string{"==", "!=", "<=", ">=", "&&", "||", "??", "=>", "<", ">", "!", "+", "-", "*", "/", "%", "(", ")", "[", "]", "{", "}", ",", ".", ":"}

// lexer reads the tokens of an expression one by one.
type lexer struct {
	text string
	at   int
}

func (l *lexer) next() token {
	tok := l.read()
	tok.end = l.at
	return tok
}

func (l *lexer) read() token {
	for l.at < len(l.text) && strings.ContainsRune(" \t\r\n", rune(l.text[l.at])) {
		l.at++
	}
	if l.at >= len(l.text) {
		return token{kind: endToken, at: l.at}
	}
	start := l.at
	r, _ := utf8.DecodeRuneInString(l.text[l.at:])
	switch {
	case IsNameStart(r):
		for l.at < len(l.text) && IsNameRune(rune(l.text[l.at])) {
			l.at++
		}
		return token{kind: nameToken, text: l.text[start:l.at], at: start}
	case r >= '0' && r <= '9':
		return l.number()
	case r == '"' || r == '\'':
		return l.quoted(r)
	}
	for _, op := range operators {
		if strings.HasPrefix(l.text[l.at:], op) {
			l.at += len(op)
			return token{kind: opToken, text: op, at: start}
		}
	}
	if r == '$' {
		return token{kind: badToken, text: "$ reads a value in a text; in an expression write the name alone: ok, build.output", at: start}
	}
	if r == '=' {
		return token{kind: badToken, text: "= is no comparison: write ==", at: start}
	}
	return token{kind: badToken, text: fmt.Sprintf("%q is not part of an expression", r), at: start}
}

// number reads digits, with a fraction after a dot when digits follow it.
func (l *lexer) number() token {
	start := l.at
	digits := func() {
		for l.at < len(l.text) && l.text[l.at] >= '0' && l.text[l.at] <= '9' {
			l.at++
		}
	}
	digits()
	if l.at+1 < len(l.text) && l.text[l.at] == '.' && l.text[l.at+1] >= '0' && l.text[l.at+1] <= '9' {
		l.at++
		digits()
	}
	return token{kind: numberToken, text: l.text[start:l.at], at: start}
}

// quoted reads a string in double or single quotes: \ escapes the quote, \ itself, \n and \t.
func (l *lexer) quoted(quote rune) token {
	start := l.at
	l.at++
	var text strings.Builder
	for l.at < len(l.text) {
		c := l.text[l.at]
		switch {
		case rune(c) == quote:
			l.at++
			return token{kind: stringToken, text: text.String(), at: start}
		case c == '\\' && l.at+1 < len(l.text):
			text.WriteString(unescape(l.text[l.at+1]))
			l.at += 2
		default:
			text.WriteByte(c)
			l.at++
		}
	}
	return token{kind: badToken, text: fmt.Sprintf("the text that starts with %c has no %c to end it", quote, quote), at: start}
}

func unescape(c byte) string {
	switch c {
	case 'n':
		return "\n"
	case 't':
		return "\t"
	}
	return string(c)
}

// IsNameStart tells a character a name can start with: a letter or _.
func IsNameStart(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// IsNameRune tells a character a name can hold: a letter, a digit or _.
func IsNameRune(r rune) bool {
	return IsNameStart(r) || r >= '0' && r <= '9'
}
