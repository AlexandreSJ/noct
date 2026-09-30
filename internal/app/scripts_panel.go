package app

import (
	"fmt"
	"image"
	"os"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/script"
)

// minDetailsWidth is the narrowest panel that shows the selected script's details beside the list.
const minDetailsWidth = 70

const scriptsHints = "{enter} run  {e} edit  {n} new  {P} promote draft  {a} add folder  {ctrl+o} open  {delete} delete  {ctrl+d} close"

// scriptsPanel lists the saved scripts, with the selected one's details and notes beside them.
// Folders come first, closed until opened, each followed by its scripts behind a bar. It is where the
// multiplexer ends and noct's own features begin; flows join it in phase 2.
type scriptsPanel struct {
	app *App
	// close removes the panel from its tab.
	close    func()
	selected int
	// selectedKey and seenLoad keep the selection on the same row when the list is read again,
	// sorted in another order or a folder collapses.
	selectedKey string
	seenLoad    int

	// list is where the list was last drawn, relative to the panel, and first the row on its
	// top line: what a click lands on. height is how many rows it showed, a page for pgup.
	list   image.Rectangle
	first  int
	height int

	// notes caches the selected script's notes, read again when the selection or library changes.
	notesOf     string
	notesLoaded int
	notes       string
}

// panelRow is a line of the list: a folder (script nil), or a script, inside folder when set.
type panelRow struct {
	folder *script.Folder
	script *script.Script
}

func (r panelRow) key() string {
	if r.script != nil {
		return r.script.Dir
	}
	return "folder:" + r.folder.Dir
}

// rows are the lines of the list: each folder with its scripts below it unless collapsed, then
// the scripts outside folders.
func (p *scriptsPanel) rows() []panelRow {
	lib := &p.app.scripts
	var rows []panelRow
	for i := range lib.Folders {
		f := &lib.Folders[i]
		rows = append(rows, panelRow{folder: f})
		if !p.app.expanded[f.Dir] {
			continue
		}
		for j := range f.Scripts {
			rows = append(rows, panelRow{folder: f, script: &f.Scripts[j]})
		}
	}
	for j := range lib.Scripts {
		rows = append(rows, panelRow{script: &lib.Scripts[j]})
	}
	return rows
}

// current is the selected row, kept in range as the list changes.
func (p *scriptsPanel) current() (panelRow, bool) {
	rows := p.rows()
	if len(rows) == 0 {
		return panelRow{}, false
	}
	if p.seenLoad != p.app.scriptsLoaded {
		p.seenLoad = p.app.scriptsLoaded
		if i := slices.IndexFunc(rows, func(r panelRow) bool { return r.key() == p.selectedKey }); i >= 0 {
			p.selected = i
		}
	}
	p.selected = min(max(p.selected, 0), len(rows)-1)
	p.selectedKey = rows[p.selected].key()
	return rows[p.selected], true
}

func (p *scriptsPanel) SendKey(key uv.KeyPressEvent) {
	p.current() // the list may have changed since the last key: find the selection first
	name := key.String()
	switch name {
	case "ctrl+d":
		p.close()
		return
	case "a":
		p.app.linkFolder()
		return
	case "up", "k":
		p.selected--
	case "down", "j":
		p.selected++
	case "home", "g":
		p.selected = 0
	case "end", "G", "shift+g":
		p.selected = len(p.rows()) - 1
	case "pgup", "pgdown":
		p.selected, _ = listJump(name, p.selected, len(p.rows()), p.height)
	}
	row, ok := p.current()
	switch {
	case name == "n" || name == "ctrl+n":
		p.app.createInLibrary(row.folder)
	case name == "ctrl+o":
		dir := p.app.scriptsDir()
		if row.folder != nil {
			dir = row.folder.Dir
		}
		p.app.openFolder(dir)
	case ok && row.script == nil:
		p.folderKey(name, row)
	case ok:
		p.scriptKey(name, row)
	}
}

func (p *scriptsPanel) folderKey(name string, row panelRow) {
	expanded := p.app.expanded[row.folder.Dir]
	switch name {
	case "enter", "space":
		p.app.setExpanded(row.folder.Dir, !expanded)
	case "left", "h":
		p.app.setExpanded(row.folder.Dir, false)
	case "right", "l":
		p.app.setExpanded(row.folder.Dir, true)
	case "delete":
		p.app.removeFolder(*row.folder)
	}
}

func (p *scriptsPanel) scriptKey(name string, row panelRow) {
	s := *row.script
	switch name {
	case "enter":
		p.app.runScript(s)
	case "e":
		if s.Problem != nil {
			p.app.editScriptFiles(s) // the form needs a script that loads
			return
		}
		p.app.openScriptForm(&s, row.folder)
	case "delete":
		p.app.deleteScript(s)
	case "P", "shift+p":
		p.app.promoteScript(s)
	case "left", "h":
		if row.folder != nil {
			p.selectKey(panelRow{folder: row.folder}.key())
		}
	}
}

