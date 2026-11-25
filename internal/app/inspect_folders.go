package app

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/tree"
)

// The inspect's list organizes the flows and functions as files in folders: N makes a folder,
// m moves or renames a flow, a function or a folder (a / in the name puts it in folders, none
// takes it out of them), c copies a flow, delete removes a folder, and dragging a flow onto a
// folder moves it there.

// organizeKey runs the keys that organize the list on a row: N a new folder, m a move, c a copy.
func (p *inspectPanel) organizeKey(name string, row inspectRow) {
	switch name {
	case "N", "shift+n":
		p.newFolder(row)
	case "m":
		p.moveRow(row)
	case "c":
		p.copyEntry(row)
	}
}

// isFunctionRow tells a row of the functions' folders apart from one of the flows'.
func isFunctionRow(row inspectRow) bool {
	if row.entry != nil {
		return row.entry.Function
	}
	return strings.HasPrefix(row.folder, functionFolders)
}

// rowDir is the folder on disk a row's flows or functions are kept under.
func rowDir(row inspectRow) string {
	if isFunctionRow(row) {
		return flow.FunctionsDir()
	}
	return flow.Dir()
}

// rowFolder is the path of the folder a row is, or is in: what a name made there starts with.
func rowFolder(row inspectRow) string {
	if row.entry != nil {
		return folderPath(folderID(*row.entry))
	}
	return folderPath(row.folder)
}

// newFolder asks for a folder's path, starting in the row's folder, and makes it.
func (p *inspectPanel) newFolder(row inspectRow) {
	dir, id := rowDir(row), flowFolders
	if isFunctionRow(row) {
		id = functionFolders
	}
	p.app.openPrompt("new folder", rowFolder(row), "{enter} create  esc cancel  letters, digits, . - _  / for folders", func(path string) {
		path = strings.Trim(path, "/")
		if err := flow.CreateFolder(dir, path); err != nil {
			p.app.statusMessage = err.Error()
			return
		}
		p.app.reloadFlows()
		p.rowKey = id + path
		p.revealFolder(folderParent(p.rowKey))
	})
}

// revealFolder opens a folder of the list and the folders it is in.
func (p *inspectPanel) revealFolder(id string) {
	for ; id != ""; id = folderParent(id) {
		p.app.setExpanded(id, true)
	}
}

// moveRow asks where a flow, function or folder goes, its name or path to start with, and moves
// it there.
func (p *inspectPanel) moveRow(row inspectRow) {
	switch {
	case row.folder == functionFolders && row.entry == nil:
		p.app.notice = "ƒ functions holds every function: move the folders in it"
	case row.entry != nil:
		f := *row.entry
		p.app.openPrompt("move "+kindOf(f)+" "+f.Name, f.Name, "{enter} move  esc cancel  a / puts it in folders, none takes it out", func(to string) {
			p.moveEntry(f, strings.Trim(to, "/"))
		})
	default:
		from := strings.TrimSuffix(folderPath(row.folder), "/")
		p.app.openPrompt("move folder "+from, from, "{enter} move  esc cancel  its flows go with it", func(to string) {
			p.moveFolder(row, from, strings.Trim(to, "/"))
		})
	}
}

func kindOf(f flow.Flow) string {
	if f.Function {
		return "function"
	}
	return "flow"
}

// moveEntry gives a flow or function a new name, with its remembered values; a function's callers
// call it by the new one.
func (p *inspectPanel) moveEntry(f flow.Flow, to string) {
	if to == f.Name {
		return
	}
	dir := flow.Dir()
	if f.Function {
		dir = flow.FunctionsDir()
	}
	moved, err := f.Rename(dir, to)
	if err == nil {
		p.app.reloadFlows() // the callers are read again, a function that calls itself too
		err = p.afterMove(f.Name, moved)
	}
	if err != nil {
		p.app.statusMessage = err.Error()
	}
	p.app.reloadFlows()
	if moved.Name != "" {
		p.flowName, p.rowKey = entryKey(moved), ""
		p.reveal()
		p.app.notice = kindOf(f) + " " + f.Name + " is now " + moved.Name
	}
}

// afterMove moves what goes by a flow's name, from: its remembered values, or a function's calls.
func (p *inspectPanel) afterMove(from string, to flow.Flow) error {
	if !to.Function {
		return p.app.memory.RenameFlow(from, to.Name)
	}
	_, err := flow.RenameCalls(p.entries(), from, to.Name)
	return err
}

