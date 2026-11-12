package app

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
)

const (
	flowsListHints = "{up}{down} flow  {enter} open  {a} new  {r} run  {E} edit file  {delete} delete  {?} all keys  {tab} hide list"
	flowStepHints  = "{up}{down}{left}{right} select  {enter} about  {a} add below  {e} edit  {delete} remove  {r} run  {t} test  {m} move  alt+drag move  {z} undo  {+} {-} zoom  {?} all keys  {tab} list"
	flowMoveHints  = "{up}{down} move the step, in and out of blocks  {m} {enter} {esc} put it down"
	flowGapHints   = "{up}{down}{left}{right} select  {a} add here  {r} run  {+} {-} zoom  {?} all keys  {tab} list"
	// flowsListWidth is the list of flows beside the chart, in panels wide enough for both.
	flowsListWidth = 24
	// panStep is how far shift+arrows move the chart.
	panStep = 4
)

// inspectPanel lists the flows and draws the selected one as a flowchart, to read it, build it
// step by step, run it and see how its last run went.
type inspectPanel struct {
	app   *App
	close func()

	flowName   string
	focusChart bool
	listHidden bool
	// anchor is the step a range started at with shift, or "" for the selected step alone.
	anchor    string
	showScope bool
	// moving moves the selected step with the arrows, until m, enter or esc.
	moving bool
	// warnings are the flow's, found when it is drawn.
	warnings []flow.Warning
	// run is a run opened from the logs, shown instead of the flow's last one.
	run      *flow.Result
	selected string
	zoom     int
	// pan is the chart's point at the chart area's top left; while centered, it follows the
	// selection instead.
	pan      image.Point
	centered bool

	chart     *flowChart
	list      image.Rectangle
	chartArea image.Rectangle
	// details tell about the selected block under the chart; detailsFirst is the first of their
	// lines shown, back to 0 when detailsKey, the block, changes.
	details      image.Rectangle
	detailsFirst int
	detailsKey   string
	drag         *chartDrag
	lastClick    time.Time
	clickedKey   string
}

// chartDrag is the mouse held down on the chart: it pans the chart, or, pressed on a step's card
// with alt or ctrl held, carries the card to drop it between others or into a block.
type chartDrag struct {
	from  image.Point
	pan   image.Point
	moved bool
	card  string
	over  image.Point
}

func (a *App) startInspect(id layout.PaneID, _ image.Rectangle) (view, error) {
	a.reloadScripts()
	a.reloadFlows()
	p := &inspectPanel{app: a, close: func() { a.detachView(id) }, zoom: 1, centered: true}
	if len(a.flows) > 0 {
		p.flowName = entryKey(a.flows[0])
	}
	return p, nil
}

// openFlowRun shows a flow's chart with how each block went in one of its runs.
func (a *App) openFlowRun(name string, r flow.Result) {
	a.reloadScripts()
	a.reloadFlows()
	a.openPanel(func(id layout.PaneID, _ image.Rectangle) (view, error) {
		return &inspectPanel{
			app: a, close: func() { a.detachView(id) }, zoom: 1, centered: true,
			flowName: name, run: &r, listHidden: true, focusChart: true,
		}, nil
	})
}

// result is the run the chart shows: the one opened from the logs, or the flow's last.
func (p *inspectPanel) result(f flow.Flow) *flow.Result {
	if f.Function {
		return nil // a function's steps run inside the flows that call it
	}
	if p.run != nil {
		return p.run
	}
	if r, ran := flow.LastResult(flow.ResultsDir(), f.Name); ran {
		return &r
	}
	return nil
}

// entries are what the list shows: the flows, then the functions they call.
func (p *inspectPanel) entries() []flow.Flow {
	return append(slices.Clip(p.app.flows), p.app.functions...)
}

// entryKey names an entry of the list apart from a flow and a function of the same name.
func entryKey(f flow.Flow) string {
	if f.Function {
		return "ƒ " + f.Name
	}
	return f.Name
}

// current is the selected flow or function, following it by name as the list is read again.
func (p *inspectPanel) current() (flow.Flow, int, bool) {
	flows := p.entries()
	if len(flows) == 0 {
		return flow.Flow{}, 0, false
	}
	i := max(slices.IndexFunc(flows, func(f flow.Flow) bool { return entryKey(f) == p.flowName }), 0)
	p.flowName = entryKey(flows[i])
	return flows[i], i, true
}

