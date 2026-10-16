package app

import (
	"slices"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/pty"
)

// The log under the list follows the selection; enter gives it the keys to scroll, and esc goes
// back to the list.
func TestLogPreviewFollowsTheSelection(t *testing.T) {
	a := testApp(t)
	today := dateOf(time.Now()).Add(10 * time.Hour)
	newer := writeTestLog(t, today.Add(time.Minute), "newer", 10)
	older := writeTestLog(t, today, "older", 10)
	var shown []string
	var views []*fakeView
	var size pty.Size
	a.launch = func(p program, s pty.Size) (view, error) {
		shown, size = append(shown, p.argv[len(p.argv)-1]), s
		views = append(views, &fakeView{})
		return views[len(views)-1], nil
	}
	pressDrawn(a, prefixPress, letter('v'))
	p := a.currentTab().focusedView().(*logsPanel)
	pressDrawn(a, enterPress, downPress)
	if !slices.Equal(shown, []string{newer.Path, older.Path}) || !views[0].closed {
		t.Fatalf("shown %v, first closed %v; want each log in turn, the one left closed", shown, views[0].closed)
	}
	if size.Rows < a.size.Y/2-4 {
		t.Errorf("the log has %d rows, want about half of %d", size.Rows, a.size.Y)
	}
	pressDrawn(a, enterPress, upPress)
	if !p.preview.focused || p.selected != 1 {
		t.Errorf("after enter, up moved the list: focused %v, selected %d", p.preview.focused, p.selected)
	}
	pressDrawn(a, escPress, upPress)
	if p.preview.focused || p.selected != 0 || !p.focusList {
		t.Errorf("esc in the log did not go back to the list: focused %v, selected %d", p.preview.focused, p.selected)
	}
	pressDrawn(a, uv.KeyPressEvent{Code: 'd', Mod: uv.ModCtrl})
	if !views[2].closed {
		t.Error("closing the panel left the log's terminal running")
	}
}
