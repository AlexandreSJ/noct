package app

import (
	"image"
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
	effectSpring
	effectFade
	effectJitter
	effectFlicker
)

type effectInfo struct {
	name     string
	duration time.Duration
	// warn asks before the effect is used: flicker can trigger seizures.
	warn bool
}

var effects = []effectInfo{
	{"none", 0, false}, {"01", 400 * time.Millisecond, false}, {"spring", 250 * time.Millisecond, false},
	{"fade", 150 * time.Millisecond, false}, {"jitter", 300 * time.Millisecond, false}, {"flicker", time.Second, true},
}

func effectNamed(name string) effect {
	for i, e := range effects {
		if e.name == name {
			return effect(i)
		}
	}
	return effectNone
}

// effectFor is the effect the moment animates with, none while animations are off.
func (a *App) effectFor(m moment) effect {
	if a.config.Appearance.Animations == config.AnimationsOff {
		return effectNone
	}
	return effectNamed(a.config.Animations.Effect(moments[m].key))
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
	seed    int64
}

func (an *animation) progress(at time.Time) float64 {
	d := effects[an.effect].duration
	if d <= 0 {
		return 1
	}
	return clamp01(float64(at.Sub(an.start)) / float64(d))
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
	if prev.tab == nil || prev.tab != now.tab || prev.fullscreen || now.fullscreen {
		return
	}
	if e := a.effectFor(momentPanels); e != effectNone {
		for id, frame := range now.frames {
			if _, ok := prev.frames[id]; !ok {
				a.addAnimation(&animation{moment: momentPanels, effect: e, start: at, pane: id, area: frame, seed: int64(id)})
			}
		}
		for id, frame := range prev.frames {
			if _, ok := now.frames[id]; !ok && a.lastFrame != nil {
				a.addAnimation(&animation{moment: momentPanels, effect: e, start: at, pane: id, area: frame, seed: int64(id), closing: true, cells: a.snapshot(frame)})
			}
		}
	}
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

// finishAnimations ends every animation at once, as a key does.
func (a *App) finishAnimations() {
	a.animations, a.animTick = nil, nil
}

// snapshot copies what the last frame showed in area.
func (a *App) snapshot(area image.Rectangle) uv.ScreenBuffer {
	buf := uv.NewScreenBuffer(a.size.X, a.size.Y)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if c := a.lastFrame.CellAt(x, y); c != nil {
				cell := *c
				buf.SetCell(x, y, &cell)
			}
		}
	}
	return buf
}

// recordFrame keeps what was drawn, for the panes that close from it.
func (a *App) recordFrame(screen uv.Screen) {
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

// drawAnimations draws the animations running over what the frame drew.
func (a *App) drawAnimations(screen uv.Screen, at time.Time) {
	for _, an := range a.animations {
		p := an.progress(at)
		if an.closing {
			a.drawClosing(screen, an, p, at)
			continue
		}
		if an.effect == effectSpring {
			a.reveal(screen, an.area, rectAt(centerOf(an.area), an.area, min(springAt(p), 1)))
			continue
		}
		a.applyEffect(screen, an, an.area, p, at)
	}
}

// drawClosing draws what a closed pane showed, going away: shrinking with the spring, or the
// effect played backwards.
func (a *App) drawClosing(screen uv.Screen, an *animation, p float64, at time.Time) {
	shown := an.area
	if an.effect == effectSpring {
		shown = rectAt(centerOf(an.area), an.area, clamp01(1-springAt(p)))
	}
	for y := shown.Min.Y; y < shown.Max.Y; y++ {
		for x := shown.Min.X; x < shown.Max.X; x++ {
			if c := an.cells.CellAt(x, y); c != nil {
				cell := *c
				screen.SetCell(x, y, &cell)
			}
		}
	}
	if an.effect != effectSpring {
		a.applyEffect(screen, an, shown, 1-p, at)
	}
}

// reveal blanks the cells of area outside shown.
func (a *App) reveal(screen uv.Screen, area, shown image.Rectangle) {
	blank := uv.Cell{Content: " ", Width: 1, Style: uv.Style{Bg: a.palette.background}}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if !image.Pt(x, y).In(shown) {
				cell := blank
				screen.SetCell(x, y, &cell)
			}
		}
	}
}

// applyEffect changes the cells of area as the effect looks at progress p.
func (a *App) applyEffect(screen uv.Screen, an *animation, area image.Rectangle, p float64, at time.Time) {
	background := a.palette.background
	if background == nil {
		background = a.themeNamed(a.sessionTheme(a.session)).Colors[theme.Background].Resolve()
	}
	frame := int64(at.Sub(an.start) / frameInterval) // characters change from frame to frame
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			c := screen.CellAt(x, y)
			if c == nil || c.Width != 1 {
				continue
			}
			cell := *c
			runes := []rune(cell.Content)
			switch an.effect {
			case effectFade:
				if cell.Style.Fg != nil {
					cell.Style.Fg = mix(background, cell.Style.Fg, easeOut(p))
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
			case effectFlicker:
				if flickerOn(at.Sub(an.start)) {
					cell.Style.Attrs ^= uv.AttrReverse
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
