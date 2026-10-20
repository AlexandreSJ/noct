// Package session keeps noct's sessions on disk: the tabs and panes of each, to open them again
// after they were closed or the workspace stopped.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/logstore"
)

// Kind is what a pane shows, among the panes a session can open again.
type Kind string

const (
	// Terminal is the user's shell; it opens again in the directory it was in.
	Terminal Kind = "terminal"
	Scripts  Kind = "scripts"
	Inspect  Kind = "inspect"
	Logs     Kind = "logs"
)

// File is every session noct knows, open or closed.
type File struct {
	Sessions []Session `json:"sessions"`
}

type Session struct {
	Name       string    `json:"name"`
	Created    time.Time `json:"created"`
	LastAccess time.Time `json:"last_access"`
	ActiveTab  int       `json:"active_tab,omitempty"`
	// Theme is the session's theme; without one, the config's is used.
	Theme string `json:"theme,omitempty"`
	Tabs  []Tab  `json:"tabs,omitempty"`
}

type Tab struct {
	Name       string        `json:"name,omitempty"`
	Layout     layout.Shape  `json:"layout"`
	Focused    layout.PaneID `json:"focused"`
	Fullscreen bool          `json:"fullscreen,omitempty"`
	Panes      []Pane        `json:"panes"`
	// Colors are the tab's own in the tab bar, by the theme file's table and key.
	Colors map[string]string `json:"colors,omitempty"`
}

// Pane is a pane of a tab's layout, or one floating over it with its frame.
type Pane struct {
	ID       layout.PaneID `json:"id"`
	Kind     Kind          `json:"kind"`
	Dir      string        `json:"dir,omitempty"`
	Name     string        `json:"name,omitempty"`
	Floating *Frame        `json:"floating,omitempty"`
	// Command is what a terminal was running when it closed, offered to run again when it opens.
	Command string `json:"command,omitempty"`
	// Log is a terminal's log, which it goes on writing and shows the end of when it opens again.
	Log string `json:"log,omitempty"`
	// Colors are the pane's own over the theme, by the theme file's table and key.
	Colors map[string]string `json:"colors,omitempty"`
}

type Frame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Info is what the command line lists about a session.
type Info struct {
	Name       string    `json:"name"`
	Tabs       int       `json:"tabs"`
	Open       bool      `json:"open"`
	Attached   bool      `json:"attached,omitempty"`
	LastAccess time.Time `json:"last_access"`
}

// Infos lists the sessions of the file, all closed: what there is while noct is not running.
func (f File) Infos() []Info {
	infos := []Info{}
	for _, s := range f.Sessions {
		infos = append(infos, Info{Name: s.Name, Tabs: len(s.Tabs), LastAccess: s.LastAccess})
	}
	return infos
}

// Path is where the sessions are kept, beside the logs.
func Path() string {
	return filepath.Join(logstore.Dir(), "sessions.json")
}

// Load reads the sessions; without the file there are none yet.
func Load(path string) (File, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, fmt.Errorf("reading the sessions: %w", err)
	}
	var f File
	if err := json.Unmarshal(content, &f); err != nil {
		return File{}, fmt.Errorf("reading %s: %w", path, err)
	}
	return f, nil
}

// Save writes the sessions whole, through a temporary file, so a crash never leaves half of it.
func Save(path string, f File) error {
	content, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the sessions: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, content, 0o600); err != nil {
		return fmt.Errorf("saving the sessions: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("saving the sessions: %w", err)
	}
	return nil
}

// Find is the index of the session with that name, or -1.
func (f File) Find(name string) int {
	return slices.IndexFunc(f.Sessions, func(s Session) bool { return s.Name == name })
}

// CheckName tells why a name cannot be a session's: sessions are attached by name on the command
// line, so names have no spaces.
func CheckName(name string) error {
	if name == "" {
		return errors.New("a session needs a name")
	}
	if strings.ContainsFunc(name, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return fmt.Errorf("%q has spaces; a session's name is typed after noct attach", name)
	}
	return nil
}

// FreeName is the first of 1, 2, 3… that no session has, for a session created without a name.
func FreeName(taken func(name string) bool) string {
	for n := 1; ; n++ {
		if name := strconv.Itoa(n); !taken(name) {
			return name
		}
	}
}
