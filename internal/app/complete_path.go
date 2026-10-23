package app

import (
	"os"
	"path/filepath"
	"strings"
)

// completePath completes the last part of a typed path, as a shell's tab does: to the longest
// start the names in its folder share, with a slash after a single folder. A relative path is
// read from dir and ~/ from the home folder; a path nothing matches stays as typed.
func completePath(typed, dir string) string {
	folder, start := "", typed
	if slash := strings.LastIndex(typed, "/"); slash >= 0 {
		folder, start = typed[:slash+1], typed[slash+1:]
	}
	entries, err := os.ReadDir(resolveFolder(folder, dir))
	if err != nil {
		return typed
	}
	var matches []os.DirEntry
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), start) && (strings.HasPrefix(start, ".") || !strings.HasPrefix(entry.Name(), ".")) {
			matches = append(matches, entry)
		}
	}
	switch len(matches) {
	case 0:
		return typed
	case 1:
		name := matches[0].Name()
		if matches[0].IsDir() {
			name += "/"
		}
		return folder + name
	}
	shared := matches[0].Name()
	for _, entry := range matches[1:] {
		shared = sharedStart(shared, entry.Name())
	}
	return folder + shared
}

// resolveFolder is where a typed folder is on disk.
func resolveFolder(folder, dir string) string {
	if rest, ok := strings.CutPrefix(folder, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	if folder == "" {
		return dir
	}
	if filepath.IsAbs(folder) {
		return folder
	}
	return filepath.Join(dir, folder)
}

func sharedStart(a, b string) string {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return a[:n]
}
