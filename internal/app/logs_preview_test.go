package app

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/logstore"
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

// writeNumberedLog writes a log of numbered lines, the first older of them in its rotated part.
func writeNumberedLog(t *testing.T, lines, older int) logstore.Entry {
	t.Helper()
	e := writeTestLog(t, time.Now(), "big", 2)
	var rotated, current strings.Builder
	for i := 1; i <= lines; i++ {
		part := &current
		if i <= older {
			part = &rotated
		}
		fmt.Fprintf(part, "line %d\n", i)
	}
	if older > 0 {
		if err := os.WriteFile(e.Path+".1", []byte(rotated.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(e.Path, []byte(current.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return e
}

// The parts of a long log, read from the first to the last, hold each of its lines once, across
// its rotated part too.
func TestLogPartsHoldEveryLineOnce(t *testing.T) {
	for _, c := range []struct{ lines, older int }{{12345, 7000}, {9000, 7000}, {12000, 0}, {30, 0}} {
		testApp(t)
		e := writeNumberedLog(t, c.lines, c.older)
		v := logPreview{lines: c.lines}
		var got []string
		for back := v.parts() - 1; back >= 0; back-- {
			v.back = back
			argv := slices.Clone(v.partCommand(e))
			argv[2] = strings.ReplaceAll(argv[2], " -f ", " ") // the last part would follow the log
			out, err := exec.Command(argv[0], argv[1:]...).Output()
			if err != nil {
				t.Fatalf("%+v, part %d: %v", c, back, err)
			}
			got = append(got, strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")...)
		}
		if len(got) != c.lines || got[0] != "line 1" || got[len(got)-1] != fmt.Sprintf("line %d", c.lines) {
			t.Fatalf("%+v: %d lines from %q to %q", c, len(got), got[0], got[len(got)-1])
		}
		for i, line := range got {
			if line != fmt.Sprintf("line %d", i+1) {
				t.Fatalf("%+v: line %d is %q", c, i+1, line)
			}
		}
	}
}

// Scrolling past the top of the part shown goes on into the part before; g goes to the first
// and G back to the last, which follows the log. Leaving the log goes back to the live end.
func TestLogPreviewScrollsThroughTheParts(t *testing.T) {
	a := testApp(t)
	writeNumberedLog(t, 12345, 0)
	var commands []string
	a.launch = func(p program, _ pty.Size) (view, error) {
		commands = append(commands, p.argv[2])
		return &fakeView{}, nil
	}
	pressDrawn(a, prefixPress, letter('v'), enterPress, enterPress)
	p := a.currentTab().focusedView().(*logsPanel)
	if p.preview.parts() != 3 || len(commands) != 1 || !strings.Contains(commands[0], "tail") {
		t.Fatalf("parts %d, commands %q; want 3 parts and the last one followed", p.preview.parts(), commands)
	}
	pressDrawn(a, upPress)
	if p.preview.back != 1 || !strings.Contains(commands[len(commands)-1], "sed") {
		t.Errorf("up at the top: part %d, command %q; want the part before", p.preview.back, commands[len(commands)-1])
	}
	pressDrawn(a, letter('g'))
	if p.preview.back != 2 || p.preview.offset != atPartTop {
		t.Errorf("g: part %d, offset %d; want the first part at its top", p.preview.back, p.preview.offset)
	}
	pressDrawn(a, letter('G'))
	if p.preview.back != 0 {
		t.Errorf("G: part %d, want the last", p.preview.back)
	}
	pressDrawn(a, upPress, letter(' '))
	if p.preview.focused || p.preview.back != 0 {
		t.Errorf("space in the log: focused %v, part %d; want the list and the live end", p.preview.focused, p.preview.back)
	}
	if !strings.Contains(screenText(a), "lines 7,346–12,345 of 12,345") {
		t.Errorf("the title does not say which lines show:\n%s", screenText(a))
	}
}

// z shows the log alone and back, v hides it and closes its terminal, p moves it to another side
// and + and - change its size, both kept in the config. Beside a log too wide for the calendar,
// it goes below.
func TestLogsLayoutKeys(t *testing.T) {
	a := testApp(t)
	writeTestLog(t, dateOf(time.Now()).Add(10*time.Hour), "one", 10)
	var views []*fakeView
	a.launch = func(program, pty.Size) (view, error) {
		views = append(views, &fakeView{})
		return views[len(views)-1], nil
	}
	pressDrawn(a, prefixPress, letter('v'), letter('z'))
	p := a.currentTab().focusedView().(*logsPanel)
	if p.shown != logsLogOnly || !p.preview.focused || !p.list.Empty() {
		t.Errorf("z: shown %v, focused %v, list %v; want the log alone with the keys", p.shown, p.preview.focused, p.list)
	}
	pressDrawn(a, escPress)
	if p.shown != logsBoth || p.preview.focused {
		t.Errorf("esc from the log alone: shown %v, focused %v; want both, the list with the keys", p.shown, p.preview.focused)
	}
	pressDrawn(a, letter('v'))
	if p.shown != logsListOnly || !views[len(views)-1].closed || !p.preview.area.Empty() {
		t.Errorf("v: shown %v, closed %v; want the log hidden and its terminal closed", p.shown, views[len(views)-1].closed)
	}
	pressDrawn(a, letter('p'), letter('+'), letter('+'))
	if p.shown != logsBoth || a.config.Logs.Preview != "right" || a.config.Logs.PreviewPercent != 70 {
		t.Errorf("p + +: shown %v, logs %+v; want the log back, on the right, at 70%%", p.shown, a.config.Logs)
	}
	if p.preview.area.Min.Y <= p.list.Max.Y {
		t.Errorf("on %d columns the calendar does not fit beside the log, but the log at %v is not under the list at %v", a.size.X, p.preview.area, p.list)
	}
	a.size.X = 160
	drawApp(a)
	if p.preview.area.Min.X <= p.list.Max.X {
		t.Errorf("the log at %v is not right of the list at %v", p.preview.area, p.list)
	}
	saved, err := config.Load(a.configPath)
	if err != nil || saved.Logs.Preview != "right" || saved.Logs.PreviewPercent != 70 {
		t.Errorf("saved %+v, %v", saved.Logs, err)
	}
}

// The part before, opened at its top, scrolls down at the first page down.
func TestLogPartOpenedAtItsTopScrollsAtOnce(t *testing.T) {
	a := testApp(t)
	writeNumberedLog(t, 12345, 0)
	a.launch = func(program, pty.Size) (view, error) {
		return &scrollableView{history: 4985}, nil
	}
	pressDrawn(a, prefixPress, letter('v'), enterPress, enterPress, letter('g'), pgDownPress)
	p := a.currentTab().focusedView().(*logsPanel)
	if p.preview.back != 2 || p.preview.offset >= 4985 {
		t.Errorf("part %d, offset %d; want the first part a page under its top", p.preview.back, p.preview.offset)
	}
}
