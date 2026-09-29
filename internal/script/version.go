package script

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// NotFound is the version shown for a program that is not installed.
const NotFound = "(not found)"

// versionTimeout bounds each attempt, since some programs ignore version flags and keep running.
const versionTimeout = 2 * time.Second

var versionPattern = regexp.MustCompile(`\d+(\.\d+)+`)

// versionFlags are tried in order when a script does not say how to ask for its version. A bare
// "version" argument is left out: to many programs it is a file name to create or delete.
var versionFlags = []string{"--version", "-v"}

// Versions detects program versions in the background and remembers them until the program's
// file changes. Lookups never block, so a slow program cannot freeze the screen.
type Versions struct {
	lock    sync.Mutex
	known   map[string]versionEntry
	pending map[string]bool
	// onFound is called from a background goroutine after a version is detected.
	onFound func()
}

type versionEntry struct {
	path    string
	modTime time.Time
	version string
}

func NewVersions(onFound func()) *Versions {
	return &Versions{known: map[string]versionEntry{}, pending: map[string]bool{}, onFound: onFound}
}

// Lookup returns the version of the script's program, or false while it is still being detected.
// Scripts without a program have no version: "" and true.
func (v *Versions) Lookup(s Script) (string, bool) {
	if s.Program == "" {
		return "", true
	}
	path, err := exec.LookPath(s.Program)
	if err != nil {
		return NotFound, true
	}
	info, err := os.Stat(path)
	if err != nil {
		return NotFound, true
	}
	key := s.Program + "\x00" + s.VersionCmd
	v.lock.Lock()
	defer v.lock.Unlock()
	if entry, ok := v.known[key]; ok && entry.path == path && entry.modTime.Equal(info.ModTime()) {
		return entry.version, true
	}
	if !v.pending[key] {
		v.pending[key] = true
		go v.detect(key, path, info.ModTime(), s)
	}
	return "", false
}

func (v *Versions) detect(key, path string, modTime time.Time, s Script) {
	version := Detect(path, s.VersionCmd)
	v.lock.Lock()
	v.known[key] = versionEntry{path: path, modTime: modTime, version: version}
	delete(v.pending, key)
	v.lock.Unlock()
	if v.onFound != nil {
		v.onFound()
	}
}

// Detect runs versionCmd, or the program with each of the usual version flags, and returns the
// first version-looking number printed, or the first line of a successful run when there is none.
func Detect(program, versionCmd string) string {
	if versionCmd != "" {
		output, _ := runForVersion("/bin/sh", "-c", versionCmd)
		return versionIn(output, output)
	}
	fallback := ""
	for _, flag := range versionFlags {
		output, ok := runForVersion(program, flag)
		if match := versionPattern.FindString(output); match != "" {
			return match
		}
		if ok && fallback == "" {
			fallback = output
		}
	}
	return versionIn("", fallback)
}

// runForVersion returns what the command printed and whether it succeeded.
func runForVersion(program string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.WaitDelay = time.Second // a child left holding the output must not outlive the timeout
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err == nil
}

// versionIn is the version found in output, or the first line of fallback, or "?".
func versionIn(output, fallback string) string {
	if match := versionPattern.FindString(output); match != "" {
		return match
	}
	first, _, _ := strings.Cut(fallback, "\n")
	if first == "" {
		return "?"
	}
	return first
}
