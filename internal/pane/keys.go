package pane

import (
	"fmt"
	"unicode"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// Programs in panes get keys in the legacy xterm encoding, even when noct's own terminal reports
// them with keyboard enhancements. These functions cover what the emulator's encoder drops.

// xtermCursorKeys end with a letter, e.g. Ctrl+Left is ESC [1;5D.
var xtermCursorKeys = map[rune]byte{
	uv.KeyUp:    'A',
	uv.KeyDown:  'B',
	uv.KeyRight: 'C',
	uv.KeyLeft:  'D',
	uv.KeyHome:  'H',
	uv.KeyEnd:   'F',
}

// xtermTildeKeys end with a tilde, e.g. Ctrl+Delete is ESC [3;5~.
var xtermTildeKeys = map[rune]int{
	uv.KeyInsert: 2,
	uv.KeyDelete: 3,
	uv.KeyPgUp:   5,
	uv.KeyPgDown: 6,
}

const encodableModifiers = uv.ModShift | uv.ModAlt | uv.ModCtrl

// lockModifiers are states, not keys held down; legacy encodings never include them.
const lockModifiers = uv.ModCapsLock | uv.ModNumLock | uv.ModScrollLock

// modifiedSpecialKeySequence encodes navigation keys combined with Shift, Alt or Ctrl
// (word jumps in zsh, selections in editors).
func modifiedSpecialKeySequence(key uv.KeyPressEvent) (string, bool) {
	mods := key.Mod & encodableModifiers
	if mods == 0 || key.Mod&^encodableModifiers != 0 {
		return "", false
	}
	param := xtermModifierParam(mods)
	if final, ok := xtermCursorKeys[key.Code]; ok {
		return fmt.Sprintf("\x1b[1;%d%c", param, final), true
	}
	if number, ok := xtermTildeKeys[key.Code]; ok {
		return fmt.Sprintf("\x1b[%d;%d~", number, param), true
	}
	return "", false
}

func xtermModifierParam(mods uv.KeyMod) int {
	param := 1
	if mods.Contains(uv.ModShift) {
		param++
	}
	if mods.Contains(uv.ModAlt) {
		param += 2
	}
	if mods.Contains(uv.ModCtrl) {
		param += 4
	}
	return param
}

// ctrlAltSequence encodes Ctrl and Alt combinations of printable keys, Enter, Backspace and Escape:
// Ctrl turns the key into a control character, Alt prefixes ESC. Shift only matters for Alt+letter,
// since a legacy terminal cannot tell Ctrl+Shift+L from Ctrl+L.
func ctrlAltSequence(key uv.KeyPressEvent) (string, bool) {
	if key.Mod&(uv.ModCtrl|uv.ModAlt) == 0 || key.Mod&^encodableModifiers != 0 || key.Code >= uv.KeyExtended {
		return "", false
	}
	sequence := string(key.Code)
	if key.Mod.Contains(uv.ModCtrl) {
		control, ok := controlCharacter(key.Code)
		if !ok {
			return "", false
		}
		sequence = string(control)
	} else if key.Mod.Contains(uv.ModShift) {
		sequence = string(unicode.ToUpper(key.Code))
	}
	if key.Mod.Contains(uv.ModAlt) {
		sequence = "\x1b" + sequence
	}
	return sequence, true
}

func controlCharacter(code rune) (rune, bool) {
	switch {
	case code >= 'a' && code <= 'z':
		return code - 'a' + 1, true
	case code == uv.KeySpace || code == '@':
		return 0, true
	case code >= '[' && code <= '_':
		return code - '@', true
	case code == uv.KeyEnter || code == uv.KeyBackspace || code == uv.KeyEscape:
		return code, true
	}
	return 0, false
}

// isPlainText reports whether the key is printable text that can be sent as is.
// Shifted letters and symbols count as text; Ctrl and Alt combinations do not.
func isPlainText(key uv.KeyPressEvent) bool {
	return key.Text != "" && key.Mod&(uv.ModCtrl|uv.ModAlt) == 0
}

// withoutUnencodedShift drops Shift from keys whose legacy encoding has no Shift variant, so
// Shift+Enter still arrives as Enter. Shift+Tab keeps it: it has its own sequence.
func withoutUnencodedShift(key uv.KeyPressEvent) uv.KeyPressEvent {
	if key.Code != uv.KeyTab {
		key.Mod &^= uv.ModShift
	}
	return key
}
