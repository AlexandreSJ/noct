package app

import (
	"image"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// animationRowBase is the first of the config's rows of animated moments, one per moment.
const animationRowBase configRow = 3000

const flickerWarning = "flicker flashes colors up to 3 times a second, which can trigger seizures in people with photosensitive epilepsy. use it?"

func animationRowAt(row configRow) (moment, bool) {
	m := moment(row - animationRowBase)
	return m, row >= animationRowBase && int(m) < len(moments)
}

// animationLines are the config's rows of the animations: on or off, then each moment's effect.
func (a *App) animationLines() []configLine {
	lines := []configLine{
		{label: "Animations", heading: true},
		{label: "animations", value: onOff(a.config.Appearance.Animations != config.AnimationsOff), row: animationsRow},
	}
	for m, info := range moments {
		lines = append(lines, configLine{label: info.label, value: "◂ " + a.config.Animations.Effect(info.key) + " ▸", row: animationRowBase + configRow(m)})
	}
	return lines
}

// changeAnimation steps a moment's effect; flicker asks first, since it can trigger seizures.
func (a *App) changeAnimation(m moment, step int) {
	key := moments[m].key
	i := 0
	for j, name := range config.Effects {
		if name == a.config.Animations.Effect(key) {
			i = j
		}
	}
	name := config.Effects[(i+step+len(config.Effects))%len(config.Effects)]
	use := func() {
		next := a.config
		next.Animations.SetEffect(key, name)
		a.applyConfig(next)
		a.mode = configMode
		a.playStagePreview(m)
	}
	if !effects[effectNamed(name)].warn {
		use()
		return
	}
	a.configPanel.preview = nil // nothing flashes before the warning is answered
	a.askToConfirm(flickerWarning, use)
	a.confirm.cancel = func() { a.mode = configMode }
}

// configShowsStage tells whether the config shows the stage beside its rows: on the animations'.
func (a *App) configShowsStage() bool {
	_, ok := animationRowAt(a.configPanel.selected)
	return a.configPanel.page == settingsPage && (ok || a.configPanel.selected == animationsRow)
}

// playStagePreview plays the moment's effect once on the stage, as long as it plays for real.
func (a *App) playStagePreview(m moment) {
	e := a.effectFor(m)
	if e == effectNone {
		a.configPanel.preview = nil
		return
	}
	a.configPanel.preview = &animation{moment: m, effect: e, start: a.now()}
	if m == momentActivity {
		a.configPanel.preview.duration = pulseDuration
	}
	a.stageTick = time.After(frameInterval)
}

func (a *App) stagePreviewRunning() bool {
	p := a.configPanel.preview
	return a.mode == configMode && p != nil && p.progress(a.now()) < 1
}

// tickStagePreview keeps the frames coming while the preview plays.
func (a *App) tickStagePreview() {
	if a.stagePreviewRunning() {
		a.stageTick = time.After(frameInterval)
	}
}

// stageGeometry is where the stage draws its parts in area: the tabs' names, the two panels and
// the popup in the left one.
func stageGeometry(area image.Rectangle) (active, inactive, left, right, popup image.Rectangle) {
	active = image.Rect(area.Min.X, area.Min.Y, area.Min.X+5, area.Min.Y+1)
	inactive = image.Rect(active.Max.X, area.Min.Y, active.Max.X+6, area.Min.Y+1)
	panes := image.Rect(area.Min.X, area.Min.Y+1, area.Max.X, area.Max.Y)
	left, right = panes, panes
	left.Max.X = panes.Min.X + panes.Dx()/2
	right.Min.X = left.Max.X
	popup = image.Rect(left.Min.X+2, left.Min.Y+4, left.Max.X-2, left.Min.Y+7)
	return active, inactive, left, right, popup
}

// drawStagePreview draws the stage in the theme on screen, with the preview playing over it, or
// a note that animations are off.
func (a *App) drawStagePreview(screen uv.Screen, area image.Rectangle) {
	if area.Dx() < stageMinWidth || area.Dy() < stageMinHeight {
		return
	}
	th := a.themeNamed(a.sessionTheme(a.session))
	p := newPalette(th, true)
	drawStage(screen, area, &p, theme.Background, false)
	if a.config.Appearance.Animations == config.AnimationsOff {
		drawText(screen, area.Min.X+2, area.Max.Y-1, area.Max.X, " animations off ", p.panelMuted)
		return
	}
	an := a.configPanel.preview
	if an == nil {
		return
	}
	at := a.now()
	progress := an.progress(at)
	_, inactive, left, right, popup := stageGeometry(area)
	play := func(where image.Rectangle, only func(x, y int) bool) {
		if an.effect == effectSpring && only == nil {
			a.reveal(screen, where, rectAt(centerOf(where), where, min(springAt(progress), 1)), nil)
			return
		}
		a.applyEffect(screen, an, where, progress, at, only)
	}
	switch an.moment {
	case momentPanels:
		play(right, nil)
	case momentTabs, momentActivity:
		play(inactive, nil)
	case momentHover:
		x := inactive.Min.X + int(float64(inactive.Dx()-1)*progress)
		setRune(screen, x, inactive.Max.Y, "▲", p.status)
		play(inactive, nil)
	case momentFocus:
		drawFrame(screen, left, "zsh", p.unfocusedFrame, p.unfocusedTitle)
		drawFrame(screen, right, "logs", p.focusedFrame, p.focusedTitle)
		focus := *an
		focus.fromColor = p.unfocusedFrame.Fg
		a.applyEffect(screen, &focus, right, progress, at, onBorder(right))
	case momentMove:
		if an.effect == effectSpring {
			drawFrame(screen, rectAt(right, left, springAt(progress)), "", p.focusedFrame, p.focusedTitle)
		} else {
			play(right, onBorder(right))
		}
	case momentFullscreen:
		panes := left.Union(right)
		if an.effect == effectSpring {
			a.reveal(screen, panes, rectAt(left, panes, min(springAt(progress), 1)), nil)
		} else {
			play(panes, nil)
		}
	case momentPopup:
		play(popup, nil)
	}
}
