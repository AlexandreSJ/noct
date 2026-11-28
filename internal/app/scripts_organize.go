package app

import (
	"errors"
	"image"
	"path/filepath"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/script"
)

// The scripts panel organizes the scripts as the inspect does the flows: a makes a script, N a
// directory, m moves or renames a script or a directory (a / in the path puts it in directories,
// none takes it out of them), c copies a script, and dragging a script onto a directory moves it
// there. A script's ID is its path in the directories, ops/db/backup, which these start from.

// organizeKey runs the keys that make and organize the list's rows; it tells whether it used the key.
func (p *scriptsPanel) organizeKey(name string, row panelRow, ok bool) bool {
	switch name {
	case "a", "n", "ctrl+n":
		p.app.openScriptForm(nil, folderOf(row, ok))
	case "N", "shift+n":
		p.newDirectory(folderOf(row, ok))
	case "m":
		if ok {
			p.moveRow(row)
		}
	case "c":
		if ok {
			p.copyScript(row)
		}
	default:
		return false
	}
	return true
}

// folderOf is the directory a row is, or is in, where what is made on it goes; nil at the top.
func folderOf(row panelRow, ok bool) *script.Folder {
	if !ok {
		return nil
	}
	return row.folder
}

// pathIn is where a name made in folder starts: its path and a /, or nothing at the top.
func pathIn(folder *script.Folder) string {
	if folder == nil {
		return ""
	}
	return folder.Name + "/"
}

// libraryPlace is the directory on disk a path of the library is in, and the rest of the path
// inside it: the directory whose path the path starts with, the deepest one, or the personal
// scripts directory for a path of none.
func (a *App) libraryPlace(path string) (dir, rest string) {
	dir, rest = a.scriptsDir(), path
	longest := 0
	for _, f := range a.scripts.Folders {
		if after, inside := strings.CutPrefix(path, f.Name+"/"); inside && len(f.Name) > longest {
			dir, rest, longest = f.Dir, after, len(f.Name)
		}
	}
	return dir, rest
}

// libraryDir is the directory on disk at a path of the library, as ops/db/backup.
func (a *App) libraryDir(path string) (string, error) {
	path = strings.Trim(path, "/")
	dir, rest := a.libraryPlace(path)
	if !naming.IsValidPath(rest) {
		return "", errors.New(path + ": letters, digits, . - _, and / between directories")
	}
	return filepath.Join(dir, filepath.FromSlash(rest)), nil
}

// newDirectory asks for a directory's path, starting in folder, and makes it.
func (p *scriptsPanel) newDirectory(folder *script.Folder) {
	p.app.openPrompt("new directory", pathIn(folder), "{enter} create  esc cancel  letters, digits, . - _  / for directories", func(path string) {
		path = strings.Trim(path, "/")
		root, rest := p.app.libraryPlace(path)
		dir, err := script.CreateFolder(root, rest)
		if err != nil {
			p.app.statusMessage = err.Error()
			return
		}
		p.app.reloadScripts()
		p.revealDir(filepath.Dir(dir))
		p.selectDir(dir)
	})
}

// moveRow asks where a script or a directory goes, its path to start with, and moves it there.
func (p *scriptsPanel) moveRow(row panelRow) {
	if row.script != nil {
		s := *row.script
		p.app.openPrompt("move script "+s.ID(), s.ID(), "{enter} move  esc cancel  a / puts it in directories, none takes it out", func(to string) {
			dir, err := p.app.libraryDir(to)
			if err != nil {
				p.app.statusMessage = err.Error()
				return
			}
			p.moveScript(s, dir)
		})
		return
	}
	f := *row.folder
	p.app.openPrompt("move directory "+f.Name, f.Name, "{enter} move  esc cancel  its scripts go with it", func(to string) {
		dir, err := p.app.libraryDir(to)
		if err != nil {
			p.app.statusMessage = err.Error()
			return
		}
		p.moveDirectory(f, dir)
	})
}

// scriptMove is a script that leaves its place: its ID before, and where it goes.
type scriptMove struct {
	id, dir string
}

// moveScript moves a script to dir, with its remembered values and the flows that run it.
func (p *scriptsPanel) moveScript(s script.Script, dir string) {
	if dir == s.Dir {
		return
	}
	if _, err := script.MoveTo(s, dir); err != nil {
		p.app.statusMessage = err.Error()
		return
	}
	p.app.afterScriptsMoved([]scriptMove{{id: s.ID(), dir: dir}})
	p.revealDir(filepath.Dir(dir))
	p.selectKey(dir)
	if moved, ok := p.app.scriptAt(dir); ok {
		p.app.notice = "script " + s.ID() + " is now " + moved.ID()
	}
}

