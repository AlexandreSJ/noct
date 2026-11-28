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

	"github.com/AlexandreSJ/noct/internal/atomicfile"
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
	// Item is what a panel had selected: the inspect's flow, the scripts panel's script. One that
	// is gone leaves the panel on the first of its list.
	Item string `json:"item,omitempty"`
	// Filter is what a panel's list was filtered by.
	Filter string `json:"filter,omitempty"`
	// Inspect and Logs are how those panels were left, to open again at the same point.
	Inspect *InspectState `json:"inspect,omitempty"`
	Logs    *LogsState    `json:"logs,omitempty"`
	// Colors are the pane's own over the theme, by the theme file's table and key.
	Colors map[string]string `json:"colors,omitempty"`
}

// InspectState is what the inspect showed of its flow: the block selected, what was hidden or
// shown, and the editor open, with what was typed in it and not saved.
type InspectState struct {
	Selected    string `json:"selected,omitempty"`
	Chart       bool   `json:"chart,omitempty"`
	ListHidden  bool   `json:"list_hidden,omitempty"`
	Scope       bool   `json:"scope,omitempty"`
	HideDetails bool   `json:"hide_details,omitempty"`
	HelpHidden  bool   `json:"help_hidden,omitempty"`
	Zoom        int    `json:"zoom"`
	// Editing is the editor open: params, step, add, export, retry or returns.
	Editing string `json:"editing,omitempty"`
	Draft   *Draft `json:"draft,omitempty"`
}

// Draft is an editor's rows as they were typed, a secret's text left out, and its focused cell.
type Draft struct {
	Rows []DraftRow `json:"rows"`
	Row  int        `json:"row,omitempty"`
	Col  int        `json:"col,omitempty"`
}

type DraftRow struct {
	Key   string      `json:"key,omitempty"`
	Cells []DraftCell `json:"cells"`
}

type DraftCell struct {
	Kind   int      `json:"kind"`
	Text   string   `json:"text,omitempty"`
	On     bool     `json:"on,omitempty"`
	Items  []string `json:"items,omitempty"`
	Marked int      `json:"marked,omitempty"`
	// Secret marks a cell whose text is left out; AutoFor is the name its type was filled for.
	Secret  bool   `json:"secret,omitempty"`
	AutoFor string `json:"auto_for,omitempty"`
}

// LogsState is the logs panel as it was left: the day, the log selected and how it was shown.
type LogsState struct {
	Day       string `json:"day,omitempty"`
	Log       string `json:"log,omitempty"`
	FocusList bool   `json:"focus_list,omitempty"`
	AllDays   bool   `json:"all_days,omitempty"`
	Order     string `json:"order,omitempty"`
	Shown     int    `json:"shown,omitempty"`
	Preview   bool   `json:"preview,omitempty"`
	Times     bool   `json:"times,omitempty"`
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
	if err := atomicfile.Write(path, content, 0o600); err != nil {
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