// moveFolder moves a folder with what is in it, and what goes by the names of its flows.
func (p *inspectPanel) moveFolder(row inspectRow, from, to string) {
	if to == from {
		return
	}
	var inside []flow.Flow
	for _, f := range p.entries() {
		if f.Function == isFunctionRow(row) {
			inside = append(inside, f)
		}
	}
	moved, err := flow.MoveFolder(rowDir(row), from, to, inside)
	p.app.reloadFlows() // the callers are read again where they are now
	problems := []error{err}
	for _, f := range moved {
		problems = append(problems, p.afterMove(from+strings.TrimPrefix(f.Name, to), f))
	}
	if err = errors.Join(problems...); err != nil {
		p.app.statusMessage = err.Error()
	}
	p.app.reloadFlows()
	id := strings.TrimSuffix(row.folder, from) + to
	p.rowKey = id
	p.revealFolder(id)
	p.app.notice = "folder " + from + " is now " + to
}

// copyEntry asks for a name and copies the flow or function there.
func (p *inspectPanel) copyEntry(row inspectRow) {
	if row.entry == nil {
		p.app.notice = "c copies a flow or a function; m moves a folder"
		return
	}
	f := *row.entry
	dir := flow.Dir()
	if f.Function {
		dir = flow.FunctionsDir()
	}
	p.app.openPrompt("copy "+kindOf(f)+" "+f.Name+" as", f.Name+"-copy", "{enter} copy  esc cancel  a / puts it in folders", func(to string) {
		copied, err := f.Copy(dir, strings.Trim(to, "/"))
		if err != nil {
			p.app.statusMessage = err.Error()
			return
		}
		p.app.reloadFlows()
		p.flowName, p.rowKey = entryKey(copied), ""
		p.reveal()
	})
}

// deleteFolder asks, then removes a folder with everything in it.
func (p *inspectPanel) deleteFolder(row inspectRow) {
	if row.folder == functionFolders {
		p.app.notice = "ƒ functions holds every function: delete the folders in it"
		return
	}
	path := strings.TrimSuffix(folderPath(row.folder), "/")
	question := "delete the empty folder " + path + "?"
	if row.count > 0 {
		question = fmt.Sprintf("delete folder %s and the %d in it?", path, row.count)
	}
	p.app.askToConfirm(question, func() {
		if err := flow.DeleteFolder(rowDir(row), path); err != nil {
			p.app.statusMessage = err.Error()
		}
		p.app.reloadFlows()
		p.rowKey = ""
	})
}

// listDrag is a flow or function pressed in the list, which a release on a folder moves there.
type listDrag struct {
	key   string
	moved bool
}

// listMouse presses, drags and drops rows of the list; it tells whether the event was its.
func (p *inspectPanel) listMouse(event uv.MouseEvent, point image.Point) bool {
	switch event.(type) {
	case uv.MouseClickEvent:
		if !point.In(p.list) || event.Mouse().Button != uv.MouseLeft {
			return false
		}
		p.clickFlow(p.first + point.Y - p.list.Min.Y)
		if row, _, ok := p.selectedRow(); ok && row.entry != nil {
			p.listDrag = &listDrag{key: row.key()}
		}
		return true
	case uv.MouseMotionEvent:
		if p.listDrag == nil {
			return false
		}
		p.listDrag.moved = true
		return true
	case uv.MouseReleaseEvent:
		if p.listDrag == nil {
			return false
		}
		drag := *p.listDrag
		p.listDrag = nil
		if drag.moved {
			p.drop(drag.key, point)
		}
		return true
	}
	return false
}

// drop moves the flow or function dragged into the folder under point: a folder's row, the
// folder of a flow's row, or out of every folder below the rows.
func (p *inspectPanel) drop(key string, point image.Point) {
	rows := p.rows()
	from := slices.IndexFunc(rows, func(r inspectRow) bool { return r.key() == key })
	if from < 0 || !point.In(p.list) {
		return
	}
	f := *rows[from].entry
	target := ""
	if i := p.first + point.Y - p.list.Min.Y; i < len(rows) {
		target = rows[i].folder
		if rows[i].entry != nil {
			target = folderID(*rows[i].entry)
		}
	} else if f.Function {
		target = functionFolders
	}
	if target != "" && f.Function != strings.HasPrefix(target, functionFolders) {
		p.app.notice = "a " + kindOf(f) + " stays among the " + kindOf(f) + "s"
		return
	}
	p.moveEntry(f, folderPath(target)+tree.Base(f.Name))
}
