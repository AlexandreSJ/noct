package app

import (
	"errors"
	"image"
	"path/filepath"
	"strings"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/templates"
)

const templatesHints = "{up}{down} select  {enter} open  e edit  r rename  d delete  esc close"

// templatesListWidth is the width of the list of templates; the layout of the selected one
// takes the rest.
const templatesListWidth = 30

// templatesPopup is the popup of prefix + T: the templates, and the layout of the selected one.
type templatesPopup struct {
	list     []templates.Entry
	selected int
	// first is the first template shown when the list does not fit.
	first int
}

func (a *App) openTemplates() {
	dir, err := templates.Dir()
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	list := templates.List(dir)
	a.templates = templatesPopup{list: list, selected: min(a.templates.selected, max(len(list)-1, 0))}
	a.mode = templatesMode
}

func (a *App) handleTemplatesKey(key uv.KeyPressEvent) {
	p := &a.templates
	switch key.String() {
	case "esc", "q":
		a.mode = normalMode
		return
	case "up", "k":
		p.selected = max(p.selected-1, 0)
		return
	case "down", "j":
		p.selected = min(p.selected+1, max(len(p.list)-1, 0))
		return
	}
	if len(p.list) == 0 {
		return
	}
	dir, err := templates.Dir()
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	name := p.list[p.selected].Name
	switch key.String() {
	case "enter":
		t, err := templates.Load(dir, name) // read again: the file may have changed since the list
		if err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.mode = normalMode
		if err := a.openTemplate(t); err != nil {
			a.statusMessage = err.Error()
		}
	case "e":
		a.mode = normalMode
		a.openEditor(filepath.Join(dir, name+".yaml"), "template-"+name, func() {})
	case "r":
		a.openPrompt("rename template", name, "{enter} save  esc cancel", func(to string) {
			if err := templates.Rename(dir, name, to); err != nil {
				a.statusMessage = err.Error()
			}
			a.openTemplates()
		})
		a.prompt.cancel = a.openTemplates
	case "d":
		a.askToConfirm("delete template "+name+"?", func() {
			if err := templates.Delete(dir, name); err != nil {
				a.statusMessage = err.Error()
			}
			a.openTemplates()
		})
		a.confirm.cancel = a.openTemplates
	}
}

// openTemplate opens t as a new tab at the end. Its terminals start where a terminal opened from
// the focused pane would; a pane that cannot start is left out, and told.
func (a *App) openTemplate(t templates.Template) error {
	dir := a.terminalDir(a.currentTab())
	shape, panes := t.Root.Shape(a.newPaneID)
	tree, err := layout.FromShape(shape, func(id layout.PaneID) layout.PaneID { return id })
	if err != nil {
		return err
	}
	tb := newEmptyTab()
	tb.tree, tb.name, tb.colors = tree, t.Tab, colorSetFrom(t.Colors)
	area := a.workspaceArea()
	var problems []error
	for _, id := range tree.Panes() {
		p := panes[id]
		v, err := a.startTemplatePane(id, area, p, dir)
		if err != nil {
			problems = append(problems, err)
		}
		if v == nil {
			continue // a shell that started keeps its pane, even when its command was left out
		}
		tb.views[id] = v
		if p.Name != "" {
			tb.paneNames[id] = p.Name
		}
		if colors := colorSetFrom(p.Colors); len(colors) > 0 {
			tb.paneColors[id] = colors
		}
	}
	for _, id := range tree.Panes() {
		if _, ok := tb.views[id]; !ok {
			tb.removeView(id)
		}
		if tb.isEmpty() {
			return errors.Join(append(problems, errors.New("nothing in template "+t.Name+" could open"))...)
		}
	}
	tb.focused = tb.tree.Panes()[0]
	a.session.tabs = append(a.session.tabs, tb)
	a.session.activeTab = len(a.session.tabs) - 1
	tb.resizeViews(area)
	return errors.Join(problems...)
}

