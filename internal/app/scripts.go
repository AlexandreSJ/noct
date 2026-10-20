package app

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/script"
)

// scriptsDir is the folder of the user's own scripts, as configured.
func (a *App) scriptsDir() string {
	return script.ResolveDir(a.config.ScriptsDir)
}

// reloadScripts reads the library again, after it may have changed on disk.
func (a *App) reloadScripts() {
	a.scriptsSignature = dirSignature(a.scriptDirs(), scriptsDepth) // before reading: a change meanwhile shows next time
	a.scripts = script.LoadLibrary(a.scriptsDir(), a.config.ScriptFolders, a.workDir)
	a.sortScripts()
}

func runsPath() string {
	return filepath.Join(logstore.Dir(), script.RunsFile)
}

// sortScripts orders the library as configured. Panels keep the script they had selected.
func (a *App) sortScripts() {
	sortLibrary(&a.scripts, a.config.ScriptsSort)
	a.scriptsLoaded++
}

// sortLibrary orders the scripts of each folder, and those outside folders, by name, by last edit
// or by last run. Folders stay in alphabetical order, before the scripts outside them.
func sortLibrary(lib *script.Library, order string) {
	var lastRun map[string]time.Time
	if order == config.SortByRun {
		runs, _ := script.Runs(runsPath()) // without the runs, scripts fall back to their names
		lastRun = script.LastRuns(runs)
	}
	sort := func(scripts []script.Script) {
		switch order {
		case config.SortByEdited:
			script.SortByEdited(scripts)
		case config.SortByRun:
			script.SortByLastRun(scripts, lastRun)
		default:
			script.SortByName(scripts)
		}
	}
	for i := range lib.Folders {
		sort(lib.Folders[i].Scripts)
	}
	sort(lib.Scripts)
}

// setExpanded opens or closes a folder in every scripts panel.
func (a *App) setExpanded(dir string, expanded bool) {
	if a.expanded == nil {
		a.expanded = map[string]bool{}
	}
	a.expanded[dir] = expanded
	a.scriptsLoaded++ // panels keep their selection on the same row
}

// createInLibrary asks whether to make a folder or a script; the script goes into folder.
func (a *App) createInLibrary(folder *script.Folder) {
	a.openChoice("new",
		choiceOption{key: "d", label: "directory", pick: a.newFolder},
		choiceOption{key: "s", label: "script", pick: func() { a.openScriptForm(nil, folder) }},
	)
}

// newFolder asks for a name and makes a folder of scripts in the scripts folder.
func (a *App) newFolder() {
	a.openPrompt("new folder", "", "{enter} create  esc cancel  letters, digits, . - _", func(name string) {
		if _, err := script.CreateFolder(a.scriptsDir(), name); err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.reloadScripts()
	})
}

// linkFolder asks for the path of a folder of scripts anywhere and shows it in noct.
func (a *App) linkFolder() {
	a.openPrompt("add a folder of scripts", "", "{enter} add  esc cancel  paste its path; ~ is your home", func(path string) {
		if info, err := os.Stat(script.ResolveDir(path)); err != nil || !info.IsDir() {
			a.statusMessage = "not a folder: " + path
			return
		}
		next := a.config
		next.ScriptFolders = append(slices.Clone(next.ScriptFolders), path)
		a.applyConfig(next)
		a.reloadScripts()
	})
}

// removeFolder deletes a folder of scripts, or stops showing a linked one, after asking.
func (a *App) removeFolder(f script.Folder) {
	switch f.Kind {
	case script.GroupFolder:
		a.askToConfirm(fmt.Sprintf("delete folder %s and its %d scripts?", f.Name, len(f.Scripts)), func() {
			if err := script.DeleteFolder(f); err != nil {
				a.statusMessage = err.Error()
			}
			a.reloadScripts()
		})
	case script.LinkedFolder:
		a.askToConfirm("stop showing "+f.Name+"? its files stay", func() {
			next := a.config
			next.ScriptFolders = slices.DeleteFunc(slices.Clone(next.ScriptFolders), func(path string) bool {
				return script.ResolveDir(path) == f.Dir
			})
			a.applyConfig(next)
			a.reloadScripts()
		})
	default:
		a.statusMessage = "the project's folder belongs to the project: remove .noct/scripts there"
	}
}

// getenv reads the environment programs in panes get, the one a script's env parameters see.
func (a *App) getenv(name string) string {
	for _, entry := range a.shellEnv {
		if key, value, ok := strings.Cut(entry, "="); ok && key == name {
			return value
		}
	}
	return ""
}