func (p *scriptsPanel) selectKey(key string) {
	if i := slices.IndexFunc(p.rows(), func(r panelRow) bool { return r.key() == key }); i >= 0 {
		p.selected, p.selectedKey = i, key
	}
}

func (p *scriptsPanel) Draw(screen uv.Screen, area uv.Rectangle) {
	pal := &p.app.palette
	body := image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Max.Y-1)
	drawText(screen, area.Min.X+1, area.Max.Y-1, area.Max.X, p.app.hint("scripts", scriptsHints), pal.panelMuted)
	body = p.drawProblems(screen, body)
	row, ok := p.current()
	if !ok {
		drawCentered(screen, body, body.Min.Y+body.Dy()/2-1, "no scripts yet", pal.panelHeading)
		drawCentered(screen, body, body.Min.Y+body.Dy()/2+1, p.app.hint("scripts", "{n} creates a script or a directory, {a} adds a folder"), pal.panelMuted)
		return
	}
	list := body
	if body.Dx() >= minDetailsWidth {
		list.Max.X = body.Min.X + max(body.Dx()*2/5, 30)
		details := image.Rect(list.Max.X+2, body.Min.Y, body.Max.X, body.Max.Y)
		if row.script != nil {
			p.drawDetails(screen, details, *row.script)
		} else {
			p.drawFolder(screen, details, *row.folder)
		}
	}
	p.drawList(screen, list)
	p.list = list.Sub(area.Min)
}

// WantsMouseAt takes clicks on the list; the details and notes stay selectable.
func (p *scriptsPanel) WantsMouseAt(point image.Point) bool {
	return point.In(p.list)
}

// SendMouse selects the row clicked, opening or closing a folder; the wheel moves the selection.
func (p *scriptsPanel) SendMouse(event uv.MouseEvent) {
	p.current() // the list may have changed since the last click: find the selection first
	m := event.Mouse()
	rows := p.rows()
	switch event.(type) {
	case uv.MouseClickEvent:
		if !image.Pt(m.X, m.Y).In(p.list) || m.Button != uv.MouseLeft {
			return
		}
		index := p.first + m.Y - p.list.Min.Y
		if index >= len(rows) {
			return
		}
		p.selected, p.selectedKey = index, rows[index].key()
		if f := rows[index]; f.script == nil {
			p.app.setExpanded(f.folder.Dir, !p.app.expanded[f.folder.Dir])
		}
	case uv.MouseWheelEvent:
		switch m.Button {
		case uv.MouseWheelUp:
			p.selected = max(p.selected-1, 0)
		case uv.MouseWheelDown:
			p.selected = min(p.selected+1, len(rows)-1)
		}
	}
}

// drawProblems lists the folders that could not be read at the bottom of area and returns the rest.
func (p *scriptsPanel) drawProblems(screen uv.Screen, area image.Rectangle) image.Rectangle {
	problems := p.app.scripts.Problems
	shown := min(len(problems), 3, max(area.Dy()-3, 0))
	for i := range shown {
		drawText(screen, area.Min.X+1, area.Max.Y-shown+i, area.Max.X, "! "+problems[i].Error(), p.app.palette.status)
	}
	if shown > 0 {
		area.Max.Y -= shown + 1
	}
	return area
}

// drawList shows one row per line, scrolled to keep the selection visible. The scripts of a folder
// hang from it behind a bar; the folder being browsed, and its bar, are in the accent color.
func (p *scriptsPanel) drawList(screen uv.Screen, area image.Rectangle) {
	pal := &p.app.palette
	rows := p.rows()
	active := rows[p.selected].folder
	first := listFirst(p.first, p.selected, len(rows), area.Dy())
	p.first, p.height = first, area.Dy()
	for i := first; i < len(rows) && i-first < area.Dy(); i++ {
		row, y := rows[i], area.Min.Y+i-first
		label, value := p.rowText(row)
		inActive := row.folder != nil && row.folder == active
		style := pal.panelText
		switch {
		case i == p.selected:
			style = pal.selectedRow
		case row.script == nil && inActive:
			style = pal.focusedTitle
		case row.script == nil:
			style = pal.panelHeading
		}
		if row.script == nil || row.folder == nil {
			drawRow(screen, area, y, label, value, style)
			continue
		}
		bar := pal.unfocusedFrame
		if inActive {
			bar = pal.focusedFrame
		}
		drawRow(screen, image.Rect(area.Min.X+2, area.Min.Y, area.Max.X, area.Max.Y), y, label, value, style)
		setRune(screen, area.Min.X+1, y, "│", bar)
	}
}

// folderKinds say where a folder comes from, beside its number of scripts.
var folderKinds = map[script.FolderKind]string{script.GroupFolder: "", script.LinkedFolder: "linked · ", script.ProjectFolder: "project · "}

