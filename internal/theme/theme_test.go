package theme

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestParseColorAcceptsHexIndexAndEmpty(t *testing.T) {
	for in, want := range map[string]Color{"#9D7CD8": "#9d7cd8", "60": "60", "0": "0", "255": "255", "": ""} {
		got, err := ParseColor(in)
		if err != nil || got != want {
			t.Errorf("ParseColor(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"#12345", "#1234567", "256", "red", "-1", "#gggggg", "08"} {
		if _, err := ParseColor(in); err == nil {
			t.Errorf("ParseColor(%q) took a bad color", in)
		}
	}
}

func TestIndexedKeepsTheTerminalsSixteen(t *testing.T) {
	for n, want := range map[uint8]Color{3: "3", 15: "15", 16: "#000000", 231: "#ffffff", 60: "#5f5f87"} {
		if got := Indexed(n); got != want {
			t.Errorf("Indexed(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestResolveGivesNilForTheBackground(t *testing.T) {
	if c := Color("").Resolve(); c != nil {
		t.Errorf("an empty color resolved to %v", c)
	}
	if c := Color("4").Resolve(); c != ansi.IndexedColor(4) {
		t.Errorf("index 4 resolved to %v", c)
	}
	r, g, b, _ := Color("#ff8000").Resolve().RGBA()
	if r>>8 != 0xff || g>>8 != 0x80 || b>>8 != 0 {
		t.Errorf("#ff8000 resolved to %x %x %x", r>>8, g>>8, b>>8)
	}
}

func TestBuiltinsKeepTodaysThemes(t *testing.T) {
	var names []string
	for _, th := range Builtins() {
		names = append(names, th.Name)
		if !th.Builtin {
			t.Errorf("%s is not marked built in", th.Name)
		}
		if th.Colors[Background] == "" || th.Colors[Text] == "" {
			t.Errorf("%s has no background or text", th.Name)
		}
	}
	want := []string{"noct", "aurora", "ember", "rose", "blood", "frost", "mono", "tide", "moss", "pumpkin", "neon", "cobalt", "sand", "slate", "amber", "phosphor", "paper", "sakura"}
	if len(names) != len(want) {
		t.Fatalf("built-ins %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("built-ins %v, want %v", names, want)
		}
	}
	noct := Builtins()[0]
	for c, want := range map[Component]Color{
		ActiveTabBackground: "#9d7cd8", ActiveTabText: "#1a1b26", InactiveTabText: "#565f89",
		Background: "#1a1b26", InactiveTabBackground: "", Success: "#9ece6a", Error: "#f7768e",
	} {
		if got := noct.Colors[c]; got != want {
			t.Errorf("noct %s = %q, want %q", Components[c].Key, got, want)
		}
	}
}

func TestEveryComponentHasAGroupKeyAndLabel(t *testing.T) {
	seen := map[string]bool{}
	for c, info := range Components {
		if info.Group == "" || info.Key == "" || info.Label == "" {
			t.Errorf("component %d is missing a group, key or label: %+v", c, info)
		}
		if seen[info.Group+"."+info.Key] {
			t.Errorf("%s.%s is there twice", info.Group, info.Key)
		}
		seen[info.Group+"."+info.Key] = true
	}
}
