package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCrashTellsTheLastPanicOfARecentLog(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	path := filepath.Join(state, "noct", "daemon.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	log := "panic: an old one\n\ngoroutine 1 [running]:\n" +
		"panic: runtime error: index out of range [18] with length 16\n\ngoroutine 48 [running]:\nmain.main()\n"
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}

	got := Crash()
	want := "panic: runtime error: index out of range [18] with length 16 (details in " + path + ")"
	if got != want {
		t.Errorf("Crash() = %q, want %q", got, want)
	}

	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if got := Crash(); got != "see "+path {
		t.Errorf("with an old log, Crash() = %q, want only where the log is", got)
	}
}

func TestCrashWithoutALog(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if got := Crash(); !strings.HasPrefix(got, "see ") {
		t.Errorf("Crash() = %q, want where the log would be", got)
	}
}
