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
	// head is the index among a sequence's children of its first step, and steps how many it
	// has; root marks the flow's own, whose children also hold the params, start and end.
	head, steps int
	root        bool
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
func headKey(slot chartSlot) string              { return "head:" + slot.parent + ":" + slot.branch }
func (b chartBlock) isInsertion() bool           { return b.path == "" && !b.params }
func (b chartBlock) contains(p image.Point) bool { return p.In(b.rect) }

// flowChart is a flow drawn at a zoom, with where each block is and the arrow out of each step.
type flowChart struct {
	canvas uv.ScreenBuffer
	size   image.Point
	blocks []chartBlock
	arrows map[string]chartArrow
	// gaps are the places a carried card can drop at, between the cards and in empty lists.
	gaps []chartGap
}

// chartGap is a place a step can go: index of the list at slot, shown at rect.
type chartGap struct {
	slot  chartSlot
	index int
	rect  image.Rectangle
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
	// moving marks the selected step as moved by the arrows; carried is the path of the step a
	// card carried by the mouse is, drawn faded.
	moving  bool
	carried string
	// running is the path of the step a run is on, lit on the chart.
	running string
	// paramLines are the flow's params as its card lists them.
	paramLines []string
}

const startLabel = "● start"

func buildChart(f flow.Flow, style chartStyle) *flowChart {
	style.paramLines = append(paramLines(f), returnLines(f)...)
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
	root.head, root.steps, root.root = 2, len(f.Steps), true
	c := &flowChart{size: image.Pt(root.w+4, root.h+2), arrows: map[string]chartArrow{}}
	c.canvas = uv.NewScreenBuffer(c.size.X, c.size.Y)
	style.draw(c, root, 2, 1)
	return c
}

