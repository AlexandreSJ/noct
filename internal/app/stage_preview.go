package app

import (
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// animationRowBase is the first of the config's rows of animated moments' effects, one per
// moment, and animationTimeRowBase the first of their times.
const (
	animationRowBase     configRow = 3000
	animationTimeRowBase configRow = 3100
)

// animationTimeStep is how much the arrows change an animation's time.
const animationTimeStep = 50

func animationRowAt(row configRow) (moment, bool) {
	m := moment(row - animationRowBase)
	return m, row >= animationRowBase && int(m) < len(moments)
}

func animationTimeRowAt(row configRow) (moment, bool) {
	m := moment(row - animationTimeRowBase)
	return m, row >= animationTimeRowBase && int(m) < len(moments)
}

// previewedMoment is the moment of the selected row of the animations, effect or time.
func previewedMoment(row configRow) (moment, bool) {
	if m, ok := animationRowAt(row); ok {
		return m, true
	}
	return animationTimeRowAt(row)
}

// animationLines are the config's rows of the animations: on or off, then each moment's effect
// and, under it, its time.
func (a *App) animationLines() []configLine {
	lines := []configLine{
		{label: "Animations", heading: true},
		{label: "animations", value: onOff(a.config.Appearance.Animations != config.AnimationsOff), row: animationsRow},
	}
	for m, info := range moments {
		lines = append(lines, configLine{label: info.label, value: "◂ " + a.config.Animations.Effect(info.key) + " ▸", row: animationRowBase + configRow(m)})
		ms := fmt.Sprintf("%d ms", a.config.AnimationTimes.Duration(info.key))
		lines = append(lines, a.numberLines("  time", animationTimeRowBase+configRow(m), ms)...)
	}
	return lines
}

// changeAnimation steps a moment's effect.
func (a *App) changeAnimation(m moment, step int) {
	key := moments[m].key
	next := a.config
	next.Animations.SetEffect(key, cycle(config.Effects, a.config.Animations.Effect(key), step))
	a.applyConfig(next)
	a.playStagePreview(m)
}

// changeAnimationTime steps a moment's time within its bounds.
func (a *App) changeAnimationTime(m moment, step int) {
	a.setAnimationTime(m, a.config.AnimationTimes.Duration(moments[m].key)+step*animationTimeStep)
}

func (a *App) setAnimationTime(m moment, ms int) {
	next := a.config
	next.AnimationTimes.SetDuration(moments[m].key, min(max(ms, config.MinAnimationMs), config.MaxAnimationMs))
	a.applyConfig(next)
	a.playStagePreview(m)
}

var animationTimePattern = regexp.MustCompile(`^(\d+)\s*(ms)?$`)

// parseAnimationTime reads "150" or "150 ms".
func parseAnimationTime(text string) (int, error) {
	match := animationTimePattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(text)))
	ms := 0
	if match != nil {
		ms, _ = strconv.Atoi(match[1])
	}
	if ms < config.MinAnimationMs || ms > config.MaxAnimationMs {
		return 0, fmt.Errorf("a time goes from %d to %d ms", config.MinAnimationMs, config.MaxAnimationMs)
	}
	return ms, nil
}

// configShowsStage tells whether the config shows the stage beside its rows: on the animations'.
func (a *App) configShowsStage() bool {
	_, ok := previewedMoment(a.configPanel.selected)
	return a.configPanel.page == settingsPage && (ok || a.configPanel.selected == animationsRow)
}

// playStagePreview plays the moment's effect once on the stage, as long as it plays for real.
func (a *App) playStagePreview(m moment) {
	a.configPanel.preview = a.newAnimation(m, animation{}, a.now())
	if a.configPanel.preview != nil {
		a.stageTick = time.After(frameInterval)
	}
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
		play(right, onBorder(right))
	case momentFullscreen:
		play(left.Union(right), nil)
	case momentPopup:
		play(popup, nil)
	}
}