// startTemplatePane starts one pane of a template, and its terminal's command as it says.
func (a *App) startTemplatePane(id layout.PaneID, area image.Rectangle, p templates.Pane, dir string) (view, error) {
	switch p.Kind {
	case "scripts":
		return a.startScripts(id, area)
	case "inspect":
		return a.startInspect(id, area)
	case "logs":
		return a.startLogs(id, area)
	case "terminal":
	default:
		return nil, errors.New("a template cannot open a " + p.Kind + " pane")
	}
	v, err := a.startShell(id, area, dir)
	if err != nil || p.Command == "" {
		return v, err
	}
	if strings.ContainsFunc(p.Command, unicode.IsControl) {
		return v, errors.New("left out a command with a line break: " + strings.ReplaceAll(p.Command, "\n", " "))
	}
	switch p.Start {
	case templates.Run:
		typeInView(v, p.Command, true)
	case templates.Ask:
		a.shells[id].rerun = p.Command
	}
	return v, nil
}

// drawLayoutStage draws a layout in area: a frame per pane, with its name and its own colors.
func drawLayoutStage(screen uv.Screen, area image.Rectangle, p *palette, shape layout.Shape, names map[layout.PaneID]string, colors map[layout.PaneID]colorSet) {
	tree, err := layout.FromShape(shape, func(id layout.PaneID) layout.PaneID { return id })
	if err != nil {
		return
	}
	for id, frame := range tree.Rects(area) {
		pal := p.forPane(colors[id])
		fillRect(screen, frame, uv.Style{Bg: pal.background})
		drawFrame(screen, frame, names[id], pal.unfocusedFrame, pal.unfocusedTitle)
	}
}

func (a *App) drawTemplates(screen uv.Screen) {
	p := &a.templates
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, themePanelWidth, max(panelHeight(len(p.list)+2), 20)), "templates")
	if len(p.list) == 0 {
		drawText(screen, inner.Min.X+1, inner.Min.Y, inner.Max.X, "no templates yet: "+a.keysFor("R")+" saves the tab as one", a.palette.panelMuted)
		drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, "esc close", a.palette.panelMuted)
		return
	}
	list := inner
	list.Max.X = inner.Min.X + templatesListWidth
	rows := max(list.Dy()-2, 1)
	p.first = listFirst(p.first, p.selected, len(p.list), rows)
	for i := p.first; i < len(p.list) && i-p.first < rows; i++ {
		e := p.list[i]
		y := list.Min.Y + i - p.first
		style, value := a.palette.panelText, ""
		if e.Err != nil {
			style, value = a.palette.panelMuted, "cannot read"
		}
		if i == p.selected {
			style = a.palette.selectedRow
		}
		drawRow(screen, list, y, e.Name, value, style)
	}
	for y := inner.Min.Y; y < inner.Max.Y-2; y++ {
		setRune(screen, list.Max.X, y, "│", a.palette.unfocusedFrame)
	}
	stage := image.Rect(list.Max.X+2, inner.Min.Y, inner.Max.X, inner.Max.Y-2)
	if selected := p.list[p.selected]; selected.Err != nil {
		for i, line := range wrapText(selected.Err.Error(), max(stage.Dx(), 1)) {
			drawText(screen, stage.Min.X, stage.Min.Y+i, stage.Max.X, line, a.palette.status)
		}
	} else {
		a.drawTemplateLayout(screen, stage, selected.Template)
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("prompt", templatesHints), a.palette.panelMuted)
}

// drawTemplateLayout draws t's layout in area, each pane with its name, or its kind, and colors.
func (a *App) drawTemplateLayout(screen uv.Screen, area image.Rectangle, t templates.Template) {
	next := layout.PaneID(0)
	shape, panes := t.Root.Shape(func() layout.PaneID { next++; return next })
	names, colors := map[layout.PaneID]string{}, map[layout.PaneID]colorSet{}
	for id, pane := range panes {
		names[id] = pane.Name
		if names[id] == "" {
			names[id] = pane.Kind
		}
		colors[id] = colorSetFrom(pane.Colors)
	}
	drawLayoutStage(screen, area, &a.palette, shape, names, colors)
}
