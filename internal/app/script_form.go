package app

import (
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/script"
)

const scriptFormHints = "{enter} edit  {tab} next  {ctrl+s} save  esc cancel"

// The form's fields, in tab order: two typed on their line, then four parts edited in the editor,
// each alone, shown in boxes two by two.
const (
	nameField = iota
	programField
	runPart
	descriptionPart
	paramsPart
	notesPart
	scriptFieldCount
)

// partFiles name the file each part is edited in, so the editor highlights it.
var partFiles = map[int]string{runPart: "script.sh", descriptionPart: "description.txt", paramsPart: "params.yaml", notesPart: "notes.md"}

var partTitles = map[int]string{runPart: "script", descriptionPart: "description", paramsPart: "params", notesPart: "notes"}

// Arrows move between the boxes as they are laid out; tab follows the order above.
var (
	formDown  = map[int]int{nameField: programField, programField: runPart, runPart: paramsPart, descriptionPart: notesPart, paramsPart: nameField, notesPart: nameField}
	formUp    = map[int]int{nameField: paramsPart, programField: nameField, runPart: programField, descriptionPart: programField, paramsPart: runPart, notesPart: descriptionPart}
	formRight = map[int]int{runPart: descriptionPart, paramsPart: notesPart}
	formLeft  = map[int]int{descriptionPart: runPart, notesPart: paramsPart}
)

// scriptForm edits a script, or creates one, part by part: enter opens the selected part alone in
// the editor, and ctrl+s saves them all.
type scriptForm struct {
	// original is the script being edited, nil for a new one.
	original *script.Script
	// folder is where a new script goes, nil for the scripts folder itself.
	folder   *script.Folder
	fields   [scriptFieldCount]string
	selected int
	// tempDir holds the files parts are edited in while the form is open.
	tempDir string
	// id tells this form apart from a later one when an editor it opened exits.
	id int
}

func (a *App) openScriptForm(original *script.Script, folder *script.Folder) {
	a.formsOpened++
	f := scriptForm{original: original, folder: folder, id: a.formsOpened}
	fields := script.Fields{}
	if original != nil {
		fields = script.FieldsOf(*original)
	}
	f.fields = [scriptFieldCount]string{fields.Name, fields.Program, fields.Run, fields.Description, script.FormatParams(fields.Params), fields.Notes}
	a.scriptForm = f
	a.mode = scriptFormMode
}

func isPart(field int) bool {
	return field >= runPart
}

func (a *App) handleScriptFormKey(key uv.KeyPressEvent) {
	f := &a.scriptForm
	name := key.String()
	switch name {
	case "esc":
		a.closeScriptForm()
	case "ctrl+s":
		a.saveScriptForm()
	case "enter":
		if isPart(f.selected) {
			a.editPart(f.selected)
		} else {
			f.selected++
		}
	case "tab":
		f.selected = (f.selected + 1) % scriptFieldCount
	case "shift+tab":
		f.selected = (f.selected + scriptFieldCount - 1) % scriptFieldCount
	case "down", "up", "left", "right":
		moves := map[string]map[int]int{"down": formDown, "up": formUp, "left": formLeft, "right": formRight}[name]
		if next, ok := moves[f.selected]; ok {
			f.selected = next
		}
	default:
		a.typeInScriptForm(key)
	}
}

// typeInScriptForm edits the name and program in place; the parts are edited in the editor.
func (a *App) typeInScriptForm(key uv.KeyPressEvent) {
	f := &a.scriptForm
	if isPart(f.selected) {
		return
	}
	text := &f.fields[f.selected]
	switch key.String() {
	case "backspace":
		*text = string(deleteLastRune([]rune(*text)))
	case "ctrl+u":
		*text = ""
	default:
		if isPlainText(key) {
			*text += key.Text
		}
	}
}

// editPart opens one part of the script alone in the editor, beside the focused pane. When the
// editor exits, the part takes what was saved and the form comes back.
func (a *App) editPart(part int) {
	f := &a.scriptForm
	if f.tempDir == "" {
		dir, err := os.MkdirTemp("", "noct-script-")
		if err != nil {
			a.statusMessage = err.Error()
			return
		}
		f.tempDir = dir
	}
	file := filepath.Join(f.tempDir, partFiles[part])
	if err := os.WriteFile(file, []byte(f.fields[part]), 0o600); err != nil {
		a.statusMessage = err.Error()
		return
	}
	argv, ok := commandArgv(a.config.Editor, file)
	if !ok {
		a.statusMessage = "editor " + a.config.Editor + " not found; choose another in the config"
		return
	}
	id := f.id
	a.mode = normalMode
	a.openPanel(func(pane layout.PaneID, frame image.Rectangle) (view, error) {
		return a.launch(program{
			argv:    argv,
			dir:     f.tempDir,
			logName: "edit-" + partFiles[part],
			onExit:  func(int) { a.partEdited(id, pane, part, file) },
		}, contentSize(frame))
	})
}

// partEdited takes the edited part back into the form it came from, if that form is still open.
func (a *App) partEdited(formID int, pane layout.PaneID, part int, file string) {
	a.detachView(pane)
	if a.scriptForm.id != formID || a.scriptForm.tempDir == "" {
		return
	}
	content, err := os.ReadFile(file)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	text := string(content)
	if part != notesPart {
		text = strings.TrimRight(text, "\n")
	}
	a.scriptForm.fields[part] = text
	if a.mode == normalMode {
		a.mode = scriptFormMode
	}
}

