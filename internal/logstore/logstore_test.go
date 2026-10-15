package logstore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLogRotatesWhenFull(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "pane.log")
	log, err := Create(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range []string{"12345", "67890", "abc"} {
		if _, err := log.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}

	current, _ := os.ReadFile(path)
	older, _ := os.ReadFile(path + ".1")
	if string(current) != "abc" || string(older) != "1234567890" {
		t.Errorf("current %q, older %q", current, older)
	}
}

func TestDirFollowsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state")
	if got := Dir(); got != "/tmp/state/noct" {
		t.Errorf("got %q", got)
	}
}

func writeLog(t *testing.T, dir, day, file string, size int) string {
	t.Helper()
	path := filepath.Join(dir, day, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x\n"), size/2), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListReadsLogsWithTheirRotatedParts(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "2026-09-26", "235959-pane1.log", 10)
	writeLog(t, dir, "2026-09-27", "080000-greet.log", 20)
	writeLog(t, dir, "2026-09-27", "080000-greet.log.1", 100)
	writeLog(t, dir, "2026-09-27", "notes.txt", 4)
	entries, err := List(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("List = %+v, %v", entries, err)
	}
	last := entries[1]
	if last.Name != "greet" || last.Size != 120 || last.Day() != "2026-09-27" || last.At.Hour() != 8 {
		t.Errorf("entry = %+v", last)
	}
	if lines := CountLines(last); lines != 60 {
		t.Errorf("lines = %d, want 60", lines)
	}
}

func TestPruneRemovesOldLogsThenTheOldestOverTheSize(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)
	writeLog(t, dir, "2026-05-01", "080000-old.log", 10)
	writeLog(t, dir, "2026-09-20", "080000-a.log", 60)
	writeLog(t, dir, "2026-09-27", "080000-b.log", 60)
	removed, err := Prune(dir, 90, 100, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 || removed[0].Name != "old" || removed[1].Name != "a" {
		t.Fatalf("removed %+v, want old (too old) and a (over the size)", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-05-01")); !os.IsNotExist(err) {
		t.Error("the emptied day folder is still there")
	}
	left, _ := List(dir)
	if len(left) != 1 || left[0].Name != "b" {
		t.Errorf("left %+v", left)
	}
}

func TestExpiredListsWhatPruneWouldRemoveAndRemovesNothing(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)
	writeLog(t, dir, "2026-05-01", "080000-old.log", 10)
	writeLog(t, dir, "2026-09-27", "080000-b.log", 60)
	expired, err := Expired(dir, 30, 1000, now)
	if err != nil || len(expired) != 1 || expired[0].Name != "old" {
		t.Fatalf("expired %+v, %v; want old", expired, err)
	}
	if left, _ := List(dir); len(left) != 2 {
		t.Errorf("Expired removed logs: %d left", len(left))
	}
}

func TestPrettySize(t *testing.T) {
	for size, want := range map[int64]string{812: "812 B", 12700: "12.4 KB", 3 << 20: "3 MB", 5 << 30: "5 GB", 1536 << 20: "1.5 GB"} {
		if got := PrettySize(size); got != want {
			t.Errorf("PrettySize(%d) = %q, want %q", size, got, want)
		}
	}
}

// A flow run's result lives beside its log: counted in its size and removed with it, never
// listed as a log of its own.
func TestResultGoesWithItsLog(t *testing.T) {
	dir := t.TempDir()
	log := writeLog(t, dir, "2026-09-27", "080000-flow-x.log", 20)
	result := ResultPath(log)
	if filepath.Base(result) != "080000-flow-x.result.json" {
		t.Fatalf("result path = %s", result)
	}
	writeLog(t, dir, "2026-09-27", filepath.Base(result), 30)
	entries, err := List(dir)
	if err != nil || len(entries) != 1 || entries[0].Size != 50 {
		t.Fatalf("List = %+v, %v", entries, err)
	}
	if lines := CountLines(entries[0]); lines != 10 {
		t.Errorf("lines = %d, want the log's 10 without the result's", lines)
	}
	if err := Remove(entries...); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(result); !os.IsNotExist(err) {
		t.Errorf("the result outlived its log: %v", err)
	}
}

func TestAppendGoesOnAfterTheLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "pane.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	log, err := Append(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range []string{"678", "abc"} {
		if _, err := log.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	_ = log.Close()
	current, _ := os.ReadFile(path)
	older, _ := os.ReadFile(path + ".1")
	if string(current) != "abc" || string(older) != "12345678" {
		t.Errorf("current %q, older %q", current, older)
	}
}

func TestTailStartsAtALine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pane.log")
	if err := os.WriteFile(path, []byte("first line\nsecond\nthird\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := string(Tail(path, 10)); got != "third\n" {
		t.Errorf("tail of 10 bytes = %q, want the last whole line", got)
	}
	if got := string(Tail(path, 100)); got != "first line\nsecond\nthird\n" {
		t.Errorf("tail of a short log = %q, want all of it", got)
	}
	if Tail(filepath.Join(t.TempDir(), "missing.log"), 10) != nil {
		t.Error("a missing log has a tail")
	}
}
