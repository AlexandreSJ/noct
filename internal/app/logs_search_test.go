package app

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/pty"
)

// textTerminal is a terminal showing lines, its history and then its screen of rows.
type textTerminal struct {
	fakeView
	lines []string
	rows  int
}

func (v *textTerminal) ScrollbackLen() int { return max(len(v.lines)-v.rows, 0) }

func (v *textTerminal) DrawScrolled(screen uv.Screen, area uv.Rectangle, offset int) {
	history := v.ScrollbackLen()
	first := history - min(max(offset, 0), history)
	for y := range area.Dy() {
		if first+y < len(v.lines) {
			drawText(screen, area.Min.X, area.Min.Y+y, area.Max.X, v.lines[first+y], uv.Style{})
		}
	}
}

func (v *textTerminal) LineCount() int        { return len(v.lines) }
func (v *textTerminal) LineText(i int) string { return v.lines[i] }

// searchedLog opens the logs on a log whose terminal shows 100 lines, ERROR on lines 20, 50 and
// 80, with the keys in the log; launched are the commands its terminals ran.
func searchedLog(t *testing.T) (a *App, p *logsPanel, launched *[]string) {
	t.Helper()
	a = testApp(t)
	writeTestLog(t, dateOf(time.Now()).Add(10*time.Hour), "build", 100)
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
		if i%30 == 20 {
			lines[i] = fmt.Sprintf("ERROR at line %d", i)
		}
	}
	launched = &[]string{}
	a.launch = func(prog program, size pty.Size) (view, error) {
		*launched = append(*launched, strings.Join(prog.argv, " "))
		return &textTerminal{lines: lines, rows: size.Rows}, nil
	}
	pressDrawn(a, prefixPress, letter('v'), enterPress, enterPress)
	p = a.currentTab().focusedView().(*logsPanel)
	if !p.preview.focused {
		t.Fatal("enter did not give the log the keys")
	}
	return a, p, launched
}

func search(a *App, text string) {
	pressDrawn(a, letter('/'), uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl})
	typeText(a, text)
	pressDrawn(a, enterPress)
}

// shownLines are the first and last lines of the log's terminal in view.
func shownLines(p *logsPanel) (int, int) {
	v := &p.preview
	history := v.scrollback()
	first := history - min(v.offset, history)
	return first, first + v.size.Rows - 1
}

func TestSearchingTheLogGoesUpFromItsEnd(t *testing.T) {
	a, p, _ := searchedLog(t)
	search(a, "error")
	for _, want := range []int{80, 50, 20} {
		first, last := shownLines(p)
		if !p.preview.matched || p.preview.match != want || want < first || want > last {
			t.Fatalf("on line %d (matched %v), lines %d–%d in view; want line %d in view", p.preview.match, p.preview.matched, first, last, want)
		}
		pressDrawn(a, letter('n'))
	}
	if p.preview.match != 20 || !strings.Contains(a.notice, "no match") {
		t.Errorf("past the first match: on line %d, notice %q; want line 20 and no match told", p.preview.match, a.notice)
	}
	pressDrawn(a, uv.KeyPressEvent{Code: 'n', Mod: uv.ModShift, Text: "N"})
	if p.preview.match != 50 {
		t.Errorf("N went to line %d, want 50, back down the log", p.preview.match)
	}
}

func TestSearchingInLowerCaseMatchesAnyCase(t *testing.T) {
	a, p, _ := searchedLog(t)
	search(a, "Error")
	if p.preview.matched {
		t.Errorf("Error matched line %d, want no match on ERROR", p.preview.match)
	}
	search(a, "error")
	if !p.preview.matched {
		t.Error("error did not match ERROR")
	}
}

func TestLogSearchLightsItsMatches(t *testing.T) {
	a, p, _ := searchedLog(t)
	search(a, "error")
	area := p.preview.area.Add(contentRect(mustFrame(t, a)).Min)
	screen := drawnApp(a)
	lit := 0
	for y := area.Min.Y + 1; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if c := screen.CellAt(x, y); c != nil && c.Style.Attrs&uv.AttrReverse != 0 {
				lit++
			}
		}
	}
	if lit != len("error") {
		t.Errorf("%d cells lit, want the %d of the one match in view", lit, len("error"))
	}
}

func TestEscClearsTheSearchBeforeLeavingTheLog(t *testing.T) {
	a, p, _ := searchedLog(t)
	search(a, "error")
	pressDrawn(a, escPress)
	if !p.preview.focused || p.preview.query != "" {
		t.Errorf("esc: focused %v, query %q; want the search cleared, the log still focused", p.preview.focused, p.preview.query)
	}
	pressDrawn(a, escPress)
	if p.preview.focused {
		t.Error("a second esc did not go back to the list")
	}
}

