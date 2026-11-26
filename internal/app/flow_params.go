package app

import (
	"cmp"
	"encoding/json"
	"image"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/naming"
)

// A flow's params are edited in the inspect as a table, a param a row, like declarations in code:
// type, name, default and the rest in columns. A row is saved as it changes, when it is valid, in
// the order the rows are in; a cell in the failure color tells what keeps it from being saved.

const (
	paramTypeColumn = iota
	paramNameColumn
	paramDefaultColumn
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
}

var paramColumns = []gridColumn{
	{title: "type", width: 7},
	{title: "name", width: 8, grows: true},
	{title: "value", width: 20, grows: true},
	{title: "from env", width: 8, grows: true},
	{title: "label", width: 8, grows: true},
	{title: "required", width: 8},
	{title: "secret", width: 6},
}

// moveGutter is the room before a row, where the focused one has the arrows that move it.
const moveGutter = moveArrowsWidth + 2

// paramTypeNotes say what each type holds, for the help beside the table.
var paramTypeNotes = map[string]string{
	flow.VarType:      "any; JSON is read inside",
	flow.TextParam:    "any text",
	flow.NumberParam:  "3, -2 or 1.5",
	flow.BooleanParam: "true or false",
	flow.OptionParam:  "one of its options",
	flow.PathParam:    "a file or folder",
	flow.ListParam:    `JSON: ["a", "b"]`,
	flow.ObjectParam:  `JSON: {"key": "value"}`,
}

var booleanOptions = append([]string{""}, flow.BooleanValues...)

type paramsTable struct {
	app  *App
	flow flow.Flow
	grid editGrid
	// seen are the names the table has had, so a save keeps the params added to the file since.
	seen map[string]bool
}

func (a *App) newParamsTable(f flow.Flow) *paramsTable {
	t := &paramsTable{app: a, flow: f, seen: map[string]bool{}}
	t.grid.buttons, t.grid.arranged = []string{"+ add variable"}, true
	for _, name := range f.ParamNames() {
		t.grid.rows = append(t.grid.rows, paramRow(name, f.Params[name]))
		t.seen[name] = true
	}
	return t
}

func paramRow(name string, p flow.Param) gridRow {
	env, _ := p.EnvName()
	cells := make([]gridCell, len(paramColumns))
	cells[paramTypeColumn] = optionCellOf(cmp.Or(p.Type, flow.VarType), flow.ParamTypes)
	cells[paramNameColumn] = textCellOf(name)
	cells[paramDefaultColumn] = defaultCell(p)
	cells[paramEnvColumn] = textCellOf(env)
	cells[paramLabelColumn] = textCellOf(p.Prompt)
	cells[paramRequiredColumn] = gridCell{kind: toggleCell, on: p.Required}
	cells[paramSecretColumn] = gridCell{kind: toggleCell, on: p.Secret}
	return gridRow{cells: cells, key: name}
}

// defaultCell is how a param's default is given for its type: picked for a boolean, marked
// among the options of an option, as items for a list, typed for the rest.
func defaultCell(p flow.Param) gridCell {
	var cell gridCell
	switch items, isList := flow.ListItems(p.Default); {
	case p.Type == flow.BooleanParam:
		cell = optionCellOf(p.Default, booleanOptions)
	case p.Type == flow.OptionParam:
		cell = listCellOf(p.Choices, slices.Index(p.Choices, p.Default), true, "option")
		cell.field = newTextField(p.Default) // kept when it is no option, to be told and fixed
	case p.Type == flow.ListParam && isList:
		cell = listCellOf(items, -1, false, "item")
	default:
		cell = textCellOf(p.Default)
		cell.template = true // a default reads the variables before it: {"user": "$user"}
	}
	cell.secret = p.Secret
	return cell
}

// fitDefault remakes a row's default when its type wants another kind of cell.
func fitDefault(row gridRow) {
	cells := row.cells
	kind := cells[paramTypeColumn].value()
	want := defaultCell(flow.Param{Type: kind})
	if old := cells[paramDefaultColumn]; want.kind != old.kind || want.marking != old.marking {
		cells[paramDefaultColumn] = convertDefault(old, kind)
	}
	cells[paramDefaultColumn].secret = cells[paramSecretColumn].on
}

