package app

import (
	"cmp"
	"errors"
	"image"
	"regexp"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
)

// A step is added or changed in the inspect from one form in place of the chart: a row for each
// thing it takes, with the variables it can read listed beside them. ctrl+s, or the button after
// the rows, adds or saves it; esc saves what changed too, asks first, or leaves the step as it
// was, as the config's leave_form says.

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
	kind string
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
	types    []string
	typeOf   func(name string) string
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
	name, value, kind string
	items             []string
}

type stepForm struct {
	app  *App
	spec stepFormSpec
	grid editGrid
	save func(flow.Step) error
	// message is why the last save was refused.
	message string
	// values are the variables in scope with their last values, for the help.
	values          map[string]string
	leaving         string
	opened          []formValue
	asking, refused bool
}

const (
	stepFormLabelColumn = iota
	stepFormValueColumn
	stepFormTypeColumn
)

const stepFormTypeWidth = 9

func (a *App) newStepForm(spec stepFormSpec, vars []flow.Variable, values map[string]string, adding bool, save func(flow.Step) error) *stepForm {
	form := &stepForm{app: a, spec: spec, save: save, values: values, leaving: cmp.Or(a.config.Flows.LeaveForm, config.LeaveAsks)}
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
	if spec.named && len(spec.fields) == 1 && spec.fields[0].label == "" {
		form.grid.startEdit()
	}
	form.opened = form.formValues()
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
	label.skip, label.names = !s.spec.named, s.spec.named
	field.cell.template, field.cell.expression = field.template || field.expression, field.expression
	field.cell.field.multiline = field.cell.kind == textCell && !field.cell.secret
	switch pal := &s.app.palette; {
	case field.cell.expression:
		field.cell.field.paint = logicPaint(pal)
	case field.cell.escapes:
		field.cell.field.paint = escapePaint(pal)
	}
	field.cell.json = s.spec.types != nil
	row := gridRow{cells: []gridCell{label, field.cell}}
	if s.spec.types != nil {
		kind := optionCellOf(cmp.Or(field.kind, s.spec.types[0]), s.spec.types)
		kind.autoFor = field.label
		row.cells = append(row.cells, kind)
	}
	return row
}

func (s *stepForm) fillTypes() {
	if s.spec.typeOf == nil {
		return
	}
	for i := range s.grid.rows {
		name, kind := strings.TrimSpace(s.grid.at(i, stepFormLabelColumn).value()), s.grid.at(i, stepFormTypeColumn)
		if name == kind.autoFor {
			continue
		}
		kind.autoFor = name
		if known := s.spec.typeOf(name); slices.Contains(s.spec.types, known) {
			kind.field = newTextField(known)
		}
	}
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
		values[i] = formValue{name: strings.TrimSpace(row.cells[stepFormLabelColumn].value()), value: strings.TrimSpace(cell.value()), items: cell.items}
		if len(row.cells) > stepFormTypeColumn {
			values[i].kind = row.cells[stepFormTypeColumn].value()
		}
	}
	return values
}

// problem is why a row's name or value does not fit, or "".
func (s *stepForm) problem(row int) string {
	if row >= len(s.grid.rows) {
		return ""
	}
	return cmp.Or(s.nameProblem(row), s.valueProblem(row), s.readsItself(row))
}

