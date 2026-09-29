package script

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ProjectDirName is the folder, at a project's root, that holds its noct files.
const ProjectDirName = ".noct"

// PersonalDir holds the user's own scripts: $XDG_DATA_HOME/noct/scripts, or
// ~/.local/share/noct/scripts.
func PersonalDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "noct", "scripts")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "noct-scripts")
	}
	return filepath.Join(home, ".local", "share", "noct", "scripts")
}

// ResolveDir is where the user's scripts are: the configured folder, with ~ for the home
// directory, or PersonalDir when none is configured.
func ResolveDir(configured string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return PersonalDir()
	}
	if rest, ok := strings.CutPrefix(configured, "~"); ok && (rest == "" || strings.HasPrefix(rest, "/")) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return configured
}

// ProjectDir finds the scripts of the project that contains dir: the nearest .noct/scripts in dir
// or one of its parents, the way git finds its repository. It is "" outside a project.
func ProjectDir(dir string) string {
	if dir == "" {
		return ""
	}
	for {
		candidate := filepath.Join(dir, ProjectDirName, "scripts")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// FolderKind says where a folder of scripts comes from.
type FolderKind int

const (
	// GroupFolder is a folder inside the personal scripts folder.
	GroupFolder FolderKind = iota
	// LinkedFolder is a folder anywhere else, added to noct by its path.
	LinkedFolder
	// ProjectFolder is the .noct/scripts of the project around the working directory.
	ProjectFolder
)

// Folder groups scripts: each of its subfolders with a script.yaml is a script.
type Folder struct {
	Name    string
	Dir     string
	Kind    FolderKind
	Scripts []Script
}

// Library is every script available: the personal ones, alone or in folders, and those of linked
// folders and of the project. Scripts that cannot be loaded are listed too, with their Problem.
type Library struct {
	Folders []Folder
	// Scripts are the personal scripts outside any folder.
	Scripts []Script
	// Problems are folders of scripts that could not be read at all.
	Problems []error
}

// All lists every script: those in folders first, then those outside.
func (l Library) All() []Script {
	var all []Script
	for _, f := range l.Folders {
		all = append(all, f.Scripts...)
	}
	return append(all, l.Scripts...)
}

// Broken lists the problems of the scripts that cannot be loaded.
func (l Library) Broken() []error {
	var broken []error
	for _, s := range l.All() {
		if s.Problem != nil {
			broken = append(broken, s.Problem)
		}
	}
	return broken
}

// Find returns the script with the given ID (folder/name, or name outside folders), or with the
// given name when only one script has it.
func (l Library) Find(id string) (Script, bool) {
	var named []Script
	for _, s := range l.All() {
		if s.ID() == id {
			return s, true
		}
		if s.Name == id {
			named = append(named, s)
		}
	}
	if len(named) == 1 {
		return named[0], true
	}
	return Script{}, false
}

// LoadLibrary reads the personal scripts in personal, where a subfolder without a script.yaml is a
// folder of scripts; the linked folders; and the scripts of the project around workDir.
func LoadLibrary(personal string, linked []string, workDir string) Library {
	var lib Library
	entries, err := readDir(personal)
	if err != nil {
		lib.Problems = append(lib.Problems, err)
	}
	for _, entry := range entries {
		dir := filepath.Join(personal, entry)
		if isScript(dir) {
			lib.Scripts = append(lib.Scripts, loadScript(dir, ""))
		} else {
			lib.addFolder(entry, dir, GroupFolder)
		}
	}
	for _, path := range linked {
		dir := ResolveDir(path)
		if dir == personal || slices.ContainsFunc(lib.Folders, func(f Folder) bool { return f.Dir == dir }) {
			continue // already listed
		}
		if _, err := os.Stat(dir); err != nil {
			lib.Problems = append(lib.Problems, fmt.Errorf("folder %s: %w", path, err))
			continue
		}
		lib.addFolder(filepath.Base(dir), dir, LinkedFolder)
	}
	if project := ProjectDir(workDir); project != "" {
		lib.addFolder("project", project, ProjectFolder)
	}
	slices.SortStableFunc(lib.Folders, func(a, b Folder) int { return cmp.Compare(a.Name, b.Name) })
	SortByName(lib.Scripts)
	return lib
}

func (l *Library) addFolder(name, dir string, kind FolderKind) {
	folder := Folder{Name: name, Dir: dir, Kind: kind}
	entries, err := readDir(dir)
	if err != nil {
		l.Problems = append(l.Problems, err)
	}
	for _, entry := range entries {
		if scriptDir := filepath.Join(dir, entry); isScript(scriptDir) {
			folder.Scripts = append(folder.Scripts, loadScript(scriptDir, name))
		}
	}
	SortByName(folder.Scripts)
	l.Folders = append(l.Folders, folder)
}

// readDir lists the subfolders of dir; a missing dir has none.
func readDir(dir string) ([]string, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, err
}

func isScript(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, FileName))
	return err == nil
}

// loadScript loads the script in dir, keeping a broken one with its problem.
func loadScript(dir, folder string) Script {
	s, err := Load(dir)
	if err != nil {
		s = Script{Name: filepath.Base(dir), Dir: dir, Edited: modTime(filepath.Join(dir, FileName)), Problem: err}
	}
	s.Folder = folder
	return s
}

// CreateFolder makes a new folder of scripts inside root.
func CreateFolder(root, name string) (string, error) {
	if !validName.MatchString(name) {
		return "", fmt.Errorf("folder name %q: use letters, digits, '.', '-' and '_'", name)
	}
	dir := filepath.Join(root, name)
	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("%s already exists", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating folder %s: %w", name, err)
	}
	return dir, nil
}

// DeleteFolder removes a folder of scripts and the scripts in it.
func DeleteFolder(f Folder) error {
	if f.Kind != GroupFolder || !validName.MatchString(filepath.Base(f.Dir)) {
		return fmt.Errorf("refusing to delete %q", f.Dir)
	}
	if err := os.RemoveAll(f.Dir); err != nil {
		return fmt.Errorf("deleting folder %s: %w", f.Name, err)
	}
	return nil
}
