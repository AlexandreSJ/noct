package app

import (
	"cmp"
	"image"
	"maps"
	"os"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/naming"
)

// A flow's params are edited in the inspect as a table, a param a row, like declarations in code:
// type, name, default and the rest in columns. Each row is saved when the focus leaves it, if it
// is valid; a cell in the failure color tells what keeps its row from being saved. Emptying a
// param's name removes it.

const (
	paramTypeColumn = iota
	paramNameColumn
	paramDefaultColumn
	paramOptionsColumn
	paramEnvColumn
	paramLabelColumn
	paramRequiredColumn
	paramSecretColumn
)

type gridColumn struct {
	title string
	width int
	// grows takes a share of the room the columns leave.
	grows bool
	// text columns type ? as text rather than open the keys.
	text bool
}

var paramColumns = []gridColumn{
	{title: "type", width: 7},
	{title: "name", width: 10, grows: true},
	{title: "default", width: 10, grows: true, text: true},
	{title: "options", width: 8, grows: true, text: true},
	{title: "from env", width: 8, grows: true},
	{title: "label", width: 8, grows: true, text: true},
	{title: "required", width: 8},
	{title: "secret", width: 6},
}

const paramsTableHints = "{tab} next  {enter} new param  esc close  {?} all keys"

// paramTypeNotes say what each type holds, for the help beside the table.
var paramTypeNotes = map[string]string{
	flow.TextParam:    "any text",
	flow.NumberParam:  "3, -2 or 1.5",
	flow.BooleanParam: "true or false",
	flow.OptionParam:  "one of its options",
	flow.PathParam:    "a file or folder",
	flow.ListParam:    `JSON: ["a", "b"]`,
	flow.ObjectParam:  `JSON: {"key": "value"}`,
}

type paramsTable struct {
	app  *App
	flow flow.Flow
	grid editGrid
	// names are each row's param name in the file, "" for a row not saved yet; edited marks the
	// rows changed since they were saved.
	names  []string
	edited []bool
}

func (a *App) newParamsTable(f flow.Flow) *paramsTable {
	t := &paramsTable{app: a, flow: f}
	for _, name := range slices.Sorted(maps.Keys(f.Params)) {
		t.addRow(name, f.Params[name])
	}
	if len(f.Params) == 0 {
		t.addRow("", flow.Param{})
	}
	return t
}

func (t *paramsTable) addRow(name string, p flow.Param) {
	env, _ := p.EnvName()
	row := make([]gridCell, len(paramColumns))
	row[paramTypeColumn] = optionCellOf(cmp.Or(p.Type, flow.TextParam), flow.ParamTypes)
	row[paramNameColumn] = textCellOf(name)
	row[paramDefaultColumn] = textCellOf(p.Default)
	row[paramOptionsColumn] = textCellOf(strings.Join(p.Choices, " "))
	row[paramEnvColumn] = textCellOf(env)
	row[paramLabelColumn] = textCellOf(p.Prompt)
	row[paramRequiredColumn] = gridCell{kind: toggleCell, on: p.Required}
	row[paramSecretColumn] = gridCell{kind: toggleCell, on: p.Secret}
	t.grid.rows = append(t.grid.rows, row)
	t.names = append(t.names, name)
	t.edited = append(t.edited, false)
	t.fitRow(len(t.grid.rows) - 1)
}

// fitRow makes a row's cells follow its type: options only for an option, and a default picked
// from the values of a boolean or an option.
func (t *paramsTable) fitRow(i int) {
	row := t.grid.rows[i]
	row[paramOptionsColumn].skip = row[paramTypeColumn].value() != flow.OptionParam
	_, p := t.param(i)
	if values := p.Values(); values != nil {
		row[paramDefaultColumn] = optionCellOf(p.Default, append([]string{""}, values...))
		return
	}
	row[paramDefaultColumn] = gridCell{kind: textCell, field: row[paramDefaultColumn].field}
}

// param is the param a row holds, with its name.
func (t *paramsTable) param(i int) (string, flow.Param) {
	row := t.grid.rows[i]
	p := flow.Param{
		Default:  row[paramDefaultColumn].value(),
		Prompt:   strings.TrimSpace(row[paramLabelColumn].value()),
		Required: row[paramRequiredColumn].on,
		Secret:   row[paramSecretColumn].on,
	}
	if kind := row[paramTypeColumn].value(); kind != flow.TextParam {
		p.Type = kind
	}
	if p.Type == flow.OptionParam {
		p.Choices = strings.FieldsFunc(row[paramOptionsColumn].value(), func(r rune) bool { return r == ' ' || r == ',' || r == '|' })
	}
	if env := strings.TrimSpace(row[paramEnvColumn].value()); env != "" {
		p.From = "env:" + env
	}
	return strings.TrimSpace(row[paramNameColumn].value()), p
}

