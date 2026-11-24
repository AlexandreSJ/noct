package app

import (
	"errors"
	"image"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// A step is added or changed in the inspect from one form in place of the chart: a row for each
// thing it takes, with the variables it can read listed beside them. enter adds or saves it from
// any row, and esc leaves it as it was.

// stepField is one row of a step form.
type stepField struct {
	label string
	cell  gridCell
	// note tells what the row takes; examples show it with the variables in scope.
	note     string
	examples []string
	// template rows complete the variables in scope as {{. is typed.
	template bool
	// check tells why a value does not fit, or "".
	check func(value string) string
	// describe, when set, tells about the value chosen instead of note.
	describe func(value string) string
	// when, when set, shows the row only while the form's values give true, as the name of a
	// custom agent only once custom agent is picked.
	when func(values []formValue) bool
}

// stepFormSpec is what a kind of step asks and how the answers make the step.
type stepFormSpec struct {
	title  string
	fields []stepField
	// named rows take a name in the label column too, as a set step's variables, with an empty
	// row last to add one more, which asks what rowField does.
	named    bool
	rowField stepField
	build    func(values []formValue) (flow.Step, error)
	// finish, when set, saves the step built in its own way, as adding a custom agent first.
	finish func(step flow.Step, save func(flow.Step) error) error
	// saveValues, when set, saves the answers itself, for a form that makes no step, as a
	// function's returns.
	saveValues func(values []formValue) error
}

// formValue is what a row of a step form holds: its name, in a named form, and its value.
type formValue struct{ name, value string }

type stepForm struct {
	app  *App
	spec stepFormSpec
	grid editGrid
	// adding tells enter's word: add for a new step, save for a step changed.
	adding  bool
	save    func(flow.Step) error
	message string
	// values are the variables in scope with their last values, for the help.
	values map[string]string
}

const (
	stepFormLabelColumn = iota
	stepFormValueColumn
)

func (a *App) newStepForm(spec stepFormSpec, vars []flow.Variable, values map[string]string, adding bool, save func(flow.Step) error) *stepForm {
	form := &stepForm{app: a, spec: spec, adding: adding, save: save, values: values}
	form.grid.variables = flow.Readable(vars)
	for _, field := range spec.fields {
		form.grid.rows = append(form.grid.rows, form.row(field))
	}
	if spec.named {
		form.grid.rows = append(form.grid.rows, form.row(spec.rowField))
	}
	form.showRows()
	form.grid.focus(0, form.firstColumn())
	return form
}

// showRows hides the rows whose when the values make false; a hidden row is never focused.
func (s *stepForm) showRows() {
	values := s.formValues()
	for i := range s.grid.rows {
		if when := s.field(i).when; when != nil {
			s.grid.rows[i][stepFormValueColumn].skip = !when(values)
		}
	}
}

func (s *stepForm) hidden(row int) bool {
	return !s.spec.named && s.grid.rows[row][stepFormValueColumn].skip
}

func (s *stepForm) firstColumn() int {
	if s.spec.named {
		return stepFormLabelColumn
	}
	return stepFormValueColumn
}

func (s *stepForm) row(field stepField) []gridCell {
	label := textCellOf(field.label)
	label.skip = !s.spec.named
	field.cell.template = field.template
	return []gridCell{label, field.cell}
}

// field is the spec of a row; the rows a named form adds ask what its rowField does.
func (s *stepForm) field(row int) stepField {
	if row >= len(s.spec.fields) {
		return s.spec.rowField
	}
	return s.spec.fields[row]
}

// note tells about the focused row: what it takes, or about the value chosen.
func (s *stepForm) note() string {
	field := s.field(s.grid.row)
	if field.describe != nil {
		return field.describe(s.grid.rows[s.grid.row][stepFormValueColumn].value())
	}
	return field.note
}

func (s *stepForm) formValues() []formValue {
	values := make([]formValue, len(s.grid.rows))
	for i, row := range s.grid.rows {
		values[i] = formValue{strings.TrimSpace(row[stepFormLabelColumn].value()), strings.TrimSpace(row[stepFormValueColumn].value())}
	}
	return values
}

// problem is why a row's name or value does not fit, or "".
func (s *stepForm) problem(row int) string {
	return orDefault(s.nameProblem(row), s.valueProblem(row))
}

// nameProblem is why a named form's row cannot have its name; an empty one is left out.
func (s *stepForm) nameProblem(row int) string {
	name := strings.TrimSpace(s.grid.rows[row][stepFormLabelColumn].value())
	value := strings.TrimSpace(s.grid.rows[row][stepFormValueColumn].value())
	switch {
	case !s.spec.named || name == "" && value == "":
		return ""
	case name == "":
		return "a name is needed for this value"
	}
	return checkVariable(name)
}

func (s *stepForm) valueProblem(row int) string {
	check := s.field(row).check
	if check == nil || s.hidden(row) || s.spec.named && strings.TrimSpace(s.grid.rows[row][stepFormLabelColumn].value()) == "" {
		return ""
	}
	return check(strings.TrimSpace(s.grid.rows[row][stepFormValueColumn].value()))
}

func (s *stepForm) title() string { return s.spec.title }

func (s *stepForm) hints() string {
	if s.adding {
		return "{enter} add  {tab} next  esc cancel"
	}
	return "{enter} save  {tab} next  esc cancel"
}

func (s *stepForm) handleKey(key uv.KeyPressEvent) bool {
	switch key.String() {
	case "esc":
		return true
	case "enter":
		return s.submit()
	}
	s.message = ""
	if s.grid.handleKey(key) {
		s.changed()
	}
	return false
}

func (s *stepForm) changed() {
	if s.spec.named {
		s.keepAnEmptyRow()
	}
	s.showRows()
}

func (s *stepForm) paste(text string) {
	if s.grid.paste(text) {
		s.changed()
	}
}

// keepAnEmptyRow adds a row to a named form once its last one has a name.
func (s *stepForm) keepAnEmptyRow() {
	last := s.grid.rows[len(s.grid.rows)-1]
	if last[stepFormLabelColumn].value() != "" {
		s.grid.rows = append(s.grid.rows, s.row(s.spec.rowField))
	}
}

// submit makes the step and saves it, telling whether the form is done; a value that does not
// fit, or a step the flow refuses, keeps it open with the reason.
func (s *stepForm) submit() bool {
	for i := range s.grid.rows {
		if problem := s.problem(i); problem != "" {
			s.grid.focus(i, stepFormValueColumn)
			s.message = problem
			return false
		}
	}
	if s.spec.saveValues != nil {
		return s.done(s.spec.saveValues(s.formValues()))
	}
	step, err := s.spec.build(s.formValues())
	switch {
	case err != nil:
	case s.spec.finish != nil:
		err = s.spec.finish(step, s.save)
	default:
		err = s.save(step)
	}
	return s.done(err)
}

// done tells the form is done unless err keeps it open with the reason.
func (s *stepForm) done(err error) bool {
	if err != nil {
		s.message = err.Error()
		return false
	}
	return true
}

func (s *stepForm) draw(screen uv.Screen, area image.Rectangle, helpBeside bool) {
	pal := &s.app.palette
	labelWidth := 8
	for _, row := range s.grid.rows {
		labelWidth = max(labelWidth, ansi.StringWidth(row[stepFormLabelColumn].value()))
	}
	valueX := area.Min.X + 1 + labelWidth + 2
	y := area.Min.Y
	for i, row := range s.grid.rows {
		if s.hidden(i) {
			continue
		}
		if y >= area.Max.Y-2 {
			break
		}
		for column, cell := range row {
			x, width := area.Min.X+1, labelWidth
			if column == stepFormValueColumn {
				x, width = valueX, area.Max.X-1-valueX
			}
			s.drawCell(screen, x, y, width, i, column, &cell)
		}
		y++
	}
	if !helpBeside {
		s.drawSuggestions(screen, area, valueX)
	}
	note, style := s.note(), pal.panelMuted
	if s.message != "" || s.problem(s.grid.row) != "" {
		note, style = "✗ "+orDefault(s.message, s.problem(s.grid.row)), pal.failure
	}
	drawTextCut(screen, area.Min.X+1, area.Max.Y-1, area.Max.X-1, note, style)
}

// drawSuggestions lists under the focused row the variables that complete what is typed, for a
// panel too narrow for the help beside.
func (s *stepForm) drawSuggestions(screen uv.Screen, area image.Rectangle, x int) {
	matches, _ := s.grid.suggestions()
	for i, name := range matches[:min(len(matches), maxSuggestions)] {
		y := area.Min.Y + s.shownAbove(s.grid.row) + 1 + i
		if y >= area.Max.Y-1 {
			return
		}
		style := s.app.palette.panelMuted
		if i == min(s.grid.suggested, len(matches)-1) {
			style = s.app.palette.typing()
		}
		fillRect(screen, image.Rect(x, y, area.Max.X-1, y+1), s.app.palette.panelText)
		drawTextCut(screen, x, y, area.Max.X-1, name, style)
	}
}

// shownAbove counts the rows drawn above row, which is where row is drawn.
func (s *stepForm) shownAbove(row int) int {
	count := 0
	for i := range row {
		if !s.hidden(i) {
			count++
		}
	}
	return count
}

func (s *stepForm) drawCell(screen uv.Screen, x, y, width, row, column int, cell *gridCell) {
	pal := &s.app.palette
	focused := row == s.grid.row && column == s.grid.col
	style := pal.panelText
	switch {
	case column == stepFormLabelColumn && !s.spec.named:
		style = pal.panelMuted
	case column == stepFormLabelColumn && s.nameProblem(row) != "", column == stepFormValueColumn && s.valueProblem(row) != "":
		style = pal.failure
	case focused:
		style = pal.typing()
	}
	if focused && cell.kind == textCell {
		s.grid.rows[row][column].field.draw(screen, x, y, width, false, style, pal.selectedRow)
		return
	}
	drawTextCut(screen, x, y, x+width, cell.value(), style)
}

// help is the row's note and examples, then the variables in scope: the ones that complete what
// is typed, with the one tab puts in lit, or all of them.
func (s *stepForm) help() []styledLine {
	pal := &s.app.palette
	lines := []styledLine{{s.note(), pal.panelText}}
	for _, example := range s.field(s.grid.row).examples {
		lines = append(lines, styledLine{example, pal.panelMuted})
	}
	lines = append(lines, styledLine{"", pal.panelText}, styledLine{"variables here", pal.panelHeading})
	matches, _ := s.grid.suggestions()
	if len(matches) > 0 {
		for i, name := range matches {
			style := pal.panelText
			if i == min(s.grid.suggested, len(matches)-1) {
				style = pal.typing()
			}
			lines = append(lines, styledLine{s.variableLine(name), style})
		}
		return append(lines, styledLine{"", pal.panelText}, styledLine{s.app.hint("flow-editor", "{up}{down} pick  {tab} put in"), pal.panelMuted})
	}
	if len(s.grid.variables) == 0 {
		return append(lines, styledLine{"none yet", pal.panelMuted})
	}
	for _, v := range s.grid.variables {
		lines = append(lines, styledLine{certaintyMarks[v.Certainty] + s.variableLine(variableName(v)), pal.panelText})
	}
	return append(lines, styledLine{"", pal.panelText}, styledLine{"{{. completes them; ↻ this pass  ? if a branch ran", pal.panelMuted})
}

func (s *stepForm) variableLine(name string) string {
	if value := s.values[name]; value != "" {
		return name + " = " + value
	}
	return name
}

// openEditor shows editor in place of the chart, with the keys going to it.
func (p *inspectPanel) openEditor(editor inspectEditor) {
	p.editor, p.focusChart, p.moving = editor, true, false
}

// closeEditor goes back to the chart; a params table with rows it could not save is kept for the
// next time the flow's params are opened.
func (p *inspectPanel) closeEditor() {
	if table, ok := p.editor.(*paramsTable); ok {
		if p.paramDrafts == nil {
			p.paramDrafts = map[string]*paramsTable{}
		}
		delete(p.paramDrafts, entryKey(table.flow))
		if table.unsaved() > 0 {
			p.paramDrafts[entryKey(table.flow)] = table
		}
	}
	p.editor = nil
}

// openParams edits the flow's params as a table, as it was left when rows were not saved.
func (p *inspectPanel) openParams(f flow.Flow) {
	if draft, ok := p.paramDrafts[entryKey(f)]; ok {
		draft.flow = f
		p.openEditor(draft)
		return
	}
	p.openEditor(p.app.newParamsTable(f))
}

// addStep asks what kind of step to add where the selected block is: at a gap or into an empty
// list, or right after a step. A script or a function is picked from a list; the rest, and a
// function's values, are asked in a form in place of the chart.
func (p *inspectPanel) addStep(f flow.Flow, at chartBlock) {
	parent, branch, index := at.slot.parent, at.slot.branch, at.index
	if !at.isInsertion() {
		parent, branch, index = flow.SplitPath(at.path)
		index++
	}
	vars := f.Scope(parent, branch, index, p.app.scripts.Find)
	insert := func(step flow.Step) error {
		path, err := p.app.insertStep(f, parent, branch, index, step)
		if err == nil {
			p.selected = stepKey(path)
		}
		return err
	}
	form := func(spec stepFormSpec) func() {
		return func() { p.openEditor(p.app.newStepForm(spec, vars, p.lastValues(f, vars), true, insert)) }
	}
	agentForm := func() { form(p.app.agentForm(flow.Step{}))() } // built when picked: finding the agents runs programs
	call := func(fn flow.Flow) {
		if len(fn.Params) == 0 {
			p.app.reportError(insert(flow.Step{Call: fn.Name}))
			return
		}
		form(callForm(fn, flow.Step{}))()
	}
	var zero flow.Step
	p.app.openChoice("add step",
		choiceOption{key: "s", label: "script", pick: func() { p.app.pickScript(func(s flow.Step) { p.app.reportError(insert(s)) }) }},
		choiceOption{key: "f", label: "function call", pick: func() { p.app.pickFunction(call) }},
		choiceOption{key: "a", label: "AI agent", pick: agentForm},
		choiceOption{key: "c", label: "command", pick: form(commandForm(zero))},
		choiceOption{key: "i", label: "if", pick: form(ifForm(zero, vars))},
		choiceOption{key: "o", label: "switch", pick: form(switchForm(zero))},
		choiceOption{key: "l", label: "repeat", pick: form(repeatForm(zero))},
		choiceOption{key: "e", label: "for each line", pick: form(eachForm(zero))},
		choiceOption{key: "w", label: "wait", pick: form(waitForm(zero))},
		choiceOption{key: "p", label: "prompt", pick: form(promptForm(zero))},
		choiceOption{key: "v", label: "set", pick: form(setForm(zero))},
		choiceOption{key: "u", label: "unset", pick: form(unsetForm(zero))},
	)
}

// editStep changes the step at path in its form, or as YAML in the editor for the kinds without
// one.
func (p *inspectPanel) editStep(f flow.Flow, path string) {
	step, ok := f.Step(path)
	if !ok {
		return
	}
	parent, branch, index := flow.SplitPath(path)
	vars := f.Scope(parent, branch, index, p.app.scripts.Find)
	spec, ok := p.app.stepFormFor(step, vars)
	if !ok {
		p.app.editFlowStep(f, path)
		return
	}
	replace := func(changed flow.Step) error {
		return p.app.changeFlow(f, func(f *flow.Flow) error {
			if now, ok := f.Step(path); !ok || now.ID != step.ID || now.Kind() != step.Kind() {
				return errors.New("the step moved or changed since the form opened: esc, then e on it again")
			}
			return f.Replace(path, changed)
		})
	}
	p.openEditor(p.app.newStepForm(spec, vars, p.lastValues(f, vars), false, replace))
}

// lastValues are the variables' values in the run the chart shows, by name as templates read them.
func (p *inspectPanel) lastValues(f flow.Flow, vars []flow.Variable) map[string]string {
	values := map[string]string{}
	for _, v := range vars {
		values[variableName(v)] = variableValue(f, v, p.result(f))
	}
	return values
}
