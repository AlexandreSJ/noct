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
// thing it takes, with the variables it can read listed beside them. ctrl+s, or the button after
// the rows, adds or saves it, and esc leaves it as it was.

// stepField is one row of a step form.
type stepField struct {
	label string
	cell  gridCell
	// note tells what the row takes; examples show it with the variables in scope.
	note     string
	examples []string
	// template rows complete the variables in scope as a $ name is typed; expression rows are a
	// condition or value read whole as an expression, where names are written alone.
	template, expression bool
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
	// named rows take a name in the label column too, as a set step's variables; each asks what
	// rowField does, and the button + add <adds> adds one.
	named    bool
	rowField stepField
	adds     string
	build    func(values []formValue) (flow.Step, error)
	// finish, when set, saves the step built in its own way, as adding a custom agent first.
	finish func(step flow.Step, save func(flow.Step) error) error
	// saveValues, when set, saves the answers itself, for a form that makes no step, as a
	// function's returns.
	saveValues func(values []formValue) error
}

// formValue is what a row of a step form holds: its name, in a named form, and its value, with
// a list's items.
type formValue struct {
	name, value string
	items       []string
}

type stepForm struct {
	app  *App
	spec stepFormSpec
	grid editGrid
	save func(flow.Step) error
	// message is why the last save was refused.
	message string
	// values are the variables in scope with their last values, for the help.
	values map[string]string
}

const (
	stepFormLabelColumn = iota
	stepFormValueColumn
)

func (a *App) newStepForm(spec stepFormSpec, vars []flow.Variable, values map[string]string, adding bool, save func(flow.Step) error) *stepForm {
	form := &stepForm{app: a, spec: spec, save: save, values: values}
	form.grid.variables = flow.Readable(vars)
	form.grid.arranged = spec.named
	for _, field := range spec.fields {
		form.grid.rows = append(form.grid.rows, form.row(field))
	}
	if spec.named {
		form.grid.buttons = append(form.grid.buttons, "+ add "+orDefault(spec.adds, "row"))
	}
	form.grid.buttons = append(form.grid.buttons, map[bool]string{true: "add step", false: "save"}[adding && spec.saveValues == nil])
	form.showRows()
	form.grid.focusNear(0, form.firstColumn())
	return form
}

// showRows hides the rows whose when the values make false; a hidden row is never focused.
func (s *stepForm) showRows() {
	values := s.formValues()
	for i := range s.grid.rows {
		if when := s.field(i).when; when != nil {
			s.grid.rows[i].hidden = !when(values)
		}
	}
	s.grid.keepFocus()
}

func (s *stepForm) firstColumn() int {
	if s.spec.named {
		return stepFormLabelColumn
	}
	return stepFormValueColumn
}

func (s *stepForm) row(field stepField) gridRow {
	label := textCellOf(field.label)
	label.skip = !s.spec.named
	field.cell.template, field.cell.expression = field.template || field.expression, field.expression
	return gridRow{cells: []gridCell{label, field.cell}}
}

// field is the spec of a row: a named form's rows all ask what its rowField does.
func (s *stepForm) field(row int) stepField {
	if s.spec.named || row >= len(s.spec.fields) {
		return s.spec.rowField
	}
	return s.spec.fields[row]
}

// note tells about the focused row: what it takes, or about the value chosen.
func (s *stepForm) note() string {
	if _, onButton := s.grid.button(); onButton {
		return ""
	}
	field := s.field(s.grid.row)
	if field.describe != nil {
		return field.describe(s.grid.at(s.grid.row, stepFormValueColumn).value())
	}
	return field.note
}

func (s *stepForm) formValues() []formValue {
	values := make([]formValue, len(s.grid.rows))
	for i, row := range s.grid.rows {
		cell := row.cells[stepFormValueColumn]
		values[i] = formValue{strings.TrimSpace(row.cells[stepFormLabelColumn].value()), strings.TrimSpace(cell.value()), cell.items}
	}
	return values
}

// problem is why a row's name or value does not fit, or "".
func (s *stepForm) problem(row int) string {
	if row >= len(s.grid.rows) {
		return ""
	}
	return orDefault(s.nameProblem(row), s.valueProblem(row))
}