// runScript asks for the script's parameters, when it has any to ask, and runs it.
func (a *App) runScript(s script.Script) {
	if s.Problem != nil {
		a.statusMessage = s.Problem.Error()
		return
	}
	if s.Draft {
		a.statusMessage = "script " + s.ID() + " is a draft: review it (e) and promote it (P)"
		return
	}
	values := s.StartingValues(a.memory.Values(s), a.getenv)
	for _, p := range s.Params {
		if p.Asked() {
			a.openForm(s, values)
			return
		}
	}
	a.startRun(s, values)
}

// startRun runs the script in the user's shell, in a pane beside the focused one.
func (a *App) startRun(s script.Script, values map[string]string) {
	if err := s.CheckValues(values); err != nil {
		a.statusMessage = err.Error()
		return
	}
	command, err := s.Command(values)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	if err := a.memory.Remember(s, values); err != nil {
		a.statusMessage = err.Error() // the run goes on; only the memory is lost
	}
	run := program{argv: []string{a.shellArgv[0], "-c", command}, dir: s.WorkingDir(a.workDir)}
	a.openTrackedRun(s.Name, script.Run{Script: s.ID()}, run)
}

// openTrackedRun runs p in a run view beside the focused pane. Each start, the first and every
// "run again", is kept in the list of runs with its log and, when it ends, its exit code. It
// returns the first start's pane and log, or no log when the pane could not open.
func (a *App) openTrackedRun(title string, record script.Run, p program) (layout.PaneID, string) {
	var pane layout.PaneID
	var firstLog string
	a.openPanel(func(id layout.PaneID, frame image.Rectangle) (view, error) {
		start := func(size pty.Size, onExit func(int)) (view, error) {
			run := record
			run.At = time.Now()
			run.Log = logstore.Path(logNameOf(record), run.At)
			if firstLog == "" {
				pane, firstLog = id, run.Log
			}
			a.recordRun(run)
			p.logPath = run.Log
			if run.Flow != "" {
				// The flow runner leaves the record to noct, and keeps its result beside the log.
				p.env = []string{flow.TrackedEnv + "=1", flow.ResultEnv + "=" + logstore.ResultPath(run.Log), paneEnvFor(id)}
			}
			p.onExit = func(code int) {
				if err := script.FinishRun(runsPath(), run, code); err != nil {
					a.statusMessage = err.Error()
				}
				onExit(code)
			}
			return a.launch(p, size)
		}
		return newRunView(title, &a.palette, start, func() { a.detachView(id) }, contentSize(frame), p.dir)
	})
	return pane, firstLog
}

// logNameOf names a run's log after its script, or after its flow.
func logNameOf(run script.Run) string {
	if run.Flow != "" {
		return "flow-" + run.Flow
	}
	return strings.ReplaceAll(run.Script, "/", "-")
}

func (a *App) recordRun(run script.Run) {
	if err := script.AddRun(runsPath(), run); err != nil {
		a.statusMessage = err.Error()
	}
	if run.Script != "" && a.config.ScriptsSort == config.SortByRun {
		a.sortScripts()
	}
}

// editScriptFiles opens the script and its notes in the configured editor beside the focused
// pane, keeping a copy of the working version first. When the editor exits, the library is read
// again and a script the edit broke is reported.
func (a *App) editScriptFiles(s script.Script) {
	argv, ok := commandArgv(a.config.Editor, s.File(), s.NotesFile())
	if !ok {
		a.statusMessage = "editor " + a.config.Editor + " not found; choose another in the config"
		return
	}
	if err := script.Backup(s); err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.openPanel(func(id layout.PaneID, frame image.Rectangle) (view, error) {
		return a.launch(program{
			argv:    argv,
			dir:     s.Dir,
			logName: "edit-" + s.Name,
			onExit: func(code int) {
				a.detachView(id)
				a.reloadScripts()
				a.reportEdit(s, code)
			},
		}, contentSize(frame))
	})
}

// reportEdit tells what went wrong with an edit: the editor failing, or the script no longer
// loading, with where its last working version is.
func (a *App) reportEdit(edited script.Script, editorExit int) {
	for _, s := range a.scripts.All() {
		if s.Dir != edited.Dir || s.Problem == nil {
			continue
		}
		a.statusMessage = s.Problem.Error()
		if _, err := os.Stat(s.BackupFile()); err == nil {
			a.statusMessage += "; last working version: " + s.BackupFile()
		}
		return
	}
	if editorExit != 0 {
		a.statusMessage = fmt.Sprintf("the editor exited with code %d", editorExit)
	}
}

func (a *App) deleteScript(s script.Script) {
	a.askToConfirm("delete script "+s.Name+"?", func() {
		if err := script.Delete(s); err != nil {
			a.statusMessage = err.Error()
		}
		a.reloadScripts()
	})
}
