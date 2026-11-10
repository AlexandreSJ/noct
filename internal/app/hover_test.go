package app

import (
	"image"
	"testing"
	"time"
)

// hoverApp has two tabs, the first active, with hover fading.
func hoverApp(t *testing.T) (*App, image.Point) {
	t.Helper()
	a, _ := animatedApp(t)
	a.config.Animations.Hover = "fade"
	a.session.tabs = append(a.session.tabs, newTab(5, &cellView{}))
	drawnApp(a)
	for _, span := range a.tabBar() {
		if span.tab == 1 && !span.mark {
			return a, image.Pt(span.start+1, 0)
		}
	}
	t.Fatal("no second tab in the bar")
	return nil, image.Point{}
}

func cellStyleAt(a *App, at image.Point) (fg, bg interface{ RGBA() (r, g, b, a uint32) }) {
	c := drawnApp(a).CellAt(at.X, at.Y)
	return c.Style.Fg, c.Style.Bg
}

func TestHoveringATabColorsOnlyItsName(t *testing.T) {
	a, name := hoverApp(t)
	a.handleMouse(motion(name.X, name.Y))
	if fg, _ := cellStyleAt(a, name); !colorsEqual(fg, a.palette.tab.Fg) {
		t.Errorf("as the hover starts the name is %v, want the inactive tab's %v", fg, a.palette.tab.Fg)
	}
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	fg, bg := cellStyleAt(a, name)
	if !colorsEqual(fg, a.palette.activeTab.Bg) {
		t.Errorf("hovered, the name is %v, want the active tab's color %v", fg, a.palette.activeTab.Bg)
	}
	if bg != nil && (a.palette.tab.Bg == nil || !colorsEqual(bg, a.palette.tab.Bg)) {
		t.Errorf("hovered, the name is on %v, want the inactive tab's background %v", bg, a.palette.tab.Bg)
	}
}

func TestHoveringATabWorksFromAnEmptyTab(t *testing.T) {
	a, _ := animatedApp(t)
	a.config.Animations.Hover = "fade"
	a.session.tabs = []*tab{newEmptyTab(), newTab(5, &cellView{})}
	a.session.activeTab = 0
	drawnApp(a)
	var name image.Point
	for _, span := range a.tabBar() {
		if span.tab == 1 && !span.mark {
			name = image.Pt(span.start+1, 0)
		}
	}
	a.handleMouse(motion(name.X, name.Y))
	if animationOf(a, momentHover) == nil {
		t.Fatal("hovering a tab from an empty tab played nothing")
	}
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	if fg, _ := cellStyleAt(a, name); !colorsEqual(fg, a.palette.activeTab.Bg) {
		t.Errorf("hovered from an empty tab, the name is %v, want %v", fg, a.palette.activeTab.Bg)
	}
}

func TestLeavingATabFadesItBack(t *testing.T) {
	a, name := hoverApp(t)
	a.handleMouse(motion(name.X, name.Y))
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	a.handleMouse(motion(a.size.X/2, a.size.Y/2))
	if fg, _ := cellStyleAt(a, name); !colorsEqual(fg, a.palette.activeTab.Bg) {
		t.Errorf("as the mouse leaves the name is %v, want it still in the active tab's color %v", fg, a.palette.activeTab.Bg)
	}
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	if fg, _ := cellStyleAt(a, name); !colorsEqual(fg, a.palette.tab.Fg) {
		t.Errorf("after leaving the name is %v, want the inactive tab's %v", fg, a.palette.tab.Fg)
	}
}

func TestHoveringAPaneTitleShowsItAsFocused(t *testing.T) {
	a, tb := animatedApp(t)
	a.config.Animations.Hover = "fade"
	a.openPanel(a.startTerminal)
	a.finishAnimations()
	drawnApp(a)
	var other image.Rectangle
	for _, p := range tb.visible(a.workspaceArea()) {
		if p.id != tb.focused {
			other = p.frame
		}
	}
	title := image.Pt(other.Min.X+3, other.Min.Y)
	a.handleMouse(motion(title.X, title.Y))
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	if fg, _ := cellStyleAt(a, title); !colorsEqual(fg, a.palette.focusedTitle.Fg) {
		t.Errorf("hovered, the title is %v, want the focused title's %v", fg, a.palette.focusedTitle.Fg)
	}
}

func TestLeavingAPaneTitleLeavesItsBorderAlone(t *testing.T) {
	a, tb := animatedApp(t)
	a.config.Animations.Hover = "fade"
	a.openPanel(a.startTerminal)
	a.finishAnimations()
	drawnApp(a)
	var other image.Rectangle
	for _, p := range tb.visible(a.workspaceArea()) {
		if p.id != tb.focused {
			other = p.frame
		}
	}
	a.handleMouse(motion(other.Min.X+3, other.Min.Y))
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	a.handleMouse(motion(a.size.X/2, a.size.Y-2))
	border := image.Pt(other.Max.X-3, other.Min.Y)
	if fg, _ := cellStyleAt(a, border); !colorsEqual(fg, a.palette.unfocusedFrame.Fg) {
		t.Errorf("as the mouse leaves the title the border is %v, want its own %v", fg, a.palette.unfocusedFrame.Fg)
	}
}