// rowText is a row's name and what shows on its right. A folder shows + while closed and - while
// open; the bar before the scripts of a folder is drawn apart.
func (p *scriptsPanel) rowText(row panelRow) (label, value string) {
	if row.script == nil {
		sign := "+ "
		if p.app.expanded[row.folder.Dir] {
			sign = "- "
		}
		return sign + row.folder.Name, fmt.Sprintf("%s%d", folderKinds[row.folder.Kind], len(row.folder.Scripts))
	}
	s := row.script
	if s.Problem != nil {
		return "✗ " + s.Name, "broken"
	}
	if s.Draft {
		return "✎ " + s.Name, "draft"
	}
	return s.Name, p.version(*s)
}

func (p *scriptsPanel) version(s script.Script) string {
	version, known := p.app.versions.Lookup(s)
	if !known {
		return "…"
	}
	return version
}

type styledLine struct {
	text  string
	style uv.Style
}

func (p *scriptsPanel) drawDetails(screen uv.Screen, area image.Rectangle, s script.Script) {
	pal := &p.app.palette
	if s.Problem != nil {
		p.drawBroken(screen, area, s)
		return
	}
	lines := []styledLine{
		{s.ID(), pal.panelHeading},
		{s.Description, pal.panelText},
		{"", pal.panelText},
		{"$ " + strings.Join(strings.Fields(s.Run), " "), pal.panelMuted},
		{s.Dir, pal.panelMuted},
	}
	if s.Program != "" {
		lines = append(lines, styledLine{s.Program + " " + p.version(s), pal.panelMuted})
	}
	y := drawLines(screen, area, area.Min.Y, lines)
	for _, line := range strings.Split(ansi.Wrap(p.notesFor(s), area.Dx(), ""), "\n") {
		if y++; y >= area.Max.Y {
			return
		}
		drawText(screen, area.Min.X, y, area.Max.X, line, pal.panelText)
	}
}

// drawFolder shows where a folder is and what can be done with it.
func (p *scriptsPanel) drawFolder(screen uv.Screen, area image.Rectangle, f script.Folder) {
	pal := &p.app.palette
	about := map[script.FolderKind]string{
		script.GroupFolder:   "A folder of your scripts. del deletes it with its scripts.",
		script.LinkedFolder:  "A folder added by its path. del stops showing it; its files stay.",
		script.ProjectFolder: "The scripts of the project noct was opened in (.noct/scripts).",
	}[f.Kind]
	drawLines(screen, area, area.Min.Y, []styledLine{
		{f.Name, pal.panelHeading},
		{fmt.Sprintf("%d scripts", len(f.Scripts)), pal.panelText},
		{"", pal.panelText},
		{f.Dir, pal.panelMuted},
		{"", pal.panelText},
		{about, pal.panelMuted},
		{"enter or space opens and closes it; n creates a script in it.", pal.panelMuted},
	})
}

// drawLines writes lines from row y, returning the row after the last one written.
func drawLines(screen uv.Screen, area image.Rectangle, y int, lines []styledLine) int {
	for _, line := range lines {
		if y >= area.Max.Y {
			break
		}
		drawText(screen, area.Min.X, y, area.Max.X, line.text, line.style)
		y++
	}
	return y
}

// drawBroken shows why a script cannot be loaded and how to fix it.
func (p *scriptsPanel) drawBroken(screen uv.Screen, area image.Rectangle, s script.Script) {
	pal := &p.app.palette
	text := "✗ " + s.ID() + "\n\n" + s.Problem.Error() + "\n\ne opens it in the editor to fix it."
	if _, err := os.Stat(s.BackupFile()); err == nil {
		text += "\nThe last working version is in " + s.BackupFile() + "."
	}
	for i, line := range strings.Split(ansi.Wrap(text, area.Dx(), ""), "\n") {
		if area.Min.Y+i >= area.Max.Y {
			return
		}
		style := pal.panelText
		if i == 0 {
			style = pal.status
		}
		drawText(screen, area.Min.X, area.Min.Y+i, area.Max.X, line, style)
	}
}

func (p *scriptsPanel) notesFor(s script.Script) string {
	if p.notesOf != s.Dir || p.notesLoaded != p.app.scriptsLoaded {
		p.notesOf, p.notesLoaded, p.notes = s.Dir, p.app.scriptsLoaded, strings.TrimSpace(s.Notes())
	}
	return p.notes
}

func (p *scriptsPanel) Resize(pty.Size) {}
func (p *scriptsPanel) Paste(string)    {}
func (p *scriptsPanel) Close()          {}

func (p *scriptsPanel) Cursor() (uv.Position, bool) {
	return image.Point{}, false
}

func (p *scriptsPanel) Title() string {
	return "scripts"
}
