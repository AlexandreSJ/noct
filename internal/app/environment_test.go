package app

import (
	"slices"
	"testing"
)

func TestPaneEnvironmentDescribesNoct(t *testing.T) {
	outer := []string{
		"HOME=/home/me", "TERM=xterm-kitty", "KITTY_WINDOW_ID=3", "TERM_PROGRAM=WezTerm",
		"ZELLIJ_SESSION_NAME=x", "PATH=/usr/bin",
	}
	env := paneEnvironment(outer)

	for _, kept := range []string{"HOME=/home/me", "PATH=/usr/bin", "TERM=xterm-256color", "TERM_PROGRAM=noct"} {
		if !slices.Contains(env, kept) {
			t.Errorf("missing %q in %v", kept, env)
		}
	}
	for _, dropped := range []string{"TERM=xterm-kitty", "KITTY_WINDOW_ID=3", "TERM_PROGRAM=WezTerm", "ZELLIJ_SESSION_NAME=x"} {
		if slices.Contains(env, dropped) {
			t.Errorf("outer terminal variable %q leaked into %v", dropped, env)
		}
	}
}

func TestOuterShowsPlaceholders(t *testing.T) {
	tests := []struct {
		env  []string
		want bool
	}{
		{[]string{"TERM=xterm-kitty"}, true},
		{[]string{"TERM=xterm-256color", "KITTY_WINDOW_ID=1"}, true},
		{[]string{"TERM_PROGRAM=ghostty"}, true},
		{[]string{"TERM=xterm-256color", "TERM_PROGRAM=WezTerm"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := outerShowsPlaceholders(tt.env); got != tt.want {
			t.Errorf("%v: got %v, want %v", tt.env, got, tt.want)
		}
	}
}
