package app

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/pty"
)

// resizeCounter is a view that counts how often its program would be resized.
type resizeCounter struct {
	cellView
	resizes int
}

func (v *resizeCounter) Resize(size pty.Size) { v.resizes++; v.size = size }

// animatedApp is an app with animations on and one tab with one pane, drawn once.
func animatedApp(t *testing.T) (*App, *tab) {
	t.Helper()
	a := testApp(t, newTab(1, &cellView{}))
	a.config.Appearance.Animations = config.AnimationsOn
	a.applyPalette()
	a.launch = func(program, pty.Size) (view, error) { return &resizeCounter{}, nil }
	frozen := time.Now()
	a.clock = func() time.Time { return frozen } // animations move only when a test moves them
	drawnApp(a)
	return a, a.currentTab()
}

// rewind moves every running animation's start back by d.
func rewind(a *App, d time.Duration) {
	for _, anim := range a.animations {
		anim.start = anim.start.Add(-d)
	}
}

func TestAnimationsAreOffWhenTurnedOff(t *testing.T) {
	a, _ := animatedApp(t)
	a.config.Appearance.Animations = config.AnimationsOff
	if a.effectFor(momentPanels) != effectNone {
		t.Error("panels animate with animations off")
	}
	a.config.Appearance.Animations = config.AnimationsOn
	if a.effectFor(momentPanels) != effectSpring {
		t.Errorf("panels animate with %v, want the spring", a.effectFor(momentPanels))
	}
}

func TestOpeningAPaneStartsASpring(t *testing.T) {
	a, tb := animatedApp(t)
	a.openPanel(a.startTerminal)
	drawnApp(a)
	if len(a.animations) != 1 || a.animations[0].moment != momentPanels || a.animations[0].pane != tb.focused || a.animTick == nil {
		t.Fatalf("animations %v after opening a pane", a.animations)
	}
}

func TestTheSpringGrowsThePaneWithoutResizingIt(t *testing.T) {
	a, tb := animatedApp(t)
	a.openPanel(a.startTerminal)
	opened := tb.views[tb.focused].(*resizeCounter)
	before := opened.resizes
	drawnApp(a)
	rewind(a, 30*time.Millisecond)
	buf := drawnApp(a)
	frame := a.frameOfPane(tb, tb.focused)
	if c := buf.CellAt(frame.Min.X, frame.Min.Y); c != nil && c.Content == "┌" {
		t.Error("the new pane's corner is already drawn at the start of the spring")
	}
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	buf = drawnApp(a)
	if c := buf.CellAt(frame.Min.X, frame.Min.Y); c == nil || c.Content != "┌" {
		t.Error("the new pane's corner is missing after the spring")
	}
	if opened.resizes != before {
		t.Errorf("the program was resized %d times during the animation", opened.resizes-before)
	}
}

func TestAnAnimationEndsInTheNormalFrame(t *testing.T) {
	a, _ := animatedApp(t)
	a.openPanel(a.startTerminal)
	drawnApp(a)
	rewind(a, 300*time.Millisecond)
	a.advanceAnimations(a.now())
	if len(a.animations) != 0 || a.animTick != nil {
		t.Fatalf("animations %v left after their time", a.animations)
	}
	animated := drawnApp(a)
	a.config.Appearance.Animations = config.AnimationsOff
	plain := drawnApp(a)
	if !sameCells(animated, plain, image.Rect(0, 0, a.size.X, a.size.Y)) {
		t.Error("the frame after the animation is not the plain one")
	}
}

func TestClosingAPaneShrinksWhatItShowed(t *testing.T) {
	a, tb := animatedApp(t)
	a.openPanel(a.startTerminal)
	drawnApp(a)
	a.finishAnimations()
	drawnApp(a)
	id := tb.focused
	inside := contentRect(a.frameOfPane(tb, id)).Min
	a.closeFocused()
	buf := drawnApp(a)
	if an := animationOf(a, momentPanels); an == nil || !an.closing {
		t.Fatalf("animations %v after closing a pane", a.animations) // the focus moving animates too
	}
	if c := buf.CellAt(inside.X, inside.Y); c == nil || c.Content != "x" {
		t.Errorf("where the pane was shows %+v, want what it showed", c)
	}
}

func TestAKeyFinishesTheAnimationsAndIsHandled(t *testing.T) {
	a, tb := animatedApp(t)
	a.openPanel(a.startTerminal)
	drawnApp(a)
	if err := a.handleEvent(letter('x')); err != nil {
		t.Fatal(err)
	}
	if len(a.animations) != 0 {
		t.Error("a key left the animations running")
	}
	if keys := tb.focusedView().(*resizeCounter).keys; len(keys) != 1 || keys[0].Text != "x" {
		t.Errorf("the pane got %v, want the x", keys)
	}
}

func TestNewAnimationsReplaceOldOnesOfTheSamePane(t *testing.T) {
	a, _ := animatedApp(t)
	for range 3 {
		a.openPanel(a.startTerminal)
		drawnApp(a)
	}
	seen := map[layout.PaneID]bool{}
	for _, anim := range a.animations {
		if seen[anim.pane] {
			t.Errorf("pane %d animates twice", anim.pane)
		}
		seen[anim.pane] = true
	}
}

