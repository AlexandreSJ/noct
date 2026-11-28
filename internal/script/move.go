package script

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexandreSJ/noct/internal/naming"
)

// Scripts are directories in directories, as flows are files in them: these move and copy a
// script, and move a directory with the scripts in it.

// MoveTo moves the script's directory to dir, which must not exist yet, making the directories
// around it.
func MoveTo(s Script, dir string) (Script, error) {
	name := filepath.Base(dir)
	if !naming.IsValid(name) {
		return Script{}, fmt.Errorf("script name %q: use letters, digits, '.', '-' and '_'", name)
	}
	if err := free(dir, "script "+name); err != nil {
		return Script{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return Script{}, fmt.Errorf("moving script %s: %w", s.Name, err)
	}
	if err := os.Rename(s.Dir, dir); err != nil {
		return Script{}, fmt.Errorf("moving script %s: %w", s.Name, err)
	}
	s.Name, s.Dir = name, dir
	return s, nil
}

// Copy writes a copy of the script's directory, its notes and files too, at dir, which must not
// exist yet.
func Copy(s Script, dir string) error {
	name := filepath.Base(dir)
	if !naming.IsValid(name) {
		return fmt.Errorf("script name %q: use letters, digits, '.', '-' and '_'", name)
	}
	if err := free(dir, "script "+name); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("copying script %s: %w", s.Name, err)
	}
	if err := os.CopyFS(dir, os.DirFS(s.Dir)); err != nil {
		return fmt.Errorf("copying script %s: %w", s.Name, err)
	}
	return nil
}

// MoveFolder moves a directory of personal scripts, with what is in it, to dir, which must not
// exist yet nor be inside it.
func MoveFolder(f Folder, dir string) error {
	switch {
	case f.Kind != GroupFolder:
		return fmt.Errorf("%s is not among your own scripts: move it where it is", f.Name)
	case dir == f.Dir || strings.HasPrefix(dir, f.Dir+string(filepath.Separator)):
		return fmt.Errorf("%s cannot go inside itself", f.Name)
	case !naming.IsValid(filepath.Base(dir)):
		return fmt.Errorf("directory %q: letters, digits, . - _, and / between directories", filepath.Base(dir))
	}
	if err := free(dir, filepath.Base(dir)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return fmt.Errorf("moving directory %s: %w", f.Name, err)
	}
	if err := os.Rename(f.Dir, dir); err != nil {
		return fmt.Errorf("moving directory %s: %w", f.Name, err)
	}
	return nil
}

// free tells that nothing is at dir yet.
func free(dir, what string) error {
	_, err := os.Lstat(dir)
	switch {
	case err == nil:
		return fmt.Errorf("%s already exists", what)
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	return nil
}
