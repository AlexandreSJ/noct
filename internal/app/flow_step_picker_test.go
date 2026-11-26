package app

import (
	"image"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// a opens the step picker over the chart, in the panel: typing narrows it, the wheel and the
// arrows move through it, a click picks, and noct's prefix still works while it is open.
func TestTheStepPickerFiltersAndTakesTheMouse(t *testing.T) {
	a, p := openMovesFlow(t)
	pressDrawn(a, letter('a'))
	picker, ok := p.editor.(*stepPicker)
	if !ok || a.mode != normalMode {
		t.Fatalf("a opened %T, mode %v; want the picker in the panel", p.editor, a.mode)
	}
	typeDrawn(a, "wa")
	if len(picker.list.matches) == 0 || picker.list.matches[0].label != "wait" {
		t.Fatalf("wa matches %v", picker.list.matches)
	}
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyBackspace}, uv.KeyPressEvent{Code: uv.KeyBackspace})
	a.handleEvent(uv.MouseWheelEvent{X: picker.rows.Min.X + 2, Y: picker.rows.Min.Y, Button: uv.MouseWheelDown})
	if picker.list.selected != 1 {
		t.Errorf("the wheel left the selection at %d", picker.list.selected)
	}
	pressDrawn(a, prefixPress)
	if a.mode != commandMode {
		t.Errorf("the prefix does not reach noct with the picker open: mode %v", a.mode)
	}
	pressDrawn(a, escPress)

	row := picker.rows.Min.Add(image.Pt(2, 3)) // the fourth item: command
	a.handleMouse(uv.MouseClickEvent{X: row.X, Y: row.Y, Button: uv.MouseLeft})
	drawnApp(a)
	if _, ok := p.editor.(*stepForm); !ok || !strings.Contains(p.editor.title(), "command") {
		t.Fatalf("a click on command opened %T", p.editor)
	}
	pressDrawn(a, escPress, escPress)
}

// tab in the picker adds again what was added last, down to the script it ran.
func TestTabInThePickerAddsTheLastStepAgain(t *testing.T) {
	a, p := openMovesFlow(t)
	a.lastStepAdded = []string{"wait"}
	pressDrawn(a, letter('a'))
	if !strings.Contains(screenText(a), "tab adds wait again") {
		t.Errorf("no hint of tab:\n%s", screenText(a))
	}
	pressDrawn(a, tabPress)
	if _, ok := p.editor.(*stepForm); !ok || p.editor.title() != "wait" {
		t.Fatalf("tab opened %T", p.editor)
	}
	pressDrawn(a, escPress, letter('a'))
	typeDrawn(a, "unset")
	pressDrawn(a, enterPress, escPress)
	if strings.Join(a.lastStepAdded, " ") != "unset" {
		t.Errorf("the last step added is %v", a.lastStepAdded)
	}
}

// The step added last shows dim after what is typed while it starts so, and tab adds it only then.
func TestThePickersTabFollowsWhatIsTyped(t *testing.T) {
	a, p := openMovesFlow(t)
	a.lastStepAdded = []string{"set"}
	pressDrawn(a, letter('a'), letter('s'))
	picker := p.editor.(*stepPicker)
	if ghost, ok := picker.ghost(); !ok || ghost != "et" {
		t.Errorf("after s the ghost is %q (%v)", ghost, ok)
	}
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyBackspace}, letter('a'), tabPress)
	if p.editor != picker {
		t.Fatalf("tab after a, which set does not start with, opened %T", p.editor)
	}
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyBackspace}, letter('s'), tabPress)
	if form, ok := p.editor.(*stepForm); !ok || form.title() != "set" {
		t.Errorf("s and tab opened %T", p.editor)
	}
}
