package app

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"sync/atomic"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// Animations are only drawn: they never resize a program, and any key ends them. Before each
// frame, what changed since the last one (a pane opened or closed, the focus moved...) starts the
// animation of its moment, with the effect the config gives that moment.

// moment is something that happens on screen and can animate.
type moment int

const (
	momentHover moment = iota
	momentPanels
	momentTabs
	momentFocus
	momentMove
	momentFullscreen
	momentPopup
	momentActivity
)

// moments name each moment: key as in the config file, label as in the config panel.
var moments = []struct{ key, label string }{
	{"hover", "hover"}, {"panels", "open and close panels"}, {"tabs", "open and close tabs"},
	{"focus", "focus"}, {"move", "move a panel"}, {"fullscreen", "fullscreen"},
	{"popup", "popups"}, {"activity", "activity"},
}

// effect is how a moment animates; effects are in the order of config.Effects.
type effect int

const (
	effectNone effect = iota
	effect01
	effectFade
	effectJitter
)

// effects name each effect as the config file does.
var effects = []string{"none", "01", "fade", "jitter"}

func effectNamed(name string) effect {
	for i, e := range effects {
		if e == name {
			return effect(i)
		}
	}
	return effectNone
}

func (a *App) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

// effectFor is the effect the moment animates with, none while animations are off.
func (a *App) effectFor(m moment) effect {
	if a.config.Appearance.Animations == config.AnimationsOff {
		return effectNone
	}
	return effectNamed(a.config.Animations.Effect(moments[m].key))
}

// durationOf is how long the moment's animation plays, as the config has it.
func (a *App) durationOf(m moment) time.Duration {
	return time.Duration(a.config.AnimationTimes.Duration(moments[m].key)) * time.Millisecond
}

// newAnimation is the moment's animation starting at start over area, with the config's effect
// and time, or nil when the moment does not animate.
func (a *App) newAnimation(m moment, an animation, start time.Time) *animation {
	if an.effect = a.effectFor(m); an.effect == effectNone {
		return nil
	}
	an.moment, an.start, an.duration = m, start, a.durationOf(m)
	return &an
}

// animation is one moment playing over a part of the screen: its target, found again on each
// frame so the animation follows what it plays on when that moves or changes size.
type animation struct {
	moment moment
	effect effect
	start  time.Time
	pane   layout.PaneID
	target target
	// area is where a targetArea animation plays, such as what a closed pane showed.
	area image.Rectangle
	// closing plays backwards over cells, what the area showed before it went away.
	closing bool
	cells   uv.ScreenBuffer
	// frameOnly animates only the border of area, so what a program shows is never touched.
	frameOnly bool
	// fromColor is the color a fade starts from (nil for the background).
	fromColor color.Color
	seed      int64
	duration  time.Duration
}

func (an *animation) progress(at time.Time) float64 {
	if an.duration <= 0 {
		return 1
	}
	return clamp01(float64(at.Sub(an.start)) / float64(an.duration))
}

// shot is what the screen shows, as the animations compare it from one frame to the next.
type shot struct {
	session    *session
	tab        *tab
	size       image.Point
	frames     map[layout.PaneID]image.Rectangle
	focused    layout.PaneID
	fullscreen bool
	mode       mode
	tabs       int
}

func (a *App) takeShot() shot {
	s := shot{session: a.session, size: a.size, frames: map[layout.PaneID]image.Rectangle{}, mode: a.mode}
	if a.session != nil {
		s.tabs = len(a.session.tabs)
	}
	t := a.currentTab()
	if t == nil || a.showsHome() {
		return s
	}
	s.tab, s.focused, s.fullscreen = t, t.focused, t.fullscreen
	for _, p := range t.visible(a.workspaceArea()) {
		s.frames[p.id] = p.frame
	}
	return s
}

