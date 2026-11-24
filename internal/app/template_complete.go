package app

import (
	"image"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
)

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

// maxSuggestions is how many variables show under a prompt's field.
const maxSuggestions = 6

// suggestions are the variables that complete what is typed in a template prompt.
func (p *textPrompt) suggestions() ([]string, int) {
	return templateSuggestions(&p.input, p.variables)
}

// complete puts in the suggestion picked, closing the braces.
func (p *textPrompt) complete() {
	matches, from := p.suggestions()
	if len(matches) > 0 {
		completeTemplate(&p.input, matches[min(p.suggested, len(matches)-1)], from)
		p.suggested = 0
	}
}

// moveSuggestion picks the suggestion above or below.
func (p *textPrompt) moveSuggestion(step int) {
	if matches, _ := p.suggestions(); len(matches) > 0 {
		p.suggested = (p.suggested + step + len(matches)) % len(matches)
	}
}

// drawSuggestions lists the variables that complete what is typed, under the field at y.
func (a *App) drawSuggestions(inner image.Rectangle, y int, draw func(x, y, maxX int, text string, selected bool)) int {
	matches, _ := a.prompt.suggestions()
	shown := matches[:min(len(matches), maxSuggestions)]
	for i, m := range shown {
		draw(inner.Min.X+3, y+i, inner.Max.X-1, m, i == min(a.prompt.suggested, len(shown)-1))
	}
	return len(shown)
}

// withVariables lets a prompt complete the variables readable where its template goes.
func (a *App) withVariables(vars []flow.Variable) {
	a.prompt.variables = flow.Readable(vars)
}
