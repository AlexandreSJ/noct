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

// The flowchart of a flow: its steps as boxes from top to bottom, joined by arrows. An if splits
// into its two branches side by side and joins them again; a repeat frames the steps it repeats.
// The arrows before, between and after the steps of a list are gaps, where a step can be added.
// The chart is drawn on a canvas of its own size, which the inspect panel shows a part of.

// boxWidths are the widths of a step's box at each zoom, from far to near; boxLines how many
// lines of detail it shows.
var (
	boxWidths = []int{18, 28, 40}
	boxLines  = []int{1, 2, 4}
)

// Gaps of the chart: between an if's branches, and around a repeat's steps.
const (
	branchGap   = 3
	repeatInset = 2
)

type nodeKind int

const (
	stepNode   nodeKind = iota // a script, run or wait step's box
	ifNode                     // an if's box, its branches below it
	repeatNode                 // a frame around the steps repeated
	slotNode                   // an empty list of steps, where one can be added
	seqNode                    // steps one below the other
	pillNode                   // start or end of the flow
)

// chartNode is one part of the chart, measured: w by h cells, with the column arrows enter and
// leave it at, center, counted from its left.
type chartNode struct {
	kind     nodeKind
	w, h     int
	center   int
	step     flow.Step
	path     string
	slot     chartSlot
	label    string
	children []*chartNode
	// gapBase is the index of the step a sequence's first arrow comes before: -1 when the
	// sequence starts with the start pill. Only a sequence of steps has gaps.
	gapBase int
	hasGaps bool
}

// chartSlot is a list of steps: the flow's own when parent is "", or an if's or repeat's.
type chartSlot struct {
	parent, branch string
}

// chartBlock is something on the chart that can be selected: a step, or a place in a list where
// a step can be added (an empty list, or a gap between steps), at index.
type chartBlock struct {
	key   string
	path  string
	slot  chartSlot
	index int
	rect  image.Rectangle
}

// Keys of the chart's blocks, which stay the same while the chart is drawn again.
func stepKey(path string) string    { return "step:" + path }
func slotKey(slot chartSlot) string { return "slot:" + slot.parent + ":" + slot.branch }
func gapKey(slot chartSlot, index int) string {
	return fmt.Sprintf("gap:%s:%s:%d", slot.parent, slot.branch, index)
}
func (b chartBlock) isInsertion() bool           { return b.path == "" }
func (b chartBlock) contains(p image.Point) bool { return p.In(b.rect) }

// flowChart is a flow drawn at a zoom, with where each block is.
type flowChart struct {
	canvas uv.ScreenBuffer
	size   image.Point
	blocks []chartBlock
}

// chartStyle is what drawing a chart needs besides the flow.
type chartStyle struct {
	pal      *palette
	zoom     int
	selected string
	result   *flow.Result
	// describe gives what a script step runs, for its box.
	describe func(id string) string
	// ranged are the steps selected together with the selected one; warned, by path, the steps
	// whose templates read a step that may not be there.
	ranged map[string]bool
	warned map[string]bool
}

func buildChart(f flow.Flow, style chartStyle) *flowChart {
	body := style.sequence(f.Steps, chartSlot{})
	start := &chartNode{kind: pillNode, label: "● start"}
	end := &chartNode{kind: pillNode, label: "◉ end"}
	for _, pill := range []*chartNode{start, end} {
		pill.w, pill.h = ansi.StringWidth(pill.label), 1
		pill.center = pill.w / 2
	}
	root := measureSeq(append(append([]*chartNode{start}, body.children...), end))
	root.hasGaps, root.gapBase = len(f.Steps) > 0, -1
	c := &flowChart{size: image.Pt(root.w+4, root.h+2)}
	c.canvas = uv.NewScreenBuffer(c.size.X, c.size.Y)
	style.draw(c, root, 2, 1)
	return c
}

// sequence lays out a list of steps; an empty list is a slot to add steps into.
func (s chartStyle) sequence(steps []flow.Step, slot chartSlot) *chartNode {
	if len(steps) == 0 {
		width := min(boxWidths[s.zoom], 14)
		return measureSeq([]*chartNode{{kind: slotNode, slot: slot, w: width, h: 3, center: width / 2}})
	}
	var items []*chartNode
	for i, step := range steps {
		items = append(items, s.node(step, flow.Path(slot.parent, slot.branch, i)))
	}
	seq := measureSeq(items)
	seq.slot, seq.hasGaps = slot, true
	return seq
}

func measureSeq(items []*chartNode) *chartNode {
	left, right, h := 0, 0, 0
	for _, item := range items {
		left, right = max(left, item.center), max(right, item.w-item.center)
		h += item.h
	}
	return &chartNode{kind: seqNode, w: left + right, h: h + len(items) - 1, center: left, children: items}
}