// startAnimations starts the animations of what changed from prev to now. A frame of another
// session or size has nothing to compare, and ends what runs.
func (a *App) startAnimations(prev, now shot, at time.Time) {
	if prev.session == nil {
		return
	}
	if prev.session != now.session || prev.size != now.size {
		a.finishAnimations()
		return
	}
	if prev.tab != now.tab {
		a.dropAnimations(momentFocus) // the panes they light are not on screen any more
	}
	a.startPopupAnimation(prev, now, at)
	if a.startTabAnimation(prev, now, at) || prev.tab == nil || prev.tab != now.tab {
		return
	}
	if prev.fullscreen != now.fullscreen {
		a.startAnimation(momentFullscreen, animation{pane: now.focused, area: a.workspaceArea()}, at)
		return
	}
	a.startPaneAnimations(prev, now, at)
}

// startAnimation plays the moment's animation from at, when the moment animates.
func (a *App) startAnimation(m moment, an animation, at time.Time) {
	if started := a.newAnimation(m, an, at); started != nil {
		a.addAnimation(started)
	}
}

// startPopupAnimation plays a popup that opened over the whole screen, or the one that closed
// backwards over its last cells.
func (a *App) startPopupAnimation(prev, now shot, at time.Time) {
	switch {
	case now.mode == prev.mode:
	case isPopupMode(now.mode):
		a.startAnimation(momentPopup, animation{pane: popupPane, target: target{kind: targetPopup}}, at)
	case isPopupMode(prev.mode) && !a.lastPopup.area.Empty():
		box := a.lastPopup.area
		a.startAnimation(momentPopup, animation{pane: popupPane, area: box, closing: true, cells: a.lastPopup.cells, seed: int64(box.Min.X)}, at)
	}
}

// startTabAnimation plays a tab that opened or closed, telling whether one did.
func (a *App) startTabAnimation(prev, now shot, at time.Time) bool {
	area := a.workspaceArea()
	switch {
	case now.tabs > prev.tabs:
		a.startAnimation(momentTabs, animation{pane: tabsPane, area: area}, at)
		a.startAnimation(momentTabs, animation{pane: tabBarPane, target: target{kind: targetTabName, tab: a.currentTab()}}, at)
		return true
	case now.tabs < prev.tabs && a.lastFrame != nil:
		a.startAnimation(momentTabs, animation{pane: tabsPane, area: area, closing: true, cells: a.snapshot(area)}, at)
		return true
	}
	return false
}

// startPaneAnimations plays what changed among the panes of a tab that stayed on screen: the
// focus, panes that opened or closed, and the focused pane moving.
func (a *App) startPaneAnimations(prev, now shot, at time.Time) {
	_, wasThere := prev.frames[now.focused]
	if _, ok := now.frames[now.focused]; ok && prev.focused != now.focused && wasThere {
		// only the pane that gets the focus lights up; the one it left keeps fading, as a trail
		a.startAnimation(momentFocus, animation{pane: now.focused, target: target{kind: targetPane}, frameOnly: true, fromColor: a.palette.unfocusedFrame.Fg}, at)
	}
	if prev.fullscreen || now.fullscreen {
		return // only the focused pane shows: panes come and go with the focus
	}
	opened := false
	for id := range now.frames {
		if _, ok := prev.frames[id]; !ok {
			a.startAnimation(momentPanels, animation{pane: id, target: target{kind: targetPane}, seed: int64(id)}, at)
			opened = true
		}
	}
	for id, frame := range prev.frames {
		if _, ok := now.frames[id]; !ok && a.lastFrame != nil {
			a.startAnimation(momentPanels, animation{pane: id, area: frame, seed: int64(id), closing: true, cells: a.snapshot(frame)}, at)
			opened = true
		}
	}
	before, after := prev.frames[now.focused], now.frames[now.focused]
	if !opened && wasThere && before != after && now.mode != resizeMode {
		a.startAnimation(momentMove, animation{pane: now.focused, target: target{kind: targetPane}, frameOnly: true, seed: int64(now.focused)}, at)
	}
}

// target is what an animation plays on.
type target struct {
	kind targetKind
	// tab is the tab whose name a targetTabName animation plays on.
	tab *tab
}

type targetKind int

const (
	targetArea      targetKind = iota // the animation's own area, which stays put
	targetPane                        // the frame of the animation's pane
	targetPaneTitle                   // the top row of that frame, where its title is
	targetTabName                     // the tab's name in the tab bar
	targetPopup                       // the popup the frame draws
)

