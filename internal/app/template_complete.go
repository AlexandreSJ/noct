package app

import (
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// maxSuggestions is how many variables a template field offers at once.
const maxSuggestions = 6

// A template field offers the variables in scope as soon as {{. is typed: the ones that start as
// typed show beside it, up and down pick one, and tab puts it in, closing the braces.

// templateSuggestions are the variables, as a template reads them, that complete the word typed
// before the cursor inside a {{ not closed yet, and where that word starts: right after the
// braces, or after a function's name as in {{eq .ver.
func templateSuggestions(field *textField, vars []flow.Variable) ([]string, int) {
	if len(vars) == 0 {
		return nil, 0
	}
	before := string(field.text[:field.cursor])
	open := strings.LastIndex(before, "{{")
	if open < 0 || strings.Contains(before[open:], "}}") {
		return nil, 0
	}
	start := open + 2 + strings.LastIndexAny(before[open+2:], " (") + 1
	typed := before[start:]
	if strings.TrimSpace(before[open+2:start]) == "" {
		typed = strings.TrimSpace(before[open+2:])
	}
	var matches []string
	for _, v := range vars {
		inner := variableName(v)
		if strings.HasPrefix(inner, typed) && inner != typed && !contains(matches, inner) && len(matches) < maxSuggestions {
			matches = append(matches, inner)
		}
	}
	return matches, len([]rune(before[:start]))
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
