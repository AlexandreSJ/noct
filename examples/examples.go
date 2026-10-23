// Package examples embeds the demo scripts and flows the installer copies into a
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

// InstallDemo copies the demo scripts into scriptsDir and the demo flows into flowsDir, creating
// the folders as needed. A file that already exists is left untouched, so a user's edits survive a
// second install. It returns the paths it wrote, in the order it wrote them.
func InstallDemo(scriptsDir, flowsDir string) (written []string, err error) {
	scripts, err := copyTree("demo/scripts", scriptsDir)
	if err != nil {
		return written, err
	}
	written = append(written, scripts...)
	flows, err := copyTree("demo/flows", flowsDir)
	if err != nil {
		return written, err
	}
	return append(written, flows...), nil
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