func (p *inspectPanel) selectFlow(step int) {
	_, i, ok := p.current()
	if !ok {
		return
	}
	entries := p.entries()
	i = min(max(i+step, 0), len(entries)-1)
	p.flowName, p.selected, p.centered, p.anchor, p.run = entryKey(entries[i]), "", true, "", nil
}

func (p *inspectPanel) SendKey(key uv.KeyPressEvent) {
	name := key.String()
	f, _, ok := p.current()
	switch name {
	case "ctrl+d":
		p.close()
		return
	case "tab", "shift+tab":
		p.listHidden = !p.listHidden && ok
		p.focusChart = p.listHidden
		return
	case "?":
		p.app.showContextKeys("flows", "inspect keys",
			"click selects a block; a double click tells about it",
			"alt+drag (or ctrl+drag) carries a card: the dashed outline is where it lands",
			"drag pans the chart; the wheel scrolls it, or the details under it")
		return
	}
	if !ok || !p.focusChart && name == "a" {
		p.listKey(name, f)
		return
	}
	if f.Function && slices.Contains([]string{"r", "p", "T", "shift+t"}, name) && !(name == "r" && p.paramsSelected()) {
		p.app.notice = "function " + f.Name + " runs when a flow calls it; a call step in a flow does"
		return
	}
	switch name {
	case "r":
		if p.paramsSelected() && f.Function {
			p.app.editReturn(f)
			return
		}
		p.app.runFlow(f)
	case "p":
		p.app.openFlowForm(f, p.app.flowValues(f))
	case "E":
		p.app.editFlowFile(f)
	case "T", "shift+t":
		p.app.addTrigger(f)
	case "P", "shift+p":
		p.app.promoteFlow(f)
	default:
		if p.focusChart {
			p.chartKey(name, f)
		} else {
			p.listKey(name, f)
		}
	}
}

// toggleTriggers turns the flow's triggers on or off on this machine.
func (p *inspectPanel) toggleTriggers(f flow.Flow) {
	if len(f.TriggerLabels()) == 0 {
		p.app.notice = p.app.hint("flows", "flow "+f.Name+" has no triggers; add them to its file ({E})")
		return
	}
	on := !flow.Enabled(flow.TriggersFile())[f.Name]
	if err := flow.SetEnabled(flow.TriggersFile(), f.Name, on); err != nil {
		p.app.statusMessage = err.Error()
		return
	}
	p.app.notice = "triggers of " + f.Name + " " + onOff(on)
}

// triggersText tells a flow's triggers and whether they are on, or "" without any.
func triggersText(f flow.Flow, on bool) string {
	labels := f.TriggerLabels()
	if len(labels) == 0 {
		return ""
	}
	return "triggers " + onOff(on) + ": " + strings.Join(labels, ", ")
}

// listKey runs the list's keys; f is the zero flow when there are none.
func (p *inspectPanel) listKey(name string, f flow.Flow) {
	switch name {
	case "a":
		p.app.newFlow(func(created string) { p.flowName, p.focusChart, p.selected = created, true, "" })
	case "F", "shift+f":
		p.app.newFunction(func(created string) { p.flowName, p.focusChart, p.selected = created, true, "" })
	case "up", "k":
		p.selectFlow(-1)
	case "down", "j":
		p.selectFlow(1)
	case "pgup", "pgdown", "home", "end":
		if _, i, ok := p.current(); ok {
			to, _ := listJump(name, i, len(p.entries()), max(p.list.Dy()-2, 1)) // inside the frame
			p.selectFlow(to - i)
		}
	case "enter", "right", "l":
		p.focusChart = true
	case "o":
		p.toggleTriggers(f)
	case "delete":
		if f.Name == "" {
			return
		}
		question := "delete flow " + f.Name + "?"
		if f.Function {
			question = "delete function " + f.Name + "?"
			if callers := flow.Callers(p.entries(), f.Name); len(callers) > 0 {
				question = "delete function " + f.Name + ", which " + strings.Join(callers, ", ") + " call?"
			}
		}
		p.app.askToConfirm(question, func() {
			if err := f.Delete(); err != nil {
				p.app.statusMessage = err.Error()
			}
			p.app.reloadFlows()
		})
	}
}

