package app

import (
	"image/color"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

type theme struct {
	name       string
	accent     color.Color
	onAccent   color.Color
	muted      color.Color
	text       color.Color
	attention  color.Color
	background color.Color
	// logo is the raven on the home screen.
	logo color.Color
}

func rgb(hex uint32) color.Color {
	return color.RGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xff}
}

// themes are listed in the order the settings cycle through them; the first is the default.
var themes = []theme{
	{"noct", rgb(0x9d7cd8), rgb(0x1a1b26), rgb(0x565f89), rgb(0xc0caf5), rgb(0xe0af68), rgb(0x1a1b26), rgb(0x565f89)},
	{"aurora", rgb(0x7dcfb6), rgb(0x0f1a1c), rgb(0x4f6b6d), rgb(0xd0e8e2), rgb(0xf2c572), rgb(0x0f1a1c), rgb(0x4f6b6d)},
	{"ember", rgb(0xf28c38), rgb(0x1c1410), rgb(0x6e5a4b), rgb(0xecdcca), rgb(0xe8c547), rgb(0x1c1410), rgb(0x6e5a4b)},
	{"rose", rgb(0xeb6f92), rgb(0x191724), rgb(0x6e6a86), rgb(0xe0def4), rgb(0xf6c177), rgb(0x191724), rgb(0x6e6a86)},
	{"blood", rgb(0xff6666), rgb(0x2e3440), rgb(0x7a8494), rgb(0xe5e9f0), rgb(0xebcb8b), rgb(0x2e3440), rgb(0xc25a5a)},
	{"frost", rgb(0x88c0d0), rgb(0x2e3440), rgb(0x616e88), rgb(0xe5e9f0), rgb(0xebcb8b), rgb(0x2e3440), rgb(0x616e88)},
	{"mono", rgb(0xd0d0d0), rgb(0x111111), rgb(0x5c5c5c), rgb(0xd0d0d0), rgb(0xffffff), rgb(0x111111), rgb(0x5c5c5c)},
}

func themeIndex(name string) int {
	for i, t := range themes {
		if t.name == name {
			return i
		}
	}
	return 0
}

func themeNamed(name string) theme {
	return themes[themeIndex(name)]
}

func nextThemeName(current string, step int) string {
	next := (themeIndex(current) + step + len(themes)) % len(themes)
	return themes[next].name
}

// palette is every style the workspace draws with, for one theme and background choice.
type palette struct {
	// background and text are nil when the terminal's own colors are kept.
	background color.Color
	text       color.Color

	activeTab      uv.Style
	tab            uv.Style
	focusedFrame   uv.Style
	unfocusedFrame uv.Style
	focusedTitle   uv.Style
	unfocusedTitle uv.Style
	badge          uv.Style
	status         uv.Style
	panelText      uv.Style
	panelMuted     uv.Style
	panelHeading   uv.Style
	logo           uv.Style
	selectedRow    uv.Style
}

func newPalette(t theme, paintBackground bool) palette {
	var background, text color.Color
	if paintBackground {
		background, text = t.background, t.text
	}
	plain := func(foreground color.Color, attrs uint8) uv.Style {
		return uv.Style{Fg: foreground, Bg: background, Attrs: attrs}
	}
	highlighted := uv.Style{Fg: t.onAccent, Bg: t.accent, Attrs: uv.AttrBold}
	return palette{
		background:     background,
		text:           text,
		activeTab:      highlighted,
		tab:            plain(t.muted, 0),
		focusedFrame:   plain(t.accent, 0),
		unfocusedFrame: plain(t.muted, 0),
		focusedTitle:   plain(t.accent, uv.AttrBold),
		unfocusedTitle: plain(t.muted, 0),
		badge:          highlighted,
		status:         plain(t.attention, 0),
		panelText:      plain(t.text, 0),
		panelMuted:     plain(t.muted, 0),
		panelHeading:   plain(t.text, uv.AttrBold),
		logo:           plain(t.logo, 0),
		selectedRow:    highlighted,
	}
}