func (s chartStyle) node(step flow.Step, path string) *chartNode {
	switch step.Kind() {
	case flow.IfStep:
		return s.ifNode(step, path)
	case flow.RepeatStep:
		return s.repeatNode(step, path)
	}
	w := boxWidths[s.zoom]
	return &chartNode{kind: stepNode, step: step, path: path, w: w, h: len(s.boxText(step)) + 2, center: w / 2}
}

// ifNode puts the if's box over its branches, then on the left, else on the right, centered
// between them.
func (s chartStyle) ifNode(step flow.Step, path string) *chartNode {
	header := &chartNode{kind: stepNode, step: step, path: path, w: boxWidths[s.zoom], h: len(s.boxText(step)) + 2}
	header.center = header.w / 2
	then := s.sequence(step.Then, chartSlot{parent: path, branch: "then"})
	otherwise := s.sequence(step.Else, chartSlot{parent: path, branch: "else"})
	mid := (then.center + then.w + branchGap + otherwise.center) / 2
	shift := max(header.center-mid, 0)
	w := max(shift+then.w+branchGap+otherwise.w, shift+mid-header.center+header.w)
	return &chartNode{
		kind: ifNode, step: step, path: path,
		w: w, h: header.h + 2 + max(then.h, otherwise.h) + 2, center: shift + mid,
		children: []*chartNode{header, then, otherwise},
	}
}

// repeatNode frames the steps repeated, the repeat's condition in the frame's title.
func (s chartStyle) repeatNode(step flow.Step, path string) *chartNode {
	body := s.sequence(step.Steps, chartSlot{parent: path, branch: "steps"})
	least := max(ansi.StringWidth(s.repeatTitle(step))+6, boxWidths[s.zoom])
	center := max(body.center+repeatInset, least/2)
	w := max(center+body.w-body.center+repeatInset, least)
	return &chartNode{kind: repeatNode, step: step, path: path, w: w, h: body.h + 4, center: center, children: []*chartNode{body}}
}

func (s chartStyle) repeatTitle(step flow.Step) string {
	r := step.Repeat
	title := "↻ " + step.Label()
	if s.zoom == 0 {
		return title
	}
	switch {
	case r.Times > 0:
		title += fmt.Sprintf(" × %d", r.Times)
	default:
		title += fmt.Sprintf(" until %s (max %d)", shortTemplate(r.Until), r.Max)
	}
	if r.Every != "" && s.zoom == 2 {
		title += " every " + r.Every
	}
	return title
}

// shortTemplate drops the braces of a template, so a condition reads shorter.
func shortTemplate(text string) string {
	return strings.TrimSpace(strings.NewReplacer("{{", "", "}}", "").Replace(text))
}

// boxText is what a step's box says below its title, as many lines as the zoom shows.
func (s chartStyle) boxText(step flow.Step) []string {
	var lines []string
	switch step.Kind() {
	case flow.ScriptStep:
		lines = append(lines, step.Script)
		lines = append(lines, withText(step.With)...)
		if d := s.describe(step.Script); d != "" {
			lines = append(lines, d)
		}
	case flow.CommandStep:
		lines = append(lines, "$ "+strings.Join(strings.Fields(step.Run), " "))
	case flow.IfStep:
		lines = append(lines, shortTemplate(step.If))
	case flow.WaitStep:
		lines = append(lines, "wait "+step.Wait)
	case flow.PromptStep:
		lines = append(lines, shortTemplate(step.Prompt), "→ ."+step.Var)
	case flow.SetStep:
		lines = append(lines, withText(step.Set)...)
	case flow.UnsetStep:
		lines = append(lines, "clear "+strings.Join(step.Unset, ", "))
	}
	if len(step.Capture) > 0 {
		lines = append(lines, "capture "+strings.Join(slices.Sorted(maps.Keys(step.Capture)), ", "))
	}
	return lines[:min(len(lines), boxLines[s.zoom])]
}

func withText(with map[string]string) []string {
	var lines []string
	for _, name := range slices.Sorted(maps.Keys(with)) {
		lines = append(lines, name+" = "+shortTemplate(with[name]))
	}
	return lines
}

// draw puts node on the canvas with its top left corner at x, y.
func (s chartStyle) draw(c *flowChart, n *chartNode, x, y int) {
	switch n.kind {
	case seqNode:
		s.drawSeq(c, n, x, y)
	case stepNode:
		s.drawBox(c, n, x, y)
	case ifNode:
		s.drawIf(c, n, x, y)
	case repeatNode:
		s.drawRepeat(c, n, x, y)
	case slotNode:
		s.drawSlot(c, n, x, y)
	case pillNode:
		drawText(c.canvas, x, y, x+n.w, n.label, s.pal.panelMuted)
	}
}

