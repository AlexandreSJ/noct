package app

import (
	"strings"

	"github.com/AlexandreSJ/noct/internal/expr"
	"github.com/AlexandreSJ/noct/internal/flow"
)

// maxSuggestions is how many names a field offers at once.
const maxSuggestions = 6

// A field that reads values offers the variables in scope, and the functions, as a name is typed:
// alone in a condition, after a $ in a text, and alone again inside a text's $f( or ${ not closed
// yet. The ones that start as typed show beside it, up and down pick one, and tab puts it in.

// nameRunes are what a name and its fields are written with.
const nameRunes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_."

// templateSuggestions are the names that complete the word typed before the cursor, and where
// that word starts. expression tells a condition, read whole as an expression.
func templateSuggestions(field *textField, vars []flow.Variable, expression bool) ([]string, int) {
	before := string(field.text[:field.cursor])
	if expression || insideReference(before) {
		return nameSuggestions(before, vars)
	}
	return dollarSuggestions(before, vars)
}

// insideReference tells that the cursor is inside a text's $f( or ${ that is not closed yet,
// where names are written alone.
func insideReference(before string) bool {
	for at := strings.LastIndex(before, "$"); at >= 0; at = strings.LastIndex(before[:at], "$") {
		rest := before[at+1:]
		name := strings.TrimRight(rest[:len(rest)-len(strings.TrimLeft(rest, nameRunes))], ".")
		after := rest[len(name):]
		if strings.HasPrefix(after, "(") && expr.IsFunction(name) || name == "" && strings.HasPrefix(after, "{") {
			return strings.Count(after, "(")+strings.Count(after, "{") > strings.Count(after, ")")+strings.Count(after, "}")
		}
	}
	return false
}

// nameSuggestions complete a name typed alone before the cursor: a variable as an expression
// reads it, or a function with its (.
func nameSuggestions(before string, vars []flow.Variable) ([]string, int) {
	typed := before[len(strings.TrimRight(before, nameRunes)):]
	if typed == "" || strings.HasSuffix(before[:len(before)-len(typed)], "$") || typed[0] >= '0' && typed[0] <= '9' {
		return nil, 0
	}
	names := make([]string, 0, len(vars))
	for _, v := range vars {
		names = append(names, v.Name())
	}
	for _, name := range expr.FunctionNames() {
		names = append(names, name+"(")
	}
	return suggestionsFor(typed, names), len([]rune(before[:len(before)-len(typed)]))
}

// dollarSuggestions complete a $name typed before the cursor: $ and the letters, digits, _ and
// dots of a name, not $$ nor ${, which are the shell's.
func dollarSuggestions(before string, vars []flow.Variable) ([]string, int) {
	at := strings.LastIndex(before, "$")
	typed := before[max(at, 0):]
	if at < 0 || at > 0 && before[at-1] == '$' || strings.TrimLeft(typed[1:], nameRunes) != "" {
		return nil, 0
	}
	names := make([]string, 0, len(vars))
	for _, v := range vars {
		names = append(names, v.Dollar())
	}
	if typed != "$" { // functions after a letter, so a $ alone offers the variables first
		for _, name := range expr.FunctionNames() {
			names = append(names, "$"+name+"(")
		}
	}
	return suggestionsFor(typed, names), len([]rune(before[:at]))
}

// suggestionsFor are the names that start as typed, each once.
func suggestionsFor(typed string, names []string) []string {
	var matches []string
	for _, name := range names {
		if strings.HasPrefix(name, typed) && name != typed && !contains(matches, name) && len(matches) < maxSuggestions {
			matches = append(matches, name)
		}
	}
	return matches
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// completeTemplate puts chosen in for what is typed from from on.
func completeTemplate(field *textField, chosen string, from int) {
	head := string(field.text[:from]) + chosen
	field.text = []rune(head + string(field.text[field.cursor:]))
	field.cursor = len([]rune(head))
}

func variableNameSuggestions(typed string, vars []flow.Variable) []string {
	var names []string
	for _, v := range vars {
		if v.Step == "" && !strings.Contains(v.Field, ".") {
			names = append(names, v.Field)
		}
	}
	if typed == "" {
		return nil
	}
	return suggestionsFor(typed, names)
}
