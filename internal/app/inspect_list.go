package app

import (
	"slices"
	"strconv"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/tree"
)

// The inspect lists the flows in their folders, as the scripts panel lists the scripts, and the
// functions after them under ƒ functions, in theirs. A folder opens and closes as a script folder
// does; the chart shows the last flow selected while a folder is.

const (
	// flowFolders and functionFolders start the ids of the list's folders; functionFolders alone
	// is the folder of every function.
	flowFolders     = "flows:"
	functionFolders = "functions:"
)

// inspectRow is a line of the inspect's list: a folder, by its id, or a flow or function.
type inspectRow struct {
	folder string
	entry  *flow.Flow
	depth  int
	count  int
}

func (r inspectRow) key() string {
	if r.entry != nil {
		return entryKey(*r.entry)
	}
	return r.folder
}

// folderID is the id of the list's folder a flow or function is in.
func folderID(f flow.Flow) string {
	if f.Function {
		return functionFolders + f.Folder()
	}
	if f.Folder() == "" {
		return ""
	}
	return flowFolders + f.Folder()
}

// folderPath is the path of the folders a list's folder is, as a name typed in it starts.
func folderPath(id string) string {
	path := strings.TrimPrefix(strings.TrimPrefix(id, flowFolders), functionFolders)
	if path == "" {
		return ""
	}
	return path + "/"
}

// folderParent is the id of the folder a list's folder is in, or "" at the top.
func folderParent(id string) string {
	switch {
	case id == "" || id == functionFolders:
		return ""
	case strings.HasPrefix(id, functionFolders):
		return functionFolders + tree.Parent(strings.TrimPrefix(id, functionFolders))
	}
	if up := tree.Parent(strings.TrimPrefix(id, flowFolders)); up != "" {
		return flowFolders + up
	}
	return ""
}

// listFolderName is what the list shows for a folder.
func listFolderName(id string) string {
	if id == functionFolders {
		return "ƒ functions"
	}
	return tree.Base(strings.TrimSuffix(folderPath(id), "/"))
}

// listFolders are the folders the flows and functions are in, those on disk with none, and the
// folders those are in in turn, in name order, and the folder of all the functions, last.
func listFolders(entries []flow.Flow, flowDirs, functionDirs []string) []treeFolder {
	seen := map[string]bool{}
	var folders []treeFolder
	add := func(id string) {
		for ; id != "" && !seen[id]; id = folderParent(id) {
			seen[id] = true
			folders = append(folders, treeFolder{id: id, parent: folderParent(id), name: listFolderName(id), last: id == functionFolders})
		}
	}
	for _, f := range entries {
		add(folderID(f))
	}
	for _, path := range flowDirs {
		add(flowFolders + path)
	}
	for _, path := range functionDirs {
		add(functionFolders + path)
	}
	// Siblings share their ids up to their names, so the ids' order is their names'.
	slices.SortFunc(folders, func(a, b treeFolder) int { return strings.Compare(a.id, b.id) })
	return folders
}

// folderOpen tells whether a list's folder is open: the functions' starts open, the rest closed.
func (a *App) folderOpen(id string) bool {
	open, set := a.expanded[id]
	if !set {
		return id == functionFolders
	}
	return open
}

// rows are the lines of the list, as the filter and the open folders leave them.
func (p *inspectPanel) rows() []inspectRow {
	entries := p.entries()
	folders := listFolders(entries, p.app.flowFolders, p.app.functionFolders)
	in := make([]string, len(entries))
	for i, f := range entries {
		in[i] = folderID(f)
	}
	list := treeList{
		folders: folders, in: in, open: p.app.folderOpen, filtering: p.filter.query() != "",
		keepItem:   func(i int) bool { return p.keepsEntry(entries[i]) },
		keepFolder: func(id string) bool { return p.filter.keeps(listFolderName(id)) },
	}
	var rows []inspectRow
	for _, r := range list.rows() {
		row := inspectRow{folder: r.folder, depth: r.depth, count: r.count}
		if !r.isFolder() {
			row.entry = &entries[r.item]
		}
		rows = append(rows, row)
	}
	return rows
}

// keepsEntry tells whether the filter keeps a flow or function: by its name or description.
func (p *inspectPanel) keepsEntry(f flow.Flow) bool {
	kind := "flow"
	if f.Function {
		kind = "function"
	}
	return p.filter.keeps(f.Name, f.Description, kind)
}

