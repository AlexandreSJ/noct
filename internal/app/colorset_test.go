package app

import (
	"image"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// cellView draws one cell in the program's default colors at the top left of its area.
type cellView struct{ fakeView }

func (v *cellView) Draw(screen uv.Screen, area uv.Rectangle) {
	screen.SetCell(area.Min.X, area.Min.Y, &uv.Cell{Content: "x", Width: 1})
}

func drawnApp(a *App) uv.ScreenBuffer {
	buf := uv.NewScreenBuffer(a.size.X, a.size.Y)
	a.draw(buf, image.Rect(0, 0, a.size.X, a.size.Y))
	return buf
}

// colorsApp has panes 1 | 2 side by side, drawn with noct.
func colorsApp(t *testing.T) (*App, *tab) {
	t.Helper()
	tb := newTab(1, &cellView{})
	tb.tree.Split(1, 2, layout.SideBySide)
	tb.views[2] = &cellView{}
	a := testApp(t, tb)
	a.applyPalette()
	return a, tb
}

func (a *App) frameOfPane(t *tab, id layout.PaneID) image.Rectangle {
	return t.rects(a.workspaceArea())[id]
}

func TestAPaneColorOverridesTheTheme(t *testing.T) {
	a, tb := colorsApp(t)
	tb.paneColors[2] = colorSet{theme.FocusedBorder: "#ff0000"}
	tb.focused = 2
	buf := drawnApp(a)
	two, one := a.frameOfPane(tb, 2), a.frameOfPane(tb, 1)
	if c := buf.CellAt(two.Min.X, two.Min.Y+1); !colorsEqual(c.Style.Fg, colorOf("#ff0000")) {
		t.Errorf("pane 2's border is %v, want its own red", c.Style.Fg)
	}
	if c := buf.CellAt(one.Min.X, one.Min.Y+1); !colorsEqual(c.Style.Fg, a.palette.unfocusedFrame.Fg) {
		t.Errorf("pane 1's border is %v, want the theme's", c.Style.Fg)
	}
	tb.focused = 1
	buf = drawnApp(a)
	if c := buf.CellAt(two.Min.X, two.Min.Y+1); !colorsEqual(c.Style.Fg, a.palette.unfocusedFrame.Fg) {
		t.Errorf("pane 2 out of focus has %v, want the theme's unfocused border", c.Style.Fg)
	}
}

func TestAPaneBackgroundPaintsOnlyThatPane(t *testing.T) {
	a, tb := colorsApp(t)
	if a.config.Appearance.Background != config.BackgroundTerminal {
		t.Fatal("the test needs the terminal's background")
	}
	tb.paneColors[2] = colorSet{theme.Background: "#202020", theme.Text: "#eeeeee"}
	buf := drawnApp(a)
	inside := contentRect(a.frameOfPane(tb, 2)).Min
	if c := buf.CellAt(inside.X, inside.Y); !colorsEqual(c.Style.Bg, colorOf("#202020")) || !colorsEqual(c.Style.Fg, colorOf("#eeeeee")) {
		t.Errorf("pane 2's cell has %v on %v, want #eeeeee on #202020", c.Style.Fg, c.Style.Bg)
	}
	inside = contentRect(a.frameOfPane(tb, 1)).Min
	if c := buf.CellAt(inside.X, inside.Y); c.Style.Bg != nil {
		t.Errorf("pane 1's cell has the background %v, want the terminal's", c.Style.Bg)
	}
}

func TestATabColorPaintsItsNameInTheBar(t *testing.T) {
	a, _ := colorsApp(t)
	second := newTab(3, &cellView{})
	second.colors = colorSet{theme.InactiveTabText: "#00ff00"}
	a.session.tabs = append(a.session.tabs, second)
	var span tabSpan
	for _, s := range a.tabBar() {
		if s.tab == 1 && !s.mark {
			span = s
		}
	}
	buf := drawnApp(a)
	if c := buf.CellAt(span.start+1, 0); !colorsEqual(c.Style.Fg, colorOf("#00ff00")) {
		t.Errorf("the second tab's name is %v, want its own green", c.Style.Fg)
	}
	a.session.activeTab = 1
	buf = drawnApp(a)
	if c := buf.CellAt(span.start+1, 0); !colorsEqual(c.Style.Fg, a.palette.activeTab.Fg) {
		t.Errorf("the active second tab's name is %v, want the theme's active name", c.Style.Fg)
	}
}

func TestColorsMoveWithThePane(t *testing.T) {
	a, tb := colorsApp(t)
	tb.focused = 2
	tb.paneColors[2] = colorSet{theme.FocusedTitle: "#123456"}
	a.movePaneToAdjacentTab(layout.Right)
	target := a.currentTab()
	if target == tb || target.paneColors[2][theme.FocusedTitle] != "#123456" {
		t.Errorf("the moved pane's colors are %v", target.paneColors[2])
	}
	if _, ok := tb.paneColors[2]; ok {
		t.Error("the tab it left kept its colors")
	}
}

func TestAClosedPanesColorsAreForgotten(t *testing.T) {
	_, tb := colorsApp(t)
	tb.paneColors[2] = colorSet{theme.FocusedTitle: "#123456"}
	tb.removeView(2)
	if _, ok := tb.paneColors[2]; ok {
		t.Error("a closed pane kept its colors")
	}
}

func TestColorsAreSavedWithTheSession(t *testing.T) {
	a := testApp(t)
	a.openPanel(a.startScripts)
	a.openPanel(a.startScripts)
	tb := a.currentTab()
	ids := tb.tree.Panes()
	tb.colors = colorSet{theme.ActiveTabBackground: "#111111"}
	tb.paneColors[ids[1]] = colorSet{theme.FocusedBorder: "#ff0000", theme.Background: "60"}
	st, ok := a.savedTab(tb)
	if !ok {
		t.Fatal("the tab was not saved")
	}
	if st.Colors["tabs.active_background"] != "#111111" || st.Panes[1].Colors["panels.focused_border"] != "#ff0000" || st.Panes[1].Colors["background.color"] != "60" {
		t.Fatalf("saved %v and %v", st.Colors, st.Panes[1].Colors)
	}
	restored := a.restoreTab(st)
	panes := restored.tree.Panes()
	if restored.colors[theme.ActiveTabBackground] != "#111111" || restored.paneColors[panes[1]][theme.FocusedBorder] != "#ff0000" || len(restored.paneColors[panes[0]]) != 0 {
		t.Errorf("restored %v and %v", restored.colors, restored.paneColors)
	}
}

func TestUnknownSavedColorsAreLeftOut(t *testing.T) {
	s := colorSetFrom(map[string]string{"panels.focused_border": "#ff0000", "panels.sparkles": "#00ff00", "tabs.active_text": "#12"})
	if len(s) != 1 || s[theme.FocusedBorder] != "#ff0000" {
		t.Errorf("read %v", s)
	}
}
