package logstore

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// DayLayout names the folder of each day's logs.
const DayLayout = "2006-01-02"

// LogsDir holds the logs of the programs in panes, one folder per day.
func LogsDir() string {
	return filepath.Join(Dir(), "logs")
}

// Path is where a log started at a moment goes: logs/2026-09-27/143005-name.log.
func Path(name string, at time.Time) string {
	return filepath.Join(LogsDir(), at.Format(DayLayout), at.Format("150405")+"-"+name+".log")
}

// resultSuffix names the result a flow run keeps beside its log.
const resultSuffix = ".result.json"

// ResultPath is where the result of the flow run that wrote a log goes: beside it, so it is
// counted and removed with it.
func ResultPath(logPath string) string {
	return strings.TrimSuffix(logPath, ".log") + resultSuffix
}

// Entry is one program's log, with its rotated older part counted in.
type Entry struct {
	Path string
	// Name is what the log was started for: a script, pane3, edit-notes.md.
	Name string
	At   time.Time
	Size int64
}

// Day is the date the log was started on.
func (e Entry) Day() string {
	return e.At.Format(DayLayout)
}

// LogFiles are what the program printed: the log and its rotated older part, when there is one.
func (e Entry) LogFiles() []string {
	files := []string{e.Path}
	if _, err := os.Stat(e.Path + ".1"); err == nil {
		files = append(files, e.Path+".1")
	}
	return files
}

// Files are the log files and, for a flow run, its result: all that goes with the log.
func (e Entry) Files() []string {
	files := e.LogFiles()
	if _, err := os.Stat(ResultPath(e.Path)); err == nil {
		files = append(files, ResultPath(e.Path))
	}
	return files
}

// List reads every log under dir (a LogsDir), oldest first. Files that do not look like logs
// are left out; a missing dir is no logs.
func List(dir string) ([]Entry, error) {
	days, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the logs: %w", err)
	}
	var entries []Entry
	for _, day := range days {
		date, err := time.ParseInLocation(DayLayout, day.Name(), time.Local)
		if !day.IsDir() || err != nil {
			continue
		}
		entries = append(entries, dayEntries(filepath.Join(dir, day.Name()), date)...)
	}
	slices.SortStableFunc(entries, func(a, b Entry) int { return a.At.Compare(b.At) })
	return entries, nil
}

func dayEntries(dir string, date time.Time) []Entry {
	files, _ := os.ReadDir(dir) // an unreadable day shows as empty
	sizes := map[string]int64{}
	var entries []Entry
	for _, file := range files {
		info, err := file.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		name := file.Name()
		if base, rotated := strings.CutSuffix(name, ".1"); rotated {
			sizes[base] += info.Size()
			continue
		}
		if base, isResult := strings.CutSuffix(name, resultSuffix); isResult {
			sizes[base+".log"] += info.Size()
			continue
		}
		entry, ok := parseEntry(name, date)
		if !ok {
			continue
		}
		entry.Path, entry.Size = filepath.Join(dir, name), info.Size()
		entries = append(entries, entry)
	}
	for i := range entries {
		entries[i].Size += sizes[filepath.Base(entries[i].Path)]
	}
	return entries
}

// parseEntry reads the time and name out of a log's file name, 143005-name.log.
func parseEntry(file string, date time.Time) (Entry, bool) {
	base, isLog := strings.CutSuffix(file, ".log")
	clock, name, ok := strings.Cut(base, "-")
	if !isLog || !ok {
		return Entry{}, false
	}
	at, err := time.ParseInLocation("150405", clock, time.Local)
	if err != nil {
		return Entry{}, false
	}
	at = time.Date(date.Year(), date.Month(), date.Day(), at.Hour(), at.Minute(), at.Second(), 0, time.Local)
	return Entry{Name: name, At: at}, true
}

// Remove deletes logs with their rotated parts, and the day folders left empty.
func Remove(entries ...Entry) error {
	var problems []error
	for _, e := range entries {
		for _, file := range e.Files() {
			if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
				problems = append(problems, err)
			}
		}
		_ = os.Remove(filepath.Dir(e.Path)) // only succeeds once the day is empty
	}
	return errors.Join(problems...)
}

// Prune removes the logs started more than keepDays before now, then the oldest while all of
// them take more than maxBytes. It returns what it removed.
func Prune(dir string, keepDays int, maxBytes int64, now time.Time) ([]Entry, error) {
	entries, err := List(dir)
	if err != nil {
		return nil, err
	}
	cutoff := now.AddDate(0, 0, -keepDays)
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	var removed []Entry
	for _, e := range entries { // oldest first
		if !e.At.Before(cutoff) && total <= maxBytes {
			break
		}
		removed = append(removed, e)
		total -= e.Size
	}
	return removed, Remove(removed...)
}

// CountLines counts the lines a log holds, its rotated part included.
func CountLines(e Entry) int {
	lines := 0
	for _, file := range e.LogFiles() {
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		lines += countLines(bufio.NewReaderSize(f, 64<<10))
		f.Close()
	}
	return lines
}

func countLines(r io.Reader) int {
	buffer := make([]byte, 64<<10)
	lines := 0
	for {
		n, err := r.Read(buffer)
		lines += bytes.Count(buffer[:n], []byte{'\n'})
		if err != nil {
			return lines
		}
	}
}

// PrettySize writes a size in bytes the way people read it: 812 B, 12.4 KB, 3 MB, 1.2 GB.
func PrettySize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value, prefix := float64(bytes)/unit, 0
	for value >= unit && prefix < 3 {
		value /= unit
		prefix++
	}
	number := strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
	return number + " " + []string{"KB", "MB", "GB", "TB"}[prefix]
}