// placeOf is where the animation plays on this frame, or false when its target is not on screen.
func (a *App) placeOf(an *animation) (image.Rectangle, bool) {
	switch an.target.kind {
	case targetPane, targetPaneTitle:
		frame, ok := a.lastShot.frames[an.pane]
		if an.target.kind == targetPaneTitle {
			frame.Max.Y = frame.Min.Y + 1
		}
		return frame, ok
	case targetTabName:
		return a.tabNameSpan(an.target.tab)
	case targetPopup:
		return a.popupArea, !a.popupArea.Empty()
	}
	return an.area, true
}

// tabNameSpan is where t's name is in the tab bar, when it shows there.
func (a *App) tabNameSpan(t *tab) (image.Rectangle, bool) {
	index := slices.Index(a.session.tabs, t)
	for _, span := range a.tabBar() {
		if span.tab == index && !span.mark {
			return image.Rect(span.start, 0, span.end, 1), true
		}
	}
	return image.Rectangle{}, false
}

// Animations that are not of one pane take these places of a pane id.
const (
	popupPane  layout.PaneID = -1
	tabsPane   layout.PaneID = -2
	tabBarPane layout.PaneID = -3
	hoverPane  layout.PaneID = -4
)

// hoverAt hovers what is under the mouse, a tab's name or a pane's title, once per visit,
// telling whether it started to.
func (a *App) hoverAt(point image.Point) bool {
	name, on := a.hoverTargetAt(point)
	if name == a.hoverTarget {
		return false
	}
	a.hoverTarget = name
	on.seed = int64(point.X)
	an := a.newAnimation(momentHover, on, a.now())
	if name == "" || an == nil {
		return false
	}
	a.addAnimation(an)
	return true
}

// hoverTargetAt names what is under the mouse, a tab's name or a pane's title, with the
// animation that plays on it; "" when it is neither.
func (a *App) hoverTargetAt(point image.Point) (string, animation) {
	if point.Y == 0 {
		for _, span := range a.tabBar() {
			if !span.mark && point.X >= span.start && point.X < span.end {
				return fmt.Sprintf("tab %d", span.tab), animation{pane: hoverPane, target: target{kind: targetTabName, tab: a.session.tabs[span.tab]}}
			}
		}
		return "", animation{}
	}
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		return "", animation{}
	}
	for _, p := range t.visible(a.workspaceArea()) {
		if point.Y == p.frame.Min.Y && point.X >= p.frame.Min.X && point.X < p.frame.Max.X {
			return fmt.Sprintf("pane %d", p.id), animation{pane: p.id, target: target{kind: targetPaneTitle}}
		}
	}
	return "", animation{}
}

// screenChangeOf is what a pane's program calls on new output: a frame, and, while the pane is
// out of sight, its activity marked once until it is seen.
func (a *App) screenChangeOf(id layout.PaneID) func() {
	if id == 0 {
		return a.notifyScreenChanged
	}
	marked := &atomic.Bool{}
	if a.activity == nil {
		a.activity = map[layout.PaneID]*atomic.Bool{}
	}
	a.activity[id] = marked
	return func() {
		a.notifyScreenChanged()
		if !marked.Swap(true) {
			a.onLoop(func() { a.markActivity(id) })
		}
	}
}

// markActivity pulses the pane's title, or its tab's name when the tab is not on screen, unless
// the pane is in focus on screen or already pulsed since it was last seen.
func (a *App) markActivity(id layout.PaneID) {
	t, _ := a.tabOfPane(id)
	if t == nil || a.unseen[id] {
		return
	}
	if t == a.currentTab() && t.focused == id {
		a.seen(id)
		return
	}
	if a.unseen == nil {
		a.unseen = map[layout.PaneID]bool{}
	}
	a.unseen[id] = true
	on := target{kind: targetTabName, tab: t}
	if _, visible := a.takeShot().frames[id]; visible && t == a.currentTab() {
		on = target{kind: targetPaneTitle}
	}
	if an := a.newAnimation(momentActivity, animation{pane: id, target: on}, a.now()); an != nil {
		a.addAnimation(an)
	}
}

