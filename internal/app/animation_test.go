package app

import (
	"image"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

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

// shyMouseView is a program that turned the mouse on but wants none of it here.
type shyMouseView struct {
	cellView
	got []uv.MouseEvent
}

func (v *shyMouseView) WantsMouseAt(image.Point) bool { return false }
func (v *shyMouseView) SendMouse(e uv.MouseEvent)     { v.got = append(v.got, e) }

func motion(x, y int) uv.MouseMotionEvent { return uv.MouseMotionEvent{X: x, Y: y} }

func TestHoverAnimatesATabNameOncePerVisit(t *testing.T) {
	a, _ := animatedApp(t)
	a.session.tabs = append(a.session.tabs, newTab(5, &cellView{}))
	drawnApp(a)
	var second image.Rectangle
	for _, span := range a.tabBar() {
		if span.tab == 1 && !span.mark {
			second = image.Rect(span.start, 0, span.end, 1)
		}
	}
	a.handleMouse(motion(second.Min.X+1, 0))
	first := animationOf(a, momentHover)
	if first == nil || first.area != second {
		t.Fatalf("hovering the tab gave %v", a.animations)
	}
	a.handleMouse(motion(second.Min.X+2, 0))
	if animationOf(a, momentHover) != first {
		t.Error("moving inside the tab started the hover again")
	}
	a.handleMouse(motion(a.size.X/2, a.size.Y/2))
	a.handleMouse(motion(second.Min.X+1, 0))
	if animationOf(a, momentHover) == first {
		t.Error("coming back to the tab did not hover it again")
	}
}

func TestHoverNeedsTheMotionModeOnlyWhenOn(t *testing.T) {
	a, _ := animatedApp(t)
	if a.mouseMode() != uv.MouseModeMotion {
		t.Errorf("with hover on, the mouse mode is %v", a.mouseMode())
	}
	a.config.Animations.Hover = "none"
	if a.mouseMode() != uv.MouseModeDrag {
		t.Errorf("with hover off, the mouse mode is %v", a.mouseMode())
	}
}

func TestProgramsGetMotionOnlyIfTheyAskedForIt(t *testing.T) {
	a, tb := animatedApp(t)
	v := &shyMouseView{}
	tb.views[1] = v
	inside := contentRect(a.frameOfPane(tb, 1)).Min
	a.handleMouse(motion(inside.X+2, inside.Y+2))
	if len(v.got) != 0 {
		t.Errorf("the program got %v without asking", v.got)
	}
}

func TestActivityPulsesOnceUntilSeen(t *testing.T) {
	a, tb := twoPanesAnimated(t)
	a.markActivity(2)
	first := animationOf(a, momentActivity)
	if first == nil {
		t.Fatal("output in a pane out of focus did not pulse")
	}
	for range 5 {
		a.markActivity(2)
	}
	if animationOf(a, momentActivity) != first {
		t.Error("more output pulsed again before the pane was seen")
	}
	a.focusPane(tb, 2)
	drawnApp(a)
	a.focusPane(tb, 1)
	drawnApp(a)
	a.markActivity(2)
	if an := animationOf(a, momentActivity); an == nil || an == first {
		t.Error("output after the pane was seen did not pulse")
	}
}

func TestActivityOfTheFocusedPaneIsIgnored(t *testing.T) {
	a, _ := twoPanesAnimated(t)
	a.markActivity(1)
	if animationOf(a, momentActivity) != nil {
		t.Error("the focused pane pulsed")
	}
}

func TestClosingAPopupPlaysItBackwards(t *testing.T) {
	a, _ := animatedApp(t)
	a.openConfig()
	drawnApp(a)
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	drawnApp(a)
	a.closeConfig()
	drawnApp(a)
	closing := animationOf(a, momentPopup)
	if closing == nil || !closing.closing {
		t.Fatalf("animations %v after closing a popup", a.animations)
	}
	if text := screenText(a); !strings.ContainsAny(text, jitterRunes) && !strings.Contains(text, "Appearance") {
		t.Errorf("the closing popup does not show:\n%s", text)
	}
	rewind(a, time.Second)
	a.advanceAnimations(a.now())
	if text := screenText(a); strings.Contains(text, "Appearance") {
		t.Errorf("the popup stays after its animation:\n%s", text)
	}
}

func TestTheFrameIsKeptOnlyWhenAnimationsNeedIt(t *testing.T) {
	a, _ := animatedApp(t)
	if a.lastFrame == nil {
		t.Fatal("the last frame is not kept, with panes and tabs animating as they close")
	}
	a.config.Appearance.Animations = config.AnimationsOff
	drawnApp(a)
	if a.lastFrame != nil {
		t.Error("the last frame is copied with animations off")
	}
}

func TestAMotionThatChangesNothingNeedsNoFrame(t *testing.T) {
	a, _ := animatedApp(t)
	middle := motion(a.size.X/2, a.size.Y/2)
	a.handleMouse(middle)
	if !a.quietMotion {
		t.Error("a motion over a pane's content asks for a frame")
	}
	a.handleMouse(motion(1, 0))
	if a.quietMotion {
		t.Error("hovering a tab's name asks for no frame")
	}
}

func TestAKeyAfterAQuietMotionGetsAFrame(t *testing.T) {
	a, _ := animatedApp(t)
	a.handleMouse(motion(a.size.X/2, a.size.Y/2))
	if err := a.handleEvent(letter('x')); err != nil {
		t.Fatal(err)
	}
	if a.quietMotion {
		t.Error("a key after a motion over nothing would get no frame")
	}
}
