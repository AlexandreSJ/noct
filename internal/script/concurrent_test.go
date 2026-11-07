package script

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// writers is how many nocts write the same file at once in these tests, as the TUI and the CLI do.
const writers = 20

func TestConcurrentRunsAreAllKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), RunsFile)
	start := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AddRun(path, Run{Script: fmt.Sprint("s", i), At: start.Add(time.Duration(i) * time.Second)}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	runs, err := Runs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != writers {
		t.Errorf("kept %d runs of %d", len(runs), writers)
	}
}

func TestConcurrentRememberedValuesAreAllKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), MemoryFile)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, err := LoadMemory(path)
			if err != nil {
				t.Error(err)
				return
			}
			if err := m.RememberFlow(fmt.Sprint("f", i), map[string]string{"x": "1"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	m, err := LoadMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := range writers {
		if m.FlowValues(fmt.Sprint("f", i)) == nil {
			t.Errorf("lost the values of f%d", i)
		}
	}
}