func TestFilteringTheLogShowsOnlyItsMatchingLines(t *testing.T) {
	a, p, launched := searchedLog(t)
	pressDrawn(a, letter('f'))
	typeText(a, "error")
	pressDrawn(a, enterPress)
	if !p.preview.focused || p.preview.filter != "error" || len(*launched) != 2 || !strings.Contains((*launched)[1], "grep") {
		t.Fatalf("f: focused %v, filter %q, commands %q; want the log started over through grep", p.preview.focused, p.preview.filter, *launched)
	}
	if !strings.Contains(screenText(a), "filter: error") {
		t.Errorf("the log's title does not tell the filter:\n%s", screenText(a))
	}
	pressDrawn(a, escPress)
	if p.preview.filter != "" || len(*launched) != 3 || strings.Contains((*launched)[2], "grep") {
		t.Errorf("esc: filter %q, commands %q; want the whole log again", p.preview.filter, *launched)
	}
}

func TestALogFilterKeepsTheLinesWithItsText(t *testing.T) {
	testApp(t)
	e := writeNumberedLog(t, 30, 12)
	upper := writeTestLog(t, time.Now().Add(time.Minute), "loud", 1)
	if err := os.WriteFile(upper.Path, []byte("ERROR one\nfine\nError two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := logPreview{filter: "error"}
	argv := slices.Clone(v.partCommand(upper))
	argv[2] = strings.ReplaceAll(argv[2], " -f ", " ")
	if out, _ := exec.Command(argv[0], argv[1:]...).Output(); string(out) != "ERROR one\nError two\n" {
		t.Errorf("filter error kept %q, want the lines with it in any case", out)
	}
	for _, c := range []struct {
		filter string
		want   []string
	}{
		{"line 1", []string{"line 1", "line 10", "line 11", "line 12", "line 13", "line 14", "line 15", "line 16", "line 17", "line 18", "line 19"}},
		{"LINE 2", nil},
		{"line 2", []string{"line 2", "line 20", "line 21", "line 22", "line 23", "line 24", "line 25", "line 26", "line 27", "line 28", "line 29"}},
		{"Line 3", nil},
		{"line 3", []string{"line 3", "line 30"}},
		{"'; echo x", nil},
		{"LINE 3", nil},
	} {
		v := logPreview{lines: 30, filter: c.filter}
		argv := slices.Clone(v.partCommand(e))
		argv[2] = strings.ReplaceAll(argv[2], " -f ", " ")
		out, err := exec.Command(argv[0], argv[1:]...).Output()
		if err != nil && c.want != nil {
			t.Fatalf("filter %q: %v", c.filter, err)
		}
		got := strings.Fields(strings.ReplaceAll(string(out), "line ", "line_"))
		want := strings.Fields(strings.ReplaceAll(strings.Join(c.want, " "), "line ", "line_"))
		if !slices.Equal(got, want) {
			t.Errorf("filter %q kept %q, want %q", c.filter, got, want)
		}
	}
}

func mustFrame(t *testing.T, a *App) image.Rectangle {
	t.Helper()
	frame, ok := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	if !ok {
		t.Fatal("the logs panel is not on screen")
	}
	return frame
}

func TestTShowsWhenEachLineOfTheLogStarted(t *testing.T) {
	a, p, launched := searchedLog(t)
	pressDrawn(a, letter('t'))
	if !p.preview.times || len(*launched) != 2 || !strings.Contains((*launched)[1], " log-times --from 1 --follow -- ") {
		t.Fatalf("t: times %v, commands %q; want the log started over through noct log-times", p.preview.times, *launched)
	}
	if !strings.Contains(screenText(a), "times") {
		t.Errorf("the log's title does not tell the times:\n%s", screenText(a))
	}
	pressDrawn(a, letter('f'))
	typeText(a, "error")
	pressDrawn(a, enterPress)
	if last := (*launched)[len(*launched)-1]; !strings.Contains(last, "log-times") || !strings.Contains(last, "grep") {
		t.Errorf("filtered with times: %q, want both", last)
	}
	pressDrawn(a, escPress, letter('t'))
	if last := (*launched)[len(*launched)-1]; p.preview.times || strings.Contains(last, "log-times") {
		t.Errorf("t again: times %v, command %q; want the plain log", p.preview.times, last)
	}
}

func TestAFilterWithTimesKeepsTheLinesWithItsText(t *testing.T) {
	noct := filepath.Join(t.TempDir(), "noct")
	if out, err := exec.Command("go", "build", "-o", noct, "../../cmd/noct").CombinedOutput(); err != nil {
		t.Fatalf("building noct: %v\n%s", err, out)
	}
	testApp(t)
	e := writeNumberedLog(t, 30, 12)
	v := logPreview{lines: 30, filter: "line 3", times: true, noct: noct}
	argv := slices.Clone(v.partCommand(e))
	argv[2] = strings.ReplaceAll(argv[2], " --follow ", " ")
	out, err := exec.Command(argv[0], argv[1:]...).Output()
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n"); len(lines) != 2 || !strings.HasSuffix(lines[0], " line 3") || !strings.HasSuffix(lines[1], " line 30") {
		t.Errorf("filter with times kept %q", out)
	}
}