func (p *inspectPanel) chartKey(name string, f flow.Flow) {
	if p.moving {
		p.moveKey(name, f)
		return
	}
	block, selected := p.selectedBlock()
	switch name {
	case "z":
		p.app.undoFlow(f, false)
	case "y":
		p.app.undoFlow(f, true)
	case "esc":
		p.escapeChart()
	case "shift+up", "shift+down":
		if selected && !block.isInsertion() {
			p.extendRange(f, block.path, map[string]int{"shift+up": -1, "shift+down": 1}[name])
		}
	case "t":
		p.testSelection(f, block, selected)
	case "s":
		p.showScope = !p.showScope
	case "enter", "space", "e":
		switch {
		case selected && block.params:
			p.app.editFlowParams(f)
		case name == "e":
			if selected && !block.isInsertion() {
				p.stepKey(name, f, block.path)
			}
		case selected && !block.isInsertion():
			p.app.aboutStep(f, block.path, p.result(f))
		}
	case "a":
		switch {
		case selected && block.params:
			p.app.addFlowParam(f)
		case selected:
			p.app.addStep(f, block, func(path string) { p.selected = stepKey(path) })
		}
	default:
		if p.chartViewKey(name) {
			return
		}
		if selected && !block.isInsertion() && !block.params {
			p.stepKey(name, f, block.path)
		}
	}
}

// escapeChart drops the range anchor first, and leaves the chart when there is none.
func (p *inspectPanel) escapeChart() {
	if p.anchor != "" {
		p.anchor = ""
		return
	}
	p.focusChart, p.listHidden = false, false
}

// chartViewKey runs the keys that move the selection or the view, and tells whether name was one.
func (p *inspectPanel) chartViewKey(name string) bool {
	switch name {
	case "up", "k":
		p.moveSelection(-1)
	case "down", "j":
		p.moveSelection(1)
	case "left", "h":
		p.moveAcross(-1)
	case "right", "l":
		p.moveAcross(1)
	case "ctrl+up", "ctrl+down", "ctrl+left", "ctrl+right":
		p.panBy(arrowHeadings[strings.TrimPrefix(name, "ctrl+")])
	case "+", "=":
		p.setZoom(p.zoom + 1)
	case "-":
		p.setZoom(p.zoom - 1)
	case "0":
		p.centered = true
	default:
		return false
	}
	return true
}

// stepKey runs the keys that act on the selected step alone, ending a range.
func (p *inspectPanel) stepKey(name string, f flow.Flow, path string) {
	p.anchor = ""
	switch name {
	case "e":
		p.app.editFlowStep(f, path)
	case "delete":
		p.app.removeStep(f, path)
	case "J", "shift+j":
		p.app.moveStep(f, path, true, p.selectMoved)
	case "K", "shift+k":
		p.app.moveStep(f, path, false, p.selectMoved)
	case "m":
		p.moving = true
	case "x":
		p.app.exportFromBlock(f, path)
	}
}

func (p *inspectPanel) paramsSelected() bool {
	block, ok := p.selectedBlock()
	return ok && block.params
}

func (p *inspectPanel) selectMoved(path string) {
	p.selected = stepKey(path)
}

// moveKey moves the step being moved with the arrows; m, enter or esc puts it down.
func (p *inspectPanel) moveKey(name string, f flow.Flow) {
	block, ok := p.selectedBlock()
	switch name {
	case "up", "k", "K", "shift+k":
		if ok && !block.isInsertion() {
			p.app.moveStep(f, block.path, false, p.selectMoved)
		}
	case "down", "j", "J", "shift+j":
		if ok && !block.isInsertion() {
			p.app.moveStep(f, block.path, true, p.selectMoved)
		}
	case "m", "enter", "esc", "space":
		p.moving = false
	}
}

func (p *inspectPanel) selectedBlock() (chartBlock, bool) {
	if p.chart == nil {
		return chartBlock{}, false
	}
	return p.chart.block(p.selected)
}