// problems tell, by column, what keeps a row from being saved.
func (t *paramsTable) problems(i int) map[int]string {
	name, p := t.param(i)
	problems := map[int]string{}
	if t.removed(i) {
		return problems // its values go with it
	}
	switch {
	case name == "" && t.names[i] == "" && holdsValues(p):
		problems[paramNameColumn] = "a param needs a name"
	case name == "":
	case !naming.IsIdentifier(name) || name == "steps":
		problems[paramNameColumn] = "a name is letters, digits and _, and not steps"
	case t.nameTaken(i, name):
		problems[paramNameColumn] = "another param is " + name
	}
	if p.Type == flow.OptionParam && len(p.Choices) == 0 {
		problems[paramOptionsColumn] = "an option needs its options, a space between"
	}
	if problem := p.Fits(p.Default); problem != "" {
		problems[paramDefaultColumn] = "the default " + problem
	}
	if env, ok := p.EnvName(); ok && !naming.IsIdentifier(env) {
		problems[paramEnvColumn] = "an environment variable's name is letters, digits and _"
	}
	return problems
}

// holdsValues tells a param with anything typed past its type, which a row without a name loses.
func holdsValues(p flow.Param) bool {
	return p.Default != "" || p.Prompt != "" || p.From != "" || p.Required || p.Secret || len(p.Choices) > 0
}

// nameTaken tells whether another row has the name, or still has it in the file until saved.
func (t *paramsTable) nameTaken(i int, name string) bool {
	for j := range t.grid.rows {
		other, _ := t.param(j)
		if j != i && (other == name || t.names[j] == name) {
			return true
		}
	}
	return false
}

// removed tells a param's row whose name was emptied, which saving removes.
func (t *paramsTable) removed(i int) bool {
	name, _ := t.param(i)
	return name == "" && t.names[i] != ""
}

// saving are the rows a save writes: changed, valid, and named or emptied of a param's name.
func (t *paramsTable) saving() []int {
	var rows []int
	for i := range t.grid.rows {
		name, _ := t.param(i)
		if t.edited[i] && len(t.problems(i)) == 0 && (name != "" || t.names[i] != "") {
			rows = append(rows, i)
		}
	}
	return rows
}

// save writes the rows saving gives, every old name out before the new ones go in, so a row
// can take the name another row gave up in the same save.
func (t *paramsTable) save() {
	rows := t.saving()
	if len(rows) == 0 {
		return
	}
	err := t.app.changeFlow(t.flow, func(f *flow.Flow) error {
		for _, i := range rows {
			delete(f.Params, t.names[i])
		}
		for _, i := range rows {
			if name, p := t.param(i); name != "" {
				if f.Params == nil {
					f.Params = map[string]flow.Param{}
				}
				f.Params[name] = p
			}
		}
		return nil
	})
	if err != nil {
		t.app.statusMessage = err.Error()
		return
	}
	for _, i := range rows {
		t.names[i], _ = t.param(i)
		t.edited[i] = false
	}
	t.dropEmptyRows()
}

// dropEmptyRows takes out the rows without a param, keeping one row to type in.
func (t *paramsTable) dropEmptyRows() {
	for i := len(t.grid.rows) - 1; i >= 0 && len(t.grid.rows) > 1; i-- {
		if name, _ := t.param(i); name == "" && t.names[i] == "" && i != t.grid.row {
			t.grid.rows = slices.Delete(t.grid.rows, i, i+1)
			t.names = slices.Delete(t.names, i, i+1)
			t.edited = slices.Delete(t.edited, i, i+1)
			if i < t.grid.row {
				t.grid.row--
			}
		}
	}
}

// unsaved counts the rows with changes a save could not write, for a problem or a flow that
// refused them.
func (t *paramsTable) unsaved() int {
	count := 0
	for i := range t.grid.rows {
		name, p := t.param(i)
		if t.edited[i] && (name != "" || t.names[i] != "" || holdsValues(p)) {
			count++
		}
	}
	return count
}