// convertDefault makes a default of the type from what old held: a text becomes the item of a
// list or an option, a list's items the options of an option, and back.
func convertDefault(old gridCell, kind string) gridCell {
	text, items, marked := old.value(), old.items, -1
	switch {
	case old.kind == listCell && !old.marking:
		text = flow.ItemsJSON(old.items)
	case old.kind == listCell:
	case kind == flow.ListParam:
		if decoded, ok := flow.ListItems(text); ok {
			items = decoded
			break
		}
		fallthrough
	case text != "":
		items, marked = []string{text}, 0
	}
	switch kind {
	case flow.OptionParam:
		return listCellOf(items, marked, true, "option")
	case flow.ListParam:
		return listCellOf(items, -1, false, "item")
	case flow.BooleanParam:
		return optionCellOf(map[bool]string{true: text}[slices.Contains(flow.BooleanValues, text)], booleanOptions)
	}
	return textCellOf(text)
}

// param is the param a row holds, with its name.
func (t *paramsTable) param(i int) (string, flow.Param) {
	cells := t.grid.rows[i].cells
	p := flow.Param{
		Prompt:   strings.TrimSpace(cells[paramLabelColumn].value()),
		Required: cells[paramRequiredColumn].on,
		Secret:   cells[paramSecretColumn].on,
	}
	if kind := cells[paramTypeColumn].value(); kind != flow.VarType {
		p.Type = kind
	}
	switch def := &cells[paramDefaultColumn]; {
	case def.kind == listCell && def.marking:
		p.Choices, p.Default = def.items, def.markedItem()
		if def.marked < 0 {
			p.Default = def.field.String()
		}
	case def.kind == listCell:
		p.Default = flow.ItemsJSON(def.items)
	default:
		p.Default = def.value()
	}
	if env := strings.TrimSpace(cells[paramEnvColumn].value()); env != "" {
		p.From = "env:" + env
	}
	return strings.TrimSpace(cells[paramNameColumn].value()), p
}

// problems tell, by column, what keeps a row from being saved.
func (t *paramsTable) problems(i int) map[int]string {
	name, p := t.param(i)
	problems := map[int]string{}
	switch {
	case name == "" && (t.grid.rows[i].key != "" || holdsValues(p)):
		problems[paramNameColumn] = "a variable needs a name; ctrl+delete removes it"
	case name == "":
	case !naming.IsIdentifier(name) || name == "steps":
		problems[paramNameColumn] = "a name is letters, digits and _, and not steps"
	case t.nameTaken(i, name):
		problems[paramNameColumn] = "another variable is " + name
	}
	if problem := optionsProblem(p); problem != "" {
		problems[paramDefaultColumn] = problem
	} else if problem := p.DefaultProblem(); problem != "" {
		problems[paramDefaultColumn] = "the value " + problem
	}
	if env, ok := p.EnvName(); ok && !naming.IsIdentifier(env) {
		problems[paramEnvColumn] = "an environment variable's name is letters, digits and _"
	}
	return problems
}

func optionsProblem(p flow.Param) string {
	if p.Type != flow.OptionParam {
		return ""
	}
	if len(p.Choices) == 0 {
		return "an option needs its options"
	}
	for i, choice := range p.Choices {
		if slices.Contains(p.Choices[:i], choice) {
			return choice + " is an option twice"
		}
	}
	return ""
}

// holdsValues tells a param with anything given past its type, which a row without a name loses.
func holdsValues(p flow.Param) bool {
	return p.Default != "" || p.Prompt != "" || p.From != "" || p.Required || p.Secret || len(p.Choices) > 0
}

// nameTaken tells whether another row has the name, or keeps it in the file until it is fixed.
func (t *paramsTable) nameTaken(i int, name string) bool {
	for j := range t.grid.rows {
		other, _ := t.param(j)
		if j != i && (other == name || t.grid.rows[j].key == name) {
			return true
		}
	}
	return false
}

// save writes the params as the rows hold them, in their order.
func (t *paramsTable) save() {
	var saved map[int]string
	err := t.app.changeFlow(t.flow, func(f *flow.Flow) error {
		var params map[string]flow.Param
		params, f.ParamOrder, saved = t.paramsFor(*f)
		f.Params = params
		if len(params) == 0 {
			f.Params, f.ParamOrder = nil, nil
		}
		return nil
	})
	if err != nil {
		t.app.statusMessage = err.Error()
		return
	}
	for i, name := range saved {
		t.grid.rows[i].key, t.seen[name] = name, true
	}
}

// paramsFor are the params the rows make of f's, in order, and the name each row is saved
// under. A row with a problem keeps its param as f has it, and a param added to f while the
// table was open stays, last.
func (t *paramsTable) paramsFor(f flow.Flow) (map[string]flow.Param, []string, map[int]string) {
	params, order, saved := map[string]flow.Param{}, []string{}, map[int]string{}
	add := func(name string, p flow.Param) {
		params[name], order = p, append(order, name)
	}
	for i, row := range t.grid.rows {
		name, p := t.param(i)
		if len(t.problems(i)) > 0 {
			kept, ok := f.Params[row.key]
			if !ok {
				continue
			}
			name, p = row.key, kept
		}
		if name != "" {
			add(name, p)
			saved[i] = name
		}
	}
	for _, name := range f.ParamNames() {
		if _, ok := params[name]; !ok && !t.seen[name] {
			add(name, f.Params[name])
		}
	}
	return params, order, saved
}

