// Package config loads and saves the user's settings file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	BackgroundTerminal = "terminal"
	BackgroundTheme    = "theme"
)

// When the title's intro plays as a terminal attaches: on the home only, every time, or never.
const (
	IntroEmpty  = "empty"
	IntroAlways = "always"
	IntroNever  = "never"
)

type Config struct {
	Prefix     Prefix     `toml:"prefix"`
	Shortcut   Shortcut   `toml:"shortcut"`
	Appearance Appearance `toml:"appearance"`
	// Animations are the effect of each moment that animates, when Appearance.Animations is on.
	Animations Animations `toml:"animations"`
	// AnimationTimes are how long each moment's animation plays, in milliseconds.
	AnimationTimes AnimationTimes `toml:"animation_ms"`
	// NewTab is the panel a new tab opens with, by the name noct gives it (NewTabTerminal,
	// "scripts"...), or NewTabEmpty for none.
	NewTab string `toml:"new_tab"`
	// CloseEmptyTabs closes a tab when its last pane closes; off, the tab stays, showing the home.
	CloseEmptyTabs bool `toml:"close_empty_tabs"`
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
	// Keys changes what keys do: by context ("scripts") and action ("edit"), the keys that do it
	// instead of the defaults; no keys leaves the action without any. The app knows the names.
	Keys  map[string]map[string][]string `toml:"keys,omitempty"`
	Logs  Logs                           `toml:"logs"`
	Flows Flows                          `toml:"flows"`
	MCP   MCP                            `toml:"mcp"`
	// Update says whether noct looks for new releases on its own.
	Update Update `toml:"update"`
}