// extendRange moves the selection to the step before or after, in the same list, keeping the
// steps from where the range started selected.
func (p *inspectPanel) extendRange(f flow.Flow, path string, step int) {
	parent, branch, index := flow.SplitPath(path)
	next := flow.Path(parent, branch, index+step)
	if _, ok := f.Step(next); !ok {
		return
	}
	if p.anchor == "" {
		p.anchor = path
	}
	p.selected = stepKey(next)
	if block, ok := p.selectedBlock(); ok {
		p.keepInView(block.rect)
	}
}

// rangeEnds are the first and last steps of the range, in the flow's order, or the step alone.
func (p *inspectPanel) rangeEnds(path string) (string, string) {
	if p.anchor == "" {
		return path, path
	}
	_, _, from := flow.SplitPath(p.anchor)
	_, _, to := flow.SplitPath(path)
	if from > to {
		return path, p.anchor
	}
	return p.anchor, path
}

// ranged are the keys of the steps in the range, for the chart to mark.
func (p *inspectPanel) ranged() map[string]bool {
	block, ok := p.selectedBlock()
	if p.anchor == "" || !ok {
		return nil
	}
	first, last := p.rangeEnds(block.path)
	parent, branch, from := flow.SplitPath(first)
	_, _, to := flow.SplitPath(last)
	keys := map[string]bool{}
	for i := from; i <= to; i++ {
		keys[stepKey(flow.Path(parent, branch, i))] = true
	}
	return keys
}

func (p *inspectPanel) testSelection(f flow.Flow, block chartBlock, selected bool) {
	if !selected || block.isInsertion() {
		p.app.statusMessage = "select a step to test"
		return
	}
	from, to := p.rangeEnds(block.path)
	p.app.testFlowSteps(f, from, to)
}

// moveSelection goes to the block before or after the selected one, top to bottom.
func (p *inspectPanel) moveSelection(step int) {
	p.anchor = ""
	if p.chart == nil || len(p.chart.blocks) == 0 {
		return
	}
	order := slices.Clone(p.chart.blocks)
	slices.SortStableFunc(order, func(a, b chartBlock) int {
		if a.rect.Min.Y != b.rect.Min.Y {
			return a.rect.Min.Y - b.rect.Min.Y
		}
		return a.rect.Min.X - b.rect.Min.X
	})
	i := slices.IndexFunc(order, func(b chartBlock) bool { return b.key == p.selected })
	i = min(max(i+step, 0), len(order)-1)
	p.selected = order[i].key
	p.keepInView(order[i].rect)
}

// moveAcross goes to the nearest block on the left (step -1) or right (step 1) of the selected
// one, preferring blocks on the same rows: from one branch of an if to the other.
func (p *inspectPanel) moveAcross(step int) {
	p.anchor = ""
	from, ok := p.selectedBlock()
	if !ok {
		return
	}
	center := func(r image.Rectangle) image.Point { return r.Min.Add(r.Max).Div(2) }
	origin := center(from.rect)
	best, bestScore := chartBlock{}, [2]int{-1, -1}
	for _, b := range p.chart.blocks {
		dx := (center(b.rect).X - origin.X) * step
		if dx <= 0 {
			continue
		}
		score := [2]int{abs(center(b.rect).Y - origin.Y), dx}
		if bestScore[0] < 0 || slices.Compare(score[:], bestScore[:]) < 0 {
			best, bestScore = b, score
		}
	}
	if bestScore[0] >= 0 {
		p.selected = best.key
		p.keepInView(best.rect)
	}
}

func (p *inspectPanel) setZoom(zoom int) {
	p.zoom = min(max(zoom, 0), len(boxWidths)-1)
	p.centered = true
}

func (p *inspectPanel) panBy(heading layout.Heading) {
	step := map[layout.Heading]image.Point{
		layout.Left: {-panStep, 0}, layout.Right: {panStep, 0}, layout.Up: {0, -panStep / 2}, layout.Down: {0, panStep / 2},
	}[heading]
	p.pan, p.centered = p.clampPan(p.pan.Add(step)), false
}

