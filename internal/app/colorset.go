package app

import (
	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/theme"
)

// colorSet is a pane's or a tab's own colors, over the session's theme; a component left out is
// the theme's.
type colorSet map[theme.Component]theme.Color

// paneComponents and tabComponents are what a pane and a tab can color on their own.
var (
	paneComponents = []theme.Component{theme.FocusedBorder, theme.UnfocusedBorder, theme.FocusedTitle, theme.UnfocusedTitle, theme.Background, theme.Text}
	tabComponents  = []theme.Component{theme.ActiveTabText, theme.InactiveTabText, theme.ActiveTabBackground, theme.InactiveTabBackground}
)

// saved is the set as sessions and templates keep it, by the theme file's table and key.
func (s colorSet) saved() map[string]string {
	if len(s) == 0 {
		return nil
	}
	m := make(map[string]string, len(s))
	for c, color := range s {
		info := theme.Components[c]
		m[info.Group+"."+info.Key] = string(color)
	}
	return m
}

// colorSetFrom reads a saved set, leaving out what it cannot read.
func colorSetFrom(m map[string]string) colorSet {
	s := colorSet{}
	for key, value := range m {
		color, err := theme.ParseColor(value)
		if err != nil || color == "" {
			continue
		}
		for c, info := range theme.Components {
			if info.Group+"."+info.Key == key {
				s[theme.Component(c)] = color
			}
		}
	}
	return s
}

// forPane is the palette a pane draws with: the theme's, with the pane's own colors over it.
func (p palette) forPane(s colorSet) palette {
	set := func(c theme.Component, style *uv.Style) {
		if color, ok := s[c]; ok {
			style.Fg = color.Resolve()
		}
	}
	set(theme.FocusedBorder, &p.focusedFrame)
	set(theme.UnfocusedBorder, &p.unfocusedFrame)
	set(theme.FocusedTitle, &p.focusedTitle)
	set(theme.UnfocusedTitle, &p.unfocusedTitle)
	if color, ok := s[theme.Background]; ok {
		p.background = color.Resolve()
		for _, style := range []*uv.Style{&p.focusedFrame, &p.unfocusedFrame, &p.focusedTitle, &p.unfocusedTitle} {
			style.Bg = p.background
		}
	}
	if color, ok := s[theme.Text]; ok {
		p.text = color.Resolve()
	}
	return p
}

// forTab is how a tab's name shows in the tab bar, active and not, with the tab's own colors.
func (p palette) forTab(s colorSet) (active, inactive uv.Style) {
	active, inactive = p.activeTab, p.tab
	for c, style := range map[theme.Component]*uv.Style{theme.ActiveTabText: &active, theme.InactiveTabText: &inactive} {
		if color, ok := s[c]; ok {
			style.Fg = color.Resolve()
		}
	}
	for c, style := range map[theme.Component]*uv.Style{theme.ActiveTabBackground: &active, theme.InactiveTabBackground: &inactive} {
		if color, ok := s[c]; ok {
			style.Bg = color.Resolve()
		}
	}
	return active, inactive
}
