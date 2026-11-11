package app

import (
	"fmt"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// filterTerminal is a textTerminal whose lines can be drawn alone.
type filterTerminal struct{ textTerminal }

func (v *filterTerminal) LastLinesMatching(match func(string) bool, n int) []int {
	var lines []int
	for i := len(v.lines) - 1; i >= 0 && len(lines) < n; i-- {
		if match(v.lines[i]) {
			lines = append([]int{i}, lines...)
		}
	}
	return lines
}

func (v *filterTerminal) DrawLines(screen uv.Screen, area uv.Rectangle, lines []int) {
	for y, line := range lines {
		if y < area.Dy() {
			drawText(screen, area.Min.X, area.Min.Y+y, area.Max.X, v.lines[line], uv.Style{})
		}
	}
}

func filteredPaneApp(t *testing.T) (*App, *filterTerminal) {
	t.Helper()
	v := &filterTerminal{textTerminal{rows: 10}}
	for i := range 100 {
		v.lines = append(v.lines, fmt.Sprintf("line %d", i))
		if i%25 == 3 {
			v.lines[i] = fmt.Sprintf("WARN disk at %d%%", i)
		}
	}
	return testApp(t, newTab(1, v)), v
}

func TestAFilteredPaneShowsOnlyTheMatchingLines(t *testing.T) {
	a, _ := filteredPaneApp(t)
	pressDrawn(a, prefixPress, letter('g'), letter('f'))
	if a.mode != promptMode {
		t.Fatalf("f while scrolling: mode %v, want the filter's prompt", a.mode)
	}
	typeText(a, `warn disk at \d+`)
	pressDrawn(a, enterPress)
	screen := screenText(a)
	if strings.Count(screen, "WARN disk") != 4 || strings.Contains(screen, "line 99") || !strings.Contains(screen, `filter: warn disk at \d+`) {
		t.Errorf("filtered pane:\n%s", screen)
	}

	pressDrawn(a, prefixPress, letter('g'), letter('f'), uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl}, enterPress)
	if screen := screenText(a); strings.Contains(screen, "filter:") || len(a.paneFilters) != 0 {
		t.Errorf("an empty filter left:\n%s", screen)
	}
}

func TestABadPatternTellsWhy(t *testing.T) {
	a, _ := filteredPaneApp(t)
	pressDrawn(a, prefixPress, letter('g'), letter('f'))
	typeText(a, "(unclosed")
	pressDrawn(a, enterPress)
	if _, filtered := a.paneFilters[1]; filtered || !strings.Contains(a.statusMessage, "missing closing )") {
		t.Errorf("filtered %v, status %q", filtered, a.statusMessage)
	}
}

func TestAPatternWithACapitalMatchesItsCase(t *testing.T) {
	a, _ := filteredPaneApp(t)
	if err := a.setPaneFilter(1, "Warn"); err != nil {
		t.Fatal(err)
	}
	if screen := screenText(a); strings.Contains(screen, "WARN disk") {
		t.Errorf("Warn matched WARN:\n%s", screen)
	}
}
