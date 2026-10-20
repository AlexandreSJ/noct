package app

import (
	"image"
	"slices"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/theme"
)

// The stage is a small made-up workspace: a tab bar and two panels holding a bit of everything a
// theme colors, drawn where a theme is being chosen so its colors show in place.
const (
	stageMinWidth  = 30
	stageMinHeight = 12
)

// drawStage draws the stage in area with p. With blink, the cells of the lit component are
// reversed, to show where it appears. An area too small to read is left alone.
func drawStage(screen uv.Screen, area image.Rectangle, p *palette, lit theme.Component, blink bool) {
	if area.Dx() < stageMinWidth || area.Dy() < stageMinHeight {
		return
	}
	style := func(s uv.Style, parts ...theme.Component) uv.Style {
		if blink && slices.Contains(parts, lit) {
			s.Attrs |= uv.AttrReverse
		}
		return s
	}
	x := drawText(screen, area.Min.X, area.Min.Y, area.Max.X, " dev ", style(p.activeTab, theme.ActiveTabText, theme.ActiveTabBackground))
	drawText(screen, x, area.Min.Y, area.Max.X, " logs ", style(p.tab, theme.InactiveTabText, theme.InactiveTabBackground))

	panes := image.Rect(area.Min.X, area.Min.Y+1, area.Max.X, area.Max.Y)
	left, right := panes, panes
	left.Max.X = panes.Min.X + panes.Dx()/2
	right.Min.X = left.Max.X
	background := style(uv.Style{Bg: p.background}, theme.Background)
	fillRect(screen, left.Inset(1), background)
	fillRect(screen, right.Inset(1), background)
	drawFrame(screen, left, "zsh", style(p.focusedFrame, theme.FocusedBorder), style(p.focusedTitle, theme.FocusedTitle))
	drawFrame(screen, right, "logs", style(p.unfocusedFrame, theme.UnfocusedBorder), style(p.unfocusedTitle, theme.UnfocusedTitle))

	text := style(uv.Style{Fg: p.text, Bg: p.background}, theme.Text)
	drawText(screen, left.Min.X+1, left.Min.Y+1, left.Max.X-1, "$ ls", text)
	drawText(screen, left.Min.X+1, left.Min.Y+2, left.Max.X-1, "notes  src", text)
	popup := image.Rect(left.Min.X+2, left.Min.Y+4, left.Max.X-2, left.Min.Y+7)
	popupStyle := style(uv.Style{Bg: p.popupBackground}, theme.PopupBackground)
	fillRect(screen, popup, popupStyle)
	drawText(screen, popup.Min.X+1, popup.Min.Y+1, popup.Max.X-1, "popup", uv.Style{Fg: p.panelText.Fg, Bg: p.popupBackground, Attrs: popupStyle.Attrs})

	inner := right.Inset(1)
	lines := []struct {
		text  string
		style uv.Style
	}{
		{"Heading", style(p.panelHeading, theme.PanelHeading)},
		{"text", style(p.panelText, theme.PanelText)},
		{"muted", style(p.panelMuted, theme.PanelMuted)},
		{"", style(p.selectedRow, theme.SelectedText, theme.SelectedBackground)},
		{"✓ done", style(p.success, theme.Success)},
		{"✗ exit 1", style(p.failure, theme.Error)},
		{"! message", style(p.status, theme.Attention)},
		{"", uv.Style{}},
		{"noct", style(p.logo, theme.Logo)},
	}
	for i, line := range lines {
		y := inner.Min.Y + i
		switch i {
		case 3:
			fillRect(screen, image.Rect(inner.Min.X, y, inner.Max.X, y+1), line.style)
			drawText(screen, inner.Min.X+1, y, inner.Max.X, "selected", line.style)
		case 7:
			for level, c := range p.heat {
				setRune(screen, inner.Min.X+1+2*level, y, "▪", style(uv.Style{Fg: c, Bg: p.background}, theme.Calendar))
			}
		default:
			drawText(screen, inner.Min.X+1, y, inner.Max.X, line.text, line.style)
		}
	}
}
