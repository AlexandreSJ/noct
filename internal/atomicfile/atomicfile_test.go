package atomicfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// onlyFiles fails when dir holds anything besides the names given, such as a temporary file left.
func onlyFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, entry := range entries {
		found = append(found, entry.Name())
	}
	if strings.Join(found, " ") != strings.Join(names, " ") {
		t.Errorf("files = %v, want %v", found, names)
	}
}

func TestWriteCreatesTheFileWithItsMode(t *testing.T) {
	dir := t.TempDir()
	for _, mode := range []fs.FileMode{0o600, 0o644} {
		path := filepath.Join(dir, fmt.Sprintf("%o.json", mode))
		if err := Write(path, []byte("hello"), mode); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, path); got != "hello" {
			t.Errorf("content = %q", got)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != mode&^umask {
			t.Errorf("mode = %o, want %o", info.Mode().Perm(), mode&^umask)
		}
	}
	onlyFiles(t, dir, "600.json", "644.json")
}

func TestWriteReplacesAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.yaml")
	if err := os.WriteFile(path, []byte("old content that is longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "new" {
		t.Errorf("content = %q", got)
	}
	onlyFiles(t, filepath.Dir(path), "a.yaml")
}

func TestWriteFailsWithoutTheFolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "a.yaml")
	if err := Write(path, []byte("x"), 0o644); err == nil {
		t.Fatal("wrote into a folder that does not exist")
	}
}

func TestUpdateStartsFromNothingWithoutTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runs.json")
	err := Update(path, 0o600, func(current []byte) ([]byte, error) {
		if current != nil {
			t.Errorf("current = %q, want nil", current)
		}
		return []byte("first"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "first" {
		t.Errorf("content = %q", got)
	}
	onlyFiles(t, dir, "runs.json", "runs.json.lock")
}

func TestUpdateChangesTheCurrentContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.json")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Update(path, 0o600, func(current []byte) ([]byte, error) {
		return append(current, 'b'), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "ab" {
		t.Errorf("content = %q", got)
	}
}

func TestUpdateKeepsTheFileWhenTheChangeFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.json")
	if err := os.WriteFile(path, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("refused")
	err := Update(path, 0o600, func([]byte) ([]byte, error) { return []byte("b"), refused })
	if !errors.Is(err, refused) {
		t.Errorf("err = %v, want %v", err, refused)
	}
	if got := readFile(t, path); got != "a" {
		t.Errorf("content = %q", got)
	}
}

func TestUpdateWritesNothingWhenNothingChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "params.json")
	err := Update(path, 0o600, func(current []byte) ([]byte, error) { return current, nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an unchanged missing file was written: %v", err)
	}
}

func TestConcurrentUpdatesLoseNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count")
	const writers = 50
	var wg sync.WaitGroup
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Update(path, 0o600, func(current []byte) ([]byte, error) {
				return append(current, 'x'), nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := readFile(t, path); len(got) != writers {
		t.Errorf("kept %d updates of %d", len(got), writers)
	}
}

func TestConcurrentWritesNeverLeaveAPartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	const writers = 20
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Write(path, []byte(strings.Repeat(fmt.Sprint(i%10), 4096)), 0o600); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got := readFile(t, path)
	if len(got) != 4096 || strings.Count(got, got[:1]) != 4096 {
		t.Errorf("the file mixes writers: %d bytes", len(got))
	}
	onlyFiles(t, filepath.Dir(path), "a.json")
}
