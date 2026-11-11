package theme

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Foreign reads a kitty, Ghostty or base16 theme as a noct theme named name, on noct: its
// background, text and 16 colors, with blue as the accent, bright black as muted text, yellow
// for messages, magenta for the raven, green for success and red for errors.
func Foreign(name string, content []byte) (Theme, error) {
	var (
		background, text Color
		ansi             [ANSICount]Color
		err              error
	)
	if keys := base16Keys(string(content)); keys != nil {
		background, text, ansi, err = fromBase16(keys)
	} else {
		background, text, ansi, err = fromTerminalConfig(string(content))
	}
	if err != nil {
		return Theme{}, err
	}
	for n, c := range ansi {
		if c == "" {
			return Theme{}, fmt.Errorf("the theme has no color%d", n)
		}
	}
	accent := ansi[4]
	th := Theme{Name: name, Base: "noct", ANSI: ansi}
	th.Colors = colors(accent, background, ansi[8], text, ansi[3], background, ansi[5], ansi[2], ansi[1])
	th.Colors[PopupBackground], th.Colors[InactiveTabBackground] = "", ""
	return th, nil
}

// hexOf reads a color as themes of other terminals write it: #rrggbb or rrggbb, quoted or not.
func hexOf(value string) (Color, bool) {
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	if !strings.HasPrefix(value, "#") {
		value = "#" + value
	}
	c, err := ParseColor(value)
	return c, err == nil && c != ""
}

var base16Key = regexp.MustCompile(`(?m)^\s*(base0[0-9A-Fa-f])\s*:\s*(\S+)`)

// base16Keys are a base16 scheme's colors by key (base00 to base0F), or nil for another format.
func base16Keys(content string) map[string]string {
	matches := base16Key.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	keys := map[string]string{}
	for _, m := range matches {
		keys[strings.ToUpper(m[1][len(m[1])-1:])] = m[2]
	}
	return keys
}

// base16Terminal is which base16 color each of the terminal's 16 is, as base16-shell sets them.
var base16Terminal = [ANSICount]string{"0", "8", "B", "A", "D", "E", "C", "5", "3", "8", "B", "A", "D", "E", "C", "7"}

func fromBase16(keys map[string]string) (background, text Color, ansi [ANSICount]Color, err error) {
	color := func(key string) Color {
		c, ok := hexOf(keys[key])
		if !ok && err == nil {
			err = fmt.Errorf("the base16 scheme has no base0%s color", key)
		}
		return c
	}
	for n, key := range base16Terminal {
		ansi[n] = color(key)
	}
	return color("0"), color("5"), ansi, err
}

// fromTerminalConfig reads kitty's "key value" lines or Ghostty's "key = value" ones.
func fromTerminalConfig(content string) (background, text Color, ansi [ANSICount]Color, err error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.ContainsAny(strings.TrimSpace(key), " \t") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			key, value = fields[0], fields[1]
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "palette" { // Ghostty: palette = 4=#7aa2f7
			index, color, _ := strings.Cut(value, "=")
			key, value = "color"+strings.TrimSpace(index), color
		}
		c, isColor := hexOf(value)
		switch {
		case !isColor:
		case key == "background":
			background = c
		case key == "foreground":
			text = c
		case strings.HasPrefix(key, "color"):
			if n, err := strconv.Atoi(key[len("color"):]); err == nil && n >= 0 && n < ANSICount {
				ansi[n] = c
			}
		}
	}
	if background == "" || text == "" {
		return "", "", ansi, errors.New("not a kitty, Ghostty or base16 theme: it has no background and foreground colors")
	}
	return background, text, ansi, nil
}
