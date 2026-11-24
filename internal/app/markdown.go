package app

import (
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Markdown, as the changelog of a release is written, drawn in the theme's colors: headings,
// lists, quotes, code, rules, and bold, italic, struck, code and link text inside a line. Lines
// that only break a paragraph or an item are joined, so the text wraps to the popup's width.

// mdBlock is one block of a markdown text: a heading, a list item, a quote, a fenced code line,
// a rule, a paragraph or a blank line.
type mdBlock struct {
	kind  mdKind
	level int // a heading's #s, an item's depth from 0
	mark  string
	text  string
}

type mdKind int

const (
	mdParagraph mdKind = iota
	mdBlank
	mdHeading
	mdItem
	mdQuote
	mdCode
	mdRule
)

// mdBlocks splits markdown into its blocks, joining the lines that go on a paragraph, an item or
// a quote.
func mdBlocks(text string) []mdBlock {
	var blocks []mdBlock
	fence := ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
				continue
			}
			blocks = append(blocks, mdBlock{kind: mdCode, text: strings.TrimRight(line, " \t")})
			continue
		}
		if f := codeFence(trimmed); f != "" {
			fence = f
			continue
		}
		block := mdLine(line, trimmed)
		if block.kind == mdParagraph && len(blocks) > 0 {
			if last := &blocks[len(blocks)-1]; last.kind == mdParagraph || last.kind == mdItem || last.kind == mdQuote {
				last.text += " " + block.text // a soft break: the line goes on the block before it
				continue
			}
		}
		if block.kind == mdQuote && len(blocks) > 0 && blocks[len(blocks)-1].kind == mdQuote && block.text != "" {
			blocks[len(blocks)-1].text += " " + block.text
			continue
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// codeFence is the ``` or ~~~ that opens a code block on a line, or "".
func codeFence(trimmed string) string {
	for _, f := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, f) {
			return f
		}
	}
	return ""
}

// mdLine is the block a line starts; a paragraph is one that starts nothing.
func mdLine(line, trimmed string) mdBlock {
	switch {
	case trimmed == "":
		return mdBlock{kind: mdBlank}
	case isRule(trimmed):
		return mdBlock{kind: mdRule}
	case strings.HasPrefix(trimmed, ">"):
		return mdBlock{kind: mdQuote, text: strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))}
	}
	if level := len(trimmed) - len(strings.TrimLeft(trimmed, "#")); level >= 1 && level <= 6 && strings.HasPrefix(trimmed[level:], " ") {
		return mdBlock{kind: mdHeading, level: level, text: strings.TrimSpace(strings.TrimRight(trimmed[level:], "#"))}
	}
	if mark, rest, ok := itemMark(trimmed); ok {
		depth := (len(line) - len(strings.TrimLeft(line, " \t"))) / 2
		return mdBlock{kind: mdItem, level: depth, mark: mark, text: rest}
	}
	return mdBlock{kind: mdParagraph, text: trimmed}
}

// isRule tells a line of three or more -, * or _ alone, a thematic break.
func isRule(trimmed string) bool {
	plain := strings.ReplaceAll(trimmed, " ", "")
	return len(plain) >= 3 && (strings.Trim(plain, "-") == "" || strings.Trim(plain, "*") == "" || strings.Trim(plain, "_") == "")
}

// itemMark is a list item's mark, a bullet or a number such as "1.", and the text after it.
func itemMark(trimmed string) (mark, rest string, ok bool) {
	for _, bullet := range []string{"- ", "* ", "+ "} {
		if after, found := strings.CutPrefix(trimmed, bullet); found {
			return "", strings.TrimSpace(after), true
		}
	}
	digits := len(trimmed) - len(strings.TrimLeft(trimmed, "0123456789"))
	if digits > 0 && digits <= 9 && len(trimmed) > digits+1 && (trimmed[digits] == '.' || trimmed[digits] == ')') && trimmed[digits+1] == ' ' {
		return trimmed[:digits+1], strings.TrimSpace(trimmed[digits+2:]), true
	}
	return "", "", false
}