// unsaved counts the rows a problem keeps from being saved.
func (t *paramsTable) unsaved() int {
	count := 0
	for i := range t.grid.rows {
		if len(t.problems(i)) > 0 {
			count++
		}
	}
	return count
}

func (t *paramsTable) title() string { return "variables of " + t.flow.Name }

func (t *paramsTable) hints() string { return t.grid.hints("{enter} edit  esc close  {?} keys") }

func (t *paramsTable) typing() bool { return t.grid.typing() }

func (t *paramsTable) hitAt(at image.Point) (image.Rectangle, bool, bool) { return t.grid.hitAt(at) }

func (t *paramsTable) handleKey(key uv.KeyPressEvent) bool {
	if !t.grid.busy() {
		switch key.String() {
		case "esc":
			if n := t.unsaved(); n > 0 {
				t.app.notice = plural(n, "variable") + " not saved: open the variables again to fix the cells in red"
			}
			return true
		case "?":
			t.app.showContextKeys("flow-editor", "variables keys", "rows are saved as they change; a red cell keeps its row as it was")
			return false
		}
	}
	t.grid.variables = t.paramsBefore(t.grid.row)
	t.after(t.grid.handleKey(key))
	return false
}

// paramsBefore are the params of the rows above row, which a default typed in it can read.
func (t *paramsTable) paramsBefore(row int) []flow.Variable {
	var vars []flow.Variable
	for i := range min(row, len(t.grid.rows)) {
		if name, _ := t.param(i); name != "" {
			vars = append(vars, flow.Variable{Field: name, Certainty: flow.Set})
		}
	}
	return vars
}

func (t *paramsTable) mouse(event uv.MouseEvent, at image.Point) bool {
	t.after(t.grid.mouse(event, at))
	return false
}

// after adds a row when + add param is pressed, its name typed at once, and saves a change.
func (t *paramsTable) after(out gridOutcome) {
	if out&gridPressed != 0 {
		t.addParam()
	}
	if out&gridChanged != 0 {
		if t.grid.row < len(t.grid.rows) {
			fitDefault(t.grid.rows[t.grid.row])
		}
		t.save()
	}
}

// addParam adds a row last, its name typed in at once.
func (t *paramsTable) addParam() {
	t.grid.addRow(paramRow("", flow.Param{}))
	t.grid.focus(len(t.grid.rows)-1, paramNameColumn)
	t.grid.startEdit()
}

func (t *paramsTable) paste(text string) { t.grid.paste(text) }

func (t *paramsTable) takeCopied() string { return t.grid.takeCopied() }

func (t *paramsTable) draw(screen uv.Screen, area image.Rectangle, _ bool) {
	pal := &t.app.palette
	t.grid.hits = t.grid.hits[:0]
	room := area.Dx() - 1 - moveGutter
	widths := columnWidths(paramColumns, room)
	focus := t.grid.col
	if _, onButton := t.grid.button(); onButton {
		focus = 0
	}
	from, to := visibleColumns(widths, room, focus)
	left := area.Min.X + moveGutter
	x := left
	for c := from; c < to; c++ {
		drawText(screen, x, area.Min.Y, x+widths[c], paramColumns[c].title, pal.panelMuted)
		x += widths[c] + 2
	}
	y := area.Min.Y + 1
	for _, row := range t.grid.window(area.Dy() - 2) {
		line := image.Rect(left, y, area.Max.X-1, y+1)
		if b, ok := t.grid.buttonAt(row); ok {
			t.grid.drawButton(screen, line, row, b, pal)
		} else {
			if row == t.grid.row {
				t.grid.drawMoveArrows(screen, image.Pt(area.Min.X+1, y), pal)
			}
			t.drawRow(screen, line, row, widths, from, to)
		}
		y++
	}
	t.drawProblem(screen, area)
	t.grid.drawOverlay(screen, area, pal)
}

func (t *paramsTable) drawRow(screen uv.Screen, line image.Rectangle, i int, widths []int, from, to int) {
	pal := &t.app.palette
	problems := t.problems(i)
	x := line.Min.X
	for c := from; c < to; c++ {
		style := pal.panelText
		switch {
		case t.grid.at(i, c).skip:
			style = pal.panelMuted
		case problems[c] != "":
			style = pal.failure
		}
		t.grid.drawCell(screen, image.Rect(x, line.Min.Y, x+widths[c], line.Max.Y), i, c, t.cellText(i, c), style, pal)
		x += widths[c] + 2
	}
}

