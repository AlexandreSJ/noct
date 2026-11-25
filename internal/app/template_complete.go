package app

import (
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// maxSuggestions is how many variables a template field offers at once.
const maxSuggestions = 6

// A template field offers the variables in scope as soon as $ or {{. is typed: the ones that start
// as typed show beside it, up and down pick one, and tab puts it in, closing the braces.

// templateSuggestions are the variables that complete the word typed before the cursor, and
// where that word starts: a $name, or inside a {{ not closed yet, right after the braces or after
// a function's name as in {{eq .ver.
func templateSuggestions(field *textField, vars []flow.Variable) ([]string, int) {
	if len(vars) == 0 {
		return nil, 0
	}
	before := string(field.text[:field.cursor])
	open := strings.LastIndex(before, "{{")
	if open < 0 || strings.Contains(before[open:], "}}") {
		return dollarSuggestions(before, vars)
	}
	start := open + 2 + strings.LastIndexAny(before[open+2:], " (") + 1
	typed := before[start:]
	if strings.TrimSpace(before[open+2:start]) == "" {
		typed = strings.TrimSpace(before[open+2:])
	}
	return suggestionsFor(typed, vars, fieldName), len([]rune(before[:start]))
}

// dollarSuggestions complete a $name typed before the cursor: $ and the letters, digits, _ and
// dots of a name, not $$ nor ${, which are the shell's.
func dollarSuggestions(before string, vars []flow.Variable) ([]string, int) {
	at := strings.LastIndex(before, "$")
	typed := before[max(at, 0):]
	if at < 0 || at > 0 && before[at-1] == '$' || strings.TrimLeft(typed[1:], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.") != "" {
		return nil, 0
	}
	return suggestionsFor(typed, vars, variableName), len([]rune(before[:at]))
}

// suggestionsFor are the names, as name writes each variable, that start as typed.
func suggestionsFor(typed string, vars []flow.Variable, name func(flow.Variable) string) []string {
	var matches []string
	for _, v := range vars {
		inner := name(v)
		if strings.HasPrefix(inner, typed) && inner != typed && !contains(matches, inner) && len(matches) < maxSuggestions {
			matches = append(matches, inner)
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

// completeTemplate puts chosen in for what is typed from from on. A variable alone in its braces
// closes them; one an action reads, as {{eq .ver, leaves the rest of the action to type.
func completeTemplate(field *textField, chosen string, from int) {
	head := string(field.text[:from])
	rest := string(field.text[field.cursor:])
	alone := strings.HasSuffix(strings.TrimRight(head, " "), "{{")
	if alone && !strings.HasPrefix(rest, "}}") {
		rest = "}}" + rest
	}
	head += chosen
	field.text = []rune(head + rest)
	field.cursor = len([]rune(head))
	if alone {
		field.cursor += 2
	}
}
