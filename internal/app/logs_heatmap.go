package app

import (
	"math"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// heatLevel is the color step of a day with n logs, the busiest day having the given count: 0
// for none, then 1 to heatLevels. The square root keeps a few busy days from leaving every other
// one in the first step.
func heatLevel(n, busiest int) int {
	if n <= 0 {
		return 0
	}
	return 1 + min(int(math.Sqrt(float64(n)/float64(max(busiest, 1)))*heatLevels), heatLevels-1)
}

// busiestDay is the most logs a day has.
func busiestDay(stats map[time.Time]dayStats) int {
	busiest := 0
	for _, s := range stats {
		busiest = max(busiest, s.logs)
	}
	return busiest
}

// heatStyle paints a calendar day's number over the color of its logs, in the text color the
// theme uses over that much color. A day without logs keeps the style it has.
func (p *logsPanel) heatStyle(style uv.Style, level int) uv.Style {
	if level == 0 {
		return style
	}
	pal := &p.app.palette
	style.Bg = pal.heat[level-1]
	style.Fg = pal.panelText.Fg
	if level > heatLevels/2 {
		style.Fg = pal.selectedRow.Fg
	}
	return style
}

// drawHeatLegend names the calendar's colors, from a few logs to the most.
func (p *logsPanel) drawHeatLegend(screen uv.Screen, x, y, maxX int) {
	pal := &p.app.palette
	x = drawText(screen, x, y, maxX, "activity less ", pal.panelMuted)
	for _, c := range pal.heat {
		x = drawText(screen, x, y, maxX, "  ", uv.Style{Bg: c})
		x = drawText(screen, x, y, maxX, " ", pal.panelMuted)
	}
	drawText(screen, x, y, maxX, "more", pal.panelMuted)
}
