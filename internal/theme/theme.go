// Package theme holds what a theme colors, the colors it can use, the themes noct comes with and
// the user's theme files.
package theme

import (
	"errors"
	"fmt"
	"image/color"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Component is a part of the screen a theme colors.
type Component int

const (
	Background Component = iota
	Text
	ActiveTabText
	ActiveTabBackground
	InactiveTabText
	InactiveTabBackground
	FocusedBorder
	UnfocusedBorder
	FocusedTitle
	UnfocusedTitle
	PanelText
	PanelMuted
	PanelHeading
	SelectedText
	SelectedBackground
	PopupBackground
	Attention
	Success
	Error
	Logo
	Calendar
	Count
)

// Info names a component: its table and key in a theme file, and its label in the editor.
type Info struct {
	Group, Key, Label string
}

// Components are in the order the editor lists them, grouped as the theme file is.
var Components = [Count]Info{
	Background:            {"background", "color", "background"},
	Text:                  {"background", "text", "text"},
	ActiveTabText:         {"tabs", "active_text", "active tab name"},
	ActiveTabBackground:   {"tabs", "active_background", "active tab background"},
	InactiveTabText:       {"tabs", "inactive_text", "inactive tab name"},
	InactiveTabBackground: {"tabs", "inactive_background", "inactive tab background"},
	FocusedBorder:         {"panels", "focused_border", "focused border"},
	UnfocusedBorder:       {"panels", "unfocused_border", "unfocused border"},
	FocusedTitle:          {"panels", "focused_title", "focused title"},
	UnfocusedTitle:        {"panels", "unfocused_title", "unfocused title"},
	PanelText:             {"noct", "text", "text"},
	PanelMuted:            {"noct", "muted", "muted text"},
	PanelHeading:          {"noct", "heading", "heading"},
	SelectedText:          {"noct", "selected_text", "selected row text"},
	SelectedBackground:    {"noct", "selected_background", "selected row background"},
	PopupBackground:       {"noct", "popup_background", "popup background"},
	Attention:             {"state", "attention", "message"},
	Success:               {"state", "success", "success"},
	Error:                 {"state", "error", "error"},
	Logo:                  {"other", "logo", "raven"},
	Calendar:              {"other", "calendar", "calendar"},
}

// FollowsBackground tells whether c may be "", following the theme's background: only the
// backgrounds of tabs, rows and popups can.
func FollowsBackground(c Component) bool {
	return c != Background && strings.HasSuffix(Components[c].Key, "_background")
}

// Color is "" to follow the theme's background, "#rrggbb", or a terminal color from "0" to "255".
// Colors 0 to 15 are the outer terminal's own, so they stay indexes to keep following it.
type Color string

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func ParseColor(s string) (Color, error) {
	switch {
	case s == "":
		return "", nil
	case hexColor.MatchString(s):
		return Color(strings.ToLower(s)), nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 255 || strconv.Itoa(n) != s {
		return "", fmt.Errorf("%q is not a color: use #rrggbb or a number from 0 to 255", s)
	}
	return Color(s), nil
}

// Resolve is the color to draw with, or nil for the background.
func (c Color) Resolve() color.Color {
	if c == "" {
		return nil
	}
	if c[0] == '#' {
		v, _ := strconv.ParseUint(string(c[1:]), 16, 32)
		return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
	}
	n, _ := strconv.Atoi(string(c))
	return ansi.IndexedColor(n)
}

// Indexed is the terminal's color n as a theme stores it: an index for the terminal's own 16,
// the fixed color as hex for the rest.
func Indexed(n uint8) Color {
	if n < 16 {
		return Color(strconv.Itoa(int(n)))
	}
	r, g, b, _ := ansi.IndexedColor(n).RGBA()
	return Color(fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8))
}

type Theme struct {
	Name string
	// Base is the theme the colors started from, which the file leaves out where they match.
	Base    string
	Builtin bool
	Colors  [Count]Color
}

// builtin builds a theme noct comes with from its few colors.
func builtin(name string, accent, onAccent, muted, text, attention, background, logo, success, failure Color) Theme {
	t := Theme{Name: name, Builtin: true}
	for c, v := range map[Component]Color{
		Background: background, Text: text, PanelText: text, PanelHeading: text,
		ActiveTabText: onAccent, SelectedText: onAccent,
		ActiveTabBackground: accent, FocusedBorder: accent, FocusedTitle: accent, SelectedBackground: accent, Calendar: accent,
		InactiveTabText: muted, UnfocusedBorder: muted, UnfocusedTitle: muted, PanelMuted: muted,
		Attention: attention, Logo: logo, Success: success, Error: failure,
	} {
		t.Colors[c] = v
	}
	return t
}

// raised gives a built-in theme its own heading color, and popups and inactive tabs their own
// backgrounds; "" keeps the one that follows the theme's background.
func raised(t Theme, heading, popup, inactiveTab Color) Theme {
	t.Colors[PanelHeading] = heading
	t.Colors[PopupBackground] = popup
	t.Colors[InactiveTabBackground] = inactiveTab
	return t
}

// Builtins are the themes noct comes with, in the order the config cycles through them; the
// first is the default.
func Builtins() []Theme {
	return []Theme{
		builtin("noct", "#9d7cd8", "#1a1b26", "#565f89", "#c0caf5", "#e0af68", "#1a1b26", "#565f89", "#9ece6a", "#f7768e"),
		builtin("aurora", "#7dcfb6", "#0f1a1c", "#4f6b6d", "#d0e8e2", "#f2c572", "#0f1a1c", "#4f6b6d", "#8bd5a6", "#f28b82"),
		builtin("ember", "#f28c38", "#1c1410", "#6e5a4b", "#ecdcca", "#e8c547", "#1c1410", "#6e5a4b", "#a3be8c", "#e06c50"),
		builtin("rose", "#eb6f92", "#191724", "#6e6a86", "#e0def4", "#f6c177", "#191724", "#6e6a86", "#9ccfd8", "#eb6f92"),
		builtin("blood", "#ff6666", "#2e3440", "#7a8494", "#e5e9f0", "#ebcb8b", "#2e3440", "#c25a5a", "#a3be8c", "#ff6666"),
		builtin("frost", "#88c0d0", "#2e3440", "#616e88", "#e5e9f0", "#ebcb8b", "#2e3440", "#616e88", "#a3be8c", "#bf616a"),
		builtin("mono", "#d0d0d0", "#111111", "#5c5c5c", "#d0d0d0", "#ffffff", "#111111", "#5c5c5c", "#d0d0d0", "#ffffff"),
		raised(builtin("tide", "#4fb3d9", "#0b1620", "#4a6478", "#cfe3ee", "#f0c674", "#0b1620", "#3d7fa0", "#7fd1a8", "#f07178"), "#8fd3f0", "#112230", "#112230"),
		raised(builtin("moss", "#8fbf6a", "#141a12", "#5d6b52", "#dde5d0", "#e6c06b", "#141a12", "#6b8a55", "#a7d48a", "#e07a5f"), "#b8d99c", "#1b2318", ""),
		raised(builtin("pumpkin", "#ff9e64", "#1d1526", "#6c5a7a", "#e8dcef", "#ffd36e", "#1d1526", "#b07cc6", "#9ece6a", "#ff6b8b"), "#c9a0dc", "#261c33", "#261c33"),
		raised(builtin("neon", "#ff2a8a", "#0d0a14", "#5a4d7a", "#e6e1ff", "#00e5ff", "#0d0a14", "#8a2be2", "#39ff88", "#ff3355"), "#00e5ff", "#150f22", ""),
		raised(builtin("cobalt", "#3d8bff", "#0a1230", "#4a5a8a", "#d6e0ff", "#ffcc4d", "#0a1230", "#3d8bff", "#5fd38d", "#ff5f6d"), "#9fc0ff", "#101b42", "#101b42"),
		raised(builtin("sand", "#d79921", "#282828", "#7c6f64", "#ebdbb2", "#fabd2f", "#282828", "#a89984", "#b8bb26", "#fb4934"), "#fe8019", "#32302f", "#32302f"),
		raised(builtin("slate", "#a3b1c6", "#1e2228", "#5b6370", "#c8ccd4", "#d8b878", "#1e2228", "#5b6370", "#9fbf9f", "#d08a8a"), "#dfe3ea", "#252a31", ""),
		raised(builtin("amber", "#ffb000", "#140d00", "#7a5500", "#ffcc66", "#ffe08a", "#140d00", "#b37a00", "#ffcc66", "#ff7a33"), "#ffd280", "#1f1500", ""),
		raised(builtin("phosphor", "#33ff66", "#020d04", "#1f7a37", "#a8ffbf", "#e5ff66", "#020d04", "#1fa84a", "#66ff99", "#ff5c5c"), "#66ff99", "#06180a", ""),
		raised(builtin("paper", "#5b4bc4", "#fbf8f1", "#9a9286", "#3a3530", "#b5761b", "#fbf8f1", "#8a7fd6", "#3f8a4a", "#c43d3d"), "#2a2520", "#f1ece1", "#f1ece1"),
		raised(builtin("sakura", "#d6668c", "#fff5f7", "#b8949f", "#5a3c46", "#c98a2b", "#fff5f7", "#e48fae", "#5c9a6b", "#c7364f"), "#a8406a", "#fbe6ec", "#fbe6ec"),
	}
}

func Find(themes []Theme, name string) (Theme, bool) {
	for _, t := range themes {
		if t.Name == name {
			return t, true
		}
	}
	return Theme{}, false
}

// validName is the rule scripts and flows use: safe as a file name and easy to type.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// CheckName tells whether name can be a new theme's.
func CheckName(name string) error {
	if !validName.MatchString(name) {
		return errors.New("a theme name has letters, digits, '.', '_' or '-', starting with a letter or digit")
	}
	if _, ok := Find(Builtins(), name); ok {
		return fmt.Errorf("%s is a theme noct comes with", name)
	}
	return nil
}
