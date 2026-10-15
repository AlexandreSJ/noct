// Package logstore keeps what programs in panes print on disk, so it outlives the screen.
package logstore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
}

func Create(path string, maxSize int64) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating the log directory: %w", err)
	}
	f := &File{path: path, maxSize: maxSize}
	if err := f.open(os.O_TRUNC); err != nil {
		return nil, err
	}
	return f, nil
}

// Append goes on writing the log at path after what it has, such as a terminal opened again in
// the place of one that closed; without the file, it starts one.
func Append(path string, maxSize int64) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating the log directory: %w", err)
	}
	f := &File{path: path, maxSize: maxSize}
	if err := f.open(os.O_APPEND); err != nil {
		return nil, err
	}
	if info, err := f.file.Stat(); err == nil {
		f.size = info.Size()
	}
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
	if f.size+int64(len(p)) > f.maxSize && f.size > 0 {
		if err := f.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := f.file.Write(p)
	f.size += int64(n)
	return n, err
}

func (f *File) rotate() error {
	if err := f.file.Close(); err != nil {
		return fmt.Errorf("closing log %s: %w", f.path, err)
	}
	if err := os.Rename(f.path, f.path+".1"); err != nil {
		return fmt.Errorf("rotating log %s: %w", f.path, err)
	}
	return f.open(os.O_TRUNC)
}

func (f *File) Close() error {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.file.Close()
}