// rowIndex is the selected row: the folder selected, or the current flow's row. A flow in closed
// folders opens them, so it shows.
func (p *inspectPanel) rowIndex(rows []inspectRow) int {
	key := p.rowKey
	if key == "" {
		f, _, ok := p.current()
		if !ok {
			return -1
		}
		key = entryKey(f)
	}
	for i, r := range rows {
		if r.key() == key {
			return i
		}
	}
	if p.rowKey != "" {
		p.rowKey = "" // the folder went: back to the flow
		return p.rowIndex(rows)
	}
	return -1
}

// reveal opens the folders the current flow is in, so its row shows.
func (p *inspectPanel) reveal() {
	f, _, ok := p.current()
	if !ok {
		return
	}
	for id := folderID(f); id != ""; id = folderParent(id) {
		p.app.setExpanded(id, true)
	}
}

// selectedRow is the row selected, after opening the folders the current flow is in if it
// hides there.
func (p *inspectPanel) selectedRow() (inspectRow, int, bool) {
	rows := p.rows()
	i := p.rowIndex(rows)
	if i < 0 && p.rowKey == "" {
		p.reveal()
		rows = p.rows()
		i = p.rowIndex(rows)
	}
	if i < 0 {
		return inspectRow{}, -1, false
	}
	return rows[i], i, true
}

// folderSelected tells that the list's selection is on a folder.
func (p *inspectPanel) folderSelected() bool {
	row, _, ok := p.selectedRow()
	return ok && row.entry == nil
}

// selectRow selects a row of the list: a folder, or a flow, which the chart then shows.
func (p *inspectPanel) selectRow(row inspectRow) {
	if row.entry == nil {
		p.rowKey = row.folder
		return
	}
	p.rowKey = ""
	if key := entryKey(*row.entry); key != p.flowName {
		p.flowName, p.selected, p.centered, p.anchor, p.run = key, "", true, "", nil
	}
	p.app.lastFlow = p.flowName
}

// selectStep moves the selection by step rows.
func (p *inspectPanel) selectStep(step int) {
	rows := p.rows()
	if len(rows) == 0 {
		return
	}
	_, i, _ := p.selectedRow()
	p.selectRow(rows[min(max(i+step, 0), len(rows)-1)])
}

// selectFolderOf selects the folder the row hangs from, if it is in one.
func (p *inspectPanel) selectFolderOf(row inspectRow) {
	id := row.folder
	if row.entry == nil {
		id = folderParent(row.folder)
	}
	if id != "" {
		p.rowKey = id
	}
}

// folderKey runs the list's keys on a folder: open or close it, go up, make a flow in it.
func (p *inspectPanel) folderKey(name string, row inspectRow) {
	open := p.app.folderOpen(row.folder)
	switch name {
	case "enter", "space":
		p.app.setExpanded(row.folder, !open)
	case "right", "l":
		p.app.setExpanded(row.folder, true)
	case "left", "h":
		if !open {
			p.selectFolderOf(row)
			return
		}
		p.app.setExpanded(row.folder, false)
	case "a":
		p.newEntry(false, row)
	case "F", "shift+f":
		p.newEntry(true, row)
	case "N", "shift+n", "m", "c":
		p.organizeKey(name, row)
	case "delete":
		p.deleteFolder(row)
	default:
		p.listMoveKey(name)
	}
}

// listMoveKey moves the list's selection by the arrows, j and k, and the page keys.
func (p *inspectPanel) listMoveKey(name string) {
	switch name {
	case "up", "k":
		p.selectStep(-1)
	case "down", "j":
		p.selectStep(1)
	case "pgup", "pgdown", "home", "end":
		rows := p.rows()
		if _, i, ok := p.selectedRow(); ok {
			to, _ := listJump(name, i, len(rows), max(p.list.Dy(), 1))
			p.selectRow(rows[to])
		}
	}
}

// newEntry asks for a new flow, or function, in the folder of the row; a / in the name makes
// folders.
func (p *inspectPanel) newEntry(function bool, row inspectRow) {
	folder := ""
	if id := row.folder; function == strings.HasPrefix(id, functionFolders) {
		folder = folderPath(id)
	}
	created := func(key string) {
		p.flowName, p.focusChart, p.selected, p.rowKey = key, true, "", ""
		p.reveal()
	}
	if function {
		p.app.newFunction(folder, created)
		return
	}
	p.app.newFlow(folder, created)
}

// rowLabel is what a row shows: a folder's name, open or closed, and how many it holds; a flow's
// marks and name without its folders.
func (p *inspectPanel) rowLabel(row inspectRow, enabled map[string]bool) (label, value string) {
	if row.entry == nil {
		open := p.app.folderOpen(row.folder) || p.filter.query() != ""
		return folderSign(open) + listFolderName(row.folder), strconv.Itoa(row.count)
	}
	return flowListMarks(*row.entry, enabled) + row.entry.Base(), ""
}