// drawProblem tells, on the last line, what keeps the focused row from being saved.
func (t *paramsTable) drawProblem(screen uv.Screen, area image.Rectangle) {
	if t.grid.row >= len(t.grid.rows) {
		return
	}
	if problems := t.problems(t.grid.row); len(problems) > 0 {
		column := slices.Min(slices.Collect(maps.Keys(problems)))
		drawTextCut(screen, area.Min.X+1, area.Max.Y-1, area.Max.X-1, "✗ "+paramColumns[column].title+": "+problems[column], t.app.palette.failure)
	}
}

// cellText is how a cell shows when it is not being typed in.
func (t *paramsTable) cellText(i, c int) string {
	cell := t.grid.at(i, c)
	switch {
	case cell.skip:
		return "–"
	case cell.kind == toggleCell:
		return map[bool]string{true: "✓", false: "·"}[cell.on]
	case cell.kind == listCell:
		return listText(cell)
	case cell.secret:
		return strings.Repeat("•", len(cell.field.text))
	}
	return cell.value()
}

// listText shows a list cell in a row: its marked item, or its first, and how many items it
// has, all of them counted.
func listText(c *gridCell) string {
	count := plural(len(c.items), c.noun)
	switch {
	case len(c.items) == 0:
		return "no " + c.noun + "s"
	case c.marking && c.marked < 0 && len(c.field.text) > 0:
		return strconv.Quote(c.field.String()) + " · " + count
	case c.marking && c.marked < 0:
		return count
	case c.marking:
		return itemText(c.items[c.marked]) + " · " + count
	}
	return itemText(c.items[0]) + " · " + count
}

// itemText is an item quoted as a string, or as its JSON.
func itemText(item string) string {
	if json.Valid([]byte(item)) {
		return item
	}
	return strconv.Quote(item)
}

func (t *paramsTable) help() []styledLine {
	pal := &t.app.palette
	var name string
	var p flow.Param
	if t.grid.row < len(t.grid.rows) {
		name, p = t.param(t.grid.row)
	}
	var lines []styledLine
	add := func(style uv.Style, texts ...string) {
		for _, text := range texts {
			lines = append(lines, styledLine{text, style})
		}
	}
	switch {
	case t.grid.row >= len(t.grid.rows):
		add(pal.panelText, "a variable is a value the steps read, given when the flow runs")
	case t.grid.col == paramTypeColumn:
		for _, kind := range flow.ParamTypes {
			add(pal.panelText, kind+"  "+paramTypeNotes[kind])
		}
	case t.grid.col == paramNameColumn:
		add(pal.panelText, "steps read it as $"+cmp.Or(name, "name"))
		add(pal.panelMuted, "letters, digits and _")
	case t.grid.col == paramDefaultColumn:
		add(pal.panelText, "the value when the run gives none: "+paramTypeNotes[cmp.Or(p.Type, flow.VarType)])
		add(pal.panelMuted, defaultNote(p))
	case t.grid.col == paramEnvColumn:
		add(pal.panelText, "reads this environment variable when it is set, and the run form does not ask")
		add(pal.panelMuted, environmentMatches(t.grid.cell().value())...)
	case t.grid.col == paramLabelColumn:
		add(pal.panelText, "what the run form shows instead of the name")
	case t.grid.col == paramRequiredColumn:
		add(pal.panelText, "the run refuses it empty")
	case t.grid.col == paramSecretColumn:
		add(pal.panelText, "masked in the form and never remembered")
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
	case flow.BooleanParam:
		return "none leaves it empty"
	case flow.OptionParam:
		return "its options, ★ on the one it holds"
	case flow.ListParam:
		return "its items, each text or JSON"
	case flow.ObjectParam:
		return "lists and objects hold each other: {\"db\": {\"tags\": [\"a\"]}}"
	}
	return "empty holds none"
}

// readingExamples show how steps read a param of the type.
func readingExamples(name, kind string) []string {
	switch kind {
	case flow.ListParam:
		return []string{"$" + name + "[0] the first", "$join(" + name + ", \" \") its items as one text", "$" + name + " the whole list as JSON", "repeat each: " + name + " runs once an item"}
	case flow.ObjectParam:
		return []string{"$" + name + ".key a field, at any depth", "$" + name + " the whole object as JSON"}
	case flow.BooleanParam:
		return []string{"if: " + name, "if: not " + name}
	}
	return []string{"$" + name + " in a text, one shell word in a command line", "if: " + name + ` == "value"`}
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
	kind := cmp.Or(p.Type, flow.VarType)
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
