package app

import (
	"image/color"
	"unicode/utf8"

	"github.com/AlexandreSJ/noct/internal/expr"
)

// logicPaint colors a condition's logic operators and parentheses as it is typed, in the
// primary color, as its details show them.
func logicPaint(pal *palette) func(string) map[int]color.Color {
	return func(line string) map[int]color.Color {
		colors, fg := map[int]color.Color{}, primaryStyle(pal).Fg
		for _, mark := range expr.LogicMarks(line) {
			from := utf8.RuneCountInString(line[:mark[0]])
			for i := range utf8.RuneCountInString(line[mark[0]:mark[1]]) {
				colors[from+i] = fg
			}
		}
		return colors
	}
}

// escapePaint colors each backslash of a line and the character it escapes in the primary
// color, dimmed.
func escapePaint(pal *palette) func(string) map[int]color.Color {
	return func(line string) map[int]color.Color {
		colors, fg, runes := map[int]color.Color{}, shadeStyle(pal, conditionShades/2).Fg, []rune(line)
		for i := 0; i+1 < len(runes); i++ {
			if runes[i] == '\\' {
				colors[i], colors[i+1] = fg, fg
				i++
			}
		}
		return colors
	}
}
