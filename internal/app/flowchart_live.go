package app

import (
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// While a flow runs, its chart shows where: the step running has its card lit and marked ▶, and
// three dots of different sizes and colors go down the arrow out of it, over and over, toward
// what runs next.

// flowingStep is how often the dots move a row.
const flowingStep = 110 * time.Millisecond

// flowingDots are the dots from the one in front to the last; flowingGap is how many rows the
// line stays empty before they come round again.
var flowingDots = []string{"●", "•", "∙"}

const flowingGap = 2

// drawFlowing sends the dots down the arrow out of the running step, as they are at.
func (c *flowChart) drawFlowing(pal *palette, path string, since, at time.Time) {
	arrow, ok := c.arrows[path]
	if !ok {
		return
	}
	colors := []uv.Style{pal.focusedTitle, pal.success, pal.status}
	frame := int(at.Sub(since) / flowingStep)
	cycle := arrow.rows + flowingGap
	for k, dot := range flowingDots {
		row := ((frame-k)%cycle + cycle) % cycle
		if row >= arrow.rows {
			continue
		}
		style := colors[k%len(colors)]
		style.Bg = pal.panelText.Bg
		setRune(c.canvas, arrow.x, arrow.top+row, dot, style)
	}
}

// keepFlowing asks for the dots' next frame while a run shows.
func (a *App) keepFlowing() {
	if a.flowTick == nil {
		a.flowTick = time.After(flowingStep)
	}
}
