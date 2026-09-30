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
	flowsListHints = "{up}{down} flow  {enter} open  {a} new  {r} run  {T} add trigger  {o} triggers on/off  {P} promote draft  {E} edit file  {delete} delete  {tab} hide list  {ctrl+d} close"
	flowStepHints  = "{up}{down}{left}{right} select  {shift+up}{shift+down} range  {a} add after  {t} test  {r} run  {s} scope  {enter} about  {e} edit  {delete} remove  {J} {K} move  {+} {-} zoom  {ctrl+up}{ctrl+down}{ctrl+left}{ctrl+right} pan  {0} center  {tab} list"
	flowGapHints   = "{up}{down}{left}{right} select  {a} add here  {r} run  {s} scope  {+} {-} zoom  {ctrl+up}{ctrl+down}{ctrl+left}{ctrl+right} pan  {0} center  {tab} list"
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
	// listHidden gives the chart the whole panel.
	listHidden bool
	// anchor is the step a range started at with shift, or "" for the selected step alone.
	anchor    string
	showScope bool
	// warnings are the flow's, found when it is drawn.
	warnings []flow.Warning
	// run is a run opened from the logs, shown instead of the flow's last one.
	run *flow.Result
	// selected is the key of the chart's selected block.
	selected string
	zoom     int
	// pan is the chart's point at the chart area's top left; while centered, it follows the
	// selection instead.
	pan      image.Point
	centered bool

	chart      *flowChart
	list       image.Rectangle
	chartArea  image.Rectangle
	drag       *chartDrag
	lastClick  time.Time
	clickedKey string
}

// chartDrag is the chart being moved with the mouse.
type chartDrag struct {
	from  image.Point
	pan   image.Point
	moved bool
}

func (a *App) startInspect(id layout.PaneID, _ image.Rectangle) (view, error) {
	a.reloadScripts()
	a.reloadFlows()
	p := &inspectPanel{app: a, close: func() { a.detachView(id) }, zoom: 1, centered: true}
	if len(a.flows) > 0 {
		p.flowName = a.flows[0].Name
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
	if p.run != nil {
		return p.run
	}
	if r, ran := flow.LastResult(flow.ResultsDir(), f.Name); ran {
		return &r
	}
	return nil
}

// current is the selected flow, following it by name as the list is read again.
func (p *inspectPanel) current() (flow.Flow, int, bool) {
	flows := p.app.flows
	if len(flows) == 0 {
		return flow.Flow{}, 0, false
	}
	i := max(slices.IndexFunc(flows, func(f flow.Flow) bool { return f.Name == p.flowName }), 0)
	p.flowName = flows[i].Name
	return flows[i], i, true
}

func (p *inspectPanel) selectFlow(step int) {
	_, i, ok := p.current()
	if !ok {
		return
	}
	i = min(max(i+step, 0), len(p.app.flows)-1)
	p.flowName, p.selected, p.centered, p.anchor, p.run = p.app.flows[i].Name, "", true, "", nil
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
	}
	if !ok || !p.focusChart && name == "a" {
		p.listKey(name, f)
		return
	}
	switch name {
	case "r":
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
	case "up", "k":
		p.selectFlow(-1)
	case "down", "j":
		p.selectFlow(1)
	case "pgup", "pgdown", "home", "end":
		if _, i, ok := p.current(); ok {
			to, _ := listJump(name, i, len(p.app.flows), max(p.list.Dy()-2, 1)) // inside the frame
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
		p.app.askToConfirm("delete flow "+f.Name+"?", func() {
			if err := f.Delete(); err != nil {
				p.app.statusMessage = err.Error()
			}
			p.app.reloadFlows()
		})
	}
}

func (p *inspectPanel) chartKey(name string, f flow.Flow) {
	block, selected := p.selectedBlock()
	switch name {
	case "esc":
		if p.anchor != "" {
			p.anchor = ""
			return
		}
		p.focusChart, p.listHidden = false, false
	case "shift+up", "shift+down":
		if selected && !block.isInsertion() {
			p.extendRange(f, block.path, map[string]int{"shift+up": -1, "shift+down": 1}[name])
		}
	case "t":
		p.testSelection(f, block, selected)
	case "s":
		p.showScope = !p.showScope
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
	case "enter", "space":
		if selected && !block.isInsertion() {
			p.app.aboutStep(f, block.path, p.result(f))
		}
	case "a":
		if selected {
			p.app.addStep(f, block, func(path string) { p.selected = stepKey(path) })
		}
	default:
		if selected && !block.isInsertion() {
			p.stepKey(name, f, block.path)
		}
	}
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
		p.app.moveStep(f, path, 1, func(moved string) { p.selected = stepKey(moved) })
	case "K", "shift+k":
		p.app.moveStep(f, path, -1, func(moved string) { p.selected = stepKey(moved) })
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

// testSelection tests the selected step, or the range.
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
	drawText(screen, area.Min.X+1, area.Max.Y-1, area.Max.X, p.hints(), pal.panelMuted)
	body := image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Max.Y-1)
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
		p.chart, p.chartArea = nil, image.Rectangle{}
		return
	}
	p.drawFlowChart(screen, chartArea, f)
	p.chartArea = p.chartArea.Sub(area.Min)
}

