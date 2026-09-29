package app

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/script"
)

// libraryCheckInterval is how often a panel showing scripts or flows looks for changes on disk,
// such as a draft the AI wrote through the MCP.
const libraryCheckInterval = time.Second

// scriptsDepth reaches the files of a script in a folder: <dir>/<folder>/<script>/<file>.
const scriptsDepth = 3

// checkLibrary reads the scripts or the flows again when a panel shows them and their files
// changed, telling whether it did. It only looks at names, sizes and times, so an unchanged
// library costs a few stats.
func (a *App) checkLibrary() bool {
	if !a.showsLibrary() {
		return false
	}
	if dirSignature(a.scriptDirs(), scriptsDepth) != a.scriptsSignature {
		a.reloadScripts()
		a.reloadFlows() // flows are checked against the scripts
		return true
	}
	if dirSignature([]string{flow.Dir()}, 1) != a.flowsSignature {
		a.reloadFlows()
		return true
	}
	return false
}

// showsLibrary tells whether the current tab shows an inspect or a scripts panel.
func (a *App) showsLibrary() bool {
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		return false
	}
	return slices.ContainsFunc(t.visible(a.workspaceArea()), func(p placedPane) bool {
		switch t.views[p.id].(type) {
		case *inspectPanel, *scriptsPanel:
			return true
		}
		return false
	})
}

// scriptDirs are the folders the library is read from.
func (a *App) scriptDirs() []string {
	dirs := []string{a.scriptsDir()}
	for _, path := range a.config.ScriptFolders {
		dirs = append(dirs, script.ResolveDir(path))
	}
	if project := script.ProjectDir(a.workDir); project != "" {
		dirs = append(dirs, project)
	}
	return dirs
}

// dirSignature sums up the names, sizes and times of what is in dirs, down to depth levels.
func dirSignature(dirs []string, depth int) string {
	h := fnv.New64a()
	var walk func(dir string, level int)
	walk = func(dir string, level int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			_, _ = io.WriteString(h, dir+"\x00missing\x00")
			return
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue // removed while reading; the next check sees it gone
			}
			_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d\x00", path, info.Size(), info.ModTime().UnixNano())
			if entry.IsDir() && level < depth {
				walk(path, level+1)
			}
		}
	}
	for _, dir := range dirs {
		walk(dir, 1)
	}
	return fmt.Sprintf("%x", h.Sum64())
}