// clampPan keeps at least a corner of the chart in view.
func (p *inspectPanel) clampPan(pan image.Point) image.Point {
	if p.chart == nil {
		return pan
	}
	size, view := p.chart.size, p.chartArea.Size()
	pan.X = min(max(pan.X, 4-view.X), size.X-4)
	pan.Y = min(max(pan.Y, 2-view.Y), size.Y-2)
	return pan
}

// keepInView pans the least needed for rect to show whole, when it fits.
func (p *inspectPanel) keepInView(rect image.Rectangle) {
	if p.centered {
		return // the next draw centers on the selection
	}
	view := image.Rectangle{Min: p.pan, Max: p.pan.Add(p.chartArea.Size())}
	switch {
	case rect.Min.Y < view.Min.Y:
		p.pan.Y = rect.Min.Y - 1
	case rect.Max.Y > view.Max.Y:
		p.pan.Y += rect.Max.Y - view.Max.Y + 1
	}
	switch {
	case rect.Min.X < view.Min.X:
		p.pan.X = rect.Min.X - 1
	case rect.Max.X > view.Max.X:
		p.pan.X += rect.Max.X - view.Max.X + 1
	}
}

// centerPan centers the chart across, and down on the selected block when it does not fit.
func (p *inspectPanel) centerPan() {
	size, view := p.chart.size, p.chartArea.Size()
	p.pan = image.Pt((size.X-view.X)/2, min((size.Y-view.Y)/2, 0))
	if block, ok := p.selectedBlock(); ok && size.Y > view.Y {
		p.pan.Y = min(max(block.rect.Min.Y-view.Y/3, 0), size.Y-view.Y)
	}
}

func (p *inspectPanel) Draw(screen uv.Screen, area uv.Rectangle) {
	pal := &p.app.palette
	rows := drawHints(screen, area, p.hints(), pal.panelMuted)
	body := image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Max.Y-rows)
	f, _, ok := p.current()
	if !ok {
		p.chart, p.list, p.chartArea = nil, image.Rectangle{}, image.Rectangle{}
		drawCentered(screen, body, body.Min.Y+body.Dy()/2-1, "no flows yet", pal.panelHeading)
		drawCentered(screen, body, body.Min.Y+body.Dy()/2+1, "a creates a flow, and in its chart a adds its steps", pal.panelMuted)
		return
	}
	list, chartArea := image.Rectangle{}, body
	switch {
	case p.listHidden:
	case body.Dx() >= flowsListWidth+40:
		list = image.Rect(body.Min.X, body.Min.Y, body.Min.X+flowsListWidth, body.Max.Y)
		chartArea.Min.X = list.Max.X
	case !p.focusChart:
		list, chartArea = body, image.Rectangle{}
	}
	p.list = image.Rectangle{}
	if !list.Empty() {
		p.list = p.drawFlows(screen, list).Sub(area.Min)
	}
	if chartArea.Empty() {
		p.chart, p.chartArea, p.details = nil, image.Rectangle{}, image.Rectangle{}
		return
	}
	p.drawFlowChart(screen, chartArea, f)
	p.chartArea, p.details = p.chartArea.Sub(area.Min), p.details.Sub(area.Min)
}

func (p *inspectPanel) hints() string {
	if !p.focusChart {
		return p.app.hint("flows", flowsListHints)
	}
	if p.moving {
		return p.app.hint("flows", flowMoveHints)
	}
	if block, ok := p.selectedBlock(); ok && block.isInsertion() {
		return p.app.hint("flows", flowGapHints)
	}
	return p.app.hint("flows", flowStepHints)
}

// frameStyles are the styles of the list's or chart's frame: the accent while it has the focus.
func (p *inspectPanel) frameStyles(focused bool) (uv.Style, uv.Style) {
	if focused {
		return p.app.palette.focusedFrame, p.app.palette.focusedTitle
	}
	return p.app.palette.unfocusedFrame, p.app.palette.panelHeading
}

// drawFlows lists the flows in a frame, each with how its last run went, and returns where the
// rows are.
func (p *inspectPanel) drawFlows(screen uv.Screen, area image.Rectangle) image.Rectangle {
	line, title := p.frameStyles(!p.focusChart)
	drawFrame(screen, area, "flows", line, title)
	inner := area.Inset(1)
	_, selected, _ := p.current()
	enabled := flow.Enabled(flow.TriggersFile())
	for i, f := range p.entries() {
		y := inner.Min.Y + i
		if y >= inner.Max.Y {
			break
		}
		drawRow(screen, inner, y, flowListMarks(f, enabled)+f.Name, "", p.flowRowStyle(i == selected))
	}
	return inner
}

