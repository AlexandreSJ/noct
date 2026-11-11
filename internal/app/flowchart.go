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

// The flowchart of a flow: its steps as cards from top to bottom, joined by arrows. An if splits
// into its two branches side by side and joins them again; a repeat frames the steps it repeats.
// A step is added below the one selected; the head of each list (the start, an if's yes and no,
// a repeat's way in) is selected to add above its first step, and an empty list is a slot. The
// chart is drawn on a canvas of its own size, which the inspect panel shows a part of.

// boxWidths are the widths of a step's box at each zoom, from far to near; boxLines how many
// lines of detail it shows.
var (
	boxWidths = []int{18, 28, 40}
	boxLines  = []int{1, 2, 4}
)

// Gaps of the chart: between an if's branches, around a repeat's steps, and the rows of the arrow
// from one card down to the next.
const (
	branchGap   = 3
	repeatInset = 2
	arrowRows   = 3
)

// The lines of the chart, drawn as fonts like Fira Code draw them whole.
const (
	lineDown  = "│"
	arrowDown = "↓"
	arrowUp   = "↑"
)

type nodeKind int

const (
	stepNode   nodeKind = iota // a script, run or wait step's box
	ifNode                     // an if's box, its branches below it
	repeatNode                 // a frame around the steps repeated
	slotNode                   // an empty list of steps, where one can be added
	seqNode                    // steps one below the other
	pillNode                   // start or end of the flow
	paramsNode                 // the flow's params, above its start
)

// paramsKey is the params card's key; maxParamLines bounds the params it lists.
const (
	paramsKey      = "params"
	maxParamLines  = 8
	maxParamsWidth = 60
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
}

// chartSlot is a list of steps: the flow's own when parent is "", or an if's or repeat's.
type chartSlot struct {
	parent, branch string
}

// chartBlock is something on the chart that can be selected: a step, or the place in a list where
// a step is added at index: an empty list, or the head of one.
type chartBlock struct {
	key   string
	path  string
	slot  chartSlot
	index int
	rect  image.Rectangle
	// params marks the params card, which is neither a step nor a place to add one.
	params bool
}

// Keys of the chart's blocks, which stay the same while the chart is drawn again.
func stepKey(path string) string                 { return "step:" + path }
func slotKey(slot chartSlot) string              { return "slot:" + slot.parent + ":" + slot.branch }
func headKey(slot chartSlot) string              { return "head:" + slot.parent + ":" + slot.branch }
func (b chartBlock) isInsertion() bool           { return b.path == "" && !b.params }
func (b chartBlock) contains(p image.Point) bool { return p.In(b.rect) }

// flowChart is a flow drawn at a zoom, with where each block is and the arrow out of each step.
type flowChart struct {
	canvas uv.ScreenBuffer
	size   image.Point
	blocks []chartBlock
	arrows map[string]chartArrow
}

// chartArrow is the line out of a step, down to what runs after it: rows cells from top, in
// column x.
type chartArrow struct {
	x, top, rows int
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
	// moving marks the selected step as moved by the arrows; dropOn is the block a card carried
	// by the mouse would land on.
	moving bool
	dropOn string
	// running is the path of the step a run is on, lit on the chart.
	running string
	// paramLines are the flow's params as its card lists them.
	paramLines []string
}

const startLabel = "● start"

func buildChart(f flow.Flow, style chartStyle) *flowChart {
	style.paramLines = paramLines(f)
	body := style.sequence(f.Steps, chartSlot{})
	start := &chartNode{kind: pillNode, label: startLabel}
	end := &chartNode{kind: pillNode, label: "◉ end"}
	for _, pill := range []*chartNode{start, end} {
		pill.w, pill.h = ansi.StringWidth(pill.label), 1
		pill.center = pill.w / 2
	}
	params := &chartNode{kind: paramsNode, label: "params", w: boxWidths[style.zoom]}
	for _, line := range style.paramLines {
		params.w = max(params.w, min(ansi.StringWidth(line)+4, maxParamsWidth))
	}
	params.h, params.center = len(style.paramLines)+2, params.w/2
	root := measureSeq(append(append([]*chartNode{params, start}, body.children...), end))
	c := &flowChart{size: image.Pt(root.w+4, root.h+2), arrows: map[string]chartArrow{}}
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
	seq.slot = slot
	return seq
}