// Update has noct look for new releases on GitHub when it starts and once a day while it runs.
// Installing one always waits for the user.
type Update struct {
	Check bool `toml:"check"`
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
// oldest go while all of them together take more than MaxSizeKB. Terminals logs what terminal
// panes print too; scripts, flows and the AI's commands are always logged.
type Logs struct {
	KeepDays  int  `toml:"keep_days"`
	MaxSizeKB int  `toml:"max_size_kb"`
	Terminals bool `toml:"terminals"`
	// MaxSizeMB is the limit as older files wrote it; loading moves it to MaxSizeKB.
	MaxSizeMB int `toml:"max_size_mb,omitzero"`
}

// MaxSizeBytes is the limit on all logs together, in bytes.
func (l Logs) MaxSizeBytes() int64 {
	return int64(l.MaxSizeKB) << 10
}

// Default log limits.
const (
	DefaultKeepDays  = 90
	DefaultMaxSizeKB = 200 << 10
)

// What a new tab opens with: nothing, or a terminal, the default. The app knows the other panels.
const (
	NewTabEmpty    = "empty"
	NewTabTerminal = "terminal"
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
	// AnimateIntro is IntroEmpty, IntroAlways or IntroNever.
	AnimateIntro string `toml:"animate_intro"`
	// Animations is AnimationsOn or AnimationsOff, for every moment at once.
	Animations string `toml:"animations"`
}

const (
	AnimationsOn  = "on"
	AnimationsOff = "off"
)

// Effects are the names of the animation effects, in the order the config offers them.
var Effects = []string{"none", "01", "fade", "jitter"}

// AnimationMoments are the moments that animate, by their key in the config file.
var AnimationMoments = []string{"hover", "panels", "tabs", "focus", "move", "fullscreen", "popup", "activity"}

// Animations is the effect of each moment noct animates, by the name in Effects.
type Animations struct {
	Hover      string `toml:"hover"`
	Panels     string `toml:"panels"`
	Tabs       string `toml:"tabs"`
	Focus      string `toml:"focus"`
	Move       string `toml:"move"`
	Fullscreen string `toml:"fullscreen"`
	Popup      string `toml:"popup"`
	Activity   string `toml:"activity"`
}

// defaultAnimations are the effects noct comes with.
var defaultAnimations = Animations{Hover: "fade", Panels: "jitter", Tabs: "jitter", Focus: "none", Move: "fade", Fullscreen: "01", Popup: "01", Activity: "fade"}

func (a *Animations) field(moment string) *string {
	switch moment {
	case "hover":
		return &a.Hover
	case "panels":
		return &a.Panels
	case "tabs":
		return &a.Tabs
	case "focus":
		return &a.Focus
	case "move":
		return &a.Move
	case "fullscreen":
		return &a.Fullscreen
	case "popup":
		return &a.Popup
	case "activity":
		return &a.Activity
	}
	return nil
}

// Effect is the effect of the moment named as in the config file, "" for one noct does not know.
func (a Animations) Effect(moment string) string {
	if f := a.field(moment); f != nil {
		return *f
	}
	return ""
}

func (a *Animations) SetEffect(moment, effect string) {
	if f := a.field(moment); f != nil {
		*f = effect
	}
}

// normalized gives each moment its default when its effect is not one noct knows.
func (a Animations) normalized() Animations {
	for _, moment := range AnimationMoments {
		if f := a.field(moment); !slices.Contains(Effects, *f) {
			*f = defaultAnimations.Effect(moment)
		}
	}
	return a
}

// Bounds of an animation's time, in milliseconds: shorter is not seen, longer is in the way.
const (
	MinAnimationMs = 50
	MaxAnimationMs = 2000
)

// AnimationTimes are how long each moment's animation plays, in milliseconds.
type AnimationTimes struct {
	Hover      int `toml:"hover"`
	Panels     int `toml:"panels"`
	Tabs       int `toml:"tabs"`
	Focus      int `toml:"focus"`
	Move       int `toml:"move"`
	Fullscreen int `toml:"fullscreen"`
	Popup      int `toml:"popup"`
	Activity   int `toml:"activity"`
}

var defaultAnimationTimes = AnimationTimes{Hover: 150, Panels: 300, Tabs: 300, Focus: 100, Move: 200, Fullscreen: 200, Popup: 200, Activity: 600}

func (a *AnimationTimes) field(moment string) *int {
	switch moment {
	case "hover":
		return &a.Hover
	case "panels":
		return &a.Panels
	case "tabs":
		return &a.Tabs
	case "focus":
		return &a.Focus
	case "move":
		return &a.Move
	case "fullscreen":
		return &a.Fullscreen
	case "popup":
		return &a.Popup
	case "activity":
		return &a.Activity
	}
	return nil
}

// Duration is the milliseconds the moment named plays for, 0 for one noct does not know.
func (a AnimationTimes) Duration(moment string) int {
	if f := a.field(moment); f != nil {
		return *f
	}
	return 0
}

func (a *AnimationTimes) SetDuration(moment string, ms int) {
	if f := a.field(moment); f != nil {
		*f = ms
	}
}

// normalized gives each moment its default time when its own is missing or out of bounds.
func (a AnimationTimes) normalized() AnimationTimes {
	for _, moment := range AnimationMoments {
		if f := a.field(moment); *f < MinAnimationMs || *f > MaxAnimationMs {
			*f = defaultAnimationTimes.Duration(moment)
		}
	}
	return a
}

func Default() Config {
	return Config{
		Prefix:         Prefix{Ctrl: true, Key: "space"},
		Shortcut:       Shortcut{Alt: true},
		Appearance:     Appearance{Theme: "noct", Background: BackgroundTerminal, AnimateIntro: IntroEmpty, Animations: AnimationsOn},
		Animations:     defaultAnimations,
		AnimationTimes: defaultAnimationTimes,
		NewTab:         NewTabTerminal,
		CloseEmptyTabs: true,
		Mouse:          true,
		Editor:         DefaultEditor,
		FileManager:    FileManagerTerminal,
		ScriptHints:    true,
		ScriptsSort:    SortByName,
		Logs:           Logs{KeepDays: DefaultKeepDays, MaxSizeKB: DefaultMaxSizeKB},
		MCP:            MCP{ApprovalSeconds: DefaultApprovalSeconds},
		Update:         Update{Check: true},
	}
}

// normalized replaces values noct does not know with defaults, so a hand-edited file never breaks it.
func (c Config) normalized() Config {
	if c.Appearance.Background != BackgroundTheme {
		c.Appearance.Background = BackgroundTerminal
	}
	if c.Appearance.AnimateIntro != IntroAlways && c.Appearance.AnimateIntro != IntroNever {
		c.Appearance.AnimateIntro = IntroEmpty
	}
	if c.Appearance.Animations != AnimationsOff {
		c.Appearance.Animations = AnimationsOn
	}
	c.Animations = c.Animations.normalized()
	c.AnimationTimes = c.AnimationTimes.normalized()
	if strings.TrimSpace(c.NewTab) == "" {
		c.NewTab = NewTabTerminal
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
	if c.Logs.MaxSizeMB > 0 {
		c.Logs.MaxSizeKB = c.Logs.MaxSizeMB << 10
	}
	c.Logs.MaxSizeMB = 0
	if c.Logs.MaxSizeKB <= 0 {
		c.Logs.MaxSizeKB = DefaultMaxSizeKB
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