func (p *inspectPanel) flowRowStyle(selected bool) uv.Style {
	pal := &p.app.palette
	if !selected {
		return pal.panelText
	}
	if p.focusChart {
		return pal.focusedTitle
	}
	return pal.selectedRow
}

// flowListMarks tell how a flow's last run went, whether it is a draft and whether a trigger
// starts it; a function is marked ƒ.
func flowListMarks(f flow.Flow, enabled map[string]bool) string {
	if f.Function {
		return map[bool]string{true: "✗ ", false: "ƒ "}[f.Problem != nil]
	}
	mark := "  "
	if f.Problem != nil {
		mark = "✗ "
	} else if r, ran := flow.LastResult(flow.ResultsDir(), f.Name); ran {
		mark = map[bool]string{true: "✓ ", false: "✗ "}[r.OK]
	}
	if f.Draft {
		mark += "✎ "
	}
	if enabled[f.Name] && len(f.TriggerLabels()) > 0 {
		mark += "⏰ "
	}
	return mark
}

// drawFlowChart frames the flow's chart, under its description and last run.
func (p *inspectPanel) drawFlowChart(screen uv.Screen, area image.Rectangle, f flow.Flow) {
	pal := &p.app.palette
	about := f.Name
	if f.Description != "" {
		about += " · " + f.Description
	}
	last := p.result(f)
	p.warnings = nil
	if f.Problem == nil {
		p.warnings = f.Warnings(p.app.scripts.Find)
	}
	if p.showScope && area.Dx() >= scopeWidth+30 {
		scope := image.Rect(area.Max.X-scopeWidth, area.Min.Y, area.Max.X, area.Max.Y)
		area.Max.X = scope.Min.X
		defer p.drawScope(screen, scope, f, last) // after the chart, which finds the selection
	}
	line, title := p.frameStyles(p.focusChart)
	drawFrame(screen, area, about, line, title)
	area = area.Inset(1)
	drawText(screen, area.Min.X+1, area.Min.Y, area.Max.X, p.chartInfoText(f, last), pal.panelMuted)
	canvasArea := image.Rect(area.Min.X, area.Min.Y+1, area.Max.X, area.Max.Y)
	if f.Problem != nil {
		drawText(screen, area.Min.X+1, canvasArea.Min.Y, area.Max.X, "✗ "+f.Problem.Error(), pal.status)
		canvasArea.Min.Y += 2
	}
	p.layoutFlowChart(f, last)
	canvasArea = p.drawDetails(screen, canvasArea, f, last)
	p.chartArea = canvasArea
	if p.centered {
		p.centerPan()
	}
	p.chart.blit(screen, canvasArea, p.pan)
}

// chartInfoText is the line above the chart: the run shown, the zoom, the timeout and the
// triggers.
func (p *inspectPanel) chartInfoText(f flow.Flow, last *flow.Result) string {
	info := p.runText(last) + fmt.Sprintf("  zoom %d/%d", p.zoom+1, len(boxWidths))
	if _, running := flow.ReadProgress(flow.ResultsDir(), f.Name); running && !f.Function {
		info = "running now ▶" + fmt.Sprintf("  zoom %d/%d", p.zoom+1, len(boxWidths))
	}
	if f.Function {
		info = p.usedByText(f) + fmt.Sprintf("  zoom %d/%d", p.zoom+1, len(boxWidths))
	}
	if f.Timeout != "" {
		info += "  timeout " + f.Timeout
	}
	if triggers := triggersText(f, flow.Enabled(flow.TriggersFile())[f.Name]); triggers != "" {
		info += "  " + triggers
	}
	return info
}

// usedByText tells who calls a function, which a change to it reaches.
func (p *inspectPanel) usedByText(f flow.Flow) string {
	callers := flow.Callers(p.entries(), f.Name)
	if len(callers) == 0 {
		return "function · nothing calls it yet"
	}
	return "function · used by " + strings.Join(callers, ", ")
}

