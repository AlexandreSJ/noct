package app

import (
	"image"
	"maps"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/script"
)

const formHints = "enter run  tab next  ←→ choose  esc cancel"

// paramsForm asks for a script's or a flow's parameters before it runs. A script's parameters
// read from the environment are not shown.
type paramsForm struct {
	title    string
	fields   []script.Param
	values   map[string]string
	selected int
	// preview is a line under the fields about what the values give, or "".
	preview func(values map[string]string) string
	// submit runs with the values, or tells why it cannot, and the form stays open.
	submit func(values map[string]string) error
}

func (a *App) openForm(s script.Script, values map[string]string) {
	var fields []script.Param
	for _, p := range s.Params {
		if p.Asked() {
			fields = append(fields, p)
		}
	}
	a.form = paramsForm{
		title: "run " + s.Name, fields: fields, values: maps.Clone(values),
		preview: func(values map[string]string) string { return commandPreview(s, values) },
		submit:  func(values map[string]string) error { a.startRun(s, values); return nil },
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
	case "tab", "down":
		f.move(1)
	case "shift+tab", "up":
		f.move(-1)
	case "left", "right":
		if choices := f.field().Choices; len(choices) > 0 {
			step := map[string]int{"left": -1, "right": 1}[key.String()]
			f.values[name] = cycle(choices, f.values[name], step)
		}
	case "backspace":
		f.values[name] = string(deleteLastRune([]rune(f.values[name])))
	case "ctrl+u":
		f.values[name] = ""
	default:
		if isPlainText(key) && len(f.field().Choices) == 0 {
			f.values[name] += key.Text
		}
	}
}

// formValue is how a field shows its value: choices with arrows, secrets masked.
func (f *paramsForm) formValue(p script.Param, selected bool) string {
	value := f.values[p.Name]
	switch {
	case len(p.Choices) > 0:
		return "◂ " + value + " ▸"
	case p.Secret:
		value = strings.Repeat("•", len([]rune(value)))
	}
	if selected {
		value += "▏"
	}
	return value
}

// commandPreview is the command line the values produce, with secrets masked.
func commandPreview(s script.Script, values map[string]string) string {
	shown := maps.Clone(values)
	for _, p := range s.Params {
		if p.Secret || !p.Asked() {
			shown[p.Name] = "••••"
		}
	}
	command, err := s.Command(shown)
	if err != nil {
		return err.Error()
	}
	return "$ " + strings.Join(strings.Fields(command), " ")
}

func (a *App) drawForm(screen uv.Screen) {
	f := &a.form
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, 64, panelHeight(len(f.fields)+4)), f.title)
	labelWidth := 0
	for _, p := range f.fields {
		labelWidth = max(labelWidth, ansi.StringWidth(p.Label()))
	}
	for i, p := range f.fields {
		y := inner.Min.Y + i
		style := a.palette.panelText
		if i == f.selected {
			style = a.palette.selectedRow
		}
		fillRect(screen, image.Rect(inner.Min.X, y, inner.Max.X, y+1), style)
		drawText(screen, inner.Min.X+1, y, inner.Max.X, p.Label(), style)
		drawText(screen, inner.Min.X+labelWidth+3, y, inner.Max.X-1, f.formValue(p, i == f.selected), style)
	}
	previewY := inner.Min.Y + len(f.fields) + 1
	if f.preview != nil {
		drawText(screen, inner.Min.X+1, previewY, inner.Max.X-1, f.preview(f.values), a.palette.panelMuted)
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, formHints, a.palette.panelMuted)
}
