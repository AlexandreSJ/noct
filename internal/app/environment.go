package app

import (
	"os"
	"strings"
)

func userShell() []string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return []string{shell}
	}
	return []string{"/bin/sh"}
}

// outerTerminalPrefixes identify variables set by the terminal noct runs in. Programs in a pane
// must not see them, or they would talk to that terminal's features instead of noct's.
var outerTerminalPrefixes = []string{
	"KITTY_", "WEZTERM_", "GHOSTTY_", "ALACRITTY_", "ITERM_", "KONSOLE_", "WT_", "VTE_", "TMUX", "ZELLIJ",
}

var outerTerminalVariables = map[string]bool{
	"TERM": true, "COLORTERM": true, "TERM_PROGRAM": true, "TERM_PROGRAM_VERSION": true,
}

// paneEnvironment is the user's environment, describing a noct pane instead of the outer terminal.
func paneEnvironment(base []string) []string {
	env := make([]string, 0, len(base)+4)
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if !describesOuterTerminal(name) {
			env = append(env, entry)
		}
	}
	return append(env, "TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=noct", "NOCT=1")
}

func describesOuterTerminal(name string) bool {
	if outerTerminalVariables[name] {
		return true
	}
	for _, prefix := range outerTerminalPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