// layoutFlowChart builds the chart, moving a selection that no longer exists to the first step.
func (p *inspectPanel) layoutFlowChart(f flow.Flow, last *flow.Result) {
	progress, running := flow.ReadProgress(flow.ResultsDir(), f.Name)
	if f.Function {
		running = false
	}
	if running {
		last = nil // the marks of the run before would mix with this one's
	}
	if inside := strings.Index(progress.Path, "/call/"); inside >= 0 {
		progress.Path = progress.Path[:inside] // a function's step runs inside its call's card
	}
	style := chartStyle{pal: &p.app.palette, zoom: p.zoom, selected: p.selected, describe: p.app.scriptDescription, result: last, ranged: p.ranged(), moving: p.moving, carried: p.carried(), running: progress.Path}
	style.warned = map[string]bool{}
	for _, w := range p.warnings {
		style.warned[w.Path] = true
	}
	p.chart = buildChart(f, style)
	if _, ok := p.chart.block(p.selected); !ok && len(p.chart.blocks) > 0 {
		p.selected = p.chart.firstStep().key
		style.selected = p.selected
		p.chart = buildChart(f, style)
	}
	if running {
		p.chart.drawFlowing(&p.app.palette, progress.Path, progress.Since, p.app.now())
		p.app.keepFlowing()
	}
	if gap, ok := p.dropGap(); ok {
		step, _ := f.Step(p.drag.card)
		p.chart.drawGhost(&p.app.palette, gap, step.Label())
	}
}

// runText tells which run the chart shows and how it went.
func (p *inspectPanel) runText(r *flow.Result) string {
	which := "last run"
	if p.run != nil {
		which = "run of"
	}
	switch {
	case r == nil:
		return "never run; r runs it"
	case r.OK:
		return fmt.Sprintf("%s %s ✓ in %s", which, r.Start.Format("Jan 2 15:04"), r.End.Sub(r.Start).Round(time.Millisecond))
	}
	return fmt.Sprintf("%s %s ✗ exit %d", which, r.Start.Format("Jan 2 15:04"), r.Exit)
}

// RowAt is the row of the flows' list under point, which a click selects.
func (p *inspectPanel) RowAt(point image.Point) (image.Rectangle, bool, bool) {
	_, selected, _ := p.current()
	return listRowAt(p.list, point, 0, len(p.entries()), selected)
}

// WantsMouseAt takes the mouse over the list and the chart.
func (p *inspectPanel) WantsMouseAt(point image.Point) bool {
	return point.In(p.list) || point.In(p.chartArea) || point.In(p.details)
}

// SendMouse selects the flow or the block clicked; a double click on a step tells what it does.
// Dragging the chart moves it, and the wheel scrolls it (with shift, across).
func (p *inspectPanel) SendMouse(event uv.MouseEvent) {
	m := event.Mouse()
	point := image.Pt(m.X, m.Y)
	switch event.(type) {
	case uv.MouseClickEvent:
		if m.Button != uv.MouseLeft {
			return
		}
		if point.In(p.list) {
			p.clickFlow(point.Y - p.list.Min.Y)
			return
		}
		if point.In(p.details) {
			p.focusChart = true
			return
		}
		p.drag = &chartDrag{from: point, pan: p.pan, over: point}
		if block, ok := p.blockAt(point); ok && !block.isInsertion() && !block.params && m.Mod&(uv.ModAlt|uv.ModCtrl) != 0 {
			p.drag.card = block.path
		}
	case uv.MouseMotionEvent:
		if p.drag == nil {
			return
		}
		p.drag.moved, p.drag.over = true, point
		if p.drag.card == "" {
			p.pan, p.centered = p.clampPan(p.drag.pan.Sub(point.Sub(p.drag.from))), false
			return
		}
		p.centered = false // the details go while a card is carried: the chart stays put
		p.panAtEdge(point)
	case uv.MouseReleaseEvent:
		switch {
		case p.drag == nil:
		case !p.drag.moved:
			p.clickChart(point)
		case p.drag.card != "":
			p.dropCard(p.drag.card, point)
		}
		p.drag = nil
	case uv.MouseWheelEvent:
		if point.In(p.details) {
			p.scrollDetails(map[uv.MouseButton]int{uv.MouseWheelUp: -1, uv.MouseWheelDown: 1}[m.Button])
			return
		}
		p.wheel(m)
	}
}