// closeScriptForm leaves the form, removing the files its parts were edited in.
func (a *App) closeScriptForm() {
	if a.scriptForm.tempDir != "" {
		_ = os.RemoveAll(a.scriptForm.tempDir) // leftovers in the temp folder harm nothing
	}
	a.scriptForm = scriptForm{}
	a.mode = normalMode
}

func (f *scriptForm) values() (script.Fields, error) {
	params, err := script.ParseParams(f.fields[paramsPart])
	if err != nil {
		return script.Fields{}, err
	}
	return script.Fields{
		Name:        strings.TrimSpace(f.fields[nameField]),
		Program:     strings.TrimSpace(f.fields[programField]),
		Run:         strings.TrimSpace(f.fields[runPart]),
		Description: strings.TrimSpace(f.fields[descriptionPart]),
		Params:      params,
		Notes:       f.fields[notesPart],
	}, nil
}

// saveScriptForm creates or updates the script; problems stay on screen with the form open.
func (a *App) saveScriptForm() {
	f := &a.scriptForm
	values, err := f.values()
	var saved script.Script
	switch {
	case err != nil:
	case f.original == nil:
		root := a.scriptsDir()
		if f.folder != nil {
			root = f.folder.Dir
		}
		saved, err = script.Create(root, values, a.config.ScriptHints)
	default:
		if err = script.Backup(*f.original); err == nil {
			saved, err = script.Update(*f.original, values)
		}
	}
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	if f.original != nil && saved.Name != f.original.Name {
		if err := a.memory.Rename(f.original.ID(), saved.ID()); err != nil {
			a.statusMessage = err.Error()
		}
	}
	a.closeScriptForm()
	a.reloadScripts()
}

func (a *App) drawScriptForm(screen uv.Screen) {
	f := &a.scriptForm
	title := "new script"
	switch {
	case f.original != nil:
		title = "edit " + f.original.ID()
	case f.folder != nil:
		title = "new script in " + f.folder.Name
	}
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, min(area.Dx()-4, 110), max(area.Dy()-4, 16)), title)
	a.drawTypedField(screen, inner, inner.Min.Y, "name", nameField)
	a.drawTypedField(screen, inner, inner.Min.Y+1, "program", programField)

	boxes := image.Rect(inner.Min.X, inner.Min.Y+3, inner.Max.X, inner.Max.Y-2)
	middleX, middleY := boxes.Min.X+boxes.Dx()/2, boxes.Min.Y+boxes.Dy()/2
	a.drawPartBox(screen, image.Rect(boxes.Min.X, boxes.Min.Y, middleX, middleY), runPart)
	a.drawPartBox(screen, image.Rect(middleX, boxes.Min.Y, boxes.Max.X, middleY), descriptionPart)
	a.drawPartBox(screen, image.Rect(boxes.Min.X, middleY, middleX, boxes.Max.Y), paramsPart)
	a.drawPartBox(screen, image.Rect(middleX, middleY, boxes.Max.X, boxes.Max.Y), notesPart)
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, a.hint("script-form", scriptFormHints), a.palette.panelMuted)
}

func (a *App) drawTypedField(screen uv.Screen, inner image.Rectangle, y int, label string, field int) {
	style, value := a.palette.panelText, a.scriptForm.fields[field]
	if a.scriptForm.selected == field {
		style, value = a.palette.selectedRow, value+"▏"
	}
	fillRect(screen, image.Rect(inner.Min.X, y, inner.Max.X, y+1), style)
	drawText(screen, inner.Min.X+1, y, inner.Max.X, label, style)
	drawText(screen, inner.Min.X+11, y, inner.Max.X-1, value, style)
}

// drawPartBox shows a part in a frame of its own, highlighted while selected.
func (a *App) drawPartBox(screen uv.Screen, box image.Rectangle, part int) {
	lineStyle, titleStyle := a.palette.unfocusedFrame, a.palette.unfocusedTitle
	if a.scriptForm.selected == part {
		lineStyle, titleStyle = a.palette.focusedFrame, a.palette.focusedTitle
	}
	title := partTitles[part]
	if part == paramsPart {
		title += a.paramsCount()
	}
	drawFrame(screen, box, title, lineStyle, titleStyle)
	content := contentRect(box)
	content.Min.X++
	text := a.scriptForm.fields[part]
	if part == descriptionPart || part == notesPart {
		text = ansi.Wrap(text, max(content.Dx(), 1), "")
	}
	for i, line := range strings.Split(text, "\n") {
		if content.Min.Y+i >= content.Max.Y {
			break
		}
		drawText(screen, content.Min.X, content.Min.Y+i, content.Max.X, line, a.palette.panelText)
	}
}

// paramsCount says how many parameters the params box holds, or that it does not parse.
func (a *App) paramsCount() string {
	params, err := script.ParseParams(a.scriptForm.fields[paramsPart])
	if err != nil {
		return " (invalid)"
	}
	return " (" + strconv.Itoa(len(params)) + ")"
}
