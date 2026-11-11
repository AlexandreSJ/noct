package app

import (
	"image"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// A template prompt offers the variables in scope as soon as {{. is typed: the ones that start as
// typed show under the field, up and down pick one, and tab puts it in, closing the braces.

// maxSuggestions is how many variables show under the field.
const maxSuggestions = 6

// suggestions are the templates of the variables that complete what is typed before the cursor,
// and where the part to replace starts; none outside a {{ that is not closed.
func (p *textPrompt) suggestions() ([]string, int) {
	if len(p.variables) == 0 {
		return nil, 0
	}
	before := string(p.input.text[:p.input.cursor])
	open := strings.LastIndex(before, "{{")
	if open < 0 || strings.Contains(before[open:], "}}") {
		return nil, 0
	}
	typed := strings.TrimSpace(before[open+2:])
	var matches []string
	for _, v := range p.variables {
		inner := strings.TrimSuffix(strings.TrimPrefix(v.Template(), "{{"), "}}")
		if strings.HasPrefix(inner, typed) && inner != typed && !contains(matches, inner) {
			matches = append(matches, inner)
		}
	}
	return matches, len([]rune(before[:open+2]))
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// complete puts in the suggestion picked, closing the braces.
func (p *textPrompt) complete() bool {
	matches, from := p.suggestions()
	if len(matches) == 0 {
		return false
	}
	chosen := matches[min(p.suggested, len(matches)-1)]
	text := p.input.text
	rest := string(text[p.input.cursor:])
	if !strings.HasPrefix(rest, "}}") {
		rest = "}}" + rest
	}
	head := string(text[:from]) + chosen
	p.input.text = []rune(head + rest)
	p.input.cursor = len([]rune(head)) + 2
	p.suggested = 0
	return true
}

// moveSuggestion picks the suggestion above or below, telling whether there are any.
func (p *textPrompt) moveSuggestion(step int) bool {
	matches, _ := p.suggestions()
	if len(matches) == 0 {
		return false
	}
	p.suggested = (p.suggested + step + len(matches)) % len(matches)
	return true
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

// withVariables lets a prompt complete the variables in scope.
func (a *App) withVariables(vars []flow.Variable) {
	a.prompt.variables = vars
}