func TestSwitchingSessionDropsTheAnimations(t *testing.T) {
	a, _ := animatedApp(t)
	a.sessionsPath = t.TempDir() + "/sessions.json"
	a.openPanel(a.startTerminal)
	drawnApp(a)
	other, err := a.createSession("other")
	if err != nil {
		t.Fatal(err)
	}
	a.switchSession(other)
	if len(a.animations) != 0 {
		t.Error("animations followed to another session")
	}
}

func TestResizingTheTerminalEndsTheAnimations(t *testing.T) {
	a, _ := animatedApp(t)
	a.openPanel(a.startTerminal)
	drawnApp(a)
	a.size = image.Pt(a.size.X-10, a.size.Y)
	drawnApp(a)
	if len(a.animations) != 0 {
		t.Error("animations went on in a resized terminal")
	}
}

// twoPanesAnimated is an animated app with panes 1 | 2, the focus on 1, drawn once.
func twoPanesAnimated(t *testing.T) (*App, *tab) {
	t.Helper()
	a, tb := animatedApp(t)
	tb.tree.Split(1, 2, layout.SideBySide)
	tb.views[2] = &cellView{}
	tb.resizeViews(a.workspaceArea())
	drawnApp(a)
	a.finishAnimations() // the second pane is already there
	return a, tb
}

func animationOf(a *App, m moment) *animation {
	for _, an := range a.animations {
		if an.moment == m {
			return an
		}
	}
	return nil
}

func TestOpeningATabAnimatesItsNameInTheBar(t *testing.T) {
	a, _ := animatedApp(t)
	a.openNewTab()
	drawnApp(a)
	if an := animationOf(a, momentTabs); an == nil {
		t.Fatalf("animations %v after opening a tab", a.animations)
	}
}

func TestFocusFadesTheBorders(t *testing.T) {
	a, tb := twoPanesAnimated(t)
	a.focusPane(tb, 2)
	drawnApp(a)
	if animationOf(a, momentFocus) == nil {
		t.Fatalf("animations %v after moving the focus", a.animations)
	}
	rewind(a, 75*time.Millisecond)
	buf := drawnApp(a)
	frame := a.frameOfPane(tb, 2)
	fg := buf.CellAt(frame.Min.X, frame.Min.Y+1).Style.Fg
	if colorsEqual(fg, a.palette.focusedFrame.Fg) || colorsEqual(fg, a.palette.unfocusedFrame.Fg) {
		t.Errorf("halfway, the border is %v, one of the two ends", fg)
	}
}

func TestFocusNeverAnimatesTheContent(t *testing.T) {
	a, tb := twoPanesAnimated(t)
	a.config.Animations.Focus = "jitter"
	a.focusPane(tb, 2)
	drawnApp(a)
	buf := drawnApp(a)
	inside := contentRect(a.frameOfPane(tb, 2)).Min
	if c := buf.CellAt(inside.X, inside.Y); c == nil || c.Content != "x" {
		t.Errorf("the content shows %+v during the focus animation", c)
	}
}

func TestMovingAPaneSlidesItsFrame(t *testing.T) {
	a, _ := twoPanesAnimated(t)
	a.moveFocusedPane(layout.Right)
	drawnApp(a)
	if animationOf(a, momentMove) == nil {
		t.Fatalf("animations %v after moving a pane", a.animations)
	}
}

func TestFullscreenGrowsThePane(t *testing.T) {
	a, tb := twoPanesAnimated(t)
	before := a.frameOfPane(tb, 1)
	a.toggleFullscreen()
	drawnApp(a)
	if animationOf(a, momentFullscreen) == nil {
		t.Fatalf("animations %v after fullscreen", a.animations)
	}
	rewind(a, 30*time.Millisecond)
	buf := drawnApp(a)
	area := a.workspaceArea()
	if c := buf.CellAt(area.Max.X-1, area.Min.Y); c != nil && c.Content == "┐" && !image.Pt(area.Max.X-1, area.Min.Y).In(before) {
		t.Error("the whole tab shows at the start of fullscreen")
	}
}

func TestAPopupAppearsWithJitter(t *testing.T) {
	a, _ := animatedApp(t)
	a.openConfig()
	drawnApp(a)
	if animationOf(a, momentPopup) == nil {
		t.Fatalf("animations %v after opening a popup", a.animations)
	}
	buf := drawnApp(a)
	jittered := false
	for y := range a.size.Y {
		for x := range a.size.X {
			if c := buf.CellAt(x, y); c != nil && c.Content != "" && strings.ContainsRune(jitterRunes, []rune(c.Content)[0]) {
				jittered = true
			}
		}
	}
	if !jittered {
		t.Error("the popup shows no jitter at its start")
	}
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	if text := screenText(a); !strings.Contains(text, "Appearance") {
		t.Errorf("the popup did not settle on its text:\n%s", text)
	}
}

func TestEachMomentCanBeTurnedOff(t *testing.T) {
	a, tb := twoPanesAnimated(t)
	a.config.Animations.Focus = "none"
	a.focusPane(tb, 2)
	drawnApp(a)
	if animationOf(a, momentFocus) != nil {
		t.Error("focus animated with its effect off")
	}
}
