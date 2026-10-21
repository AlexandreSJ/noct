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

// animation is one moment playing over an area of the screen.
type animation struct {
	moment moment
	effect effect
	start  time.Time
	pane   layout.PaneID
	area   image.Rectangle
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
	start := func(m moment, an animation) {
		if started := a.newAnimation(m, an, at); started != nil {
			a.addAnimation(started)
		}
	}
	switch {
	case now.mode == prev.mode:
	case isPopupMode(now.mode):
		start(momentPopup, animation{pane: popupPane, area: image.Rect(0, 0, a.size.X, a.size.Y)})
	case isPopupMode(prev.mode) && !a.lastPopup.area.Empty():
		box := a.lastPopup.area
		start(momentPopup, animation{pane: popupPane, area: box, closing: true, cells: a.lastPopup.cells, seed: int64(box.Min.X)})
	}
	area := a.workspaceArea()
	switch {
	case now.tabs > prev.tabs:
		start(momentTabs, animation{pane: tabsPane, area: area})
		if span, ok := a.activeTabSpan(); ok {
			start(momentTabs, animation{pane: tabBarPane, area: span})
		}
		return
	case now.tabs < prev.tabs && a.lastFrame != nil:
		start(momentTabs, animation{pane: tabsPane, area: area, closing: true, cells: a.snapshot(area)})
		return
	}
	if prev.tab == nil || prev.tab != now.tab {
		return
	}
	if prev.fullscreen != now.fullscreen {
		start(momentFullscreen, animation{pane: now.focused, area: area})
		return
	}
	_, wasThere := prev.frames[now.focused]
	if prev.focused != now.focused && wasThere {
		// only the pane that gets the focus lights up; the one it left keeps fading, as a trail
		if frame, ok := now.frames[now.focused]; ok {
			start(momentFocus, animation{pane: now.focused, area: frame, frameOnly: true, fromColor: a.palette.unfocusedFrame.Fg})
		}
	}
	if prev.fullscreen || now.fullscreen {
		return // only the focused pane shows: panes come and go with the focus
	}
	opened := false
	for id, frame := range now.frames {
		if _, ok := prev.frames[id]; !ok {
			start(momentPanels, animation{pane: id, area: frame, seed: int64(id)})
			opened = true
		}
	}
	for id, frame := range prev.frames {
		if _, ok := now.frames[id]; !ok && a.lastFrame != nil {
			start(momentPanels, animation{pane: id, area: frame, seed: int64(id), closing: true, cells: a.snapshot(frame)})
			opened = true
		}
	}
	before, after := prev.frames[now.focused], now.frames[now.focused]
	if !opened && wasThere && before != after && now.mode != resizeMode {
		start(momentMove, animation{pane: now.focused, area: after, frameOnly: true, seed: int64(now.focused)})
	}
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
	target, area := a.hoverTargetAt(point)
	if target == a.hoverTarget {
		return false
	}
	a.hoverTarget = target
	an := a.newAnimation(momentHover, animation{pane: hoverPane, area: area, seed: int64(point.X)}, a.now())
	if target == "" || an == nil {
		return false
	}
	a.addAnimation(an)
	return true
}