// sequence lays out a list of steps; an empty list is a slot, one row with ∅ on the line, as
// wide as a small card so a carried one has room to land there.
func (s chartStyle) sequence(steps []flow.Step, slot chartSlot) *chartNode {
	if len(steps) == 0 {
		width := min(boxWidths[s.zoom], 14)
		seq := measureSeq([]*chartNode{{kind: slotNode, slot: slot, w: width, h: 1, center: width / 2}})
		seq.slot = slot
		return seq
	}
	var items []*chartNode
	for i, step := range steps {
		items = append(items, s.node(step, flow.Path(slot.parent, slot.branch, i)))
	}
	seq := measureSeq(items)
	seq.slot, seq.steps = slot, len(steps)
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
// Each has room for a carried card's place, as tall as an arrow between two cards.
const (
	ifSplitRows = arrowRows + 1
	repeatRows  = 2*arrowRows + 2
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
		w: w, h: header.h + ifSplitRows + max(then.h, otherwise.h) + arrowRows + 1, center: shift + mid,
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
		lines = append(lines, strings.Join(strings.Fields(step.Run), " "))
	case flow.AgentStep:
		lines = append(lines, strings.Join(strings.Fields(shortTemplate(step.Agent.Ask)), " "))
		if len(step.Agent.Choices) > 0 {
			lines = append(lines, "→ "+strings.Join(step.Agent.Choices, " | "))
		}
		lines = append(lines, "asks "+orDefault(shortTemplate(step.Agent.Use), "the default agent"))
	case flow.CallStep:
		lines = append(lines, step.Call)
		lines = append(lines, withText(step.With)...)
	case flow.IfStep:
		lines = append(lines, shortTemplate(step.If))
		lines = append(lines, exportText(step.Export)...)
	case flow.WaitStep:
		lines = append(lines, step.Wait)
	case flow.PromptStep:
		lines = append(lines, shortTemplate(step.Prompt), "→ ."+step.Var)
	case flow.SetStep:
		lines = append(lines, withText(step.Set)...)
	case flow.UnsetStep:
		lines = append(lines, strings.Join(step.Unset, ", "))
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
			if index := i - n.head; n.steps > 0 && index >= 0 && index <= n.steps && (n.root || index > 0 && index < n.steps) {
				s.addGap(c, n.slot, index, x+n.center, y)
			}
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
	kind := kindShown(n.step)
	drawCard(c.canvas, rect, s.cardTitle(s.boxTitle(n.step, n.path), kind, rect.Dx()), line, title)
	s.drawKind(c, rect, kind)
	for i, text := range s.boxText(n.step) {
		style := s.pal.panelText
		if i > 0 {
			style = s.pal.panelMuted
		}
		drawTextCut(c.canvas, x+2, y+1+i, x+n.w-2, text, style)
	}
}

// stepKinds name each kind of step on its card, with the mark its title starts with, so what a
// card does reads at a glance.
var stepKinds = map[flow.Kind]struct{ mark, name string }{
	flow.ScriptStep:  {"▸ ", "script"},
	flow.CommandStep: {"$ ", "command"},
	flow.AgentStep:   {"◆ ", "AI agent"},
	flow.CallStep:    {"ƒ ", "function"},
	flow.IfStep:      {"◇ ", "if"},
	flow.RepeatStep:  {"↻ ", "repeat"},
	flow.WaitStep:    {"◷ ", "wait"},
	flow.PromptStep:  {"? ", "asks you"},
	flow.SetStep:     {"= ", "set"},
	flow.UnsetStep:   {"∅ ", "unset"},
}

// kindShown is the kind a card names at its right, or "" when the step's name already says it, as
// an if's or a set's without an id.
func kindShown(step flow.Step) flow.Kind {
	if step.Label() == stepKinds[step.Kind()].name {
		return ""
	}
	return step.Kind()
}

// boxTitle is a step's name after the mark of its kind, with how it went in the last run.
func (s chartStyle) boxTitle(step flow.Step, path string) string {
	return s.boxMarks(path) + stepKinds[step.Kind()].mark + step.Label()
}

// cardTitle cuts a card's title with "…" where the name of its kind starts.
func (s chartStyle) cardTitle(title string, kind flow.Kind, width int) string {
	room := width - 6 // the corners, and a space each side of the title
	if name := stepKinds[kind].name; name != "" && s.zoom > 0 {
		room -= ansi.StringWidth(name) + 3
	}
	if ansi.StringWidth(title) > room {
		return ansi.Truncate(title, max(room, 1), "…")
	}
	return title
}

// drawKind writes the kind of step at the right of its card's top line, when the card is wide
// enough to keep room for its name.
func (s chartStyle) drawKind(c *flowChart, rect image.Rectangle, kind flow.Kind) {
	name := stepKinds[kind].name
	if name == "" || s.zoom == 0 {
		return
	}
	text := " " + name + " "
	drawText(c.canvas, rect.Max.X-2-ansi.StringWidth(text), rect.Min.Y, rect.Max.X-2, text, s.pal.panelMuted)
}

// boxMarks go before a card's title: running, being moved, warned of, how it went.
func (s chartStyle) boxMarks(path string) string {
	moved := ""
	if s.moving && stepKey(path) == s.selected || path == s.carried {
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
	case path != "" && path == s.carried:
		return s.pal.unfocusedFrame, s.pal.panelMuted
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
	s.drawBranchHead(c, then.slot, "yes", thenCenter+2, split)
	s.drawBranchHead(c, otherwise.slot, "no", elseCenter-3, split)
	top := split + ifSplitRows
	for _, branch := range []struct {
		center int
		node   *chartNode
	}{{thenCenter, then}, {elseCenter, otherwise}} {
		s.drawArrow(c, "", branch.center, split+1, ifSplitRows-1)
		if branch.node.steps > 0 {
			s.addGap(c, branch.node.slot, 0, branch.center, split+1)
		}
	}
	s.draw(c, then, thenX, top)
	s.draw(c, otherwise, elseX, top)
	join := top + max(then.h, otherwise.h) + arrowRows
	s.drawBranchEnd(c, n.step.Then, then, thenCenter, top, join)
	s.drawBranchEnd(c, n.step.Else, otherwise, elseCenter, top, join)
	s.drawJoin(c, thenCenter, elseCenter, x+n.center, join, "╰", "┬", "╯")
}

// drawBranchHead labels a branch; the label is its head, to add a step first in it, empty or not.
func (s chartStyle) drawBranchHead(c *flowChart, slot chartSlot, label string, x, y int) {
	key := headKey(slot)
	c.blocks = append(c.blocks, chartBlock{key: key, slot: slot, rect: image.Rect(x, y, x+len(label), y+1)})
	drawText(c.canvas, x, y, x+len(label), label, s.headStyle(key))
}

// drawBranchEnd draws the line from a branch's end down to the join, the way out of its last step.
func (s chartStyle) drawBranchEnd(c *flowChart, steps []flow.Step, branch *chartNode, center, top, join int) {
	for yy := top + branch.h; yy < join; yy++ {
		setRune(c.canvas, center, yy, lineDown, s.pal.unfocusedFrame)
	}
	if len(steps) > 0 {
		c.arrows[flow.Path(branch.slot.parent, branch.slot.branch, len(steps)-1)] = chartArrow{x: center, top: top + branch.h, rows: join - top - branch.h}
		s.addGap(c, branch.slot, len(steps), center, top+branch.h)
	}
}

// paramLines are what the params card lists: each param with its type and default.
func paramLines(f flow.Flow) []string {
	names := slices.Sorted(maps.Keys(f.Params))
	if len(names) == 0 {
		return []string{"none"}
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
		drawTextCut(c.canvas, x+2, y+1+i, x+n.w-2, text, s.pal.panelMuted)
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
	drawCard(c.canvas, rect, s.cardTitle(s.boxMarks(n.path)+s.repeatTitle(n.step), flow.RepeatStep, rect.Dx()), line, title)
	s.drawKind(c, rect, flow.RepeatStep)
	setRune(c.canvas, x+n.w-1, y+1, arrowUp, line) // the way back, up the frame's side
	body := n.children[0]
	in := image.Rect(x+n.center-1, y+1, x+n.center+2, y+1+arrowRows)
	head := headKey(body.slot)
	c.blocks = append(c.blocks, chartBlock{key: head, slot: body.slot, rect: in})
	if head == s.selected {
		line = s.pal.focusedTitle
	}
	if len(n.step.Steps) > 0 {
		s.addGap(c, body.slot, 0, x+n.center, y+1)
	}
	for row := y + 1; row < y+arrowRows; row++ {
		setRune(c.canvas, x+n.center, row, lineDown, line)
	}
	setRune(c.canvas, x+n.center, y+arrowRows, arrowDown, line)
	s.draw(c, body, x+n.center-body.center, y+1+arrowRows)
	tail := y + 1 + arrowRows + body.h
	for row := tail; row < y+n.h-1; row++ {
		setRune(c.canvas, x+n.center, row, lineDown, s.pal.unfocusedFrame)
	}
	if last := len(n.step.Steps) - 1; last >= 0 {
		c.arrows[flow.Path(n.path, "steps", last)] = chartArrow{x: x + n.center, top: tail, rows: y + n.h - 1 - tail}
		s.addGap(c, body.slot, last+1, x+n.center, tail)
	}
}

// drawSlot marks an empty list with ∅ on its line. Its head (the start, an if's yes or no, a
// repeat's arrow in) is what selects it; a carried card lands on the rows around it.
func (s chartStyle) drawSlot(c *flowChart, n *chartNode, x, y int) {
	c.gaps = append(c.gaps, chartGap{slot: n.slot, rect: image.Rect(x, y-1, x+n.w, y+2)})
	style := s.pal.panelMuted
	if headKey(n.slot) == s.selected {
		style = s.pal.focusedTitle
	}
	setRune(c.canvas, x+n.center, y, "∅", style)
}

// addGap keeps the place at index of the list at slot, the arrowRows rows from top on the line at
// column x, as wide as a card.
func (s chartStyle) addGap(c *flowChart, slot chartSlot, index, x, top int) {
	w := boxWidths[s.zoom]
	c.gaps = append(c.gaps, chartGap{slot: slot, index: index, rect: image.Rect(x-w/2, top, x-w/2+w, top+arrowRows)})
}

// Reach of a drop: how far from a place, in rows and columns, a card let go still lands there.
const (
	dropReachRows = 4
	dropReachCols = 8
)

// dropAt is the place nearest point where the step at path can go: not inside itself, nor right
// before or after itself, where it already is.
func (c *flowChart) dropAt(point image.Point, path string) (chartGap, bool) {
	parent, branch, index := flow.SplitPath(path)
	best, bestScore := chartGap{}, -1
	for _, g := range c.gaps {
		inside := g.slot.parent == path || strings.HasPrefix(g.slot.parent, path+"/")
		here := g.slot == (chartSlot{parent, branch}) && (g.index == index || g.index == index+1)
		if inside || here {
			continue
		}
		dx := max(g.rect.Min.X-point.X, point.X-(g.rect.Max.X-1), 0)
		dy := max(g.rect.Min.Y-point.Y, point.Y-(g.rect.Max.Y-1), 0)
		if dx > dropReachCols || dy > dropReachRows {
			continue
		}
		if score := 4*dy + dx; bestScore < 0 || score < bestScore {
			best, bestScore = g, score
		}
	}
	return best, bestScore >= 0
}

// drawGhost outlines, dashed, the place a carried card drops at, with the step's name in it.
func (c *flowChart) drawGhost(pal *palette, g chartGap, label string) {
	style := pal.focusedTitle
	style.Bg = pal.panelText.Bg
	r := g.rect
	fillRect(c.canvas, r, pal.panelText)
	right, bottom := r.Max.X-1, r.Max.Y-1
	for x := r.Min.X + 1; x < right; x++ {
		setRune(c.canvas, x, r.Min.Y, "╌", style)
		setRune(c.canvas, x, bottom, "╌", style)
	}
	for y := r.Min.Y + 1; y < bottom; y++ {
		setRune(c.canvas, r.Min.X, y, "╎", style)
		setRune(c.canvas, right, y, "╎", style)
	}
	setRune(c.canvas, r.Min.X, r.Min.Y, "╭", style)
	setRune(c.canvas, right, r.Min.Y, "╮", style)
	setRune(c.canvas, r.Min.X, bottom, "╰", style)
	setRune(c.canvas, right, bottom, "╯", style)
	text := "↳ " + label
	if ansi.StringWidth(text) > r.Dx()-4 {
		text = ansi.Truncate(text, max(r.Dx()-4, 1), "…")
	}
	drawCentered(c.canvas, r.Inset(1), r.Min.Y+r.Dy()/2, text, style)
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
