package app

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/config"
)

func TestSpringStartsAndEndsInPlace(t *testing.T) {
	if springAt(0) != 0 || math.Abs(springAt(1)-1) > 0.001 {
		t.Errorf("spring goes from %v to %v, want 0 to 1", springAt(0), springAt(1))
	}
	over := false
	for i := 1; i < 100; i++ {
		over = over || springAt(float64(i)/100) > 1
	}
	if !over {
		t.Error("the spring never goes past its end")
	}
}

func TestFlickerNeverPassesThreeFlashesASecond(t *testing.T) {
	flashes, was := 0, false
	for ms := 0; ms < 1000; ms++ {
		on := flickerOn(time.Duration(ms) * time.Millisecond)
		if on && !was {
			flashes++
		}
		was = on
	}
	if flashes > 3 || flashes == 0 {
		t.Errorf("%d flashes in a second, want 1 to 3", flashes)
	}
}

func TestBinaryAndJitterSettleOnTheRealRune(t *testing.T) {
	if r := binaryAt('x', 0.8, 0.5, 7); r != 'x' {
		t.Errorf("binary after its reveal is %q", r)
	}
	if r := binaryAt('x', 0.2, 0.5, 7); r != '0' && r != '1' {
		t.Errorf("binary before its reveal is %q", r)
	}
	if r := jitterAt('x', 0.8, 0.5, 7); r != 'x' {
		t.Errorf("jitter after it settles is %q", r)
	}
	if r := jitterAt('x', 0.2, 0.5, 7); !strings.ContainsRune(jitterRunes, r) {
		t.Errorf("jitter before it settles is %q", r)
	}
}

func TestEveryEffectOfTheConfigIsKnown(t *testing.T) {
	if len(effects) != len(config.Effects) {
		t.Fatalf("%d effects, the config knows %d", len(effects), len(config.Effects))
	}
	for i, e := range effects {
		if e.name != config.Effects[i] {
			t.Errorf("effect %d is %s, the config's is %s", i, e.name, config.Effects[i])
		}
	}
}