// moveDirectory moves a directory with its scripts, their remembered values and the flows that
// run them.
func (p *scriptsPanel) moveDirectory(f script.Folder, dir string) {
	if dir == f.Dir {
		return
	}
	var moves []scriptMove
	for _, s := range p.app.scripts.All() {
		if rest, inside := strings.CutPrefix(s.Dir, f.Dir+string(filepath.Separator)); inside {
			moves = append(moves, scriptMove{id: s.ID(), dir: filepath.Join(dir, rest)})
		}
	}
	if err := script.MoveFolder(f, dir); err != nil {
		p.app.statusMessage = err.Error()
		return
	}
	p.app.afterScriptsMoved(moves)
	p.revealDir(dir)
	p.selectDir(dir)
	p.app.notice = "directory " + f.Name + " moved"
}

// afterScriptsMoved reads the scripts again and moves what goes by the IDs of those moved: their
// remembered values, and the script steps of the flows and functions that run them.
func (a *App) afterScriptsMoved(moves []scriptMove) {
	before := a.scripts
	a.reloadScripts()
	var problems []error
	for _, m := range moves {
		moved, ok := a.scriptAt(m.dir)
		if !ok || moved.ID() == m.id {
			continue
		}
		old, _ := before.Find(m.id)
		runs := func(id string) bool {
			s, found := before.Find(id)
			return found && s.Dir == old.Dir
		}
		problems = append(problems, a.memory.Rename(m.id, moved.ID()))
		for _, flows := range [][]flow.Flow{a.flows, a.functions} {
			_, err := flow.RenameScripts(flows, runs, moved.ID())
			problems = append(problems, err)
		}
	}
	if err := errors.Join(problems...); err != nil {
		a.statusMessage = err.Error()
	}
	a.reloadFlows()
}

// scriptAt is the script whose directory is dir.
func (a *App) scriptAt(dir string) (script.Script, bool) {
	all := a.scripts.All()
	i := slices.IndexFunc(all, func(s script.Script) bool { return s.Dir == dir })
	if i < 0 {
		return script.Script{}, false
	}
	return all[i], true
}

// copyScript asks for a path and copies the script there.
func (p *scriptsPanel) copyScript(row panelRow) {
	if row.script == nil {
		p.app.notice = "c copies a script; m moves a directory"
		return
	}
	s := *row.script
	p.app.openPrompt("copy script "+s.ID()+" as", s.ID()+"-copy", "{enter} copy  esc cancel  a / puts it in directories", func(to string) {
		dir, err := p.app.libraryDir(to)
		if err == nil {
			err = script.Copy(s, dir)
		}
		if err != nil {
			p.app.statusMessage = err.Error()
			return
		}
		p.app.reloadScripts()
		p.revealDir(filepath.Dir(dir))
		p.selectKey(dir)
	})
}

// revealDir opens the directory at dir on disk and the directories it is in.
func (p *scriptsPanel) revealDir(dir string) {
	byDir := map[string]script.Folder{}
	for _, f := range p.app.scripts.Folders {
		byDir[f.Dir] = f
	}
	for f, ok := byDir[dir]; ok; f, ok = byDir[f.Parent] {
		p.app.setExpanded(f.Dir, true)
	}
}

// selectDir selects the directory at dir on disk.
func (p *scriptsPanel) selectDir(dir string) {
	p.selectKey(panelRow{folder: &script.Folder{Dir: dir}}.key())
}

// scriptDrag is a script pressed in the list, which a release on a directory moves there.
type scriptDrag struct {
	key   string
	moved bool
}

// listMouse presses, drags and drops rows of the list; it tells whether the event was its.
func (p *scriptsPanel) listMouse(event uv.MouseEvent, point image.Point) bool {
	switch event.(type) {
	case uv.MouseClickEvent:
		if !point.In(p.list) || event.Mouse().Button != uv.MouseLeft {
			return false
		}
		rows := p.rows()
		p.clickRow(rows, p.first+point.Y-p.list.Min.Y)
		if row, ok := p.current(); ok && row.script != nil {
			p.drag = &scriptDrag{key: row.key()}
		}
		return true
	case uv.MouseMotionEvent:
		if p.drag == nil {
			return false
		}
		p.drag.moved = true
		return true
	case uv.MouseReleaseEvent:
		if p.drag == nil {
			return false
		}
		drag := *p.drag
		p.drag = nil
		if drag.moved {
			p.drop(drag.key, point)
		}
		return true
	}
	return false
}

// drop moves the script dragged into the directory under point: a directory's row, the directory
// of a script's row, or out of every directory below the rows.
func (p *scriptsPanel) drop(key string, point image.Point) {
	rows := p.rows()
	from := slices.IndexFunc(rows, func(r panelRow) bool { return r.key() == key })
	if from < 0 || !point.In(p.list) {
		return
	}
	s := *rows[from].script
	root := p.app.scriptsDir()
	if i := p.first + point.Y - p.list.Min.Y; i < len(rows) && rows[i].folder != nil {
		root = rows[i].folder.Dir
	}
	p.moveScript(s, filepath.Join(root, s.Name))
}
