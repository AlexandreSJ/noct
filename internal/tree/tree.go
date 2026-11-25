// Package tree reads folders nested in folders, as scripts, flows and functions are kept: each
// folder under a root holds items and more folders, at any depth.
package tree

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// MaxDepth is how deep folders are read, which also ends a loop of links.
const MaxDepth = 16

// Walk reads root and the folders in it, in name order, depth first. isItem tells an entry that is
// an item, which is never read into; any other folder is a folder of items. folder is called
// with each folder's path from root (ops/db), before what it holds, and item with the path of
// the folder an item is in ("" in root) and the item's own path on disk. Names starting with a
// dot are left out, as .git in a linked folder. A missing root holds nothing.
func Walk(root string, isItem func(dir string, entry fs.DirEntry) bool, folder func(path string), item func(folder, path string)) error {
	if root == "" {
		return nil
	}
	w := walker{root: root, isItem: isItem, folder: folder, item: item}
	return w.walk("", 0)
}

// walker is a Walk under way.
type walker struct {
	root   string
	isItem func(string, fs.DirEntry) bool
	folder func(string)
	item   func(string, string)
}

func (w walker) walk(rel string, depth int) error {
	dir := filepath.Join(w.root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) && rel == "" {
		return nil
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	problems := []error{err}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			problems = append(problems, w.visit(dir, rel, depth, entry))
		}
	}
	return errors.Join(problems...)
}

// visit takes an entry of the folder at rel: an item, or a folder to read in turn.
func (w walker) visit(dir, rel string, depth int, entry fs.DirEntry) error {
	switch {
	case w.isItem(dir, entry):
		if w.item != nil {
			w.item(rel, filepath.Join(dir, entry.Name()))
		}
	case entry.IsDir() && depth < MaxDepth:
		path := join(rel, entry.Name())
		if w.folder != nil {
			w.folder(path)
		}
		return w.walk(path, depth+1)
	}
	return nil
}

// join puts a name inside a folder path, written with / on every system.
func join(folder, name string) string {
	if folder == "" {
		return name
	}
	return folder + "/" + name
}

// Parent is the folder a path is in, or "" at the top.
func Parent(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return ""
	}
	return path[:i]
}

// Base is the last part of a path.
func Base(path string) string {
	return path[strings.LastIndex(path, "/")+1:]
}
