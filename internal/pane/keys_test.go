package pane

import (
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

func TestModifiedSpecialKeySequence(t *testing.T) {
	tests := []struct {
		name   string
		key    uv.KeyPressEvent
		want   string
		wantOK bool
	}{
		{"ctrl+left jumps a word", uv.KeyPressEvent{Code: uv.KeyLeft, Mod: uv.ModCtrl}, "\x1b[1;5D", true},
		{"shift+up", uv.KeyPressEvent{Code: uv.KeyUp, Mod: uv.ModShift}, "\x1b[1;2A", true},
		{"alt+right", uv.KeyPressEvent{Code: uv.KeyRight, Mod: uv.ModAlt}, "\x1b[1;3C", true},
		{"ctrl+shift+end", uv.KeyPressEvent{Code: uv.KeyEnd, Mod: uv.ModCtrl | uv.ModShift}, "\x1b[1;6F", true},
		{"ctrl+delete", uv.KeyPressEvent{Code: uv.KeyDelete, Mod: uv.ModCtrl}, "\x1b[3;5~", true},
		{"plain left is left to the emulator", uv.KeyPressEvent{Code: uv.KeyLeft}, "", false},
		{"ctrl+letter is left to the emulator", uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl}, "", false},
		{"meta is not encodable", uv.KeyPressEvent{Code: uv.KeyLeft, Mod: uv.ModMeta}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := modifiedSpecialKeySequence(tt.key)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestIsPlainText(t *testing.T) {
	tests := []struct {
		name string
		key  uv.KeyPressEvent
		want bool
	}{
		{"letter", uv.KeyPressEvent{Code: 'a', Text: "a"}, true},
		{"shifted letter", uv.KeyPressEvent{Code: 'a', Text: "A", Mod: uv.ModShift}, true},
		{"ctrl+letter", uv.KeyPressEvent{Code: 'a', Mod: uv.ModCtrl}, false},
		{"alt+letter", uv.KeyPressEvent{Code: 'a', Text: "a", Mod: uv.ModAlt}, false},
		{"enter", uv.KeyPressEvent{Code: uv.KeyEnter}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPlainText(tt.key); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCtrlAltSequence(t *testing.T) {
	tests := []struct {
		name   string
		key    uv.KeyPressEvent
		want   string
		wantOK bool
	}{
		{"ctrl+l clears the screen", uv.KeyPressEvent{Code: 'l', Mod: uv.ModCtrl}, "\x0c", true},
		{"ctrl+shift+l acts as ctrl+l", uv.KeyPressEvent{Code: 'l', Mod: uv.ModCtrl | uv.ModShift}, "\x0c", true},
		{"ctrl+space is NUL", uv.KeyPressEvent{Code: uv.KeySpace, Mod: uv.ModCtrl}, "\x00", true},
		{"ctrl+[ is escape", uv.KeyPressEvent{Code: '[', Mod: uv.ModCtrl}, "\x1b", true},
		{"alt+b jumps a word back", uv.KeyPressEvent{Code: 'b', Mod: uv.ModAlt}, "\x1bb", true},
		{"alt+shift+b", uv.KeyPressEvent{Code: 'b', Mod: uv.ModAlt | uv.ModShift}, "\x1bB", true},
		{"alt+backspace deletes a word", uv.KeyPressEvent{Code: uv.KeyBackspace, Mod: uv.ModAlt}, "\x1b\x7f", true},
		{"ctrl+alt+c", uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl | uv.ModAlt}, "\x1b\x03", true},
		{"plain letter is text, not a combination", uv.KeyPressEvent{Code: 'a', Text: "a"}, "", false},
		{"ctrl+1 has no control character", uv.KeyPressEvent{Code: '1', Mod: uv.ModCtrl}, "", false},
		{"ctrl+up is a navigation key", uv.KeyPressEvent{Code: uv.KeyUp, Mod: uv.ModCtrl}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ctrlAltSequence(tt.key)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestWithoutUnencodedShift(t *testing.T) {
	if got := withoutUnencodedShift(uv.KeyPressEvent{Code: uv.KeyEnter, Mod: uv.ModShift}); got.Mod != 0 {
		t.Errorf("shift+enter kept shift: %v", got)
	}
	if got := withoutUnencodedShift(uv.KeyPressEvent{Code: uv.KeyTab, Mod: uv.ModShift}); got.Mod != uv.ModShift {
		t.Errorf("shift+tab lost shift: %v", got)
	}
}
