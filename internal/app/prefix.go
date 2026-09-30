package app

import (
	"errors"
	"fmt"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
)

const prefixModifiers = uv.ModCtrl | uv.ModAlt | uv.ModShift

// prefixKey is the key combination that turns noct mode on.
type prefixKey struct {
	code rune
	mods uv.KeyMod
}

func prefixFromConfig(p config.Prefix) (prefixKey, error) {
	if err := p.Validate(); err != nil {
		return prefixKey{}, err
	}
	code, ok := keyCodeNamed(p.Key)
	if !ok {
		return prefixKey{}, fmt.Errorf("unknown prefix key %q", p.Key)
	}
	if !shiftIsVisible(p, code) {
		return prefixKey{}, errors.New("terminals send this without shift; use shift with alt+letter or F1–F12")
	}
	var mods uv.KeyMod
	if p.Ctrl {
		mods |= uv.ModCtrl
	}
	if p.Alt {
		mods |= uv.ModAlt
	}
	if p.Shift {
		mods |= uv.ModShift
	}
	return prefixKey{code: code, mods: mods}, nil
}

// shiftIsVisible reports whether terminals encode the combination differently with Shift:
// Ctrl+Shift+A arrives as Ctrl+A and Alt+Shift+Space as Alt+Space, but Alt+Shift+A and Shift+F2 are distinct.
func shiftIsVisible(p config.Prefix, code rune) bool {
	if !p.Shift || (code >= uv.KeyF1 && code <= uv.KeyF12) {
		return true
	}
	return !p.Ctrl && unicode.IsLetter(code)
}

func (p prefixKey) matches(key uv.KeyPressEvent) bool {
	return key.Code == p.code && key.Mod&prefixModifiers == p.mods
}

func (p prefixKey) String() string {
	return uv.Key{Code: p.code, Mod: p.mods}.String()
}

// prefixKeyName names a key that may end the prefix: a printable character, space or F1–F12.
func prefixKeyName(code rune) (string, bool) {
	switch {
	case code == uv.KeySpace:
		return "space", true
	case code >= uv.KeyF1 && code <= uv.KeyF12:
		return uv.Key{Code: code}.String(), true
	case code < uv.KeyExtended && unicode.IsPrint(code):
		return string(unicode.ToLower(code)), true
	}
	return "", false
}

func keyCodeNamed(name string) (rune, bool) {
	if name == "space" {
		return uv.KeySpace, true
	}
	for code := uv.KeyF1; code <= uv.KeyF12; code++ {
		if (uv.Key{Code: code}).String() == name {
			return code, true
		}
	}
	runes := []rune(name)
	if len(runes) == 1 && unicode.IsPrint(runes[0]) {
		return runes[0], true
	}
	return 0, false
}

// keyCapture takes a new prefix from the combination pressed, holding it until it is used.
type keyCapture struct {
	active  bool
	pending *config.Prefix
	// problem says why the last combination pressed cannot be the prefix.
	problem string
}

// take holds the combination pressed when it can be the prefix. It keeps what the terminal
// sent: a combination the terminal cannot tell apart, such as ctrl+shift+a, arrives as the one
// it sends, ctrl+a, and shows as that.
func (c *keyCapture) take(key uv.KeyPressEvent) {
	name, ok := prefixKeyName(key.Code)
	if !ok {
		c.pending, c.problem = nil, "the key must be a character, space or F1–F12"
		return
	}
	p := config.Prefix{
		Ctrl:  key.Mod&uv.ModCtrl != 0,
		Alt:   key.Mod&uv.ModAlt != 0,
		Shift: key.Mod&uv.ModShift != 0,
		Key:   name,
	}
	if _, err := prefixFromConfig(p); err != nil {
		c.pending, c.problem = nil, err.Error()
		return
	}
	c.pending, c.problem = &p, ""
}

// terminalKeys are combinations the programs in panes commonly use, which a prefix would take
// from them.
var terminalKeys = map[string]string{
	"ctrl+a": "moves to the start of the line in shells",
	"ctrl+b": "is tmux's prefix",
	"ctrl+c": "interrupts the program in a pane",
	"ctrl+d": "ends input and closes shells",
	"ctrl+e": "moves to the end of the line in shells",
	"ctrl+k": "deletes to the end of the line in shells",
	"ctrl+l": "clears the shell's screen",
	"ctrl+q": "resumes output in some terminals",
	"ctrl+r": "searches the shell's history",
	"ctrl+s": "freezes output in some terminals",
	"ctrl+u": "deletes the line in shells",
	"ctrl+w": "deletes a word in shells",
	"ctrl+z": "suspends the program in a pane",
}

// prefixConflicts tells what a prefix would take over: a command of the direct shortcut, which
// the prefix would reach first, or a combination programs in panes use.
func (a *App) prefixConflicts(p prefixKey) []string {
	var conflicts []string
	name := p.String()
	if what, ok := terminalKeys[name]; ok {
		conflicts = append(conflicts, name+" "+what)
	}
	key, _ := prefixKeyName(p.code)
	key, taken := a.commandKey(key)
	if a.shortcut.mods != 0 && p.mods&^uv.ModShift == a.shortcut.mods && taken && isCommandKey(key) {
		label := "a command"
		for _, s := range commandShortcuts {
			if s.keys == key {
				label = s.label
			}
		}
		conflicts = append(conflicts, name+" is the direct shortcut to "+label)
	}
	return conflicts
}
