package app

// lockKey is the noct mode key that locks the shortcuts and unlocks them.
const lockKey = "b"

// toggleLock locks the shortcuts, or unlocks them, and says which.
func (a *App) toggleLock() {
	a.locked = !a.locked
	a.mode = normalMode
	a.notice = "shortcuts unlocked"
	if a.locked {
		a.notice = "shortcuts locked: " + a.unlockKeys() + " unlocks"
	}
}

// lockedOut tells whether the lock holds back a command, and then shows how to unlock. Moving
// around never locks: the focus, tabs, moving the pane and search still answer.
func (a *App) lockedOut(name string, move bool) bool {
	if !a.locked || move || name == "space" || name == "tab" || name == "shift+tab" {
		return false
	}
	_, isArrow := arrowHeadings[name]
	_, isTabNumber := tabNumberKeys[name]
	if isArrow || isTabNumber {
		return false
	}
	a.lockHint = true
	return true
}

// unlockKeys are the keys that unlock, as they are now.
func (a *App) unlockKeys() string {
	return a.prefix.String() + "+" + a.hint("commands", "{"+lockKey+"}")
}
