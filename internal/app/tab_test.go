package app

import (
	"image"
	"testing"

	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
)

func TestSplitDirectionAlternatesWithShape(t *testing.T) {
	tests := []struct {
		name  string
		frame image.Rectangle
		want  layout.Direction
	}{
		{"wide window splits side by side", image.Rect(0, 0, 200, 50), layout.SideBySide},
		{"half of it splits stacked", image.Rect(0, 0, 100, 50), layout.Stacked},
		{"tall pane splits stacked", image.Rect(0, 0, 40, 50), layout.Stacked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitDirectionFor(tt.frame); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanSplitKeepsPanesUsable(t *testing.T) {
	if !canSplit(image.Rect(0, 0, 24, 10), layout.SideBySide) {
		t.Error("24 columns should fit two panes side by side")
	}
	if canSplit(image.Rect(0, 0, 23, 10), layout.SideBySide) {
		t.Error("23 columns cannot fit two 12-column panes")
	}
	if canSplit(image.Rect(0, 0, 80, 9), layout.Stacked) {
		t.Error("9 rows cannot fit two 5-row panes")
	}
}

func TestContentSizeExcludesFrame(t *testing.T) {
	if got := contentSize(image.Rect(0, 1, 80, 25)); got != (pty.Size{Cols: 78, Rows: 22}) {
		t.Errorf("got %+v", got)
	}
}

func TestClosingATabKeepsANeighborSelected(t *testing.T) {
	tests := []struct {
		name       string
		active     int
		wantActive int
	}{
		{"a middle tab picks the next one", 1, 1},
		{"the last tab picks the previous one", 2, 1},
		{"the first tab picks the next one", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := testApp(t, newTab(1, &fakeView{}), newTab(2, &fakeView{}), newTab(3, &fakeView{}))
			a.session.activeTab = tt.active
			a.closeFocused()
			if len(a.session.tabs) != 2 || a.session.activeTab != tt.wantActive {
				t.Errorf("tabs=%d active=%d, want 2 and %d", len(a.session.tabs), a.session.activeTab, tt.wantActive)
			}
		})
	}
}

func TestClosingABackgroundTabKeepsTheSelectedOne(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}), newTab(2, &fakeView{}), newTab(3, &fakeView{}))
	a.session.activeTab = 2
	a.detachView(1) // a program exited in the first tab
	if len(a.session.tabs) != 2 || a.currentTab().focused != 3 {
		t.Errorf("tabs=%d, selected pane %d, want 3", len(a.session.tabs), a.currentTab().focused)
	}
}
