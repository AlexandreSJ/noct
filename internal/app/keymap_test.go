package app

import (
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// Every default key reads back as a key of the same action: the translation can press it.
func TestEveryDefaultKeyCanBePressed(t *testing.T) {
	for _, c := range keyContexts {
		seen := map[string]string{}
		for _, action := range c.actions {
			for _, key := range action.defaults {
				event, ok := keyEvent(key)
				if !ok || !slices.Contains(action.defaults, event.String()) {
					t.Errorf("%s/%s: %q reads back as %q (ok %v)", c.id, action.id, key, event.String(), ok)
				}
				if other, taken := seen[key]; taken {
					t.Errorf("%s: %q is a default of both %s and %s", c.id, key, other, action.id)
				}
				seen[key] = action.id
			}
		}
	}
}

func TestKeyEventNames(t *testing.T) {
	for name, want := range map[string]uv.KeyPressEvent{
		"enter":     {Code: uv.KeyEnter},
		"shift+tab": {Code: uv.KeyTab, Mod: uv.ModShift},
		"ctrl+x":    {Code: 'x', Mod: uv.ModCtrl},
		"G":         {Code: 'g', Mod: uv.ModShift, Text: "G"},
		"shift+g":   {Code: 'g', Mod: uv.ModShift, Text: "G"},
		"+":         {Code: '+', Text: "+"},
		"ctrl++":    {Code: '+', Mod: uv.ModCtrl},
		"f2":        {Code: uv.KeyF2},
	} {
		if got, ok := keyEvent(name); !ok || got != want {
			t.Errorf("keyEvent(%q) = %+v, %v; want %+v", name, got, ok, want)
		}
	}
	for _, bad := range []string{"", "hyper+x", "ctrl+", "ab"} {
		if _, ok := keyEvent(bad); ok {
			t.Errorf("keyEvent(%q) was taken", bad)
		}
	}
}

func resolved(t *testing.T, k keymap, context, name string) (string, bool) {
	t.Helper()
	key, ok := keyEvent(name)
	if !ok {
		t.Fatalf("no key %q", name)
	}
	got, pass := k.resolve(context, key)
	return got.String(), pass
}

func TestKeymapWithoutChangesPassesEveryKey(t *testing.T) {
	k := newKeymap(nil)
	for _, name := range []string{"e", "k", "enter", "x", "ctrl+n"} {
		if got, ok := resolved(t, k, "scripts", name); !ok || got != name {
			t.Errorf("%q became %q (%v) without changes", name, got, ok)
		}
	}
}

func TestKeymapChangedKeyStandsForTheDefault(t *testing.T) {
	k := newKeymap(map[string]map[string][]string{"scripts": {"edit": {"x"}, "new": {"e"}}})
	if got, ok := resolved(t, k, "scripts", "x"); !ok || got != "e" {
		t.Errorf("x gave %q (%v), want the edit key e", got, ok)
	}
	if got, ok := resolved(t, k, "scripts", "e"); !ok || got != "n" {
		t.Errorf("e gave %q (%v), want new's n, as the config moved it there", got, ok)
	}
	if _, ok := resolved(t, k, "scripts", "n"); ok {
		t.Error("n still does something after new moved to e")
	}
	if got, _ := resolved(t, k, "flows", "e"); got != "e" {
		t.Errorf("a change in scripts reached flows: e gave %q", got)
	}
}

func TestKeymapLeavesTypingAndEscAlone(t *testing.T) {
	k := newKeymap(map[string]map[string][]string{"config": {"down": {"ctrl+n"}}, "scripts": {"close": {"esc"}}})
	if got, ok := resolved(t, k, "config", "j"); !ok || got != "j" {
		t.Errorf("j in the config gave %q (%v): it types text there", got, ok)
	}
	if got, ok := resolved(t, k, "config", "ctrl+n"); !ok || got != "down" {
		t.Errorf("ctrl+n in the config gave %q (%v)", got, ok)
	}
	if keys := k.keysOf("scripts", "ctrl+d"); len(keys) != 0 {
		t.Errorf("esc was taken as a key: %v", keys)
	}
	if got, ok := resolved(t, k, "scripts", "esc"); !ok || got != "esc" {
		t.Errorf("esc gave %q (%v)", got, ok)
	}
}

func TestHintsShowTheKeysAsTheyAre(t *testing.T) {
	a := testApp(t)
	cases := map[string]string{
		"{enter} run  {e} edit  esc close": "enter run  e edit  esc close",
		"{up}{down} select":                "↑↓ select",
		"{delete} delete  {P} promote":     "del delete  P promote",
	}
	for text, want := range cases {
		if got := a.hint("scripts", text); got != want {
			t.Errorf("hint(%q) = %q, want %q", text, got, want)
		}
	}
	if got := a.hint("flows", "{shift+up}{shift+down} range  {J} {K} move"); got != "shift+↑↓ range  J K move" {
		t.Errorf("flows range hint %q", got)
	}
	a.keymap = newKeymap(map[string]map[string][]string{"scripts": {"up": {"ctrl+p"}, "edit": {"x", "ctrl+e"}}})
	if got := a.hint("scripts", "{up}{down} select  {e} edit  {k}{j} vim"); got != "ctrl+p/↓ select  x edit  j vim" {
		t.Errorf("changed keys hint %q", got)
	}
}

// withKeys applies changed keys as the config would.
func withKeys(a *App, changed map[string]map[string][]string) {
	next := a.config
	next.Keys = changed
	a.applyConfig(next)
}

func TestChangedCommandKeysWorkAfterThePrefixAndDirectly(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	withKeys(a, map[string]map[string][]string{"commands": {"new-panel": {"b"}}})
	press(a, prefixPress, letter('p'))
	if len(a.currentTab().views) != 1 {
		t.Fatal("p still opened a panel after it moved to b")
	}
	press(a, escPress, prefixPress, letter('b'))
	if len(a.currentTab().views) != 2 {
		t.Fatalf("prefix b opened %d panels, want the new panel", len(a.currentTab().views)-1)
	}
	press(a, uv.KeyPressEvent{Code: 'b', Mod: uv.ModAlt})
	if len(a.currentTab().views) != 3 {
		t.Errorf("alt+b did not open a panel as the direct shortcut")
	}
	if label := a.newTabLabel("terminal"); label != "terminal (alt+b)" {
		t.Errorf("the new tab choice reads %q", label)
	}
}

func TestChangedPanelKeysReachThePanel(t *testing.T) {
	a := testApp(t)
	saveScript(t, "alpha", "run: true\n")
	openScriptsPanel(t, a)
	withKeys(a, map[string]map[string][]string{"scripts": {"edit": {"x"}}})
	press(a, letter('e'))
	if a.mode != normalMode {
		t.Fatalf("e still edits: mode %v", a.mode)
	}
	press(a, letter('x'))
	if a.mode != scriptFormMode {
		t.Errorf("x did not open the script's form: mode %v", a.mode)
	}
	if hints := a.hint("scripts", scriptsHints); !strings.Contains(hints, "x edit") {
		t.Errorf("hints %q do not show x", hints)
	}
}

func TestAChangedQuitKeyConfirmsTwice(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	withKeys(a, map[string]map[string][]string{"commands": {"quit": {"z"}}})
	press(a, prefixPress, letter('z'), letter('z'))
	if !a.quitting {
		t.Error("z twice did not quit after quit moved to z")
	}
}
