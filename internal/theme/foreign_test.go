package theme

import (
	"strings"
	"testing"
)

const kittyTheme = `# a kitty theme
foreground #c0caf5
background   #1a1b26
selection_background #33467c
selection_foreground #c0caf5
color0 #15161e
color1 #f7768e
color2 #9ece6a
color3 #e0af68
color4 #7aa2f7
color5 #bb9af7
color6 #7dcfff
color7 #a9b1d6
color8 #414868
color9 #f7768e
color10 #9ece6a
color11 #e0af68
color12 #7aa2f7
color13 #bb9af7
color14 #7dcfff
color15 #c0caf5
`

const ghosttyTheme = `palette = 0=#15161e
palette = 1=#f7768e
palette = 2=#9ece6a
palette = 3=#e0af68
palette = 4=#7aa2f7
palette = 5=#bb9af7
palette = 6=#7dcfff
palette = 7=#a9b1d6
palette = 8=#414868
palette = 9=#f7768e
palette = 10=#9ece6a
palette = 11=#e0af68
palette = 12=#7aa2f7
palette = 13=#bb9af7
palette = 14=#7dcfff
palette = 15=#c0caf5
background = 1a1b26
foreground = c0caf5
selection-background = 33467c
selection-foreground = c0caf5
`

const base16Theme = `scheme: "Tokyo"
author: "someone"
base00: "1a1b26"
base01: "16161e"
base02: "2f3549"
base03: "414868"
base04: "787c99"
base05: "c0caf5"
base06: "cbccd1"
base07: "d5d6db"
base08: "f7768e"
base09: "ff9e64"
base0A: "e0af68"
base0B: "9ece6a"
base0C: "7dcfff"
base0D: "7aa2f7"
base0E: "bb9af7"
base0F: "db4b4b"
`

func TestThemesOfOtherTerminalsImport(t *testing.T) {
	for format, content := range map[string]string{"kitty": kittyTheme, "ghostty": ghosttyTheme, "base16": base16Theme} {
		th, err := Foreign("tokyo", []byte(content))
		if err != nil {
			t.Errorf("%s: %v", format, err)
			continue
		}
		if th.Name != "tokyo" || th.Base != "noct" || th.Colors[Background] != "#1a1b26" || th.Colors[Text] != "#c0caf5" {
			t.Errorf("%s: name %q base %q background %q text %q", format, th.Name, th.Base, th.Colors[Background], th.Colors[Text])
		}
		if th.ANSI[1] != "#f7768e" || th.ANSI[4] != "#7aa2f7" || th.ANSI[8] != "#414868" || th.ANSI[15] == "" {
			t.Errorf("%s: terminal colors %q", format, th.ANSI)
		}
		if th.Colors[FocusedBorder] != "#7aa2f7" || th.Colors[Error] != "#f7768e" || th.Colors[Success] != "#9ece6a" {
			t.Errorf("%s: accent %q, error %q, success %q", format, th.Colors[FocusedBorder], th.Colors[Error], th.Colors[Success])
		}
	}
}

func TestAForeignThemeWithoutItsColorsIsRefused(t *testing.T) {
	for _, content := range []string{"", "font_size 12\n", "color1 #ff0000\n", "background = nothex\nforeground = fff\n"} {
		if _, err := Foreign("x", []byte(content)); err == nil || !strings.Contains(err.Error(), "background") && !strings.Contains(err.Error(), "color") {
			t.Errorf("Foreign(%q) = %v, want why it is not a theme", content, err)
		}
	}
}
