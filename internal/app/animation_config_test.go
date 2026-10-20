package app

import (
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
	text := screenText(a)
	for _, m := range moments {
		if !strings.Contains(text, m.label) {
			t.Errorf("the config does not list %q", m.label)
		}
	}
	press(a, rightPress)
	if a.config.Animations.Panels != "fade" {
		t.Fatalf("panels animate with %q after right, want fade", a.config.Animations.Panels)
	}
	if saved, err := config.Load(a.configPath); err != nil || saved.Animations.Panels != "fade" {
		t.Errorf("saved %q, %v", saved.Animations.Panels, err)
	}
}

func TestFlickerAsksBeforeItIsUsed(t *testing.T) {
	a := animationConfig(t, momentPopup)
	press(a, rightPress)
	if a.mode != confirmMode || !strings.Contains(a.confirm.question, "epilepsy") {
		t.Fatalf("mode %v, question %q", a.mode, a.confirm.question)
	}
	if a.configPanel.preview != nil {
		t.Error("the flicker plays before the warning is answered")
	}
	press(a, escPress)
	if a.mode != configMode || a.config.Animations.Popup != "jitter" {
		t.Fatalf("esc left mode %v and popups on %q", a.mode, a.config.Animations.Popup)
	}
	press(a, rightPress, enterPress)
	if a.config.Animations.Popup != "flicker" || a.mode != configMode {
		t.Errorf("enter left popups on %q, mode %v", a.config.Animations.Popup, a.mode)
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
	if a.configPanel.preview == before || a.configPanel.preview.effect != effectFade {
		t.Error("changing the effect did not play it")
	}
}

func TestThePreviewUsesTheRealDuration(t *testing.T) {
	a := animationConfig(t, momentPanels)
	p := a.configPanel.preview
	if p.progress(p.start.Add(200*time.Millisecond)) >= 1 || p.progress(p.start.Add(250*time.Millisecond)) < 1 {
		t.Error("the spring preview does not last 250 ms")
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