func measureSeq(items []*chartNode) *chartNode {
	left, right, h := 0, 0, 0
	for _, item := range items {
		left, right = max(left, item.center), max(right, item.w-item.center)
		h += item.h
	}
	return &chartNode{kind: seqNode, w: left + right, h: h + (len(items)-1)*arrowRows, center: left, children: items}
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

// ifSplitRows are the rows from an if's card to its branches: the split, and the arrow down into
// each; repeatRows, a repeat's frame and the arrows into and out of its steps.
const (
	ifSplitRows = 3
	repeatRows  = 5
)

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
		w: w, h: header.h + ifSplitRows + max(then.h, otherwise.h) + 2, center: shift + mid,
		children: []*chartNode{header, then, otherwise},
	}
}

// repeatNode frames the steps repeated, the repeat's condition in the frame's title.
func (s chartStyle) repeatNode(step flow.Step, path string) *chartNode {
	body := s.sequence(step.Steps, chartSlot{parent: path, branch: "steps"})
	least := max(ansi.StringWidth(s.repeatTitle(step))+6, boxWidths[s.zoom])
	center := max(body.center+repeatInset, least/2)
	w := max(center+body.w-body.center+repeatInset, least)
	return &chartNode{kind: repeatNode, step: step, path: path, w: w, h: body.h + repeatRows, center: center, children: []*chartNode{body}}
}

