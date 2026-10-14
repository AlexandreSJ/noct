package app

import (
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

func namedKey(t *testing.T, name string) uv.KeyPressEvent {
	t.Helper()
	key, ok := keyEvent(name)
	if !ok {
		t.Fatalf("no key named %q", name)
	}
	return key
}

func TestTextFieldEditsAtTheCursor(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
		// cursor is where the cursor ends, in characters.
		cursor int
	}{
		{"types at the end", []string{"!"}, "hello world!", 12},
		{"left then types", []string{"left", "left", "X"}, "hello worXld", 10},
		{"home then types", []string{"home", ">"}, ">hello world", 1},
		{"ctrl+a and ctrl+e", []string{"ctrl+a", "ctrl+e", "."}, "hello world.", 12},
		{"right stops at the end", []string{"right", "right"}, "hello world", 11},
		{"backspace erases before", []string{"left", "backspace"}, "hello word", 9},
		{"backspace at the start does nothing", []string{"home", "backspace"}, "hello world", 0},
		{"delete erases after", []string{"home", "delete"}, "ello world", 0},
		{"ctrl+w erases a word", []string{"ctrl+w"}, "hello ", 6},
		{"ctrl+backspace erases a word", []string{"ctrl+backspace"}, "hello ", 6},
		{"ctrl+h, as terminals send ctrl+backspace", []string{"ctrl+h"}, "hello ", 6},
		{"alt+backspace erases a word", []string{"alt+backspace"}, "hello ", 6},
		{"ctrl+left jumps a word", []string{"ctrl+left", "_"}, "hello _world", 7},
		{"alt+b and alt+f", []string{"alt+b", "alt+b", "alt+f", "_"}, "hello_ world", 6},
		{"ctrl+right from the start", []string{"home", "ctrl+right", "_"}, "hello_ world", 6},
		{"alt+d erases the next word", []string{"home", "alt+d"}, " world", 0},
		{"ctrl+u erases to the start", []string{"ctrl+left", "ctrl+u"}, "world", 0},
		{"ctrl+k erases to the end", []string{"ctrl+left", "ctrl+k"}, "hello ", 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := newTextField("hello world")
			for _, name := range tt.keys {
				field.handleKey(namedKey(t, name))
			}
			if field.String() != tt.want || field.cursor != tt.cursor {
				t.Errorf("text %q cursor %d, want %q cursor %d", field.String(), field.cursor, tt.want, tt.cursor)
			}
		})
	}
}

func TestTextFieldLeavesOtherKeys(t *testing.T) {
	field := newTextField("abc")
	for _, name := range []string{"enter", "esc", "tab", "up", "ctrl+s"} {
		if field.handleKey(namedKey(t, name)) {
			t.Errorf("%s was taken by the field", name)
		}
	}
	if field.String() != "abc" {
		t.Errorf("text %q, want abc", field.String())
	}
}

func TestTextFieldPastesOneLine(t *testing.T) {
	field := newTextField("ab")
	field.handleKey(namedKey(t, "left"))
	field.insert("one\ntwo\tthree\x1b\n")
	if field.String() != "aone two threeb" || field.cursor != 14 {
		t.Errorf("text %q cursor %d", field.String(), field.cursor)
	}
}

func TestTextFieldViewKeepsTheCursorInSight(t *testing.T) {
	field := newTextField("abcdefghij")
	if got := field.view(6, false); got != "…ghij▏" {
		t.Errorf("at the end: %q", got)
	}
	field.cursor = 0
	if got := field.view(6, false); got != "▏abcd…" {
		t.Errorf("at the start: %q", got)
	}
	if got := field.view(0, true); got != "▏••••••••••" {
		t.Errorf("secret: %q", got)
	}
}

func TestPromptMovesTheCursorAndTakesAPaste(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('r'), clearLine)
	typeText(a, "api")
	press(a, namedKey(t, "home"))
	if err := a.handleEvent(uv.PasteEvent{Content: "my-"}); err != nil {
		t.Fatal(err)
	}
	press(a, namedKey(t, "end"), namedKey(t, "ctrl+w"))
	typeText(a, "web")
	press(a, enterPress)
	if a.currentTab().name != "my-web" {
		t.Errorf("name %q, want my-web", a.currentTab().name)
	}
}

func TestPasteKeyAsksTheTerminalAndItsAnswerIsTyped(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('r'), clearLine, namedKey(t, "ctrl+v"))
	if a.mode != promptMode || a.prompt.input.String() != "" {
		t.Fatalf("ctrl+v: mode %v, text %q", a.mode, a.prompt.input.String())
	}
	if err := a.handleEvent(uv.ClipboardEvent{Content: "from the clipboard"}); err != nil {
		t.Fatal(err)
	}
	if a.prompt.input.String() != "from the clipboard" {
		t.Errorf("text %q", a.prompt.input.String())
	}
}

func TestParamsFormEditsInsideTheValue(t *testing.T) {
	a := testApp(t)
	saveScript(t, "greet", greetScript)
	openScriptsPanel(t, a)
	press(a, enterPress)
	if a.mode != formMode {
		t.Fatalf("mode %v, want the form", a.mode)
	}
	press(a, namedKey(t, "left"))
	if a.form.values["greeting"] != "bye" {
		t.Errorf("left on a choice gave %q, want bye", a.form.values["greeting"])
	}
	press(a, tabPress, clearLine)
	typeText(a, "wrld")
	press(a, namedKey(t, "left"), namedKey(t, "left"), namedKey(t, "left"), letter('o'))
	name := a.form.field().Name
	if a.form.values[name] != "world" {
		t.Errorf("value %q, want world", a.form.values[name])
	}
}

func TestCtrlCCopiesTheField(t *testing.T) {
	a := testApp(t, sideBySideTab(1, 2))
	press(a, prefixPress, letter('r'), clearLine)
	typeText(a, "api")
	press(a, namedKey(t, "ctrl+c"))
	if a.mode != promptMode || a.prompt.input.String() != "api" || a.notice != "copied to the clipboard" {
		t.Errorf("mode %v, text %q, notice %q", a.mode, a.prompt.input.String(), a.notice)
	}
}