func (p *inspectPanel) wheel(m uv.Mouse) {
	heading := map[uv.MouseButton]layout.Heading{uv.MouseWheelUp: layout.Up, uv.MouseWheelDown: layout.Down}[m.Button]
	if m.Mod&uv.ModShift != 0 {
		heading = map[layout.Heading]layout.Heading{layout.Up: layout.Left, layout.Down: layout.Right}[heading]
	}
	if m.Mod&uv.ModCtrl != 0 {
		p.setZoom(p.zoom + map[layout.Heading]int{layout.Up: 1, layout.Down: -1}[heading])
		return
	}
	p.panBy(heading)
}

func (p *inspectPanel) clickFlow(index int) {
	entries := p.entries()
	if index < 0 || index >= len(entries) {
		return
	}
	p.flowName, p.focusChart, p.selected, p.centered, p.anchor, p.run = entryKey(entries[index]), false, "", true, "", nil
}

// blockAt is the chart's block under point of the panel.
func (p *inspectPanel) blockAt(point image.Point) (chartBlock, bool) {
	if p.chart == nil || !point.In(p.chartArea) {
		return chartBlock{}, false
	}
	return p.chart.blockAt(point.Sub(p.chartArea.Min).Add(p.pan))
}

// edgeReach is how near the chart's edge a carried card pans the chart that way.
const edgeReach = 2

// panAtEdge pans the chart toward the edge a carried card is near, to reach what is past it.
func (p *inspectPanel) panAtEdge(point image.Point) {
	area, step := p.chartArea, image.Point{}
	switch {
	case point.Y < area.Min.Y+edgeReach:
		step.Y = -1
	case point.Y >= area.Max.Y-edgeReach:
		step.Y = 1
	}
	switch {
	case point.X < area.Min.X+edgeReach:
		step.X = -2
	case point.X >= area.Max.X-edgeReach:
		step.X = 2
	}
	if step != (image.Point{}) {
		p.pan, p.centered = p.clampPan(p.pan.Add(step)), false
	}
}

// carried is the path of the step whose card the mouse carries, or "".
func (p *inspectPanel) carried() string {
	if p.drag == nil || !p.drag.moved {
		return ""
	}
	return p.drag.card
}

// dropGap is the place the carried card would drop at, nearest the mouse.
func (p *inspectPanel) dropGap() (chartGap, bool) {
	if p.carried() == "" || p.chart == nil || !p.drag.over.In(p.chartArea) {
		return chartGap{}, false
	}
	return p.chart.dropAt(p.drag.over.Sub(p.chartArea.Min).Add(p.pan), p.drag.card)
}

// dropCard puts the carried step at the place its dashed outline shows; let go elsewhere, it
// stays where it was.
func (p *inspectPanel) dropCard(path string, point image.Point) {
	p.drag.over = point
	f, _, ok := p.current()
	gap, on := p.dropGap()
	if !ok || !on {
		return
	}
	p.app.dropStep(f, path, gap.slot.parent, gap.slot.branch, gap.index, p.selectMoved)
}

func (p *inspectPanel) clickChart(point image.Point) {
	p.focusChart = true
	if p.chart == nil {
		return
	}
	block, ok := p.chart.blockAt(point.Sub(p.chartArea.Min).Add(p.pan))
	if !ok {
		return
	}
	again := block.key == p.clickedKey && time.Since(p.lastClick) < doubleClick
	p.selected, p.clickedKey, p.lastClick, p.anchor = block.key, block.key, time.Now(), ""
	if f, _, ok := p.current(); ok && again && !block.isInsertion() {
		p.app.aboutStep(f, block.path, p.result(f))
		p.clickedKey = ""
	}
}

func (p *inspectPanel) Resize(pty.Size) {}
func (p *inspectPanel) Paste(string)    {}
func (p *inspectPanel) Close()          {}

func (p *inspectPanel) Cursor() (uv.Position, bool) {
	return image.Point{}, false
}

func (p *inspectPanel) Title() string {
	return "inspect"
}
