package app

import (
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

func alt(code rune) uv.KeyPressEvent { return uv.KeyPressEvent{Code: code, Mod: uv.ModAlt} }

// While locked, a command's direct shortcut reaches the program and the tab bar tells how to
// unlock; moving around still works, and prefix+b unlocks.
func TestLockLeavesCommandsToTheProgram(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('b'))
	if !a.locked || a.mode != normalMode {
		t.Fatalf("prefix+b: locked %v, mode %v", a.locked, a.mode)
	}

	press(a, alt('p'))
	focused := a.currentTab().focusedView().(*fakeView)
	if len(a.currentTab().views) != 2 || len(focused.keys) != 1 {
		t.Fatalf("alt+p while locked: %d panes, the program got %d keys", len(a.currentTab().views), len(focused.keys))
	}
	if text, _ := a.statusParts(); !strings.Contains(text, "ctrl+space+b unlock") {
		t.Errorf("the tab bar reads %q, want how to unlock", text)
	}

	press(a, alt('l'))
	if focusedPaneID(a) != 2 || a.lockHint {
		t.Errorf("alt+l while locked: focus %d, hint %v; want the focus moved right", focusedPaneID(a), a.lockHint)
	}
	press(a, prefixPress, letter('h'))
	if focusedPaneID(a) != 1 {
		t.Errorf("prefix+h while locked: focus %d, want 1", focusedPaneID(a))
	}
	press(a, letter('x'))
	if len(a.currentTab().views) != 2 || a.mode != normalMode || !a.lockHint {
		t.Errorf("prefix+x while locked: %d panes, mode %v, hint %v", len(a.currentTab().views), a.mode, a.lockHint)
	}

	press(a, alt('b'), prefixPress, letter('b'), alt('p'))
	if a.locked || len(a.currentTab().views) != 3 {
		t.Errorf("after unlocking: locked %v, %d panes; want alt+p to open one", a.locked, len(a.currentTab().views))
	}
}