// drawSeq stacks the items on the sequence's center, an arrow between each two.
func (s chartStyle) drawSeq(c *flowChart, n *chartNode, x, y int) {
	for i, item := range n.children {
		if i > 0 {
			s.drawArrow(c, n.hasGaps, n.slot, n.gapBase+i, x+n.center, y, "▼")
			y++
		}
		s.draw(c, item, x+n.center-item.center, y)
		y += item.h
	}
}

func (s chartStyle) drawBox(c *flowChart, n *chartNode, x, y int) {
	rect := image.Rect(x, y, x+n.w, y+n.h)
	key := stepKey(n.path)
	c.blocks = append(c.blocks, chartBlock{key: key, path: n.path, rect: rect})
	line, title := s.frameStyles(key, n.path)
	fillRect(c.canvas, rect, s.pal.panelText)
	drawFrame(c.canvas, rect, s.boxTitle(n.step, n.path), line, title)
	for i, text := range s.boxText(n.step) {
		style := s.pal.panelText
		if i > 0 {
			style = s.pal.panelMuted
		}
		drawText(c.canvas, x+2, y+1+i, x+n.w-2, text, style)
	}
}

// boxTitle is a step's name, with how it went in the last run.
func (s chartStyle) boxTitle(step flow.Step, path string) string {
	symbol := map[flow.Kind]string{flow.IfStep: "◇ ", flow.WaitStep: "◷ ", flow.PromptStep: "? ", flow.SetStep: "= ", flow.UnsetStep: "∅ "}[step.Kind()]
	return s.warning(path) + s.mark(path) + symbol + step.Label()
}

func (s chartStyle) warning(path string) string {
	if s.warned[path] {
		return "⚠ "
	}
	return ""
}

func (s chartStyle) mark(path string) string {
	if s.result == nil {
		return ""
	}
	r, ran := s.result.Steps[path]
	switch {
	case !ran:
		return "– "
	case r.Status == flow.StatusFailed:
		return "✗ "
	}
	return "✓ "
}

// frameStyles are the line and title styles of a block: the accent when selected, the attention
// color when it failed in the last run.
func (s chartStyle) frameStyles(key, path string) (uv.Style, uv.Style) {
	switch {
	case key == s.selected || s.ranged[key]:
		return s.pal.focusedFrame, s.pal.focusedTitle
	case s.result != nil && s.result.Steps[path].Status == flow.StatusFailed:
		return s.pal.status, s.pal.status
	}
	return s.pal.unfocusedFrame, s.pal.panelHeading
}

// drawIf draws the if's box, the split into the two branches, each branch, and the join.
func (s chartStyle) drawIf(c *flowChart, n *chartNode, x, y int) {
	header, then, otherwise := n.children[0], n.children[1], n.children[2]
	s.drawBox(c, header, x+n.center-header.center, y)
	left := x + n.center - (then.center+then.w+branchGap+otherwise.center)/2
	thenX, elseX := left, left+then.w+branchGap
	thenCenter, elseCenter := thenX+then.center, elseX+otherwise.center
	split := y + header.h
	s.drawJoin(c, thenCenter, elseCenter, x+n.center, split, "┌", "┴", "┐")
	drawText(c.canvas, thenCenter+2, split, x+n.center-1, "yes", s.pal.panelMuted)
	drawText(c.canvas, elseCenter-3, split, elseCenter, "no", s.pal.panelMuted)
	top := split + 2
	s.draw(c, then, thenX, top)
	s.draw(c, otherwise, elseX, top)
	join := top + max(then.h, otherwise.h) + 1
	s.drawBranchEnds(c, n.step.Then, then, thenCenter, split+1, top, join)
	s.drawBranchEnds(c, n.step.Else, otherwise, elseCenter, split+1, top, join)
	s.drawJoin(c, thenCenter, elseCenter, x+n.center, join, "└", "┬", "┘")
}

// drawBranchEnds draws the arrow into a branch and the line from its end down to the join, with
// the gaps before its first step and after its last.
func (s chartStyle) drawBranchEnds(c *flowChart, steps []flow.Step, branch *chartNode, center, arrow, top, join int) {
	gaps := len(steps) > 0
	s.drawArrow(c, gaps, branch.slot, 0, center, arrow, "▼")
	s.drawArrow(c, gaps, branch.slot, len(steps), center, top+branch.h, "│")
	for yy := top + branch.h + 1; yy < join; yy++ {
		setRune(c.canvas, center, yy, "│", s.pal.unfocusedFrame)
	}
}