// seen forgets a pane's activity: its next output out of sight pulses again.
func (a *App) seen(id layout.PaneID) {
	delete(a.unseen, id)
	if marked := a.activity[id]; marked != nil {
		marked.Store(false)
	}
}

// isPopupMode tells the modes that show a popup over the workspace.
func isPopupMode(m mode) bool {
	return m != normalMode && m != scrollMode
}

// addAnimation plays an, in place of what the same moment played on the same pane.
func (a *App) addAnimation(an *animation) {
	kept := a.animations[:0]
	for _, old := range a.animations {
		if old.moment != an.moment || old.pane != an.pane {
			kept = append(kept, old)
		}
	}
	a.animations = append(kept, an)
	if a.animTick == nil {
		a.animTick = time.After(frameInterval)
	}
}

// advanceAnimations drops the animations that ended, and keeps the frames coming while any runs.
func (a *App) advanceAnimations(at time.Time) {
	kept := a.animations[:0]
	for _, an := range a.animations {
		if an.progress(at) < 1 {
			kept = append(kept, an)
		}
	}
	a.animations, a.animTick = kept, nil
	if len(kept) > 0 {
		a.animTick = time.After(frameInterval)
	}
}

// finishAnimations ends every animation at once.
func (a *App) finishAnimations() {
	a.animations, a.animTick = nil, nil
}

// finishForKey ends the animations a key would wait for. The focus's only light borders, so
// they play on, and moving the focus quickly leaves a trail.
func (a *App) finishForKey() {
	a.animations = slices.DeleteFunc(a.animations, func(an *animation) bool { return an.moment != momentFocus })
	if len(a.animations) == 0 {
		a.animTick = nil
	}
}

// dropAnimations ends the animations of moment m.
func (a *App) dropAnimations(m moment) {
	a.animations = slices.DeleteFunc(a.animations, func(an *animation) bool { return an.moment == m })
}

// snapshot copies what the last frame showed in area.
func (a *App) snapshot(area image.Rectangle) uv.ScreenBuffer {
	return a.copyArea(a.lastFrame, area)
}

// copyArea copies the cells of area from screen, in a buffer of the screen's size.
func (a *App) copyArea(screen interface{ CellAt(x, y int) *uv.Cell }, area image.Rectangle) uv.ScreenBuffer {
	buf := uv.NewScreenBuffer(a.size.X, a.size.Y)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if c := screen.CellAt(x, y); c != nil {
				cell := *c
				buf.SetCell(x, y, &cell)
			}
		}
	}
	return buf
}

// recordFrame keeps what was drawn, for the panes and tabs that close from it, only while their
// closing animates.
func (a *App) recordFrame(screen uv.Screen) {
	if a.effectFor(momentPanels) == effectNone && a.effectFor(momentTabs) == effectNone {
		a.lastFrame = nil
		return
	}
	if a.lastFrame == nil || a.lastFrame.Bounds().Dx() != a.size.X || a.lastFrame.Bounds().Dy() != a.size.Y {
		buf := uv.NewScreenBuffer(a.size.X, a.size.Y)
		a.lastFrame = &buf
	}
	for y := range a.size.Y {
		for x := range a.size.X {
			if c := screen.CellAt(x, y); c != nil {
				cell := *c
				a.lastFrame.SetCell(x, y, &cell)
			}
		}
	}
}

// drawAnimations draws the animations running over what the frame drew, the popups' apart.
func (a *App) drawAnimations(screen uv.Screen, at time.Time) {
	for _, an := range a.animations {
		if an.moment == momentPopup && !an.closing {
			continue // drawn over the popup, once it is drawn
		}
		p := an.progress(at)
		area, ok := a.placeOf(an)
		switch {
		case !ok: // what it played on is not on screen any more
		case an.closing:
			a.drawClosing(screen, an, p, at)
		case an.moment == momentFocus:
			a.drawFocus(screen, an, area, p, at)
		case an.frameOnly:
			a.applyEffect(screen, an, area, p, at, onBorder(area))
		default:
			a.applyEffect(screen, an, area, p, at, nil)
		}
	}
}

