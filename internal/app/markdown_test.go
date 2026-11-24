package app

import (
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// rowText is a row of styled pieces as plain text.
func rowText(row []styledText) string {
	var b strings.Builder
	for _, piece := range row {
		b.WriteString(piece.text)
	}
	return b.String()
}

func rowsText(rows [][]styledText) []string {
	var lines []string
	for _, row := range rows {
		lines = append(lines, rowText(row))
	}
	return lines
}

// A changelog's paragraphs and items, broken in the file, read as one block each; code keeps its
// lines.
func TestMarkdownJoinsSoftBreaks(t *testing.T) {
	text := "Flows grow up. A step\ncan ask.\n\n### Added\n\n- one\n  two\n  - nested\n1. first\n\n```\nkeep\n  this\n```\n> quoted\n> on\n---"
	var got []string
	for _, b := range mdBlocks(text) {
		got = append(got, strings.Join([]string{[]string{"p", "blank", "h", "item", "quote", "code", "rule"}[b.kind], b.mark, b.text}, "|"))
	}
	want := []string{"p||Flows grow up. A step can ask.", "blank||", "h||Added", "blank||", "item||one two", "item||nested", "item|1.|first", "blank||", "code||keep", "code||  this", "quote||quoted on", "rule||"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("blocks\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Marks inside a line style their text and go away; an underscore inside a word is text.
func TestMarkdownStylesInlineMarks(t *testing.T) {
	pal := testApp(t).palette
	pieces := pal.inline("**bold** and *it* ~~old~~ `a_b` [site](https://x.y) agent_models \\*star\\*", pal.panelText)
	text := rowText(pieces)
	if text != "bold and it old a_b site agent_models *star*" {
		t.Fatalf("text %q", text)
	}
	styleOf := func(word string) uv.Style {
		for _, piece := range pieces {
			if piece.text == word {
				return piece.style
			}
		}
		t.Fatalf("no piece %q in %q", word, pieces)
		return uv.Style{}
	}
	if styleOf("bold").Attrs&uv.AttrBold == 0 || styleOf("it").Attrs&uv.AttrItalic == 0 || styleOf("old").Attrs&uv.AttrStrikethrough == 0 {
		t.Error("bold, italic or struck text lost its attribute")
	}
	if styleOf("a_b") != pal.code() || styleOf("site").Underline == 0 {
		t.Error("code or a link is not styled")
	}
}

// A list item wraps under its text, never past the width.
func TestMarkdownWrapsItemsUnderTheirText(t *testing.T) {
	pal := testApp(t).palette
	rows := rowsText(pal.markdownRows("### Changed\n\n- the default agent opens a list with the agents your machine has", 24))
	want := []string{"Changed", "", "• the default agent", "  opens a list with the", "  agents your machine", "  has"}
	if strings.Join(rows, "\n") != strings.Join(want, "\n") {
		t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
	}
	for _, row := range pal.markdownRows("- "+strings.Repeat("x", 50)+" https://example.com/a/very/long/path/that/goes/on", 20) {
		if w := ansi.StringWidth(rowText(row)); w > 20 {
			t.Errorf("row %q is %d wide, past 20", rowText(row), w)
		}
	}
}
