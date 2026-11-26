// Package examples embeds the demo scripts, flows and functions the installer copies into a
// user's noct folders, so they can be explored without touching anything real.
package examples

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed demo
var demo embed.FS

// InstallDemo copies the demo scripts, flows and functions into their folders, creating them as
// needed: each in a folder demo. A file that already exists is left untouched, so a user's edits
// survive a second install, and a demo flow or function an older noct installed outside the
// folder moves into it. It returns the paths it wrote or moved to, in that order.
func InstallDemo(scriptsDir, flowsDir, functionsDir string) (written []string, err error) {
	for _, tree := range []struct{ root, dest string }{
		{"demo/scripts", scriptsDir},
		{"demo/flows", flowsDir},
		{"demo/functions", functionsDir},
	} {
		moved, err := moveOutside(tree.root, tree.dest)
		written = append(written, moved...)
		if err != nil {
			return written, err
		}
		files, err := copyTree(tree.root, tree.dest)
		written = append(written, files...)
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

// moveOutside moves each demo file an older noct installed straight in dest, as
// flows/demo-deploy.yaml, to where the demo keeps it now, flows/demo/demo-deploy.yaml, unless a
// file is there already. Scripts were always in their folder, so nothing of theirs moves.
func moveOutside(root, dest string) ([]string, error) {
	var moved []string
	err := fs.WalkDir(demo, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || filepath.Dir(rel) == "." {
			return err
		}
		outside, target := filepath.Join(dest, filepath.Base(rel)), filepath.Join(dest, rel)
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		if info, err := os.Stat(outside); err != nil || info.IsDir() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("creating folder for %s: %w", target, err)
		}
		if err := os.Rename(outside, target); err != nil {
			return fmt.Errorf("moving %s: %w", outside, err)
		}
		moved = append(moved, target)
		return nil
	})
	if err != nil {
		return moved, fmt.Errorf("moving the demo of %s in %s: %w", root, dest, err)
	}
	return moved, nil
}

// copyTree copies every file under root in the embedded FS to the matching path under dest,
// skipping files that already exist. It returns the destination paths it wrote.
func copyTree(root, dest string) ([]string, error) {
	var written []string
	err := fs.WalkDir(demo, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", path, err)
		}
		target := filepath.Join(dest, rel)
		ok, err := copyFile(path, target)
		if err != nil {
			return err
		}
		if ok {
			written = append(written, target)
		}
		return nil
	})
	if err != nil {
		return written, fmt.Errorf("installing %s into %s: %w", root, dest, err)
	}
	return written, nil
}

// copyFile writes the embedded file at src to target, unless target already exists. It reports
// whether it wrote the file.
func copyFile(src, target string) (bool, error) {
	if _, err := os.Stat(target); err == nil {
		return false, nil
	}
	content, err := demo.ReadFile(src)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, fmt.Errorf("creating folder for %s: %w", target, err)
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return false, fmt.Errorf("writing %s: %w", target, err)
	}
	return true, nil
}
