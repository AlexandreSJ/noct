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

func TestHoveringATabShowsItAsTheActiveOne(t *testing.T) {
	a, name := hoverApp(t)
	a.handleMouse(motion(name.X, name.Y))
	if fg, _ := cellStyleAt(a, name); !colorsEqual(fg, a.palette.tab.Fg) {
		t.Errorf("as the hover starts the name is %v, want the inactive tab's %v", fg, a.palette.tab.Fg)
	}
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	if fg, bg := cellStyleAt(a, name); !colorsEqual(fg, a.palette.activeTab.Fg) || !colorsEqual(bg, a.palette.activeTab.Bg) {
		t.Errorf("hovered, the name is %v on %v, want the active tab's %v on %v", fg, bg, a.palette.activeTab.Fg, a.palette.activeTab.Bg)
	}
}

func TestLeavingATabFadesItBack(t *testing.T) {
	a, name := hoverApp(t)
	a.handleMouse(motion(name.X, name.Y))
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	a.handleMouse(motion(a.size.X/2, a.size.Y/2))
	if fg, _ := cellStyleAt(a, name); !colorsEqual(fg, a.palette.activeTab.Fg) {
		t.Errorf("as the mouse leaves the name is %v, want it still in the active tab's %v", fg, a.palette.activeTab.Fg)
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