// drawPopupAnimations plays the popups' opening animations on the popup the frame drew.
func (a *App) drawPopupAnimations(screen uv.Screen, at time.Time) {
	for _, an := range a.animations {
		if an.moment != momentPopup || an.closing {
			continue
		}
		if area, ok := a.placeOf(an); ok {
			a.applyEffect(screen, an, area, an.progress(at), at, nil)
		}
	}
}

// drawnPopup is where a popup was drawn, and its cells there.
type drawnPopup struct {
	area  image.Rectangle
	cells uv.ScreenBuffer
}

// drawOverlay draws the mode's popup and plays its opening over it. While a popup with an effect
// is open, its cells are kept for its closing to play backwards.
func (a *App) drawOverlay(screen uv.Screen, at time.Time) {
	a.popupArea, a.drawingPopup = image.Rectangle{}, true
	a.drawModeOverlay(screen)
	a.drawingPopup = false
	a.lastPopup = drawnPopup{}
	if isPopupMode(a.mode) && a.effectFor(momentPopup) != effectNone && !a.popupArea.Empty() {
		a.lastPopup = drawnPopup{area: a.popupArea, cells: a.copyArea(screen, a.popupArea)}
	}
	a.drawPopupAnimations(screen, at)
}

// onBorder tells the cells of area's border.
func onBorder(area image.Rectangle) func(x, y int) bool {
	return func(x, y int) bool {
		return x == area.Min.X || x == area.Max.X-1 || y == area.Min.Y || y == area.Max.Y-1
	}
}

// drawFocus lights the border of the pane, frame. A pane the focus left before its
// animation ended is a trail: what it lit fades back to its own color.
func (a *App) drawFocus(screen uv.Screen, an *animation, frame image.Rectangle, p float64, at time.Time) {
	focus := *an
	if an.pane != a.lastShot.focused {
		focus.fromColor = a.palette.focusedFrame.Fg
	}
	a.applyEffect(screen, &focus, frame, p, at, onBorder(frame))
}

// drawClosing draws what a closed pane showed, going away: its effect played backwards.
func (a *App) drawClosing(screen uv.Screen, an *animation, p float64, at time.Time) {
	for y := an.area.Min.Y; y < an.area.Max.Y; y++ {
		for x := an.area.Min.X; x < an.area.Max.X; x++ {
			if c := an.cells.CellAt(x, y); c != nil {
				cell := *c
				screen.SetCell(x, y, &cell)
			}
		}
	}
	a.applyEffect(screen, an, an.area, 1-p, at, nil)
}

// applyEffect changes the cells of area that only lets through (nil for all) as the effect looks
// at progress p.
func (a *App) applyEffect(screen uv.Screen, an *animation, area image.Rectangle, p float64, at time.Time, only func(x, y int) bool) {
	from := an.fromColor
	if from == nil {
		from = a.palette.background
	}
	if from == nil {
		from = a.shownTheme(a.sessionTheme(a.session)).Colors[theme.Background].Resolve()
	}
	frame := int64(at.Sub(an.start) / frameInterval) // characters change from frame to frame
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			c := screen.CellAt(x, y)
			if c == nil || c.Width != 1 || only != nil && !only(x, y) {
				continue
			}
			cell := *c
			runes := []rune(cell.Content)
			switch an.effect {
			case effectFade:
				f := easeOut(p)
				if an.moment == momentActivity {
					f = 1 - 0.7*math.Sin(math.Pi*p) // dims and comes back: a pulse
				}
				if cell.Style.Fg != nil {
					cell.Style.Fg = mix(from, cell.Style.Fg, f)
				}
			case effectJitter:
				if len(runes) == 1 {
					cell.Content = string(jitterAt(runes[0], p, 0.2+0.8*noise(x, y, an.seed), int64(noise(x, y, frame)*1000)))
				}
			case effect01:
				reveal := 0.7*float64(x-area.Min.X)/float64(max(area.Dx(), 1)) + 0.3*noise(x, y, an.seed)
				if len(runes) == 1 {
					cell.Content = string(binaryAt(runes[0], p, reveal, int64(noise(x, y, frame)*2)))
				}
			}
			screen.SetCell(x, y, &cell)
		}
	}
}
