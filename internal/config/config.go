// Package config loads and saves the user's settings file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	BackgroundTerminal = "terminal"
	BackgroundTheme    = "theme"
)

type Config struct {
	Prefix     Prefix     `toml:"prefix"`
	Shortcut   Shortcut   `toml:"shortcut"`
	Appearance Appearance `toml:"appearance"`
	// Mouse lets clicks focus panes and tabs, drag dividers and scroll. While on, selecting text
	// in the outer terminal takes shift (in most terminals).
	Mouse bool `toml:"mouse"`
	// Editor is the command that opens files for editing, with any arguments ("code -w").
	Editor string `toml:"editor"`
	// FileManager opens a folder: FileManagerTerminal for a terminal pane in it, or a command.
	FileManager string `toml:"file_manager"`
	// ScriptsDir holds the user's own scripts; "" is the standard place.
	ScriptsDir string `toml:"scripts_dir"`
	// ScriptHints puts a comment explaining each setting at the top of new scripts.
	ScriptHints bool `toml:"script_hints"`
	// ScriptsSort orders the scripts panel: SortByName, SortByEdited or SortByRun.
	ScriptsSort string `toml:"scripts_sort"`
	// ScriptFolders are folders of scripts outside ScriptsDir, shown in noct as they are.
	ScriptFolders []string `toml:"script_folders"`
	Logs          Logs     `toml:"logs"`
	Flows         Flows    `toml:"flows"`
	MCP           MCP      `toml:"mcp"`
}

// MCP sets what an AI may do through noct mcp. Policy maps a tool's name to PolicyAllow,
// PolicyAsk or PolicyDeny, over each tool's own default; ApprovalSeconds is how long an action
// waits for the user before it is denied.
type MCP struct {
	Policy          map[string]string `toml:"policy,omitempty"`
	ApprovalSeconds int               `toml:"approval_seconds"`
}

// What the policy lets an AI do with a tool.
const (
	PolicyAllow = "allow"
	PolicyAsk   = "ask"
	PolicyDeny  = "deny"
)

// DefaultApprovalSeconds is how long an action waits for the user's approval.
const DefaultApprovalSeconds = 300

// Flows bounds what a flow's last result keeps of each step's output: OutputKB kilobytes, or
// all of what the templates read when 0.
type Flows struct {
	OutputKB int `toml:"output_kb"`
}

// Logs bounds what noct keeps of what programs print: logs older than KeepDays go, and then the
// oldest go while all of them together take more than MaxSizeMB.
type Logs struct {
	KeepDays  int `toml:"keep_days"`
	MaxSizeMB int `toml:"max_size_mb"`
}

// Default log limits.
const (
	DefaultKeepDays  = 90
	DefaultMaxSizeMB = 200
)

const (
	// DefaultEditor is used when no editor is set.
	DefaultEditor = "vim"
	// FileManagerTerminal opens a terminal pane in the folder instead of a file manager.
	FileManagerTerminal = "terminal"
)

// Orders of the scripts list.
const (
	SortByName   = "name"
	SortByEdited = "edited"
	SortByRun    = "run"
)

// Shortcut is the modifier combination that, held with a noct mode key, runs that command
// directly, without the prefix. With neither modifier on, direct shortcuts are off.
type Shortcut struct {
	Ctrl bool `toml:"ctrl"`
	Alt  bool `toml:"alt"`
}

// Enabled tells whether direct shortcuts are on.
func (s Shortcut) Enabled() bool {
	return s.Ctrl || s.Alt
}

// Prefix is the key combination that switches noct into its command mode.
type Prefix struct {
	Ctrl  bool   `toml:"ctrl"`
	Alt   bool   `toml:"alt"`
	Shift bool   `toml:"shift"`
	Key   string `toml:"key"`
}

type Appearance struct {
	Theme string `toml:"theme"`
	// Background is BackgroundTerminal to keep the terminal's own background, or BackgroundTheme.
	Background string `toml:"background"`
}

func Default() Config {
	return Config{
		Prefix:      Prefix{Ctrl: true, Key: "space"},
		Shortcut:    Shortcut{Alt: true},
		Appearance:  Appearance{Theme: "noct", Background: BackgroundTerminal},
		Mouse:       true,
		Editor:      DefaultEditor,
		FileManager: FileManagerTerminal,
		ScriptHints: true,
		ScriptsSort: SortByName,
		Logs:        Logs{KeepDays: DefaultKeepDays, MaxSizeMB: DefaultMaxSizeMB},
		MCP:         MCP{ApprovalSeconds: DefaultApprovalSeconds},
	}
}

// normalized replaces values noct does not know with defaults, so a hand-edited file never breaks it.
func (c Config) normalized() Config {
	if c.Appearance.Background != BackgroundTheme {
		c.Appearance.Background = BackgroundTerminal
	}
	if strings.TrimSpace(c.Editor) == "" {
		c.Editor = DefaultEditor
	}
	if strings.TrimSpace(c.FileManager) == "" {
		c.FileManager = FileManagerTerminal
	}
	if c.ScriptsSort != SortByEdited && c.ScriptsSort != SortByRun {
		c.ScriptsSort = SortByName
	}
	if c.Logs.KeepDays <= 0 {
		c.Logs.KeepDays = DefaultKeepDays
	}
	if c.Logs.MaxSizeMB <= 0 {
		c.Logs.MaxSizeMB = DefaultMaxSizeMB
	}
	c.Flows.OutputKB = max(c.Flows.OutputKB, 0)
	if c.MCP.ApprovalSeconds <= 0 {
		c.MCP.ApprovalSeconds = DefaultApprovalSeconds
	}
	for tool, policy := range c.MCP.Policy {
		if policy != PolicyAllow && policy != PolicyAsk && policy != PolicyDeny {
			c.MCP.Policy[tool] = PolicyAsk // an unknown word must not allow more than asking
		}
	}
	return c
}

// Validate rejects prefixes that would also be typed as ordinary text.
func (p Prefix) Validate() error {
	if !p.Ctrl && !p.Alt {
		return errors.New("the prefix needs ctrl or alt")
	}
	if p.Key == "" {
		return errors.New("the prefix needs a key")
	}
	return nil
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding the config directory: %w", err)
	}
	return filepath.Join(dir, "noct", "config.toml"), nil
}

// Load reads the settings file. Missing settings keep their defaults; a missing file is not an
// error. When the file is invalid, the defaults for the invalid part come back with the error.
func Load(path string) (Config, error) {
	cfg := Default()
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Default(), nil
		}
		return Default(), fmt.Errorf("reading %s: %w", path, err)
	}
	cfg = cfg.normalized()
	if err := cfg.Prefix.Validate(); err != nil {
		cfg.Prefix = Default().Prefix
		return cfg, fmt.Errorf("%s: %w; using ctrl+space", path, err)
	}
	return cfg, nil
}

// Save writes the settings atomically, so a crash never leaves a half-written file.
func Save(path string, cfg Config) error {
	var content bytes.Buffer
	if err := toml.NewEncoder(&content).Encode(cfg); err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating the config directory: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, content.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	return nil
}
