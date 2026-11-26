package app

import (
	"image"
	"slices"
	"strings"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func keptLines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return r
		case r == '\t' || r == '\r':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, text)
}

func (f *textField) lineStart() int {
	i := f.cursor
	for i > 0 && f.text[i-1] != '\n' {
		i--
	}
	return i
}

func (f *textField) lineEnd() int {
	if i := slices.Index(f.text[f.cursor:], '\n'); i >= 0 {
		return f.cursor + i
	}
	return len(f.text)
}

func (f *textField) lineCount() int { return strings.Count(string(f.text), "\n") + 1 }

func (f *textField) cursorLine() int { return strings.Count(string(f.text[:f.cursor]), "\n") }

// moveLine puts the cursor by lines up or down, in the same column as far as the line goes, and
// tells whether there was a line there.
func (f *textField) moveLine(by int) bool {
	line := f.cursorLine() + by
	if line < 0 || line >= f.lineCount() {
		return false
	}
	col := f.cursor - f.lineStart()
	f.cursor = f.posOfLine(line, col)
	f.selecting = false
	return true
}

func (f *textField) posOfLine(line, col int) int {
	start := 0
	for i := 0; i < line; i++ {
		start += slices.Index(f.text[start:], '\n') + 1
	}
	end := len(f.text)
	if i := slices.Index(f.text[start:], '\n'); i >= 0 {
		end = start + i
	}
	return start + min(max(col, 0), end-start)
}

// posAtPoint is the position at a cell of what the last drawLines showed, x columns and row lines
// from its top left.
func (f *textField) posAtPoint(x, row int) int {
	line := min(f.top+max(row, 0), f.lineCount()-1)
	start := f.posOfLine(line, 0)
	at, width := start, 0
	for at < len(f.text) && f.text[at] != '\n' {
		width += max(ansi.StringWidth(string(f.text[at])), 1)
		if width > x {
			break
		}
		at++
	}
	return at
}

// drawLines shows the field's lines in rect, scrolled to keep the cursor's line in view, with
// the cell under the cursor in cursor's colors.
func (f *textField) drawLines(screen uv.Screen, rect image.Rectangle, style, cursor uv.Style) image.Point {
	lines := strings.Split(string(f.text), "\n")
	at := f.cursorLine()
	f.top = min(f.top, at)
	if at >= f.top+rect.Dy() {
		f.top = at - rect.Dy() + 1
	}
	for i := f.top; i < len(lines) && i-f.top < rect.Dy(); i++ {
		drawTextCut(screen, rect.Min.X, rect.Min.Y+i-f.top, rect.Max.X, lines[i], style)
	}
	col := []rune(lines[at])[:f.cursor-f.lineStart()]
	x := min(rect.Min.X+ansi.StringWidth(string(col)), rect.Max.X-1)
	under := " "
	if f.cursor < len(f.text) && f.text[f.cursor] != '\n' {
		under = string(f.text[f.cursor])
	}
	y := rect.Min.Y + at - f.top
	screen.SetCell(x, y, &uv.Cell{Content: under, Width: max(ansi.StringWidth(under), 1), Style: cursor})
	return image.Pt(x, y)
}

// posAtIn is the position at a point of the field drawn in rect.
func (f *textField) posAtIn(at image.Point, rect image.Rectangle) int {
	if f.multiline {
		return f.posAtPoint(at.X-rect.Min.X, at.Y-rect.Min.Y)
	}
	return f.posAt(at.X - rect.Min.X)
}
