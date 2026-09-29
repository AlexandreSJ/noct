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
