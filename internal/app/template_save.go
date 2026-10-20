package app

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/templates"
)

const templateSaveHints = "{up}{down} select  {left}{right} start  {enter} save  esc cancel"

// starts are the ways a terminal's command starts, as left and right cycle through them.
var starts = []templates.Start{templates.Ask, templates.None, templates.Run}

// templateSave is the popup of prefix + R: the template's name, then the panes it keeps.
type templateSave struct {
	tab  *tab
	name textField
	// panes are the ones a template can open again; selected 0 is the name, 1 on the panes.
	panes    []savePane
	selected int
}

type savePane struct {
	id                  layout.PaneID
	kind, name, command string
	start               templates.Start
}

func (a *App) openTemplateSave() {
	t := a.currentTab()
	s := templateSave{tab: t, name: newTextField(t.name)}
	for _, id := range t.tree.Panes() {
		p := savePane{id: id, name: t.paneNames[id]}
		switch v := t.views[id].(type) {
		case *scriptsPanel:
			p.kind = "scripts"
		case *inspectPanel:
			p.kind = "inspect"
		case *logsPanel:
			p.kind = "logs"
		default:
			sh := a.shells[id]
			if sh == nil {
				continue // runs, editors and help end with what they were for
			}
			p.kind, p.command = "terminal", sh.rerun
			if teller, ok := v.(commandTeller); ok && teller.ForegroundCommand() != "" {
				p.command = teller.ForegroundCommand()
			}
			if strings.ContainsFunc(p.command, unicode.IsControl) {
				p.command = "" // typed as keys, a control character would be a key
			}
			if p.command != "" {
				p.start = templates.Ask
			}
		}
		s.panes = append(s.panes, p)
	}
	a.templateSave = s
	a.mode = templateSaveMode
}

func (a *App) handleTemplateSaveKey(key uv.KeyPressEvent) {
	s := &a.templateSave
	switch key.String() {
	case "esc":
		a.mode = normalMode
	case "up", "shift+tab":
		s.selected = max(s.selected-1, 0)
	case "down", "tab":
		s.selected = min(s.selected+1, len(s.panes))
	case "enter":
		a.saveTemplate()
	case "left", "right":
		if s.selected == 0 {
			s.name.handleKey(key)
			return
		}
		p := &s.panes[s.selected-1]
		if p.start == "" {
			return
		}
		step := 1
		if key.String() == "left" {
			step = -1
		}
		i := slices.Index(starts, p.start)
		p.start = starts[(i+step+len(starts))%len(starts)]
	default:
		if s.selected == 0 {
			a.typeKey(key)
		}
	}
}

// saveTemplate writes the template, asking first when one has the name.
func (a *App) saveTemplate() {
	s := &a.templateSave
	name := strings.TrimSpace(s.name.String())
	dir, err := templates.Dir()
	if err == nil {
		err = templates.CheckName(name)
	}
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	tpl := a.templateOf(s, name)
	if tpl.Root.Pane == nil && tpl.Root.Columns == nil && tpl.Root.Rows == nil {
		a.statusMessage = "nothing in this tab can open again from a template"
		return
	}
	write := func() {
		if err := templates.Save(dir, tpl); err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.notice = "saved template " + name
	}
	if _, err := os.Stat(filepath.Join(dir, name+".yaml")); errors.Is(err, os.ErrNotExist) {
		a.mode = normalMode
		write()
		return
	}
	a.askToConfirm("replace template "+name+"?", write)
	a.confirm.cancel = func() { a.mode = templateSaveMode }
}

// templateOf is the tab as a template: its layout with the panes kept, their names and colors,
// and the tab's name and colors.
func (a *App) templateOf(s *templateSave, name string) templates.Template {
	panes := map[layout.PaneID]templates.Pane{}
	for _, p := range s.panes {
		panes[p.id] = templates.Pane{Kind: p.kind, Name: p.name, Command: p.command, Start: p.start, Colors: s.tab.paneColors[p.id].saved()}
	}
	return templates.Template{Name: name, Tab: s.tab.name, Colors: s.tab.colors.saved(), Root: templates.FromShape(s.tab.tree.Shape(), panes)}
}

func (a *App) drawTemplateSave(screen uv.Screen) {
	s := &a.templateSave
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, 72, panelHeight(len(s.panes)+4)), "save the tab as a template")
	style := func(row int) uv.Style {
		if row == s.selected {
			return a.palette.selectedRow
		}
		return a.palette.panelText
	}
	drawRow(screen, inner, inner.Min.Y, "name  "+s.name.view(inner.Dx()-10, false), "", style(0))
	for i, p := range s.panes {
		y := inner.Min.Y + 2 + i
		label := "  " + p.kind
		if p.name != "" {
			label += " " + p.name
		}
		if p.command != "" {
			label += "  $ " + p.command
		}
		value := ""
		if p.start != "" {
			value = "◂ " + string(p.start) + " ▸"
		}
		drawRow(screen, inner, y, tail(label, inner.Dx()-len([]rune(value))-3), value, style(i+1))
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("prompt", templateSaveHints), a.palette.panelMuted)
}
