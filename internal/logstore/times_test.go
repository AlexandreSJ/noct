package logstore

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// clockedLog is a log whose writes happen at the times tick gives.
func clockedLog(t *testing.T, path string, maxSize int64) (*File, func(time.Duration)) {
	t.Helper()
	log, err := Create(path, maxSize)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 11, 9, 30, 0, 0, time.Local)
	log.now = func() time.Time { return now }
	return log, func(d time.Duration) { now = now.Add(d) }
}

func write(t *testing.T, log *File, text string) {
	t.Helper()
	if _, err := log.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
}

func TestALogKeepsWhenEachLineStarted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	log, tick := clockedLog(t, path, 1<<20)
	write(t, log, "one\ntwo\n")
	write(t, log, "three") // the same second: nothing new to keep
	tick(2 * time.Second)
	write(t, log, " still three\nfour\n") // three started before; four starts now
	tick(time.Second)
	write(t, log, "five\n")
	log.Close()

	got := LineTimes(path)
	base := time.Date(2026, 11, 11, 9, 30, 0, 0, time.Local)
	want := []LineTime{{1, base}, {4, base.Add(2 * time.Second)}, {5, base.Add(3 * time.Second)}}
	if len(got) != len(want) {
		t.Fatalf("times %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Line != want[i].Line || !got[i].At.Equal(want[i].At) {
			t.Errorf("time %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestLineTimesGoWithTheirPartWhenTheLogRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	log, tick := clockedLog(t, path, 8)
	write(t, log, "12345\n")
	tick(time.Second)
	write(t, log, "abc\n")
	log.Close()
	if older := LineTimes(path + ".1"); len(older) != 1 || older[0].Line != 1 {
		t.Errorf("older part's times %v, want its line 1", older)
	}
	if current := LineTimes(path); len(current) != 1 || current[0].Line != 1 || current[0].At.Second() != 1 {
		t.Errorf("current part's times %v, want line 1 a second later", current)
	}
}

func TestAppendingGoesOnCountingLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pane.log")
	log, _ := clockedLog(t, path, 1<<20)
	write(t, log, "a\nb\nc")
	log.Close()
	again, err := Append(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	again.now = func() time.Time { return time.Date(2026, 11, 11, 10, 0, 0, 0, time.Local) }
	write(t, again, " goes on\nd\n")
	again.Close()
	if times := LineTimes(path); len(times) != 2 || times[1].Line != 4 {
		t.Errorf("times %v, want the appended line d as line 4", times)
	}
}

func TestPrintLinesTellsWhenEachStarted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	log, tick := clockedLog(t, path, 1<<20)
	write(t, log, "one\ntwo\n")
	tick(65 * time.Second)
	write(t, log, "three\n")
	log.Close()
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "old.log"), []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := PrintLines(&out, LineRange{From: 2}, nil, path); err != nil {
		t.Fatal(err)
	}
	want := dim("2026-11-11 09-30-00") + " two\n" + dim("2026-11-11 09-31-05") + " three\n"
	if out.String() != want {
		t.Errorf("printed %q\nwant    %q", out.String(), want)
	}

	out.Reset()
	if err := PrintLines(&out, LineRange{}, nil, filepath.Join(filepath.Dir(path), "old.log"), path); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 4 || lines[0] != strings.Repeat(" ", len(TimeLayout))+" before" {
		t.Errorf("a line with no time kept printed %q, want it blank where the time goes", lines)
	}
}

func TestPrintLinesFollowsTheLastFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	log, _ := clockedLog(t, path, 1<<20)
	write(t, log, "one\n")
	stop := make(chan struct{})
	r, w, _ := os.Pipe()
	done := make(chan error)
	go func() { done <- PrintLines(w, LineRange{Follow: true}, stop, path); w.Close() }()
	write(t, log, "two\n")
	read := make([]byte, 0, 256)
	deadline := time.Now().Add(5 * time.Second)
	for !bytes.Contains(read, []byte("two\n")) && time.Now().Before(deadline) {
		buf := make([]byte, 256)
		n, _ := r.Read(buf)
		read = append(read, buf[:n]...)
	}
	close(stop)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	log.Close()
	if !bytes.Contains(read, []byte(" one\n")) || !bytes.Contains(read, []byte(" two\n")) {
		t.Errorf("followed %q, want both lines", read)
	}
}