// nameProblem is why a named form's row cannot have its name; a row without a name or a value
// is left out.
func (s *stepForm) nameProblem(row int) string {
	name := strings.TrimSpace(s.grid.at(row, stepFormLabelColumn).value())
	value := strings.TrimSpace(s.grid.at(row, stepFormValueColumn).value())
	switch {
	case !s.spec.named || name == "" && value == "":
		return ""
	case name == "":
		return "a name is needed for this value; ctrl+delete removes it"
	}
	return checkVariable(name)
}

func (s *stepForm) valueProblem(row int) string {
	check := s.field(row).check
	if check == nil || s.grid.rows[row].hidden || s.spec.named && strings.TrimSpace(s.grid.at(row, stepFormLabelColumn).value()) == "" {
		return ""
	}
	return check(strings.TrimSpace(s.grid.at(row, stepFormValueColumn).value()))
}

func (s *stepForm) title() string { return s.spec.title }

func (s *stepForm) hints() string {
	return s.grid.hints("{enter} edit  {ctrl+s} save  esc cancel  {?} keys")
}

func (s *stepForm) typing() bool { return s.grid.typing() }

func (s *stepForm) hitAt(at image.Point) (image.Rectangle, bool, bool) { return s.grid.hitAt(at) }

func (s *stepForm) handleKey(key uv.KeyPressEvent) bool {
	switch name := key.String(); {
	case name == "ctrl+s":
		s.after(s.grid.settle())
		return s.submit()
	case s.grid.busy():
	case name == "esc":
		return true
	case name == "?":
		s.app.showContextKeys("flow-editor", "step form keys")
		return false
	}
	return s.after(s.grid.handleKey(key))
}

func (s *stepForm) mouse(event uv.MouseEvent, at image.Point) bool {
	return s.after(s.grid.mouse(event, at))
}

// after adds a named row or saves the step when a button is pressed, telling whether the form
// is done, and shows the rows the values now call for.
func (s *stepForm) after(out gridOutcome) bool {
	s.message = ""
	if out&gridChanged != 0 {
		s.showRows()
	}
	if out&gridPressed == 0 {
		return false
	}
	if button, _ := s.grid.button(); s.spec.named && button == 0 {
		s.grid.addRow(s.row(s.spec.rowField))
		s.grid.focus(len(s.grid.rows)-1, stepFormLabelColumn)
		s.grid.startEdit()
		return false
	}
	return s.submit()
}

func (s *stepForm) paste(text string) { s.grid.paste(text) }

func (s *stepForm) takeCopied() string { return s.grid.takeCopied() }

