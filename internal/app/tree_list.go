package app

import (
	"image"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// treeFolder is a folder of a tree list: id names it apart from every other, parent is the id of
// the folder it is in ("" at the top) and name is what the list shows. A last folder comes after
// the items beside it instead of before them.
type treeFolder struct {
	id, parent, name string
	last             bool
}

// treeRow is a line of a tree list: a folder (item < 0) or an item, depth folders deep. folder is
// the folder's id, or the id of the folder the item is in; count is how many items a folder shows,
// in the folders inside it too.
type treeRow struct {
	folder string
	item   int
	depth  int
	count  int
}

func (r treeRow) isFolder() bool { return r.item < 0 }

// treeList lays items out in folders inside folders, as the scripts panel and the inspect's list
// show them: in each folder, its folders first, closed until opened, then its items. While
// filtering, only the items keepItem keeps show, in their folders, which open; a folder that
// keepFolder keeps shows whole, and a folder that keeps nothing goes.
type treeList struct {
	folders []treeFolder
	// in is the id of the folder each item is in, "" for none, in the items' order.
	in         []string
	open       func(id string) bool
	filtering  bool
	keepItem   func(item int) bool
	keepFolder func(id string) bool
}

// treeBuild is a treeList being laid out: the folders in each folder, the items in each folder,
// and how many items each folder shows.
type treeBuild struct {
	treeList
	children map[string][]treeFolder
	items    map[string][]int
	counts   map[string]int
	rows     []treeRow
}

func (t treeList) rows() []treeRow {
	b := treeBuild{treeList: t, children: map[string][]treeFolder{}, items: map[string][]int{}, counts: map[string]int{}}
	for _, f := range t.folders {
		b.children[f.parent] = append(b.children[f.parent], f)
	}
	for i, folder := range t.in {
		b.items[folder] = append(b.items[folder], i)
	}
	b.count("", false)
	b.lay("", 0, false)
	return b.rows
}

// whole tells a folder the filter keeps all of, because its name, or a folder's around it, matches.
func (b *treeBuild) whole(id string, inWhole bool) bool {
	return inWhole || b.filtering && b.keepFolder(id)
}

func (b *treeBuild) keeps(item int, whole bool) bool {
	return !b.filtering || whole || b.keepItem(item)
}

// count is how many items the folder shows, in its folders too, kept for each folder.
func (b *treeBuild) count(id string, whole bool) int {
	n := 0
	for _, item := range b.items[id] {
		if b.keeps(item, whole) {
			n++
		}
	}
	for _, f := range b.children[id] {
		n += b.count(f.id, b.whole(f.id, whole))
	}
	b.counts[id] = n
	return n
}

// lay puts the rows of what is in the folder at depth: its folders, its items, its last folders.
func (b *treeBuild) lay(id string, depth int, whole bool) {
	b.layFolders(id, depth, whole, false)
	for _, item := range b.items[id] {
		if b.keeps(item, whole) {
			b.rows = append(b.rows, treeRow{folder: id, item: item, depth: depth})
		}
	}
	b.layFolders(id, depth, whole, true)
}

func (b *treeBuild) layFolders(id string, depth int, whole, last bool) {
	for _, f := range b.children[id] {
		inWhole := b.whole(f.id, whole)
		if f.last != last || b.filtering && !inWhole && b.counts[f.id] == 0 {
			continue
		}
		b.rows = append(b.rows, treeRow{folder: f.id, item: -1, depth: depth, count: b.counts[f.id]})
		if b.filtering || b.open(f.id) {
			b.lay(f.id, depth+1, inWhole)
		}
	}
}

// drawTreeRow draws a row of a tree list depth folders in: a bar for each folder it is in, the
// innermost one in inner, and the row after them.
func drawTreeRow(screen uv.Screen, area image.Rectangle, y, depth int, label, value string, style, bars, inner uv.Style) {
	for k := range depth {
		bar := bars
		if k == depth-1 {
			bar = inner
		}
		setRune(screen, area.Min.X+1+2*k, y, "│", bar)
	}
	drawRow(screen, image.Rect(area.Min.X+2*depth, area.Min.Y, area.Max.X, area.Max.Y), y, label, value, style)
}

// folderSign is + before a closed folder and - before an open one.
func folderSign(open bool) string {
	if open {
		return "- "
	}
	return "+ "
}
