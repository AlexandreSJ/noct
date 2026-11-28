package script

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/tree"
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

// Folder groups scripts: each of its subfolders with a script.yaml is a script, and any other is
// a folder inside it, at any depth.
type Folder struct {
	// Name is the folder's path among the folders, ops/db inside ops: the start of its scripts' IDs.
	Name string
	Dir  string
	// Kind is where it comes from; a folder inside a linked or project folder has that folder's.
	Kind FolderKind
	// Parent is the Dir of the folder it is inside, or "" for a folder at the top.
	Parent  string
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
	lib.addPersonal(personal)
	for _, path := range linked {
		lib.addLinked(path, personal)
	}
	if project := ProjectDir(workDir); project != "" {
		lib.addFolder("project", project, ProjectFolder)
	}
	slices.SortStableFunc(lib.Folders, func(a, b Folder) int { return cmp.Compare(a.Name, b.Name) })
	for i := range lib.Folders {
		SortByName(lib.Folders[i].Scripts)
	}
	SortByName(lib.Scripts)
	return lib
}

// addPersonal adds the scripts of the personal folder, where a subfolder without a script.yaml
// is a folder of scripts, and so on inside it.
func (l *Library) addPersonal(personal string) {
	l.addTree(personal, "", "", GroupFolder)
}

// addLinked adds a linked folder, unless it is already listed.
func (l *Library) addLinked(path, personal string) {
	dir := ResolveDir(path)
	if dir == personal || slices.ContainsFunc(l.Folders, func(f Folder) bool { return f.Dir == dir }) {
		return
	}
	if _, err := os.Stat(dir); err != nil {
		l.Problems = append(l.Problems, fmt.Errorf("directory %s: %w", path, err))
		return
	}
	l.addFolder(filepath.Base(dir), dir, LinkedFolder)
}

// addFolder adds a linked or project folder, named name, with the folders inside it.
func (l *Library) addFolder(name, dir string, kind FolderKind) {
	l.Folders = append(l.Folders, Folder{Name: name, Dir: dir, Kind: kind})
	l.addTree(dir, name, dir, kind)
}

// addTree adds the scripts and folders under dir: those at its top go to the folder named name
// (outside folders when name is ""), whose Dir is top.
func (l *Library) addTree(dir, name, top string, kind FolderKind) {
	at := map[string]int{} // a folder's path from dir, to the index of its Folder
	if name != "" {
		at[""] = len(l.Folders) - 1
	}
	named := func(path string) string { return strings.TrimPrefix(name+"/"+path, "/") }
	err := tree.Walk(dir, func(parent string, entry fs.DirEntry) bool {
		return entry.IsDir() && isScript(filepath.Join(parent, entry.Name()))
	}, func(path string) {
		parent := top
		if up := tree.Parent(path); up != "" {
			parent = l.Folders[at[up]].Dir
		}
		at[path] = len(l.Folders)
		l.Folders = append(l.Folders, Folder{Name: named(path), Dir: filepath.Join(dir, filepath.FromSlash(path)), Kind: kind, Parent: parent})
	}, func(folder, path string) {
		i, ok := at[folder]
		if !ok {
			l.Scripts = append(l.Scripts, loadScript(path, ""))
			return
		}
		l.Folders[i].Scripts = append(l.Folders[i].Scripts, loadScript(path, l.Folders[i].Name))
	})
	if err != nil {
		l.Problems = append(l.Problems, err)
	}
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

// CreateFolder makes the directory at path, ops/db, in root, and the directories around it.
func CreateFolder(root, path string) (string, error) {
	if !naming.IsValidPath(path) {
		return "", fmt.Errorf("directory %q: letters, digits, . - _, and / between directories", path)
	}
	dir := filepath.Join(root, filepath.FromSlash(path))
	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("%s already exists", path)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating directory %s: %w", path, err)
	}
	return dir, nil
}

func DeleteFolder(f Folder) error {
	if f.Kind != GroupFolder || !naming.IsValid(filepath.Base(f.Dir)) {
		return fmt.Errorf("refusing to delete %q", f.Dir)
	}
	if err := os.RemoveAll(f.Dir); err != nil {
		return fmt.Errorf("deleting directory %s: %w", f.Name, err)
	}
	return nil
}
