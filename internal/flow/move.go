package flow

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/tree"
)

// Flows and functions are files in folders: these move, copy and remove them, and the folders,
// keeping what noct holds by a flow's name (its last result and its triggers turned on) with it.

// Folders are the folders under dir, the empty ones too, by their path from dir (ops, ops/db).
func Folders(dir string) []string {
	var folders []string
	_ = tree.Walk(dir, func(string, fs.DirEntry) bool { return false }, func(path string) {
		folders = append(folders, path)
	}, nil) // a folder that does not read leaves out what is in it, which the flows tell
	return folders
}

// CreateFolder makes the folder at path, ops/db, in dir.
func CreateFolder(dir, path string) error {
	if !naming.IsValidPath(path) {
		return fmt.Errorf("directory %q: letters, digits, . - _, and / between directories", path)
	}
	full := filepath.Join(dir, filepath.FromSlash(path))
	if _, err := os.Stat(full); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", path, err)
	}
	return nil
}

// DeleteFolder removes the folder at path in dir and everything in it.
func DeleteFolder(dir, path string) error {
	if !naming.IsValidPath(path) {
		return fmt.Errorf("refusing to delete %q", path)
	}
	if err := os.RemoveAll(filepath.Join(dir, filepath.FromSlash(path))); err != nil {
		return fmt.Errorf("deleting directory %s: %w", path, err)
	}
	return nil
}

// Copy writes the flow or function, as its file has it, under the name to in dir.
func (f Flow) Copy(dir, to string) (Flow, error) {
	kind := map[bool]string{true: "function", false: "flow"}[f.Function]
	if err := ValidName(kind, to); err != nil {
		return Flow{}, err
	}
	content, err := os.ReadFile(f.File)
	if err != nil {
		return Flow{}, fmt.Errorf("copying %s %s: %w", kind, f.Name, err)
	}
	copied := f
	copied.Name, copied.File = to, FilePath(dir, to)
	if err := writeNew(copied.File, content); err != nil {
		return Flow{}, fmt.Errorf("copying %s %s: %w", kind, f.Name, err)
	}
	return copied, nil
}

// writeNew writes content to a file that must not exist yet, making its folders.
func writeNew(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s already exists", filepath.Base(path))
	}
	if err != nil {
		return err
	}
	_, err = file.Write(content)
	return errors.Join(err, file.Close())
}

// Rename gives the flow or function the name to in dir, which moves its file into to's folders,
// and moves its last result and its triggers turned on with it.
func (f Flow) Rename(dir, to string) (Flow, error) {
	kind := map[bool]string{true: "function", false: "flow"}[f.Function]
	if err := ValidName(kind, to); err != nil {
		return Flow{}, err
	}
	moved := f
	moved.Name, moved.File = to, FilePath(dir, to)
	if _, err := os.Stat(moved.File); err == nil {
		return Flow{}, fmt.Errorf("%s %s already exists", kind, to)
	}
	if err := os.MkdirAll(filepath.Dir(moved.File), 0o755); err != nil {
		return Flow{}, fmt.Errorf("moving %s %s: %w", kind, f.Name, err)
	}
	if err := os.Rename(f.File, moved.File); err != nil {
		return Flow{}, fmt.Errorf("moving %s %s: %w", kind, f.Name, err)
	}
	if !f.Function {
		return moved, renameKept(f.Name, to)
	}
	return moved, nil
}

// renameKept moves the last result and the triggers turned on of a flow to its new name.
func renameKept(from, to string) error {
	var problems []error
	results := ResultsDir()
	if _, err := os.Stat(filepath.Join(results, from+".json")); err == nil {
		target := filepath.Join(results, to+".json")
		problems = append(problems, os.MkdirAll(filepath.Dir(target), 0o700), os.Rename(filepath.Join(results, from+".json"), target))
	}
	if Enabled(TriggersFile())[from] {
		problems = append(problems, SetEnabled(TriggersFile(), from, false), SetEnabled(TriggersFile(), to, true))
	}
	return errors.Join(problems...)
}

// MoveFolder moves the folder at from in dir, with all in it, to the path to; flows lists what
// is in dir, whose names change with it.
func MoveFolder(dir, from, to string, flows []Flow) ([]Flow, error) {
	switch {
	case !naming.IsValidPath(from) || !naming.IsValidPath(to):
		return nil, fmt.Errorf("directory %q: letters, digits, . - _, and / between directories", to)
	case to == from || strings.HasPrefix(to+"/", from+"/"):
		return nil, fmt.Errorf("%s cannot go inside itself", from)
	}
	target := filepath.Join(dir, filepath.FromSlash(to))
	if _, err := os.Stat(target); err == nil {
		return nil, fmt.Errorf("%s already exists", to)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, fmt.Errorf("moving directory %s: %w", from, err)
	}
	if err := os.Rename(filepath.Join(dir, filepath.FromSlash(from)), target); err != nil {
		return nil, fmt.Errorf("moving directory %s: %w", from, err)
	}
	var moved []Flow
	var problems []error
	for _, f := range flows {
		if rest, inside := strings.CutPrefix(f.Name, from+"/"); inside {
			renamed := f
			renamed.Name, renamed.File = to+"/"+rest, FilePath(dir, to+"/"+rest)
			if !f.Function {
				problems = append(problems, renameKept(f.Name, renamed.Name))
			}
			moved = append(moved, renamed)
		}
	}
	return moved, errors.Join(problems...)
}

// RenameCalls makes the call steps of the flows that run the function from run it as to, and
// saves them, telling which changed.
func RenameCalls(flows []Flow, from, to string) ([]string, error) {
	return renameSteps(flows, func(s *Step) *string { return &s.Call }, func(name string) bool { return name == from }, to)
}

// RenameScripts makes the script steps of the flows that run a script moved, those whose script
// is runs tells, run it as to, its ID now, and saves them, telling which changed.
func RenameScripts(flows []Flow, runs func(id string) bool, to string) ([]string, error) {
	return renameSteps(flows, func(s *Step) *string { return &s.Script }, func(id string) bool { return id != "" && runs(id) }, to)
}

// renameSteps gives the steps whose field names something from the name to, saving the flows
// they are in and telling which changed.
func renameSteps(flows []Flow, field func(*Step) *string, from func(string) bool, to string) ([]string, error) {
	var changed []string
	var problems []error
	for _, f := range flows {
		var paths []string
		walkSteps(f.Steps, "", "", func(s Step, path string) {
			if from(*field(&s)) {
				paths = append(paths, path)
			}
		})
		if len(paths) == 0 {
			continue
		}
		fresh, err := Load(f.File)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		fresh.Name, fresh.Function = f.Name, f.Function
		for _, path := range paths {
			if s := fresh.step(path); s != nil {
				*field(s) = to
			}
		}
		problems = append(problems, fresh.Save())
		changed = append(changed, f.Name)
	}
	return changed, errors.Join(problems...)
}
