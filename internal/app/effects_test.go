package app

import (
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/config"
)

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
		if e != config.Effects[i] {
			t.Errorf("effect %d is %s, the config's is %s", i, e, config.Effects[i])
		}
	}
}