func (t *paramsTable) title() string { return "params of " + t.flow.Name }

func (t *paramsTable) hints() string { return paramsTableHints }

func (t *paramsTable) handleKey(key uv.KeyPressEvent) bool {
	name := key.String()
	switch {
	case name == "esc":
		t.save()
		if n := t.unsaved(); n > 0 {
			t.app.notice = plural(n, "param") + " not saved: open the params again to fix the cells in red"
		}
		return true
	case name == "?" && !paramColumns[t.grid.col].text:
		t.app.showContextKeys("flow-editor", "params keys",
			"a letter picks a type, space the next one; space flips required and secret",
			"each row is saved as you leave it; red cells keep it from being saved",
			"emptying a name removes its param")
		return false
	case name == "enter":
		t.save()
		t.addRow("", flow.Param{})
		t.grid.focus(len(t.grid.rows)-1, paramTypeColumn)
		return false
	}
	row := t.grid.row
	if t.grid.handleKey(key) {
		t.edited[row] = true
		t.fitRow(row)
	}
	if t.grid.row != row {
		t.save()
	}
	return false
}

func (t *paramsTable) paste(text string) {
	if t.grid.paste(text) {
		t.edited[t.grid.row] = true
	}
}

func (t *paramsTable) draw(screen uv.Screen, area image.Rectangle, _ bool) {
	pal := &t.app.palette
	widths := columnWidths(paramColumns, area.Dx()-2)
	from, to := visibleColumns(widths, area.Dx()-2, t.grid.col)
	x := area.Min.X + 1
	for c := from; c < to; c++ {
		drawText(screen, x, area.Min.Y, x+widths[c], paramColumns[c].title, pal.panelMuted)
		x += widths[c] + 2
	}
	rows := max(area.Dy()-3, 1)
	first := max(t.grid.row-rows+1, 0)
	for i := first; i < len(t.grid.rows) && i-first < rows; i++ {
		t.drawRow(screen, area.Min.Y+1+i-first, area.Min.X+1, i, widths, from, to)
	}
	if problems := t.problems(t.grid.row); len(problems) > 0 {
		column := slices.Min(slices.Collect(maps.Keys(problems)))
		drawTextCut(screen, area.Min.X+1, area.Max.Y-1, area.Max.X-1, "✗ "+paramColumns[column].title+": "+problems[column], pal.failure)
	} else if t.removed(t.grid.row) {
		drawTextCut(screen, area.Min.X+1, area.Max.Y-1, area.Max.X-1, "removes "+t.names[t.grid.row]+" as you leave the row", pal.panelMuted)
	}
}

func (t *paramsTable) drawRow(screen uv.Screen, y, x, i int, widths []int, from, to int) {
	pal := &t.app.palette
	problems := t.problems(i)
	for c := from; c < to; c++ {
		cell := &t.grid.rows[i][c]
		style := pal.panelText
		switch {
		case cell.skip || t.removed(i):
			style = pal.panelMuted
		case problems[c] != "":
			style = pal.failure
		case i == t.grid.row && c == t.grid.col:
			style = pal.typing()
		}
		focused := i == t.grid.row && c == t.grid.col
		if focused && cell.kind == textCell {
			secret := c == paramDefaultColumn && t.grid.rows[i][paramSecretColumn].on
			cell.field.draw(screen, x, y, widths[c], secret, style, pal.selectedRow)
		} else {
			drawTextCut(screen, x, y, x+widths[c], t.cellText(i, c), style)
		}
		x += widths[c] + 2
	}
}

// cellText is how a cell shows when it is not being typed in.
func (t *paramsTable) cellText(i, c int) string {
	cell := &t.grid.rows[i][c]
	switch {
	case cell.skip:
		return "–"
	case cell.kind == toggleCell:
		return map[bool]string{true: "✓", false: "·"}[cell.on]
	case c == paramDefaultColumn && t.grid.rows[i][paramSecretColumn].on:
		return strings.Repeat("•", len(cell.field.text))
	}
	return cell.value()
}

