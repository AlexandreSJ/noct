// Package atomicfile replaces files whole, so a reader never sees half of one, and changes shared
// files under a lock, so two nocts writing at once never lose each other's changes.
package atomicfile

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// umask is the process's file mode mask, read once at start (no goroutine writes files yet), so
// Write's chmod leaves files as private as os.WriteFile would.
var umask = func() fs.FileMode {
	mask := syscall.Umask(0)
	syscall.Umask(mask)
	return fs.FileMode(mask)
}()

// Write replaces the file at path with content through a temporary file of its own in the same
// folder, so a crash or another writer never leaves the file half written.
func Write(path string, content []byte, mode fs.FileMode) (err error) {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(temp.Name()) // best effort: the write already failed with its own error
		}
	}()
	_, err = temp.Write(content)
	if err == nil {
		err = temp.Sync()
	}
	if err == nil {
		err = temp.Chmod(mode &^ umask)
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// Update changes the file at path while holding a lock other nocts wait for: change gets the
// current content, nil when there is no file, and returns the new one. Returning the content
// unchanged writes nothing, and an error from change leaves the file as it was.
func Update(path string, mode fs.FileMode, change func(current []byte) ([]byte, error)) error {
	lockFile, err := lock(path)
	if err != nil {
		return err
	}
	defer func() { _ = lockFile.Close() }() // closing releases the lock; nothing was written to it
	current, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		current, err = nil, nil
	}
	if err != nil {
		return err
	}
	next, err := change(current)
	if err != nil {
		return err
	}
	if bytes.Equal(current, next) {
		return nil
	}
	return Write(path, next, mode)
}

// lock takes an exclusive lock on a file next to path, kept apart from it because the file
// itself is replaced on every write. Closing the lock file releases the lock.
func lock(path string) (*os.File, error) {
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err != nil {
		_ = file.Close() // the lock failed; that error is the one to report
		return nil, fmt.Errorf("locking %s: %w", path, err)
	}
	return file, nil
}
