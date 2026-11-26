package tree

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWalkReadsFoldersInFoldersInOrder(t *testing.T) {
	root := t.TempDir()
	for _, file := range []string{"b.yaml", "ops/deploy.yaml", "ops/db/backup.yaml", "ops/db/notes.txt", ".git/x.yaml", "a/empty/.keep"} {
		path := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var folders, items []string
	err := Walk(root, func(_ string, entry fs.DirEntry) bool {
		return !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml")
	}, func(path string) {
		folders = append(folders, path)
	}, func(folder, path string) {
		rel, _ := filepath.Rel(root, path)
		items = append(items, folder+"|"+filepath.ToSlash(rel))
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "a/empty", "ops", "ops/db"}; !slices.Equal(folders, want) {
		t.Errorf("folders %v, want %v (no .git)", folders, want)
	}
	if want := []string{"|b.yaml", "ops/db|ops/db/backup.yaml", "ops|ops/deploy.yaml"}; !slices.Equal(items, want) {
		t.Errorf("items %v, want %v", items, want)
	}
	if err := Walk(filepath.Join(root, "missing"), nil, nil, nil); err != nil {
		t.Errorf("a missing root: %v", err)
	}
}

func TestParentAndBase(t *testing.T) {
	if Parent("ops/db/backup") != "ops/db" || Parent("deploy") != "" || Base("ops/db") != "db" || Base("x") != "x" {
		t.Error("Parent or Base")
	}
}