func (t *paramsTable) help() []styledLine {
	pal := &t.app.palette
	name, p := t.param(t.grid.row)
	var lines []styledLine
	add := func(style uv.Style, texts ...string) {
		for _, text := range texts {
			lines = append(lines, styledLine{text, style})
		}
	}
	switch t.grid.col {
	case paramTypeColumn:
		add(pal.panelMuted, "a letter picks a type, space the next", "")
		for _, kind := range flow.ParamTypes {
			add(pal.panelText, kind+"  "+paramTypeNotes[kind])
		}
	case paramNameColumn:
		add(pal.panelText, "steps read it as {{."+cmp.Or(name, "name")+"}}")
		add(pal.panelMuted, "letters, digits and _")
	case paramDefaultColumn:
		add(pal.panelText, "the value when the run gives none: "+paramTypeNotes[cmp.Or(p.Type, flow.TextParam)])
		add(pal.panelMuted, defaultNote(p))
	case paramOptionsColumn:
		add(pal.panelText, "the values it takes, a space between: dev prod")
	case paramEnvColumn:
		add(pal.panelText, "reads this environment variable when it is set, and the run form does not ask")
		add(pal.panelMuted, environmentMatches(t.grid.cell().value())...)
	case paramLabelColumn:
		add(pal.panelText, "what the run form shows instead of the name")
	case paramRequiredColumn:
		add(pal.panelText, "the run refuses it empty; space flips it")
	case paramSecretColumn:
		add(pal.panelText, "masked in the form and never remembered; space flips it")
	}
	if name != "" {
		add(pal.panelText, "")
		add(pal.panelMuted, readingExamples(name, p.Type)...)
	}
	return lines
}

// defaultNote tells how a default of the param's type is given.
func defaultNote(p flow.Param) string {
	switch p.Type {
	case flow.BooleanParam, flow.OptionParam:
		return "space picks the next, or none"
	case flow.ListParam, flow.ObjectParam:
		return "lists and objects hold each other: {\"db\": {\"tags\": [\"a\"]}}"
	}
	return "empty has no default"
}

// readingExamples show how steps read a param of the type.
func readingExamples(name, kind string) []string {
	switch kind {
	case flow.ListParam:
		return []string{"{{index ." + name + " 0}} the first", "{{range ." + name + "}}{{.}} {{end}} each", "{{." + name + "}} the whole list as JSON"}
	case flow.ObjectParam:
		return []string{"{{." + name + ".key}} a field, at any depth", "{{." + name + "}} the whole object as JSON"}
	case flow.BooleanParam:
		return []string{"if: {{." + name + "}}"}
	}
	return []string{"{{quote ." + name + "}} in a command line"}
}

// maxEnvironmentMatches is how many environment variables the help lists.
const maxEnvironmentMatches = 6

// environmentMatches are the variables set now whose names start as typed.
func environmentMatches(typed string) []string {
	var names []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if typed != "" && strings.HasPrefix(name, strings.ToUpper(typed)) || typed == name {
			names = append(names, "$"+name)
		}
	}
	slices.Sort(names)
	return names[:min(len(names), maxEnvironmentMatches)]
}

// columnWidths share the room past the columns' own widths among the ones that grow.
func columnWidths(columns []gridColumn, room int) []int {
	widths := make([]int, len(columns))
	growing, used := 0, 0
	for i, c := range columns {
		widths[i] = c.width
		used += c.width + 2
		if c.grows {
			growing++
		}
	}
	extra := room - used + 2
	for i, c := range columns {
		if c.grows && extra > 0 {
			widths[i] += extra / growing
		}
	}
	return widths
}

// visibleColumns are the columns from and to, past the last, that fit room with the focused one,
// as a long line scrolls sideways to its cursor.
func visibleColumns(widths []int, room, focus int) (int, int) {
	span := func(from, to int) int {
		total := 0
		for _, w := range widths[from:to] {
			total += w + 2
		}
		return total - 2
	}
	from := 0
	for from < focus && span(from, focus+1) > room {
		from++
	}
	to := focus + 1
	for to < len(widths) && span(from, to+1) <= room {
		to++
	}
	return from, to
}

// paramText tells a param's type and default, as its card shows it, like a declaration.
func paramText(p flow.Param) string {
	kind := cmp.Or(p.Type, flow.TextParam)
	switch {
	case p.From != "":
		kind += " from $" + strings.TrimPrefix(p.From, "env:")
	case p.Secret:
		kind = "secret " + kind
	case p.Type == flow.OptionParam:
		kind += " " + strings.Join(p.Choices, "|")
	}
	text := kind
	if p.Default != "" && !p.Secret {
		text += " = " + p.Default
	}
	if p.Required {
		text += " required"
	}
	return text
}
