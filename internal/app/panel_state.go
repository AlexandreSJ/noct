package app

import (
	"cmp"
	"slices"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

// Editors the inspect can open again, by what opened them.
const (
	editingParams  = "params"
	editingStep    = "step"
	editingAdd     = "add"
	editingExport  = "export"
	editingRetry   = "retry"
	editingReturns = "returns"
)

// draftEditor is an editor whose rows a session keeps as they were typed: newDraftRow makes a
// row past its own, fitDraftRow reshapes a row's cells to the values it holds, and draftApplied
// shows what the values call for once they are all in.
type draftEditor interface {
	draftGrid() *editGrid
	newDraftRow() gridRow
	fitDraftRow(row int)
	draftApplied()
}

func (p *inspectPanel) savedState() *saved.InspectState {
	s := &saved.InspectState{
		Selected: p.selected, Chart: p.focusChart, ListHidden: p.listHidden, Scope: p.showScope,
		HideDetails: p.hideDetails, HelpHidden: p.helpHidden, Zoom: p.zoom,
	}
	switch {
	case p.reopen != nil:
		s.Editing, s.Draft = p.reopen.Editing, p.reopen.Draft
	case p.editor != nil:
		s.Editing = p.editing
		if d, ok := p.editor.(draftEditor); ok {
			s.Draft = d.draftGrid().draft()
		}
	}
	return s
}

func (p *inspectPanel) restoreState(s *saved.InspectState, filter string) {
	p.filter.field = newTextField(filter)
	if s == nil {
		return
	}
	p.selected, p.focusChart, p.listHidden, p.showScope = s.Selected, s.Chart, s.ListHidden, s.Scope
	p.hideDetails, p.helpHidden = s.HideDetails, s.HelpHidden
	p.zoom = min(max(s.Zoom, 0), len(boxWidths)-1)
	if s.Editing != "" {
		p.reopen = s
	}
}

// reopenEditor opens again the editor a session was left in, once the chart has its blocks, with
// what was typed in it; a step gone since leaves the chart as it is.
func (p *inspectPanel) reopenEditor(f flow.Flow) {
	s := p.reopen
	p.reopen = nil
	block, ok := p.selectedBlock()
	onStep := ok && block.path != "" && p.selected == s.Selected
	switch {
	case s.Editing == editingParams:
		p.openParams(f)
	case s.Editing == editingReturns:
		p.editReturns(f)
	case s.Editing == editingAdd && ok && p.selected == s.Selected && !block.params && !block.end:
		p.addStep(f, block)
	case s.Editing == editingStep && onStep:
		p.editStepForm(f, block.path)
	case s.Editing == editingExport && onStep:
		p.exportStep(f, block.path)
	case s.Editing == editingRetry && onStep:
		p.retryStep(f, block.path)
	}
	if d, ok := p.editor.(draftEditor); ok && s.Draft != nil {
		applyDraft(d, s.Draft)
	}
	p.app.notifyScreenChanged()
}

func (g *editGrid) draft() *saved.Draft {
	d := &saved.Draft{Row: g.row, Col: g.col}
	for _, row := range g.rows {
		kept := saved.DraftRow{Key: row.key}
		for _, c := range row.cells {
			cell := saved.DraftCell{Kind: int(c.kind), On: c.on, Items: slices.Clone(c.items), Marked: c.marked, Secret: c.secret, AutoFor: c.autoFor}
			if !c.secret {
				cell.Text = c.field.String()
			}
			kept.Cells = append(kept.Cells, cell)
		}
		d.Rows = append(d.Rows, kept)
	}
	return d
}

// applyDraft puts a draft's values in the editor's rows, as many rows as it had. A cell the
// editor cannot reshape to the draft's kind, as when the step changed since, leaves the rows as
// they opened. A secret's cell keeps what it opened with.
func applyDraft(e draftEditor, d *saved.Draft) {
	g := e.draftGrid()
	opened := g.snapshot()
	rows := g.rows
	for len(rows) < len(d.Rows) {
		rows = append(rows, e.newDraftRow())
	}
	g.rows = rows[:len(d.Rows)]
	for i, row := range d.Rows {
		if len(g.rows[i].cells) != len(row.Cells) {
			g.rows = opened.rows
			return
		}
		g.rows[i].key = row.Key
		fillDraftRow(g.rows[i], row, false)
		e.fitDraftRow(i)
		if !fillDraftRow(g.rows[i], row, true) {
			g.rows = opened.rows
			return
		}
	}
	g.history, g.future = append(g.history, opened), nil
	e.draftApplied()
	g.focusNear(min(max(d.Row, 0), len(g.rows)+len(g.buttons)-1), d.Col)
}

// fillDraftRow sets the cells of row whose kind is the draft's; all tells every cell must be.
func fillDraftRow(row gridRow, d saved.DraftRow, all bool) bool {
	for j, dc := range d.Cells {
		c := &row.cells[j]
		if c.kind != cellKind(dc.Kind) {
			if all {
				return false
			}
			continue
		}
		c.on, c.items, c.marked, c.autoFor = dc.On, slices.Clone(dc.Items), dc.Marked, dc.AutoFor
		if !dc.Secret && !c.secret {
			c.field.text = []rune(dc.Text)
			c.field.cursor, c.field.shown, c.field.top = len(c.field.text), 0, 0
		}
	}
	return true
}

func (t *paramsTable) draftGrid() *editGrid { return &t.grid }
func (t *paramsTable) newDraftRow() gridRow { return paramRow("", flow.Param{}) }
func (t *paramsTable) fitDraftRow(row int)  { fitDefault(t.grid.rows[row]) }
func (t *paramsTable) draftApplied()        {}

func (s *stepForm) draftGrid() *editGrid { return &s.grid }
func (s *stepForm) newDraftRow() gridRow { return s.row(s.spec.rowField) }
func (s *stepForm) fitDraftRow(row int) {
	if s.spec.typeOf != nil && s.spec.types != nil {
		s.shapeValue(row)
	}
}

func (s *stepForm) draftApplied() {
	s.showRows()
	s.fillTypes()
	if s.spec.changed != nil {
		s.spec.changed(&s.grid)
	}
}

func (p *logsPanel) savedState() *saved.LogsState {
	return &saved.LogsState{
		Day: p.day.Format(time.DateOnly), Log: cmp.Or(p.restoreLog, p.selectedLog),
		FocusList: p.focusList, AllDays: p.allDays, Order: p.order, Shown: int(p.shown),
		Preview: p.preview.focused, Times: p.preview.times,
	}
}

func (p *logsPanel) restoreState(s *saved.LogsState, filter string) {
	p.filter = filter
	if s == nil {
		return
	}
	if day, err := time.ParseInLocation(time.DateOnly, s.Day, time.Local); err == nil {
		p.day = day
	}
	if slices.Contains(logsOrders, s.Order) {
		p.order = s.Order
	}
	p.focusList, p.allDays, p.restoreLog = s.FocusList, s.AllDays, s.Log
	p.shown = logsShown(min(max(s.Shown, int(logsBoth)), int(logsListOnly)))
	p.preview.times = s.Times
	if s.Preview {
		p.focusPreview(false)
	}
}
