package app

import (
	"image"
	"maps"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/script"
)

const (
	formHints       = "{enter} run  {tab} next  esc cancel"
	choiceFormHints = "{enter} run  {tab} next  {left}{right} choose  esc cancel"
	pathFormHints   = "{enter} run  {tab} complete  {down}{up} field  esc cancel"
)

// paramsForm asks for a script's or a flow's parameters before it runs. A script's parameters
// read from the environment are not shown.
type paramsForm struct {
	title    string
	fields   []script.Param
	values   map[string]string
	selected int
	// input edits the selected field's value while it is typed, as inputOf says.
	input   textField
	inputOf string
	// preview is a line under the fields about what the values give, or "".
	preview func(values map[string]string) string
	// submit runs with the values, or tells why it cannot, and the form stays open.
	submit func(values map[string]string) error
	// dir is where the program runs, which path fields complete from.
	dir string
	// verb is what enter does, "run" when empty; pick, when set, lets ctrl+t search for a
	// variable to type into the field.
	verb string
	pick func()
}

func (a *App) openForm(s script.Script, values map[string]string, closeOnSuccess bool) {
	var fields []script.Param
	for _, p := range s.Params {
		if p.Asked(a.getenv) {
			fields = append(fields, p)
		}
	}
	a.form = paramsForm{
		title: "run " + s.Name, fields: fields, values: maps.Clone(values),
		preview: func(values map[string]string) string { return commandPreview(s, values) },
		submit: func(values map[string]string) error {
			if err := s.CheckValues(values); err != nil {
				return err
			}
			a.startRun(s, values, closeOnSuccess)
			return nil
		},
		dir: s.WorkingDir(a.workDir),
	}
	a.mode = formMode
}

func (f *paramsForm) field() script.Param {
	return f.fields[f.selected]
}

func (f *paramsForm) move(step int) {
	f.selected = (f.selected + step + len(f.fields)) % len(f.fields)
}

func (a *App) handleFormKey(key uv.KeyPressEvent) {
	f := &a.form
	name := f.field().Name
	switch key.String() {
	case "esc":
		a.mode = normalMode
	case "enter":
		a.mode = normalMode
		if err := f.submit(maps.Clone(f.values)); err != nil {
			a.statusMessage = err.Error()
			a.mode = formMode
		}
	case "tab":
		if f.field().Path {
			f.values[name] = completePath(f.values[name], f.dir)
			return
		}
		f.move(1)
	case "down":
		f.move(1)
	case "shift+tab", "up":
		f.move(-1)
	case "ctrl+t":
		if field, _ := f.typedField(); f.pick != nil && field != nil {
			f.pick()
		}
	case "left", "right":
		if choices := f.field().Choices; len(choices) > 0 {
			step := map[string]int{"left": -1, "right": 1}[key.String()]
			f.values[name] = cycle(choices, f.values[name], step)
			return
		}
		a.typeKey(key)
	default:
		a.typeKey(key)
	}
}

// typedField is the selected field's text, unless the field is a choice; edits go to the values.
func (f *paramsForm) typedField() (*textField, func()) {
	if len(f.fields) == 0 || len(f.field().Choices) > 0 {
		return nil, nil
	}
	p := f.field()
	if f.inputOf != p.Name || f.input.String() != f.values[p.Name] {
		f.input, f.inputOf = newTextField(f.values[p.Name]), p.Name
	}
	return &f.input, func() { f.values[p.Name] = f.input.String() }
}

// hints tell the arrows' choosing only on a field with choices, tab completing on a path, and
// ctrl+t on a field a variable can be typed into.
func (f *paramsForm) hints() string {
	hints := f.baseHints()
	if f.verb != "" {
		hints = strings.Replace(hints, "{enter} run", "{enter} "+f.verb, 1)
	}
	if field, _ := f.typedField(); f.pick != nil && field != nil {
		hints += "  {ctrl+t} variable"
	}
	return hints
}

func (f *paramsForm) baseHints() string {
	switch {
	case len(f.fields) == 0:
		return formHints
	case len(f.field().Choices) > 0:
		return choiceFormHints
	case f.field().Path:
		return pathFormHints
	}
	return formHints
}

// selectedInput is the field being typed in when field i is selected and typed, or nil.
func (f *paramsForm) selectedInput(i int) *textField {
	if i != f.selected {
		return nil
	}
	field, _ := f.typedField()
	return field
}

// formValue is how a field shows its value: choices with arrows, secrets masked.
func (f *paramsForm) formValue(p script.Param) string {
	value := f.values[p.Name]
	switch {
	case len(p.Choices) > 0:
		return choiceValue(value)
	case p.Secret:
		return strings.Repeat("•", len([]rune(value)))
	}
	return value
}

// commandPreview is the command line the values produce, with secrets masked.
func commandPreview(s script.Script, values map[string]string) string {
	shown := maps.Clone(values)
	for _, p := range s.Params {
		if p.Secret || p.From != "" {
			shown[p.Name] = "••••"
		}
	}
	command, err := s.Command(shown)
	if err != nil {
		return err.Error()
	}
	return "$ " + strings.Join(strings.Fields(command), " ")
}

// labelWidth is the width of the longest label, so the values line up after it.
func (f *paramsForm) labelWidth() int {
	width := 0
	for _, p := range f.fields {
		width = max(width, ansi.StringWidth(p.Label()))
	}
	return width
}

func (a *App) drawForm(screen uv.Screen) {
	f := &a.form
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	hints := a.hint("form", f.hints())
	inner := a.drawPanel(screen, centeredRect(area, 64, panelHeight(len(f.fields)+4+moreHintRows(hints, 64))), f.title)
	valueX := inner.Min.X + f.labelWidth() + 3
	for i, p := range f.fields {
		y := inner.Min.Y + i
		style := a.palette.panelText
		if i == f.selected {
			style = a.palette.typing()
		}
		fillRect(screen, image.Rect(inner.Min.X, y, inner.Max.X, y+1), a.palette.panelText)
		drawText(screen, inner.Min.X+1, y, inner.Max.X, p.Label(), style)
		if field := f.selectedInput(i); field != nil {
			field.draw(screen, valueX, y, inner.Max.X-1-valueX, p.Secret, style, a.palette.selectedRow)
			continue
		}
		drawText(screen, valueX, y, inner.Max.X-1, f.formValue(p), style)
	}
	previewY := inner.Min.Y + len(f.fields) + 1
	if f.preview != nil {
		drawTextCut(screen, inner.Min.X+1, previewY, inner.Max.X-1, f.preview(f.values), a.palette.panelMuted)
	}
	drawHints(screen, inner, hints, a.palette.panelMuted)
}
