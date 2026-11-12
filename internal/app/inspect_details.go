package app

import (
	"fmt"
	"image"
	"maps"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// Under the chart, the inspect tells about the block selected, by keys or a click: what its kind
// of step does, its whole text, which a card cuts, and how it went in the run shown. The wheel
// scrolls it while the mouse is over it.

// Sizes of the details: the least rows the chart keeps above them, and the share of the chart's
// rows they take at most.
const (
	detailsLeastChart = 10
	detailsShare      = 35 // percent
	detailsLeastRows  = 6
)

// drawDetails draws the selected block's details at the bottom of area, and returns what is left
// of area for the chart.
func (p *inspectPanel) drawDetails(screen uv.Screen, area image.Rectangle, f flow.Flow, last *flow.Result) image.Rectangle {
	p.details = image.Rectangle{}
	block, ok := p.selectedBlock()
	if !ok || p.carried() != "" { // a card carried has the whole chart to land on
		return area
	}
	width := area.Dx() - 4
	var lines []styledLine
	for _, line := range p.detailLines(f, block, last) {
		for _, part := range strings.Split(ansi.Wrap(line.text, max(width, 10), ""), "\n") {
			lines = append(lines, styledLine{part, line.style})
		}
	}
	rows := min(len(lines)+2, max(area.Dy()*detailsShare/100, detailsLeastRows))
	if area.Dy()-rows < detailsLeastChart {
		return area
	}
	if p.detailsKey != block.key {
		p.detailsKey, p.detailsFirst = block.key, 0
	}
	visible := rows - 2
	p.detailsFirst = min(max(p.detailsFirst, 0), max(len(lines)-visible, 0))
	rect := image.Rect(area.Min.X, area.Max.Y-rows, area.Max.X, area.Max.Y)
	pal := &p.app.palette
	fillRect(screen, rect, pal.panelText)
	status := ""
	if len(lines) > visible {
		status = fmt.Sprintf("↕ %d–%d of %d", p.detailsFirst+1, min(p.detailsFirst+visible, len(lines)), len(lines))
	}
	drawFrameStatus(screen, rect, p.detailsTitle(f, block), status, pal.unfocusedFrame, pal.panelHeading)
	for i := 0; i < visible && p.detailsFirst+i < len(lines); i++ {
		line := lines[p.detailsFirst+i]
		drawText(screen, rect.Min.X+2, rect.Min.Y+1+i, rect.Max.X-2, line.text, line.style)
	}
	p.details = rect
	return image.Rect(area.Min.X, area.Min.Y, area.Max.X, rect.Min.Y)
}

// scrollDetails moves the details by rows, when the wheel turns over them.
func (p *inspectPanel) scrollDetails(rows int) {
	p.detailsFirst = max(p.detailsFirst+rows, 0) // the next draw keeps the end in view
}

// detailsTitle names the block: a step by its name and kind.
func (p *inspectPanel) detailsTitle(f flow.Flow, block chartBlock) string {
	switch {
	case block.params && f.Function:
		return "params and returns"
	case block.params:
		return "params"
	case block.isInsertion():
		return "a place for a step"
	}
	step, _ := f.Step(block.path)
	kind := stepKinds[step.Kind()]
	if kindShown(step) == "" {
		return kind.mark + step.Label()
	}
	return kind.mark + step.Label() + " · " + kind.name
}

// detailLines tell about a block: a step's kind and whole text and its run, the params, or what a
// place in a list is for.
func (p *inspectPanel) detailLines(f flow.Flow, block chartBlock, last *flow.Result) []styledLine {
	pal := &p.app.palette
	switch {
	case block.params:
		return p.paramDetails(f)
	case block.isInsertion():
		return []styledLine{
			{"A step added here (a) runs first in this list.", pal.panelText},
			{"An empty list, as a branch of an if, runs nothing.", pal.panelMuted},
		}
	}
	step, ok := f.Step(block.path)
	if !ok {
		return nil
	}
	lines := p.app.stepLines(step)
	if last != nil {
		lines = append(lines, styledLine{"", pal.panelText}, styledLine{"in the run shown", pal.panelHeading})
		lines = append(lines, stepResultLines(*last, block.path, pal)...)
	}
	return append(lines, styledLine{"", pal.panelText}, styledLine{p.app.hint("flows", "{enter} opens this whole  {e} edits it"), pal.panelMuted})
}

// paramDetails list every param with its type, default and question, and a function's returns.
func (p *inspectPanel) paramDetails(f flow.Flow) []styledLine {
	pal := &p.app.palette
	if len(f.Params) == 0 && len(f.Returns) == 0 {
		return []styledLine{{"No params: a run asks nothing. a adds one.", pal.panelText}}
	}
	lines := []styledLine{{"A run asks for these in a form, or takes them as name=value:", pal.panelHeading}}
	for _, name := range slices.Sorted(maps.Keys(f.Params)) {
		param := f.Params[name]
		lines = append(lines, styledLine{name + "  " + paramText(param), pal.panelText})
		if param.Prompt != "" {
			lines = append(lines, styledLine{"  asks: " + param.Prompt, pal.panelMuted})
		}
	}
	if len(f.Returns) > 0 {
		lines = append(lines, styledLine{"", pal.panelText}, styledLine{"Gives back to the flow that calls it:", pal.panelHeading})
		for _, name := range slices.Sorted(maps.Keys(f.Returns)) {
			lines = append(lines, styledLine{name + " = " + f.Returns[name], pal.panelText})
		}
	}
	return lines
}
