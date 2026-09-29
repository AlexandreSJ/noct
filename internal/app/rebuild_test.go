package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRebuiltNoticesANewBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "noct")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := stampOf(path)
	if stamp.rebuilt() {
		t.Fatal("an unchanged binary looks rebuilt")
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if !stamp.rebuilt() {
		t.Error("a newer binary was not noticed")
	}
	if (binaryStamp{}).rebuilt() {
		t.Error("an unknown binary looks rebuilt")
	}
}
