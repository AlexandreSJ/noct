package app

import (
	"image"
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/config"
)

// openBindAt opens the binds page on an action.
func openBindAt(t *testing.T, context, action string) *App {
	t.Helper()
	a := openConfigAt(t, bindsRow)
	press(a, enterPress)
	if a.configPanel.page != bindsPage {
		t.Fatal("enter on binds did not open the binds page")
	}
	for i, row := range bindRows() {
		if row.context.id == context && row.action.id == action {
			a.configPanel.selected = bindRowBase + configRow(i)
			return a
		}
	}
	t.Fatalf("no action %s/%s", context, action)
	return nil
}

func TestBindReplacesTheKeysAndSaves(t *testing.T) {
	a := openBindAt(t, "scripts", "edit")
	press(a, enterPress, letter('x'))
	if a.config.Keys != nil {
		t.Fatal("the key was used before enter")
	}
	press(a, enterPress)
	if keys := a.config.Keys["scripts"]["edit"]; !slices.Equal(keys, []string{"x"}) {
		t.Fatalf("edit keys %v, want [x]", keys)
	}
	saved, err := config.Load(a.configPath)
	if err != nil || !slices.Equal(saved.Keys["scripts"]["edit"], []string{"x"}) {
		t.Errorf("saved keys %v (%v)", saved.Keys, err)
	}
	press(a, uv.KeyPressEvent{Code: uv.KeyDelete})
	if a.config.Keys != nil {
		t.Errorf("del left %v, want the defaults back and nothing in the file", a.config.Keys)
	}
}

func TestBindAddsAKeyAndTakesItFromAnother(t *testing.T) {
	a := openBindAt(t, "scripts", "edit")
	press(a, letter('a'), letter('n'))
	var warned bool
	for _, line := range a.configLines() {
		warned = warned || line.warning && strings.Contains(line.label, "n is new script or directory")
	}
	if !warned {
		t.Error("no warning that n is taken")
	}
	press(a, enterPress)
	if keys := a.config.Keys["scripts"]["edit"]; !slices.Equal(keys, []string{"e", "n"}) {
		t.Errorf("edit keys %v, want e and n", keys)
	}
	if keys := a.config.Keys["scripts"]["new"]; !slices.Equal(keys, []string{"ctrl+n"}) {
		t.Errorf("new keys %v, want only ctrl+n left", keys)
	}
}

func TestBindRefusesKeysThatCannotBeUsed(t *testing.T) {
	a := openBindAt(t, "config", "down")
	press(a, enterPress, enterPress, letter('n')) // down has two keys: choose the first, then take
	if c := a.configPanel.bindCapture; c.pending != "" || !strings.Contains(c.problem, "types text") {
		t.Errorf("a letter was taken where it types: pending %q, problem %q", c.pending, c.problem)
	}
	press(a, prefixPress)
	if c := a.configPanel.bindCapture; c.pending != "" || !strings.Contains(c.problem, "prefix") {
		t.Errorf("the prefix was taken: pending %q, problem %q", c.pending, c.problem)
	}
	press(a, escPress, escPress)
	if a.configPanel.page != settingsPage || a.mode != configMode {
		t.Errorf("esc twice: page %v mode %v, want back on the settings", a.configPanel.page, a.mode)
	}
}

func TestResetAllKeys(t *testing.T) {
	a := openBindAt(t, "scripts", "edit")
	press(a, enterPress, letter('x'), enterPress)
	a.configPanel.selected = resetBindsRow
	press(a, enterPress)
	if a.config.Keys == nil {
		t.Fatal("one enter reset the keys")
	}
	press(a, enterPress)
	if a.config.Keys != nil {
		t.Errorf("keys %v after the reset", a.config.Keys)
	}
}

// With several keys, enter chooses which with ←→, and enter again takes the key for that place.
func TestBindChangesTheChosenKeyOfSeveral(t *testing.T) {
	a := openBindAt(t, "scroll", "page-up")
	row, _ := bindRowAt(a.configPanel.selected)
	press(a, enterPress)
	if v := a.bindValue(a.configPanel.selected, row); v != "[pgup]  ctrl+b" {
		t.Fatalf("choosing shows %q", v)
	}
	press(a, rightPress, enterPress)
	if v := a.bindValue(a.configPanel.selected, row); v != "pgup  [press a key…]" {
		t.Fatalf("taking shows %q", v)
	}
	var told bool
	for _, line := range a.configLines() {
		told = told || strings.Contains(line.label, "replaces ctrl+b")
	}
	if !told {
		t.Error("the note does not say which key is replaced")
	}
	press(a, uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl}, enterPress)
	if keys := a.config.Keys["scroll"]["page-up"]; !slices.Equal(keys, []string{"pgup", "ctrl+u"}) {
		t.Errorf("page up keys %v, want pgup and ctrl+u", keys)
	}
	press(a, enterPress, escPress)
	if c := a.configPanel.bindCapture; c.choosing || c.active {
		t.Error("esc did not stop choosing")
	}
}

func TestClickingAKeyOfABindTakesThatOne(t *testing.T) {
	a := openBindAt(t, "scroll", "page-up")
	a.size = image.Pt(100, 40)
	press(a, letter('a'), letter('x'), enterPress) // changed, it shows • before its keys
	l := a.configLayout()
	for y := l.inner().Min.Y; y < l.inner().Max.Y; y++ {
		if line, ok := l.lineAt(y); ok && line.row == a.configPanel.selected {
			valueX := l.inner().Max.X - 1 - ansi.StringWidth("• pgup  ctrl+b  x")
			click(a, valueX+ansi.StringWidth("• pgup  "), y) // on the first cell of ctrl+b, after •
		}
	}
	if c := a.configPanel.bindCapture; !c.active || c.slot != 1 {
		t.Errorf("capture %+v after clicking ctrl+b, want it taking the second key", c)
	}
}
