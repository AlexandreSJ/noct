package app

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
)

func TestARunningStepShowsLitWithDotsGoingDownItsArrow(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(120, 40)
	now := time.Now()
	a.clock = func() time.Time { return now }
	saveFlow(t, "ship", "steps:\n  - {id: build, run: make}\n  - {id: deploy, run: echo deploy}\n")
	if err := flow.SaveProgress(flow.ResultsDir(), "ship", "0"); err != nil {
		t.Fatal(err)
	}
	progress, _ := flow.ReadProgress(flow.ResultsDir(), "ship")
	now = progress.Since
	pressDrawn(a, prefixPress, letter('i'), tabPress)
	p := a.currentTab().focusedView().(*inspectPanel)
	if !strings.Contains(screenText(a), "▶ build") || !strings.Contains(screenText(a), "running now") || a.flowTick == nil {
		t.Fatalf("running build: tick %v, screen:\n%s", a.flowTick != nil, screenText(a))
	}
	dots := func() string {
		arrow := p.chart.arrows["0"]
		var column strings.Builder
		for row := range arrow.rows {
			column.WriteString(p.chart.canvas.CellAt(arrow.x, arrow.top+row).Content)
		}
		return column.String()
	}
	first := dots()
	now = now.Add(flowingStep)
	drawnApp(a)
	if second := dots(); second == first || !strings.Contains(first+second, "●") {
		t.Errorf("the arrow out of build was %q, then %q; want dots moving down it", first, second)
	}
	flow.ClearProgress(flow.ResultsDir(), "ship")
	a.flowTick = nil
	drawnApp(a)
	if strings.Contains(screenText(a), "▶") || a.flowTick != nil {
		t.Error("the run ended and the chart still shows it")
	}
}
