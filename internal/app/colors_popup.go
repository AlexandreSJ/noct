package app

import (
	"fmt"
	"image"
	"image/color"
	"math"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/theme"
)

const colorsHints = "{up}{down} select  {enter} pick a color  esc close"

// Contrast below these reads poorly, as WCAG 2 has it: text, and borders and titles.
const (
	minTextContrast  = 4.5
	minFrameContrast = 3
)

// colorsPopup gives the focused pane and its tab colors of their own.
type colorsPopup struct {
	tab      *tab
	pane     layout.PaneID
	rows     []colorRow
	selected int
	picker   *colorPicker
}

// colorRow is a group's title, or a component of the pane or of the tab.
type colorRow struct {
	label     string
	heading   bool
	component theme.Component
	forTab    bool
}

func (a *App) openColors() {
	t := a.currentTab()
	rows := []colorRow{{label: "Panel", heading: true}}
	for _, c := range paneComponents {
		rows = append(rows, colorRow{label: theme.Components[c].Label, component: c})
	}
	rows = append(rows, colorRow{label: "Tab", heading: true})
	for _, c := range tabComponents {
		rows = append(rows, colorRow{label: theme.Components[c].Label, component: c, forTab: true})
	}
	a.colors = colorsPopup{tab: t, pane: t.focused, rows: rows, selected: 1}
	a.mode = colorsMode
}

// set is the colors the row changes: the pane's or the tab's, made when there are none yet.
func (p *colorsPopup) set(row colorRow) colorSet {
	if row.forTab {
		if p.tab.colors == nil {
			p.tab.colors = colorSet{}
		}
		return p.tab.colors
	}
	if p.tab.paneColors[p.pane] == nil {
		p.tab.paneColors[p.pane] = colorSet{}
	}
	return p.tab.paneColors[p.pane]
}

func (a *App) handleColorsKey(key uv.KeyPressEvent) {
	p := &a.colors
	if p.picker != nil {
		p.picker.handleKey(key)
		return
	}
	switch key.String() {
	case "esc", "q":
		a.mode = normalMode
	case "up", "k":
		p.move(-1)
	case "down", "j":
		p.move(1)
	case "enter", "space", "right", "l":
		row := p.rows[p.selected]
		current, ok := p.set(row)[row.component]
		if !ok {
			current = a.themeNamed(a.sessionTheme(a.session)).Colors[row.component]
		}
		p.picker = newColorPicker(current, true, func(c theme.Color, ok bool) {
			p.picker = nil
			switch {
			case !ok:
			case c == "":
				delete(p.set(row), row.component)
			default:
				p.set(row)[row.component] = c
			}
		})
	}
}

// move steps the selection over the rows, past the titles.
func (p *colorsPopup) move(step int) {
	for i := p.selected + step; i >= 0 && i < len(p.rows); i += step {
		if !p.rows[i].heading {
			p.selected = i
			return
		}
	}
}

// colorsPreview is the pane's and the tab's colors with the color under the picker's selection.
func (a *App) colorsPreview() (pane, tab colorSet) {
	p := &a.colors
	pane, tab = colorSet{}, colorSet{}
	for c, v := range p.tab.paneColors[p.pane] {
		pane[c] = v
	}
	for c, v := range p.tab.colors {
		tab[c] = v
	}
	if p.picker != nil {
		row := p.rows[p.selected]
		target := pane
		if row.forTab {
			target = tab
		}
		if c := p.picker.candidate(); c == "" {
			delete(target, row.component)
		} else {
			target[row.component] = c
		}
	}
	return pane, tab
}