// markdownRows draws markdown as rows of styled pieces at most width cells wide.
func (p *palette) markdownRows(text string, width int) [][]styledText {
	width = max(width, 8)
	var rows [][]styledText
	bullets := []string{"•", "◦", "▪"}
	for _, b := range mdBlocks(text) {
		switch b.kind {
		case mdBlank:
			if len(rows) > 0 && len(rows[len(rows)-1]) > 0 { // one blank row between blocks, not more
				rows = append(rows, nil)
			}
		case mdRule:
			rows = append(rows, []styledText{{strings.Repeat("─", width), p.panelMuted}})
		case mdCode:
			code := p.code()
			rows = append(rows, []styledText{{"  " + ansi.Truncate(strings.ReplaceAll(b.text, "\t", "    "), width-2, "…"), code}})
		case mdHeading:
			style := p.panelHeading
			if b.level > 3 {
				style = p.panelText
				style.Attrs |= uv.AttrBold
			}
			if len(rows) > 0 && len(rows[len(rows)-1]) > 0 {
				rows = append(rows, nil) // a heading stands apart from the block above it
			}
			rows = append(rows, wrapPieces(p.inline(b.text, style), width, nil, nil)...)
		case mdItem:
			mark := b.mark
			if mark == "" {
				mark = bullets[min(b.level, len(bullets)-1)]
			}
			indent := strings.Repeat("  ", b.level)
			first := []styledText{{indent + mark + " ", p.panelMuted}}
			rest := []styledText{{strings.Repeat(" ", ansi.StringWidth(first[0].text)), p.panelText}}
			rows = append(rows, wrapPieces(p.inline(b.text, p.panelText), width, first, rest)...)
		case mdQuote:
			bar := []styledText{{"│ ", p.panelMuted}}
			style := p.panelMuted
			style.Attrs |= uv.AttrItalic
			rows = append(rows, wrapPieces(p.inline(b.text, style), width, bar, bar)...)
		default:
			rows = append(rows, wrapPieces(p.inline(b.text, p.panelText), width, nil, nil)...)
		}
	}
	for len(rows) > 0 && len(rows[len(rows)-1]) == 0 {
		rows = rows[:len(rows)-1]
	}
	return rows
}

// code is the style of code, in a line or a block: the color of the focused frame's title, the
// theme's accent, the one the popup's own frame and title have.
func (p *palette) code() uv.Style {
	return uv.Style{Fg: orColor(p.focusedTitle.Fg, p.panelText.Fg), Bg: p.panelText.Bg}
}

// inline splits a line of markdown into pieces styled as it marks them: **bold**, *italic* or
// _italic_, ~~struck~~, `code`, [links](url) and <url>, with \ keeping a mark as text.
func (p *palette) inline(text string, base uv.Style) []styledText {
	var pieces []styledText
	var run strings.Builder
	var attrs uint8
	style := func(s uv.Style) uv.Style {
		s.Attrs |= attrs
		return s
	}
	flush := func() {
		if run.Len() > 0 {
			pieces = append(pieces, styledText{run.String(), style(base)})
			run.Reset()
		}
	}
	link := base
	link.Fg, link.Underline = orColor(p.focusedTitle.Fg, base.Fg), uv.UnderlineSingle
	for i := 0; i < len(text); {
		rest := text[i:]
		if rest[0] == '\\' && len(rest) > 1 && strings.ContainsRune("\\`*_[]()~<>#+-!.|", rune(rest[1])) {
			run.WriteByte(rest[1])
			i += 2
			continue
		}
		if rest[0] == '`' {
			ticks := len(rest) - len(strings.TrimLeft(rest, "`"))
			if end := strings.Index(rest[ticks:], rest[:ticks]); end >= 0 {
				flush()
				pieces = append(pieces, styledText{strings.TrimSpace(rest[ticks : ticks+end]), style(p.code())})
				i += ticks + end + ticks
				continue
			}
		}
		if rest[0] == '[' {
			if label, after, ok := linkParts(rest); ok {
				flush()
				for _, piece := range p.inline(label, link) {
					piece.style.Attrs |= attrs
					pieces = append(pieces, piece)
				}
				i += len(rest) - len(after)
				continue
			}
		}
		if rest[0] == '<' {
			if end := strings.IndexByte(rest, '>'); end > 0 && (strings.HasPrefix(rest, "<http://") || strings.HasPrefix(rest, "<https://")) {
				flush()
				pieces = append(pieces, styledText{rest[1:end], style(link)})
				i += end + 1
				continue
			}
		}
		if mark, attr := emphasis(rest); mark != "" {
			var prev, next rune
			if i > 0 {
				prev = rune(text[i-1])
			}
			if len(rest) > len(mark) {
				next = rune(rest[len(mark)])
			}
			opens := attrs&attr == 0 && next != 0 && next != ' ' && strings.Contains(rest[len(mark):], mark) && !(mark[0] == '_' && isWordRune(prev))
			closes := attrs&attr != 0 && prev != ' ' && !(mark[0] == '_' && isWordRune(next))
			if opens || closes {
				flush()
				attrs ^= attr
				i += len(mark)
				continue
			}
		}
		run.WriteByte(rest[0])
		i++
	}
	flush()
	return pieces
}