func (a *App) hoverTargetAt(point image.Point) (string, image.Rectangle) {
	if point.Y == 0 {
		for _, span := range a.tabBar() {
			if !span.mark && point.X >= span.start && point.X < span.end {
				return fmt.Sprintf("tab %d", span.tab), image.Rect(span.start, 0, span.end, 1)
			}
		}
		return "", image.Rectangle{}
	}
	t := a.currentTab()
	if t == nil || t.isEmpty() {
		return "", image.Rectangle{}
	}
	for _, p := range t.visible(a.workspaceArea()) {
		if point.Y == p.frame.Min.Y && point.X >= p.frame.Min.X && point.X < p.frame.Max.X {
			return fmt.Sprintf("pane %d", p.id), image.Rect(p.frame.Min.X, p.frame.Min.Y, p.frame.Max.X, p.frame.Min.Y+1)
		}
	}
	return "", image.Rectangle{}
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
	area, ok := image.Rectangle{}, false
	if frame, visible := a.takeShot().frames[id]; visible && t == a.currentTab() {
		area, ok = image.Rect(frame.Min.X, frame.Min.Y, frame.Max.X, frame.Min.Y+1), true
	}
	for i := range a.session.tabs {
		if !ok && a.session.tabs[i] == t {
			for _, span := range a.tabBar() {
				if span.tab == i && !span.mark {
					area, ok = image.Rect(span.start, 0, span.end, 1), true
				}
			}
		}
	}
	if an := a.newAnimation(momentActivity, animation{pane: id, area: area}, a.now()); ok && an != nil {
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

// activeTabSpan is where the active tab's name is in the tab bar.
func (a *App) activeTabSpan() (image.Rectangle, bool) {
	for _, span := range a.tabBar() {
		if span.tab == a.session.activeTab && !span.mark {
			return image.Rect(span.start, 0, span.end, 1), true
		}
	}
	return image.Rectangle{}, false
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
		switch {
		case an.closing:
			a.drawClosing(screen, an, p, at)
		case an.moment == momentFocus:
			a.drawFocus(screen, an, p, at)
		case an.frameOnly:
			a.applyEffect(screen, an, an.area, p, at, onBorder(an.area))
		default:
			a.applyEffect(screen, an, an.area, p, at, nil)
		}
	}
}

// drawPopupAnimations plays the popups' animations on the cells the popup drew over before.
func (a *App) drawPopupAnimations(screen uv.Screen, before uv.ScreenBuffer, at time.Time) {
	drawn := func(x, y int) bool {
		was, is := before.CellAt(x, y), screen.CellAt(x, y)
		return was != nil && is != nil && (was.Content != is.Content || !was.Style.Equal(&is.Style))
	}
	for _, an := range a.animations {
		if an.moment != momentPopup || an.closing {
			continue
		}
		a.applyEffect(screen, an, an.area, an.progress(at), at, drawn)
	}
}

// drawnPopup is where a popup was drawn, and its cells there.
type drawnPopup struct {
	area  image.Rectangle
	cells uv.ScreenBuffer
}

// drawOverlay draws the mode's popup, comparing it with the frame under it only when an
// animation needs that: one opening plays on the cells the popup drew, and one closing on the
// cells it last had.
func (a *App) drawOverlay(screen uv.Screen, at time.Time) {
	opening := a.hasPopupAnimation()
	closable := isPopupMode(a.mode) && a.effectFor(momentPopup) != effectNone
	a.lastPopup = drawnPopup{}
	if !opening && !closable {
		a.drawModeOverlay(screen)
		return
	}
	before := a.copyOf(screen)
	a.drawModeOverlay(screen)
	if closable {
		a.lastPopup = a.popupDrawn(screen, before)
	}
	if opening {
		a.drawPopupAnimations(screen, before, at)
	}
}

// popupDrawn is the part of the screen the popup changed from before, with its cells.
func (a *App) popupDrawn(screen uv.Screen, before uv.ScreenBuffer) drawnPopup {
	box := image.Rectangle{}
	for y := range a.size.Y {
		for x := range a.size.X {
			was, is := before.CellAt(x, y), screen.CellAt(x, y)
			if was != nil && is != nil && (was.Content != is.Content || !was.Style.Equal(&is.Style)) {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return drawnPopup{area: box, cells: a.copyArea(screen, box)}
}

// hasPopupAnimation tells whether a popup's opening animation runs, which needs the frame before
// the popup.
func (a *App) hasPopupAnimation() bool {
	for _, an := range a.animations {
		if an.moment == momentPopup && !an.closing {
			return true
		}
	}
	return false
}

// copyOf is the screen's cells as they are now.
func (a *App) copyOf(screen uv.Screen) uv.ScreenBuffer {
	return a.copyArea(screen, image.Rect(0, 0, a.size.X, a.size.Y))
}

// onBorder tells the cells of area's border.
func onBorder(area image.Rectangle) func(x, y int) bool {
	return func(x, y int) bool {
		return x == area.Min.X || x == area.Max.X-1 || y == area.Min.Y || y == area.Max.Y-1
	}
}

// drawFocus lights the border of the pane where it is now. A pane the focus left before its
// animation ended is a trail: what it lit fades back to its own color.
func (a *App) drawFocus(screen uv.Screen, an *animation, p float64, at time.Time) {
	frame, ok := a.lastShot.frames[an.pane]
	if !ok {
		return
	}
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
		from = a.themeNamed(a.sessionTheme(a.session)).Colors[theme.Background].Resolve()
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

func centerOf(r image.Rectangle) image.Rectangle {
	c := image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
	return image.Rectangle{Min: c, Max: c}
}
