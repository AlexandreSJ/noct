package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// CacheFile keeps the last check, in noct's state folder.
const CacheFile = "updates.json"

// Cache is what the last check found, kept between runs so noct does not ask GitHub again
// before it is due: the releases with their changelogs, when they were checked, the ETag of
// that answer, and how long GitHub asked to wait, when it turned a check down.
type Cache struct {
	Releases  []Release `json:"releases"`
	Checked   time.Time `json:"checked"`
	ETag      string    `json:"etag,omitempty"`
	WaitUntil time.Time `json:"wait_until,omitzero"`
}

// LoadCache reads the cache at path; without one, nothing was checked yet.
func LoadCache(path string) (Cache, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Cache{}, nil
	}
	if err != nil {
		return Cache{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var cache Cache
	if err := json.Unmarshal(content, &cache); err != nil {
		return Cache{}, fmt.Errorf("reading %s: %w", path, err)
	}
	return cache, nil
}

// SaveCache writes the cache at path atomically, so a crash never leaves half of it.
func SaveCache(path string, cache Cache) error {
	content, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the updates cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	return replaceFile(path, content, 0o600)
}