// emphasis is the mark of bold, italic or struck text that starts rest, and its attribute.
func emphasis(rest string) (string, uint8) {
	for _, m := range []struct {
		mark string
		attr uint8
	}{{"**", uv.AttrBold}, {"__", uv.AttrBold}, {"~~", uv.AttrStrikethrough}, {"*", uv.AttrItalic}, {"_", uv.AttrItalic}} {
		if strings.HasPrefix(rest, m.mark) {
			return m.mark, m.attr
		}
	}
	return "", 0
}

// linkParts reads [label](url) at the start of text: the label and what follows the link.
func linkParts(text string) (label, after string, ok bool) {
	middle := strings.Index(text, "](")
	if middle < 0 {
		return "", "", false
	}
	end := strings.IndexByte(text[middle+2:], ')')
	if end < 0 {
		return "", "", false
	}
	return text[1:middle], text[middle+2+end+1:], true
}

// wrapPieces breaks styled pieces into rows at most width wide between words, first leading the
// first row and rest the others, as a list item's mark and its hanging indent. A word wider than
// a row breaks where the row ends.
func wrapPieces(pieces []styledText, width int, first, rest []styledText) [][]styledText {
	width = max(width, styledWidth(first)+1, styledWidth(rest)+1)
	var rows [][]styledText
	row := append([]styledText(nil), first...)
	used, filled := styledWidth(first), false
	space := false
	newRow := func() {
		rows = append(rows, row)
		row, used, filled, space = append([]styledText(nil), rest...), styledWidth(rest), false, false
	}
	var spaceStyle uv.Style
	for _, word := range words(pieces) {
		if isSpace(word) {
			space, spaceStyle = filled, word[0].style
			continue
		}
		size := styledWidth(word)
		gap := 0
		if space {
			gap = 1
		}
		if filled && used+gap+size > width {
			newRow()
			gap = 0
		}
		if gap == 1 {
			row = append(row, styledText{" ", spaceStyle})
			used++
		}
		space = false
		for _, piece := range word {
			for used+ansi.StringWidth(piece.text) > width {
				head := ansi.Truncate(piece.text, width-used, "")
				if head == "" && filled {
					newRow()
					continue
				}
				if head == "" { // a grapheme wider than the whole row
					head, _ = ansi.FirstGraphemeCluster(piece.text, ansi.GraphemeWidth)
				}
				row = append(row, styledText{head, piece.style})
				piece.text = piece.text[len(head):]
				filled = true
				newRow()
			}
			if piece.text != "" {
				row = append(row, piece)
				used += ansi.StringWidth(piece.text)
				filled = true
			}
		}
	}
	return append(rows, row)
}

// words splits styled pieces at their spaces into words, each the pieces that touch, and the
// spaces between them, each one " " in the style it had.
func words(pieces []styledText) [][]styledText {
	var out [][]styledText
	var word []styledText
	for _, piece := range pieces {
		for text := piece.text; text != ""; {
			if text[0] == ' ' {
				if word != nil {
					out, word = append(out, word), nil
				}
				if len(out) > 0 && !isSpace(out[len(out)-1]) {
					out = append(out, []styledText{{" ", piece.style}})
				}
				text = strings.TrimLeft(text, " ")
				continue
			}
			end := strings.IndexByte(text, ' ')
			if end < 0 {
				end = len(text)
			}
			word = append(word, styledText{text[:end], piece.style})
			text = text[end:]
		}
	}
	if word != nil {
		out = append(out, word)
	}
	return out
}

// isSpace tells the space words puts between two words.
func isSpace(word []styledText) bool {
	return len(word) == 1 && word[0].text == " "
}