func (p *inspectPanel) hints() string {
	if !p.focusChart {
		return p.app.hint("flows", flowsListHints)
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
	pal := &p.app.palette
	line, title := p.frameStyles(!p.focusChart)
	drawFrame(screen, area, "flows", line, title)
	inner := area.Inset(1)
	_, selected, _ := p.current()
	enabled := flow.Enabled(flow.TriggersFile())
	for i, f := range p.app.flows {
		y := inner.Min.Y + i
		if y >= inner.Max.Y {
			break
		}
		style := pal.panelText
		if i == selected {
			style = pal.selectedRow
			if p.focusChart {
				style = pal.focusedTitle
			}
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
		drawRow(screen, inner, y, mark+f.Name, "", style)
	}
	return inner
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
	info := p.runText(last) + fmt.Sprintf("  zoom %d/%d", p.zoom+1, len(boxWidths))
	if triggers := triggersText(f, flow.Enabled(flow.TriggersFile())[f.Name]); triggers != "" {
		info += "  " + triggers
	}
	drawText(screen, area.Min.X+1, area.Min.Y, area.Max.X, info, pal.panelMuted)
	canvasArea := image.Rect(area.Min.X, area.Min.Y+1, area.Max.X, area.Max.Y)
	if f.Problem != nil {
		drawText(screen, area.Min.X+1, canvasArea.Min.Y, area.Max.X, "✗ "+f.Problem.Error(), pal.status)
		canvasArea.Min.Y += 2
	}
	style := chartStyle{pal: pal, zoom: p.zoom, selected: p.selected, describe: p.app.scriptDescription, result: last, ranged: p.ranged()}
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
	p.chartArea = canvasArea
	if p.centered {
		p.centerPan()
	}
	p.chart.blit(screen, canvasArea, p.pan)
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

// WantsMouseAt takes the mouse over the list and the chart.
func (p *inspectPanel) WantsMouseAt(point image.Point) bool {
	return point.In(p.list) || point.In(p.chartArea)
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
		p.drag = &chartDrag{from: point, pan: p.pan}
	case uv.MouseMotionEvent:
		if p.drag != nil {
			p.drag.moved = true
			p.pan, p.centered = p.clampPan(p.drag.pan.Sub(point.Sub(p.drag.from))), false
		}
	case uv.MouseReleaseEvent:
		if p.drag != nil && !p.drag.moved {
			p.clickChart(point)
		}
		p.drag = nil
	case uv.MouseWheelEvent:
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
	if index < 0 || index >= len(p.app.flows) {
		return
	}
	p.flowName, p.focusChart, p.selected, p.centered, p.anchor, p.run = p.app.flows[index].Name, false, "", true, "", nil
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