func (s chartStyle) repeatTitle(step flow.Step) string {
	r := step.Repeat
	title := "↻ " + step.Label()
	if s.zoom == 0 {
		return title
	}
	if r.Times > 0 {
		title += fmt.Sprintf(" × %d", r.Times)
	} else {
		title += fmt.Sprintf(" until %s (max %d)", shortTemplate(r.Until), r.Max)
	}
	if r.Every != "" && s.zoom == 2 {
		title += " every " + r.Every
	}
	if export := exportText(step.Export); export != nil && s.zoom > 0 {
		title += " · " + export[0]
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
		lines = append(lines, exportText(step.Export)...)
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

// exportText tells what a block keeps for the steps after it.
func exportText(export map[string]string) []string {
	var parts []string
	for _, inner := range slices.Sorted(maps.Keys(export)) {
		if outer := export[inner]; outer != "" && outer != inner {
			parts = append(parts, inner+" → "+outer)
		} else {
			parts = append(parts, inner)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return []string{"export " + strings.Join(parts, ", ")}
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
	case paramsNode:
		s.drawParams(c, n, x, y)
	case pillNode:
		style := s.pal.panelMuted
		if n.label == startLabel {
			key := headKey(chartSlot{})
			c.blocks = append(c.blocks, chartBlock{key: key, rect: image.Rect(x, y, x+n.w, y+1)})
			style = s.headStyle(key)
		}
		drawText(c.canvas, x, y, x+n.w, n.label, style)
	}
}

// drawSeq stacks the items on the sequence's center, an arrow between each two.
func (s chartStyle) drawSeq(c *flowChart, n *chartNode, x, y int) {
	for i, item := range n.children {
		if i > 0 {
			s.drawArrow(c, n.children[i-1].path, x+n.center, y, arrowRows)
			y += arrowRows
		}
		s.draw(c, item, x+n.center-item.center, y)
		y += item.h
	}
}

// drawArrow draws a line down rows cells from top ending in an arrowhead, the way out of the step
// at from, which a run moving on goes along.
func (s chartStyle) drawArrow(c *flowChart, from string, x, top, rows int) {
	for y := top; y < top+rows-1; y++ {
		setRune(c.canvas, x, y, lineDown, s.pal.unfocusedFrame)
	}
	setRune(c.canvas, x, top+rows-1, arrowDown, s.pal.unfocusedFrame)
	if from != "" {
		c.arrows[from] = chartArrow{x: x, top: top, rows: rows}
	}
}

// headStyle is a list head's: the accent when selected, muted otherwise.
func (s chartStyle) headStyle(key string) uv.Style {
	if key == s.dropOn {
		return s.pal.status
	}
	if key == s.selected {
		return s.pal.focusedTitle
	}
	return s.pal.panelMuted
}

func (s chartStyle) drawBox(c *flowChart, n *chartNode, x, y int) {
	rect := image.Rect(x, y, x+n.w, y+n.h)
	key := stepKey(n.path)
	c.blocks = append(c.blocks, chartBlock{key: key, path: n.path, rect: rect})
	line, title := s.frameStyles(key, n.path)
	fillRect(c.canvas, rect, s.pal.panelText)
	drawCard(c.canvas, rect, s.boxTitle(n.step, n.path), line, title)
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
	return s.boxMarks(path) + symbol + step.Label()
}

// boxMarks go before a card's title: running, being moved, warned of, how it went.
func (s chartStyle) boxMarks(path string) string {
	moved := ""
	if s.moving && stepKey(path) == s.selected {
		moved = "↕ "
	}
	if path == s.running {
		moved = "▶ " + moved
	}
	return moved + s.warning(path) + s.mark(path)
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
	case key == s.dropOn:
		return s.pal.status, s.pal.status
	case path != "" && path == s.running:
		running := s.pal.success
		running.Attrs |= uv.AttrBold
		return running, running
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
	s.drawJoin(c, thenCenter, elseCenter, x+n.center, split, "╭", "┴", "╮")
	s.drawBranchHead(c, n.step.Then, then.slot, "yes", thenCenter+2, split)
	s.drawBranchHead(c, n.step.Else, otherwise.slot, "no", elseCenter-3, split)
	top := split + ifSplitRows
	for _, center := range []int{thenCenter, elseCenter} {
		s.drawArrow(c, "", center, split+1, ifSplitRows-1)
	}
	s.draw(c, then, thenX, top)
	s.draw(c, otherwise, elseX, top)
	join := top + max(then.h, otherwise.h) + 1
	s.drawBranchEnd(c, n.step.Then, then, thenCenter, top, join)
	s.drawBranchEnd(c, n.step.Else, otherwise, elseCenter, top, join)
	s.drawJoin(c, thenCenter, elseCenter, x+n.center, join, "╰", "┬", "╯")
}

// drawBranchHead labels a branch; the label of one with steps is its head, to add above them.
func (s chartStyle) drawBranchHead(c *flowChart, steps []flow.Step, slot chartSlot, label string, x, y int) {
	style := s.pal.panelMuted
	if len(steps) > 0 {
		key := headKey(slot)
		c.blocks = append(c.blocks, chartBlock{key: key, slot: slot, rect: image.Rect(x, y, x+len(label), y+1)})
		style = s.headStyle(key)
	}
	drawText(c.canvas, x, y, x+len(label), label, style)
}

// drawBranchEnd draws the line from a branch's end down to the join, the way out of its last step.
func (s chartStyle) drawBranchEnd(c *flowChart, steps []flow.Step, branch *chartNode, center, top, join int) {
	for yy := top + branch.h; yy < join; yy++ {
		setRune(c.canvas, center, yy, lineDown, s.pal.unfocusedFrame)
	}
	if len(steps) > 0 {
		c.arrows[flow.Path(branch.slot.parent, branch.slot.branch, len(steps)-1)] = chartArrow{x: center, top: top + branch.h, rows: join - top - branch.h}
	}
}

// paramLines are what the params card lists: each param with its type and default.
func paramLines(f flow.Flow) []string {
	names := slices.Sorted(maps.Keys(f.Params))
	if len(names) == 0 {
		return []string{"none · a adds one"}
	}
	var lines []string
	for i, name := range names {
		if i == maxParamLines-1 && len(names) > maxParamLines {
			lines = append(lines, fmt.Sprintf("… %d more", len(names)-i))
			break
		}
		lines = append(lines, name+"  "+paramText(f.Params[name]))
	}
	return lines
}

func (s chartStyle) drawParams(c *flowChart, n *chartNode, x, y int) {
	rect := image.Rect(x, y, x+n.w, y+n.h)
	c.blocks = append(c.blocks, chartBlock{key: paramsKey, rect: rect, params: true})
	line, title := s.pal.unfocusedFrame, s.pal.panelHeading
	if s.selected == paramsKey {
		line, title = s.pal.focusedFrame, s.pal.focusedTitle
	}
	fillRect(c.canvas, rect, s.pal.panelText)
	drawCard(c.canvas, rect, "$ params", line, title)
	for i, text := range s.paramLines {
		drawText(c.canvas, x+2, y+1+i, x+n.w-2, text, s.pal.panelMuted)
	}
}

// drawCard frames a card with round corners.
func drawCard(screen uv.Screen, rect image.Rectangle, title string, line, titleStyle uv.Style) {
	drawFrame(screen, rect, title, line, titleStyle)
	if rect.Dx() < 2 || rect.Dy() < 2 {
		return
	}
	right, bottom := rect.Max.X-1, rect.Max.Y-1
	setRune(screen, rect.Min.X, rect.Min.Y, "╭", line)
	setRune(screen, right, rect.Min.Y, "╮", line)
	setRune(screen, rect.Min.X, bottom, "╰", line)
	setRune(screen, right, bottom, "╯", line)
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
	drawCard(c.canvas, rect, s.boxMarks(n.path)+s.repeatTitle(n.step), line, title)
	setRune(c.canvas, x+n.w-1, y+1, arrowUp, line) // the way back, up the frame's side
	body := n.children[0]
	in := image.Rect(x+n.center-1, y+1, x+n.center+2, y+3)
	if len(n.step.Steps) > 0 {
		head := headKey(body.slot)
		c.blocks = append(c.blocks, chartBlock{key: head, slot: body.slot, rect: in})
		if head == s.selected {
			line = s.pal.focusedTitle
		}
	}
	setRune(c.canvas, x+n.center, y+1, lineDown, line)
	setRune(c.canvas, x+n.center, y+2, arrowDown, line)
	s.draw(c, body, x+n.center-body.center, y+3)
	setRune(c.canvas, x+n.center, y+n.h-2, lineDown, s.pal.unfocusedFrame)
	if last := len(n.step.Steps) - 1; last >= 0 {
		c.arrows[flow.Path(n.path, "steps", last)] = chartArrow{x: x + n.center, top: y + n.h - 2, rows: 1}
	}
}

func (s chartStyle) drawSlot(c *flowChart, n *chartNode, x, y int) {
	rect := image.Rect(x, y, x+n.w, y+n.h)
	key := slotKey(n.slot)
	c.blocks = append(c.blocks, chartBlock{key: key, slot: n.slot, rect: rect})
	line, text := s.pal.unfocusedFrame, s.pal.panelMuted
	if key == s.selected {
		line, text = s.pal.focusedFrame, s.pal.focusedTitle
	}
	drawCard(c.canvas, rect, "", line, line)
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
	for _, first := range []func(chartBlock) bool{
		func(b chartBlock) bool { return b.path != "" },
		chartBlock.isInsertion,
	} {
		if i := slices.IndexFunc(c.blocks, first); i >= 0 {
			return c.blocks[i]
		}
	}
	return c.blocks[0]
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
