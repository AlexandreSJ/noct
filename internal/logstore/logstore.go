// Package logstore keeps what programs in panes print on disk, so it outlives the screen.
package logstore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Dir is noct's state directory: $XDG_STATE_HOME/noct, or ~/.local/state/noct.
func Dir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "noct")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "noct-state")
	}
	return filepath.Join(home, ".local", "state", "noct")
}

// File is a log capped at a size: when full, it becomes path.1 (replacing the older one)
// and a new file starts, so a chatty program never fills the disk.
type File struct {
	lock    sync.Mutex
	path    string
	file    *os.File
	size    int64
	maxSize int64
	now     func() time.Time
	// lastDay is the last day the log has output on, as DayLayout writes it, or "" for a log
	// outside the day folders, which keeps no days.
	lastDay string
	clock   lineClock
}

func Create(path string, maxSize int64) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating the log directory: %w", err)
	}
	_ = os.Remove(DaysPath(path)) // a log started again has had output on its first day only
	f := &File{path: path, maxSize: maxSize, now: time.Now, lastDay: lastDayOf(path)}
	if err := f.open(os.O_TRUNC); err != nil {
		return nil, err
	}
	_ = os.Remove(TimesPath(path))
	f.clock.reset(0)
	return f, nil
}

// Append goes on writing the log at path after what it has, such as a terminal opened again in
// the place of one that closed; without the file, it starts one.
func Append(path string, maxSize int64) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating the log directory: %w", err)
	}
	f := &File{path: path, maxSize: maxSize, now: time.Now, lastDay: lastDayOf(path)}
	if err := f.open(os.O_APPEND); err != nil {
		return nil, err
	}
	if info, err := f.file.Stat(); err == nil {
		f.size = info.Size()
	}
	lines, midLine := CountFileLines(path), endsMidLine(path)
	if midLine {
		lines++ // its last line started, though it has no newline yet
	}
	f.clock.reset(lines)
	f.clock.atStart = !midLine
	return f, nil
}

func (f *File) open(mode int) error {
	file, err := os.OpenFile(f.path, os.O_CREATE|os.O_WRONLY|mode, 0o600)
	if err != nil {
		return fmt.Errorf("opening log %s: %w", f.path, err)
	}
	f.file, f.size = file, 0
	return nil
}

// Tail is up to size bytes from the end of the log at path, starting at a line, or nil without it.
func Tail(path string, size int64) []byte {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil
	}
	start := max(info.Size()-size, 0)
	content := make([]byte, info.Size()-start)
	if _, err := file.ReadAt(content, start); err != nil {
		return nil
	}
	if start > 0 {
		if newline := bytes.IndexByte(content, '\n'); newline >= 0 {
			content = content[newline+1:] // the cut may fall inside an escape sequence or a character
		}
	}
	return content
}

func (f *File) Write(p []byte) (int, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.noteDay()
	if f.size+int64(len(p)) > f.maxSize && f.size > 0 {
		if err := f.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := f.file.Write(p)
	f.size += int64(n)
	f.clock.written(f.path, p[:n], f.now())
	return n, err
}

// noteDay adds today to the log's days file the first time the log gets output on a day other
// than the one it started on, so the log shows on every day it has output.
func (f *File) noteDay() {
	day := f.now().Format(DayLayout)
	if f.lastDay == "" || day == f.lastDay {
		return
	}
	f.lastDay = day
	file, err := os.OpenFile(DaysPath(f.path), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return // the log only misses that day in the calendar
	}
	_, _ = fmt.Fprintln(file, day)
	_ = file.Close()
}

// lastDayOf is the last day the log at path has output on: the last its days file holds, or the
// day folder it started in; "" when it is in none.
func lastDayOf(path string) string {
	started := filepath.Base(filepath.Dir(path))
	if _, err := time.Parse(DayLayout, started); err != nil {
		return ""
	}
	if days := recordedDays(path); len(days) > 0 {
		return days[len(days)-1]
	}
	return started
}

func (f *File) rotate() error {
	if err := f.file.Close(); err != nil {
		return fmt.Errorf("closing log %s: %w", f.path, err)
	}
	if err := os.Rename(f.path, f.path+".1"); err != nil {
		return fmt.Errorf("rotating log %s: %w", f.path, err)
	}
	if err := os.Rename(TimesPath(f.path), TimesPath(f.path+".1")); err != nil {
		_ = os.Remove(TimesPath(f.path + ".1")) // the older part's lines only lose their times
	}
	f.clock.reset(0)
	return f.open(os.O_TRUNC)
}

// endsMidLine tells whether the file's last line has no newline yet.
func endsMidLine(path string) bool {
	tail := Tail(path, 1)
	return len(tail) > 0 && tail[len(tail)-1] != '\n'
}

func (f *File) Close() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.file.Close()
}