func (s *stepForm) readsItself(row int) string {
	if s.spec.types == nil {
		return ""
	}
	name := strings.TrimSpace(s.grid.at(row, stepFormLabelColumn).value())
	value, kind := s.grid.at(row, stepFormValueColumn).value(), s.grid.at(row, stepFormTypeColumn).value()
	trimmed := strings.TrimSpace(value)
	object := kind == flow.ListParam || kind == flow.ObjectParam || kind == flow.VarType && (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["))
	if name == "" || !object || !regexp.MustCompile(`\$\{?`+regexp.QuoteMeta(name)+`\b`).MatchString(value) {
		return ""
	}
	return name + " reads itself: a list or an object is made of other variables"
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
	if s.asking {
		return "y or {enter} saves  n leaves the step as it was  esc keeps editing"
	}
	leave := map[string]string{config.LeaveSaves: "esc saves", config.LeaveAsks: "esc asks", config.LeaveDiscard: "esc cancels"}[s.leaving]
	return s.grid.hints("{enter} edit  {ctrl+s} save  " + leave + "  {?} keys")
}

func (s *stepForm) typing() bool { return s.grid.typing() }

func (s *stepForm) hitAt(at image.Point) (image.Rectangle, bool, bool) { return s.grid.hitAt(at) }

func (s *stepForm) handleKey(key uv.KeyPressEvent) bool {
	name := key.String()
	if s.asking {
		return s.answer(name)
	}
	refused := s.refused
	s.refused = false
	switch {
	case name == "ctrl+s":
		s.after(s.grid.settle())
		return s.submit()
	case s.grid.busy():
	case name == "esc":
		return refused || s.leave()
	case name == "?":
		s.app.showContextKeys("flow-editor", "step form keys")
		return false
	}
	return s.after(s.grid.handleKey(key))
}

func (s *stepForm) leave() bool {
	switch {
	case s.leaving == config.LeaveDiscard || !s.changed():
		return true
	case s.leaving == config.LeaveAsks:
		s.asking = true
		return false
	}
	return s.saveOnLeaving()
}

func (s *stepForm) answer(name string) bool {
	s.asking = false
	switch name {
	case "y", "enter":
		return s.saveOnLeaving()
	case "n":
		return true
	}
	return false
}

func (s *stepForm) saveOnLeaving() bool {
	if s.submit() {
		return true
	}
	s.refused = true
	s.message += "; esc again leaves the step as it was"
	return false
}

func (s *stepForm) changed() bool {
	return !slices.EqualFunc(s.formValues(), s.opened, func(a, b formValue) bool {
		return a.name == b.name && a.value == b.value && a.kind == b.kind && slices.Equal(a.items, b.items)
	})
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
		s.fillTypes()
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
	valueX, valueEnd := left+labelWidth+2, area.Max.X-1
	if s.spec.types != nil {
		valueEnd -= stepFormTypeWidth + 2
	}
	y := area.Min.Y
	for _, row := range s.grid.windowFit(area.Dy()-1, s.rowHeight) {
		if b, ok := s.grid.buttonAt(row); ok {
			s.grid.drawButton(screen, image.Rect(left, y, area.Max.X-1, y+1), row, b, pal)
			y++
			continue
		}
		if row == s.grid.row {
			s.grid.drawMoveArrows(screen, image.Pt(area.Min.X+1, y), pal)
		}
		height := min(s.rowHeight(row), area.Max.Y-1-y)
		s.drawCell(screen, image.Rect(left, y, left+labelWidth, y+1), row, stepFormLabelColumn)
		s.drawCell(screen, image.Rect(valueX, y, valueEnd, y+height), row, stepFormValueColumn)
		if s.spec.types != nil {
			s.drawCell(screen, image.Rect(valueEnd+2, y, area.Max.X-1, y+1), row, stepFormTypeColumn)
		}
		s.grid.drawMark(screen, image.Rect(left, y, area.Max.X-1, y+height), row)
		y += height
	}
	if !helpBeside {
		s.drawSuggestions(screen, area, valueX)
	}
	note, style := s.note(), pal.panelMuted
	if problem := orDefault(s.message, s.problem(s.grid.row)); problem != "" {
		note, style = "✗ "+problem, pal.failure
	}
	if s.asking {
		note, style = "save what changed? y saves, n leaves the step as it was", pal.panelHeading
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
	switch {
	case cell.skip:
		drawTextCut(screen, rect.Min.X, rect.Min.Y, rect.Max.X, text, style)
	case cell.field.multiline:
		s.grid.drawBlock(screen, rect, row, column, s.valueLines(cell, style), pal)
	default:
		s.grid.drawCell(screen, rect, row, column, text, style, pal)
	}
}

const maxCellLines = 12

func (s *stepForm) rowHeight(row int) int {
	if row >= len(s.grid.rows) {
		return 1
	}
	cell := s.grid.at(row, stepFormValueColumn)
	if s.grid.editing && s.grid.lit(row, stepFormValueColumn) {
		return min(cell.field.lineCount(), maxCellLines)
	}
	return min(len(s.valueLines(cell, s.app.palette.panelText)), maxCellLines)
}

// valueLines are how a cell of several lines shows while not typed in: a condition laid out by
// its logic, other text line by line.
func (s *stepForm) valueLines(cell *gridCell, style uv.Style) []styledLine {
	text := cell.value()
	if !cell.field.multiline {
		return []styledLine{{text, style}}
	}
	switch trimmed := strings.TrimSpace(text); {
	case cell.expression && style == s.app.palette.panelText:
		return conditionLines(text, &s.app.palette)
	case cell.json && style == s.app.palette.panelText && (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")):
		return jsonLines(text, &s.app.palette)
	}
	var lines []styledLine
	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, styledLine{line, style})
	}
	return lines
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
	return append(lines, styledLine{"", pal.panelText}, styledLine{"typing a name completes them, tab puts in the one shown dim; ↻ this pass  ? if a branch ran", pal.panelMuted})
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
	vars := f.WithObjectPaths(f.Scope(parent, branch, index, p.app.scripts.Find))
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
		{label: "set", kind: "gives variables values", open: form(setForm(zero, f.VariableType))},
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
	vars := f.WithObjectPaths(f.Scope(parent, branch, index, p.app.scripts.Find))
	spec, ok := p.app.stepFormFor(f, step, vars)
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
