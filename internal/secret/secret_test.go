package secret

import "testing"

func TestASecretShowsMasked(t *testing.T) {
	if got := Typed("pässword"); got != "••••••••" {
		t.Errorf("typed %q, want a dot a character", got)
	}
	if Value("hunter2") != Mask || Value("") != "" {
		t.Error("a value is the mask, and no value stays empty")
	}
	if Show("hunter2", true) != Mask || Show("hunter2", false) != "hunter2" {
		t.Error("Show masks only what is hidden")
	}
	if got := Hide("user:hunter2 token=abc", []string{"hunter2", "", "abc"}); got != "user:•••• token=••••" {
		t.Errorf("hidden %q", got)
	}
}
