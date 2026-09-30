package app

import (
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
)

const lockModifiers = uv.ModCapsLock | uv.ModNumLock | uv.ModScrollLock

// directShortcut runs noct mode commands without the prefix: its modifiers held with a command
// key run that command, and with shift added, the arrows move the focused pane. Keys the terminal
// cannot tell apart are never taken: ctrl+alt+m arrives as alt+enter, so it stays with the program.
type directShortcut struct {
	// mods is zero when direct shortcuts are off.
	mods uv.KeyMod
}

func shortcutFromConfig(s config.Shortcut) directShortcut {
	var mods uv.KeyMod
	if s.Ctrl {
		mods |= uv.ModCtrl
	}
	if s.Alt {
		mods |= uv.ModAlt
	}
	return directShortcut{mods: mods}
}

// command is the noct mode key a press stands for, and whether it moves the pane instead.
// commandKey turns the key held with the modifiers into the noct mode key it stands for.
func (d directShortcut) command(key uv.KeyPressEvent, commandKey func(string) (string, bool)) (name string, move, ok bool) {
	mods := key.Mod &^ lockModifiers
	if d.mods == 0 || mods&^uv.ModShift != d.mods {
		return "", false, false
	}
	name = uv.Key{Code: key.Code}.String()
	if mods&uv.ModCtrl != 0 && (name == "h" || name == "j") {
		return "", false, false // ctrl+h is also backspace (^H) and ctrl+j a newline (^J)
	}
	if name, ok = commandKey(name); !ok {
		return "", false, false
	}
	if mods&uv.ModShift != 0 {
		_, isArrow := arrowHeadings[name]
		return name, true, isArrow
	}
	return name, false, isCommandKey(name)
}

func isCommandKey(name string) bool {
	_, isArrow := arrowHeadings[name]
	_, isTabNumber := tabNumberKeys[name]
	return isArrow || isTabNumber || name == "tab" || slices.Contains(actionKeys, name)
}

// runShortcut runs a command as noct mode would, without leaving noct mode on afterwards.
func (a *App) runShortcut(name string, move bool) {
	if a.currentTab() == nil {
		if !move {
			a.runAction(name)
		}
		return
	}
	if move {
		a.moveFocusedPane(arrowHeadings[name])
		return
	}
	if !a.navigate(name) {
		a.runAction(name)
	}
}

func (d directShortcut) String() string {
	if d.mods == 0 {
		return "off"
	}
	return strings.TrimSuffix(uv.Key{Code: 'k', Mod: d.mods}.String(), "k") + "key"
}