// submit makes the step and saves it, telling whether the form is done; a value that does not
// fit, or a step the flow refuses, keeps it open with the reason.
func (s *stepForm) submit() bool {
	for i := range s.grid.rows {
		if problem := s.problem(i); problem != "" {
			s.grid.focusNear(i, stepFormValueColumn)
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
	s.grid.hits = s.grid.hits[:0]
	labelWidth := 8
	for _, row := range s.grid.rows {
		labelWidth = max(labelWidth, ansi.StringWidth(row.cells[stepFormLabelColumn].value()))
	}
	left := area.Min.X + 1
	if s.spec.named {
		left = area.Min.X + moveGutter
	}
	valueX := left + labelWidth + 2
	y := area.Min.Y
	for _, row := range s.grid.window(area.Dy() - 1) {
		if b, ok := s.grid.buttonAt(row); ok {
			s.grid.drawButton(screen, image.Rect(left, y, area.Max.X-1, y+1), row, b, pal)
			y++
			continue
		}
		if row == s.grid.row {
			s.grid.drawMoveArrows(screen, image.Pt(area.Min.X+1, y), pal)
		}
		s.drawCell(screen, image.Rect(left, y, left+labelWidth, y+1), row, stepFormLabelColumn)
		s.drawCell(screen, image.Rect(valueX, y, area.Max.X-1, y+1), row, stepFormValueColumn)
		y++
	}
	if !helpBeside {
		s.drawSuggestions(screen, area, valueX)
	}
	note, style := s.note(), pal.panelMuted
	if problem := orDefault(s.message, s.problem(s.grid.row)); problem != "" {
		note, style = "✗ "+problem, pal.failure
	}
	drawTextCut(screen, area.Min.X+1, area.Max.Y-1, area.Max.X-1, note, style)
	s.grid.drawOverlay(screen, area, pal)
}

// drawSuggestions lists under the cell typed in the variables that complete what is typed, for a
// panel too narrow for the help beside.
func (s *stepForm) drawSuggestions(screen uv.Screen, area image.Rectangle, x int) {
	matches, _ := s.grid.suggestions()
	at, ok := s.grid.focusRect()
	for i, name := range matches[:min(len(matches), maxSuggestions)] {
		y := at.Max.Y + i
		if !ok || y >= area.Max.Y-1 {
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

func (s *stepForm) drawCell(screen uv.Screen, rect image.Rectangle, row, column int) {
	pal := &s.app.palette
	cell := s.grid.at(row, column)
	style := pal.panelText
	switch {
	case column == stepFormLabelColumn && !s.spec.named:
		style = pal.panelMuted
	case column == stepFormLabelColumn && s.nameProblem(row) != "", column == stepFormValueColumn && s.valueProblem(row) != "":
		style = pal.failure
	}
	text := cell.value()
	switch cell.kind {
	case optionCell:
		text = optionText(text)
	case listCell:
		text = listText(cell)
	}
	if cell.skip {
		drawTextCut(screen, rect.Min.X, rect.Min.Y, rect.Max.X, text, style)
		return
	}
	s.grid.drawCell(screen, rect, row, column, text, style, pal)
}

// help is the row's note and examples, then the variables in scope: the ones that complete what
// is typed, with the one tab puts in lit, or all of them.
func (s *stepForm) help() []styledLine {
	pal := &s.app.palette
	lines := []styledLine{{s.note(), pal.panelText}}
	if _, onButton := s.grid.button(); !onButton {
		for _, example := range s.field(s.grid.row).examples {
			lines = append(lines, styledLine{example, pal.panelMuted})
		}
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
	return append(lines, styledLine{"", pal.panelText}, styledLine{"$ or {{. completes them; ↻ this pass  ? if a branch ran", pal.panelMuted})
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
	scriptStep := func(step flow.Step) {
		if sc, ok := p.app.scripts.Find(step.Script); ok && len(sc.Params) > 0 {
			form(scriptStepForm(sc, step))()
			return
		}
		p.app.reportError(insert(step))
	}
	var zero flow.Step
	var picker *stepPicker
	kinds := []finderItem{
		{label: "script", kind: "a saved script", open: func() { picker.show("script for the step", p.app.scriptStepItems(scriptStep)) }},
		{label: "function call", kind: "a function, with values", open: func() { picker.show("function to call", p.app.functionItems(call)) }},
		{label: "AI agent", kind: "asks an agent", open: agentForm},
		{label: "command", kind: "a shell line", open: form(commandForm(zero))},
		{label: "if", kind: "a branch on a condition", open: form(ifForm(zero, vars))},
		{label: "switch", kind: "a branch on a value", open: form(switchForm(zero))},
		{label: "repeat", kind: "steps again", open: form(repeatForm(zero))},
		{label: "for each line", kind: "steps per item", open: form(eachForm(zero))},
		{label: "wait", kind: "a pause", open: form(waitForm(zero))},
		{label: "prompt", kind: "asks you", open: form(promptForm(zero))},
		{label: "set", kind: "gives variables values", open: form(setForm(zero))},
		{label: "unset", kind: "clears variables", open: form(unsetForm(zero))},
	}
	picker = newStepPicker(&p.app.palette, kinds, p.app.lastStepAdded, func(path []string) { p.app.lastStepAdded = path })
	p.openEditor(picker)
}

// editStep changes the step at path in its form, or as YAML in the editor for a script without
// params or a call to a function without them.
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
	p.openEditor(p.app.newStepForm(spec, vars, p.lastValues(f, vars), false, p.replaceStep(f, path, step)))
}

// replaceStep saves a changed step in the place of original at path, unless the flow moved or
// changed it since the form opened.
func (p *inspectPanel) replaceStep(f flow.Flow, path string, original flow.Step) func(flow.Step) error {
	return func(changed flow.Step) error {
		return p.app.changeFlow(f, func(f *flow.Flow) error {
			if now, ok := f.Step(path); !ok || now.ID != original.ID || now.Kind() != original.Kind() {
				return errors.New("the step moved or changed since the form opened: esc, then open it again")
			}
			return f.Replace(path, changed)
		})
	}
}

// lastValues are the variables' values in the run the chart shows, by name as templates read them.
func (p *inspectPanel) lastValues(f flow.Flow, vars []flow.Variable) map[string]string {
	values := map[string]string{}
	for _, v := range vars {
		values[variableName(v)] = variableValue(f, v, p.result(f))
	}
	return values
}