// colorsWarning tells when the pane's own colors read poorly: its text, borders or titles on its
// background. Pairs where neither color is the pane's own are the theme's business.
func (a *App) colorsWarning(t *tab, id layout.PaneID) string {
	own := t.paneColors[id]
	p := a.palette.forPane(own)
	th := a.themeNamed(a.sessionTheme(a.session))
	background, text := p.background, p.text
	if background == nil {
		background = th.Colors[theme.Background].Resolve() // what the theme would paint
	}
	if text == nil {
		text = th.Colors[theme.Text].Resolve()
	}
	_, ownBackground := own[theme.Background]
	worst, limit := math.Inf(1), 1.0
	check := func(c color.Color, owned bool, least float64) {
		if ratio := contrast(c, background); c != nil && (owned || ownBackground) && ratio < least && ratio/least < worst/limit {
			worst, limit = ratio, least
		}
	}
	_, ownText := own[theme.Text]
	check(text, ownText, minTextContrast)
	for c, s := range map[theme.Component]uv.Style{theme.FocusedBorder: p.focusedFrame, theme.UnfocusedBorder: p.unfocusedFrame, theme.FocusedTitle: p.focusedTitle, theme.UnfocusedTitle: p.unfocusedTitle} {
		_, owned := own[c]
		check(s.Fg, owned, minFrameContrast)
	}
	if math.IsInf(worst, 1) {
		return ""
	}
	return fmt.Sprintf("low contrast: %.1f:1", worst)
}

// contrast is the WCAG 2 contrast ratio of two colors, from 1 to 21.
func contrast(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	channel := func(v uint32) float64 {
		s := float64(v) / 0xffff
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

// drawColors draws the list on the left and the stage on the right, with the colors in the
// making, and the contrast warning under the list.
func (a *App) drawColors(screen uv.Screen) {
	p := &a.colors
	height := panelHeight(len(p.rows) + 3)
	inner := a.drawPanel(screen, centeredRect(image.Rect(0, 0, a.size.X, a.size.Y), themePanelWidth, max(height, stageMinHeight+4)), "colors of the panel and tab")
	list := inner
	list.Max.X = min(inner.Min.X+themeListWidth, inner.Max.X)
	pane, tab := a.colorsPreview()
	if p.picker != nil {
		p.picker.draw(screen, image.Rect(list.Min.X+1, list.Min.Y, list.Max.X, list.Max.Y-2), &a.palette)
	} else {
		for i, row := range p.rows {
			y := list.Min.Y + i
			if y >= list.Max.Y-3 {
				break
			}
			value := "theme"
			set := pane
			if row.forTab {
				set = tab
			}
			if c, ok := set[row.component]; ok {
				value = swatchValue(c)
			}
			switch {
			case row.heading:
				drawRow(screen, list, y, row.label, "", a.palette.panelHeading)
			case i == p.selected:
				drawRow(screen, list, y, "  "+row.label, value, a.palette.selectedRow)
			default:
				drawRow(screen, list, y, "  "+row.label, value, a.palette.panelText)
			}
			if c, ok := set[row.component]; ok && !row.heading {
				x := list.Max.X - 1 - len([]rune(value))
				style := a.palette.panelText
				if i == p.selected {
					style = a.palette.selectedRow
				}
				style.Fg = c.Resolve()
				drawText(screen, x, y, list.Max.X, "██", style)
			}
		}
		drawText(screen, list.Min.X+1, inner.Max.Y-3, list.Max.X, a.colorsWarning(p.tab, p.pane), a.palette.status)
	}
	for y := inner.Min.Y; y < inner.Max.Y-2; y++ {
		setRune(screen, list.Max.X, y, "│", a.palette.unfocusedFrame)
	}
	base := newPalette(a.themeNamed(a.sessionTheme(a.session)), true)
	pal := base.forPane(pane)
	pal.activeTab, pal.tab = base.forTab(tab)
	drawStage(screen, image.Rect(list.Max.X+2, inner.Min.Y, inner.Max.X, inner.Max.Y-2), &pal, theme.Background, false)
	hints := colorsHints
	if p.picker != nil {
		hints = p.picker.hints()
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("config", hints), a.palette.panelMuted)
}
