package app

import (
	"image"
	"strconv"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// scopeWidth is the scope pane beside the chart.
const scopeWidth = 42

// certaintyMarks tell, before a variable, whether it is set where it is read.
var certaintyMarks = map[flow.Certainty]string{flow.Set: "  ", flow.ThisPass: "↻ ", flow.Maybe: "? "}

// scopeAt is what a template reads at a block of the chart: at a step, what it reads when it
// runs; at a gap or an empty list, what a step added there would read.
func (a *App) scopeAt(f flow.Flow, b chartBlock) []flow.Variable {
	parent, branch, index := b.slot.parent, b.slot.branch, b.index
	if !b.isInsertion() {
		parent, branch, index = flow.SplitPath(b.path)
	}
	return f.Scope(parent, branch, index, a.scripts.Find)
}

// variableValue is what a variable was in the flow's last run, or "" when unknown.
func variableValue(f flow.Flow, v flow.Variable, r *flow.Result) string {
	if v.Step == "" && v.Path == "" {
		return f.Params[v.Field].Default
	}
	if r == nil {
		return ""
	}
	step, ran := r.Steps[v.Path]
	if !ran {
		return ""
	}
	switch v.Field {
	case "output":
		first, _, _ := strings.Cut(step.Output, "\n")
		return first
	case "exit":
		return strconv.Itoa(step.Exit)
	case "ok":
		return strconv.FormatBool(step.Status == flow.StatusOK)
	}
	return step.Captures[v.Field]
}

// variableName is a variable as the scope pane and the picker show it: the template without
// its braces.
func variableName(v flow.Variable) string {
	return strings.TrimSuffix(strings.TrimPrefix(v.Template(), "{{"), "}}")
}

// drawScope frames the variables a block reads, with their last values, under the warnings of
// the step selected.
func (p *inspectPanel) drawScope(screen uv.Screen, area image.Rectangle, f flow.Flow, r *flow.Result) {
	pal := &p.app.palette
	drawFrame(screen, area, "scope", pal.unfocusedFrame, pal.panelHeading)
	inner := area.Inset(1)
	block, ok := p.selectedBlock()
	if !ok {
		return
	}
	var lines []styledLine
	for _, w := range p.warnings {
		if w.Path == block.path {
			lines = append(lines, styledLine{"⚠ " + w.Text, pal.status})
		}
	}
	vars := p.app.scopeAt(f, block)
	if len(vars) == 0 {
		lines = append(lines, styledLine{"nothing to read here yet", pal.panelMuted})
	}
	for _, v := range vars {
		text := certaintyMarks[v.Certainty] + variableName(v)
		if value := variableValue(f, v, r); value != "" {
			text += " = " + value
		}
		lines = append(lines, styledLine{text, pal.panelText})
	}
	lines = append(lines, styledLine{"", pal.panelText}, styledLine{"↻ this pass  ? if a branch ran", pal.panelMuted})
	for i, line := range lines {
		if inner.Min.Y+i >= inner.Max.Y {
			break
		}
		drawText(screen, inner.Min.X+1, inner.Min.Y+i, inner.Max.X, line.text, line.style)
	}
}

// pickVariable searches the variables a template prompt can read and types the one chosen;
// closing the search goes back to the prompt.
func (a *App) pickVariable(f flow.Flow, vars []flow.Variable) {
	var last *flow.Result
	if r, ran := flow.LastResult(flow.ResultsDir(), f.Name); ran {
		last = &r
	}
	var items []finderItem
	for _, v := range vars {
		kind := strings.TrimSpace(certaintyMarks[v.Certainty] + variableValue(f, v, last))
		items = append(items, finderItem{label: variableName(v), kind: kind, open: func() {
			a.prompt.text = append(a.prompt.text, []rune(v.Template())...)
			a.mode = promptMode
		}})
	}
	a.openFinderWith("variable", items)
	a.finder.cancel = func() { a.mode = promptMode }
}
