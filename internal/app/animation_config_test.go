package app

import (
	"image"
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
)

var spacePress = uv.KeyPressEvent{Code: uv.KeySpace, Text: " "}

// animationConfig is the config open on the row of moment m, with animations on and time frozen.
func animationConfig(t *testing.T, m moment) *App {
	t.Helper()
	a := testApp(t, sideBySideTab(1, 2))
	a.config.Appearance.Animations = config.AnimationsOn
	frozen := time.Now()
	a.clock = func() time.Time { return frozen }
	a.size.Y = 50
	a.openConfig()
	a.selectConfigRow(animationRowBase + configRow(m))
	return a
}

func TestTheConfigListsEveryMomentWithItsEffect(t *testing.T) {
	a := animationConfig(t, momentPanels)
	for i, m := range moments {
		a.selectConfigRow(animationRowBase + configRow(i))
		if !strings.Contains(screenText(a), m.label) {
			t.Errorf("the config does not list %q", m.label)
		}
	}
	a.selectConfigRow(animationRowBase + configRow(momentPanels))
	press(a, rightPress)
	if a.config.Animations.Panels != "jitter" {
		t.Fatalf("panels animate with %q after right, want jitter", a.config.Animations.Panels)
	}
	if saved, err := config.Load(a.configPath); err != nil || saved.Animations.Panels != "jitter" {
		t.Errorf("saved %q, %v", saved.Animations.Panels, err)
	}
}

func TestThePreviewPlaysOnceOnArrivingAndChanging(t *testing.T) {
	a := animationConfig(t, momentPopup)
	first := a.configPanel.preview
	if first == nil || first.moment != momentPopup {
		t.Fatal("arriving on the row did not play its preview")
	}
	first.start = first.start.Add(-time.Second)
	a.tickStagePreview()
	if a.stagePreviewRunning() {
		t.Error("the preview plays again after its end")
	}
	press(a, spacePress)
	if a.configPanel.preview == first || !a.stagePreviewRunning() {
		t.Error("space did not play the preview again")
	}
	a.selectConfigRow(animationRowBase + configRow(momentPanels))
	before := a.configPanel.preview
	press(a, rightPress)
	if a.configPanel.preview == before || a.configPanel.preview.effect != effectJitter {
		t.Error("changing the effect did not play it")
	}
}

func TestThePreviewUsesTheRealDuration(t *testing.T) {
	a := animationConfig(t, momentPanels)
	p := a.configPanel.preview
	if p.progress(p.start.Add(250*time.Millisecond)) >= 1 || p.progress(p.start.Add(300*time.Millisecond)) < 1 {
		t.Error("the panels' preview does not last their 300 ms")
	}
}

func TestWithAnimationsOffTheStageIsStill(t *testing.T) {
	a := animationConfig(t, momentPanels)
	a.config.Appearance.Animations = config.AnimationsOff
	a.selectConfigRow(animationRowBase + configRow(momentFocus))
	if a.configPanel.preview != nil {
		t.Error("a preview plays with animations off")
	}
	if !strings.Contains(screenText(a), "animations off") {
		t.Error("the stage does not say animations are off")
	}
}

func TestEveryPreviewDraws(t *testing.T) {
	for m := range moments {
		for e := range effects {
			a := animationConfig(t, moment(m))
			a.configPanel.preview = &animation{moment: moment(m), effect: effect(e), start: a.now().Add(-50 * time.Millisecond)}
			screenText(a) // no panic, whatever the moment and effect
		}
	}
}

func TestEachMomentsTimeIsSetInTheConfig(t *testing.T) {
	a := animationConfig(t, momentFocus)
	a.selectConfigRow(animationTimeRowBase + configRow(momentFocus))
	if text := screenText(a); !strings.Contains(text, "150 ms") {
		t.Errorf("the config does not show focus's 150 ms:\n%s", text)
	}
	press(a, rightPress)
	if a.config.AnimationTimes.Focus != 200 {
		t.Fatalf("focus lasts %d ms after right, want 200", a.config.AnimationTimes.Focus)
	}
	if p := a.configPanel.preview; p == nil || p.progress(p.start.Add(190*time.Millisecond)) >= 1 {
		t.Error("changing the time did not play the preview with it")
	}
	typeInto(a, "450")
	press(a, enterPress)
	if a.config.AnimationTimes.Focus != 450 {
		t.Fatalf("focus lasts %d ms after typing 450", a.config.AnimationTimes.Focus)
	}
	if saved, err := config.Load(a.configPath); err != nil || saved.AnimationTimes.Focus != 450 {
		t.Errorf("saved %d, %v", saved.AnimationTimes.Focus, err)
	}
	typeInto(a, "9")
	press(a, enterPress)
	if a.config.AnimationTimes.Focus != 450 || a.configPanel.numberProblem == "" {
		t.Errorf("9 ms was taken as %d, problem %q", a.config.AnimationTimes.Focus, a.configPanel.numberProblem)
	}
}

// drawnCell is where text first shows on screen at or after row from, as it was drawn.
func drawnCell(t *testing.T, a *App, text string, from int) image.Point {
	t.Helper()
	lines := strings.Split(screenText(a), "\n")
	for y := from; y < len(lines); y++ {
		if x := strings.Index(lines[y], text); x >= 0 {
			return image.Pt(len([]rune(lines[y][:x])), y)
		}
	}
	t.Fatalf("%q is not on screen:\n%s", text, strings.Join(lines, "\n"))
	return image.Point{}
}

func TestTheStageIsUnderTheRowsWithItsNote(t *testing.T) {
	a := animationConfig(t, momentPanels)
	row := drawnCell(t, a, "open and close panels", 0)
	note := drawnCell(t, a, "example · space repeats it", 0)
	if note.Y <= row.Y {
		t.Errorf("the stage's note is on row %d, above the rows' %d", note.Y, row.Y)
	}
	hints := drawnCell(t, a, "esc close", 0)
	if hints.Y <= note.Y {
		t.Errorf("the hints are on row %d, above the stage on %d", hints.Y, note.Y)
	}
}

func TestClickingTheArrowsDrawnChangesTheEffect(t *testing.T) {
	a := animationConfig(t, momentPanels)
	row := drawnCell(t, a, "open and close panels", 0)
	next := drawnCell(t, a, "▸", row.Y)
	if next.Y != row.Y {
		t.Fatalf("the row's ▸ is not on its line")
	}
	click(a, next.X, next.Y)
	if a.config.Animations.Panels != "jitter" {
		t.Fatalf("panels on %q after clicking ▸, want jitter", a.config.Animations.Panels)
	}
	back := drawnCell(t, a, "◂", row.Y)
	click(a, back.X, back.Y)
	if a.config.Animations.Panels != "fade" {
		t.Errorf("panels on %q after clicking ◂, want fade back", a.config.Animations.Panels)
	}
}

func TestAShortTerminalKeepsTheRowsWithoutTheStage(t *testing.T) {
	a := animationConfig(t, momentPanels)
	a.size.Y = 24
	if text := screenText(a); strings.Contains(text, "example ·") {
		t.Errorf("the stage's note shows where the stage does not fit:\n%s", text)
	}
}