// drawArrow draws a piece of line; when it is a gap, adding a step there puts it at index of slot,
// and a selected gap shows as a plus.
func (s chartStyle) drawArrow(c *flowChart, gap bool, slot chartSlot, index, x, y int, arrow string) {
	if !gap {
		setRune(c.canvas, x, y, arrow, s.pal.unfocusedFrame)
		return
	}
	key := gapKey(slot, index)
	c.blocks = append(c.blocks, chartBlock{key: key, slot: slot, index: index, rect: image.Rect(x-1, y, x+2, y+1)})
	if key == s.selected {
		setRune(c.canvas, x, y, "+", s.pal.focusedTitle)
		return
	}
	setRune(c.canvas, x, y, arrow, s.pal.unfocusedFrame)
}

// drawJoin draws a horizontal line from left to right with its ends and middle marked.
func (s chartStyle) drawJoin(c *flowChart, left, right, middle, y int, leftEnd, mid, rightEnd string) {
	line := s.pal.unfocusedFrame
	for xx := left; xx <= right; xx++ {
		setRune(c.canvas, xx, y, "─", line)
	}
	setRune(c.canvas, left, y, leftEnd, line)
	setRune(c.canvas, right, y, rightEnd, line)
	setRune(c.canvas, middle, y, mid, line)
}

// drawRepeat frames the repeated steps, with an arrow up its right side for the way back.
func (s chartStyle) drawRepeat(c *flowChart, n *chartNode, x, y int) {
	rect := image.Rect(x, y, x+n.w, y+n.h)
	key := stepKey(n.path)
	// Only the frame's title row selects the repeat: inside it are its own steps.
	c.blocks = append(c.blocks, chartBlock{key: key, path: n.path, rect: image.Rect(x, y, x+n.w, y+1)})
	line, title := s.frameStyles(key, n.path)
	drawFrame(c.canvas, rect, s.warning(n.path)+s.mark(n.path)+s.repeatTitle(n.step), line, title)
	for yy := y + 2; yy < y+n.h-2; yy++ {
		setRune(c.canvas, x+n.w-1, yy, "▲", line)
	}
	body := n.children[0]
	gaps := len(n.step.Steps) > 0
	s.drawArrow(c, gaps, body.slot, 0, x+n.center, y+1, "▼")
	s.drawArrow(c, gaps, body.slot, len(n.step.Steps), x+n.center, y+n.h-2, "│")
	s.draw(c, body, x+n.center-body.center, y+2)
}

func (s chartStyle) drawSlot(c *flowChart, n *chartNode, x, y int) {
	rect := image.Rect(x, y, x+n.w, y+n.h)
	key := slotKey(n.slot)
	c.blocks = append(c.blocks, chartBlock{key: key, slot: n.slot, rect: rect})
	line, text := s.pal.unfocusedFrame, s.pal.panelMuted
	if key == s.selected {
		line, text = s.pal.focusedFrame, s.pal.focusedTitle
	}
	drawFrame(c.canvas, rect, "", line, line)
	drawCentered(c.canvas, rect, y+1, "∅ a adds", text)
}

// blockAt is the block at a point of the chart; steps inside a repeat win over its frame.
func (c *flowChart) blockAt(p image.Point) (chartBlock, bool) {
	for i := len(c.blocks) - 1; i >= 0; i-- {
		if c.blocks[i].contains(p) {
			return c.blocks[i], true
		}
	}
	return chartBlock{}, false
}

// firstStep is the chart's first step, or its first block when it has no steps.
func (c *flowChart) firstStep() chartBlock {
	i := max(slices.IndexFunc(c.blocks, func(b chartBlock) bool { return !b.isInsertion() }), 0)
	return c.blocks[i]
}

func (c *flowChart) block(key string) (chartBlock, bool) {
	i := slices.IndexFunc(c.blocks, func(b chartBlock) bool { return b.key == key })
	if i < 0 {
		return chartBlock{}, false
	}
	return c.blocks[i], true
}

// blit shows the part of the chart that starts at pan in area.
func (c *flowChart) blit(screen uv.Screen, area image.Rectangle, pan image.Point) {
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			from := image.Pt(x-area.Min.X, y-area.Min.Y).Add(pan)
			if !from.In(image.Rectangle{Max: c.size}) {
				continue
			}
			if cell := c.canvas.CellAt(from.X, from.Y); cell != nil && cell.Width > 0 && x+cell.Width <= area.Max.X {
				screen.SetCell(x, y, cell)
			}
		}
	}
}
