package app

import (
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
)

func openBindsPage(t *testing.T) *App {
	t.Helper()
	a := testApp(t, newTab(1, &fakeView{}))
	a.openConfig()
	a.openBinds()
	return a
}

// n on the binds page takes a key, what it runs and a name, and saves the bind.
func TestAddingACustomBindFromTheBindsPage(t *testing.T) {
	a := openBindsPage(t)
	press(a, letter('n'), ctrlAltG, letter('c'))
	if a.mode != promptMode {
		t.Fatalf("mode %v after the key and c, want the command prompt", a.mode)
	}
	typeText(a, "less {{.file}}")
	press(a, enterPress)
	press(a, uv.KeyPressEvent{Code: 'u', Mod: uv.ModCtrl})
	typeText(a, "read")
	press(a, enterPress)
	want := []config.CustomBind{{Key: "ctrl+alt+g", Name: "read", Command: "less {{.file}}"}}
	if !slices.Equal(a.config.CustomBinds, want) || a.mode != configMode {
		t.Fatalf("binds %+v mode %v, want %+v back in the config", a.config.CustomBinds, a.mode, want)
	}
	if saved, err := config.Load(a.configPath); err != nil || !slices.Equal(saved.CustomBinds, want) {
		t.Errorf("saved %+v, %v", saved.CustomBinds, err)
	}
	if text := screenText(a); !strings.Contains(text, "read") || !strings.Contains(text, "less {{.file}}") {
		t.Errorf("the page does not list the bind:\n%s", text)
	}
	a.selectConfigRow(customBindRowBase)
	press(a, uv.KeyPressEvent{Code: uv.KeyDelete})
	if len(a.config.CustomBinds) != 0 {
		t.Errorf("delete left %+v", a.config.CustomBinds)
	}
}

// The prefix pressed first makes a bind after the prefix; a key a command has is refused there.
func TestACustomBindAfterThePrefix(t *testing.T) {
	a := openBindsPage(t)
	press(a, letter('n'), prefixPress, letter('x'))
	if c := a.configPanel.bindCapture; c.custom == nil || c.problem == "" {
		t.Fatalf("prefix x was taken: %+v", c)
	}
	press(a, letter('m'), letter('f'))
	typeText(a, "demo-deploy")
	press(a, enterPress, enterPress)
	want := []config.CustomBind{{Key: "prefix m", Name: "demo-deploy", Flow: "demo-deploy"}}
	if !slices.Equal(a.config.CustomBinds, want) {
		t.Errorf("binds %+v, want %+v", a.config.CustomBinds, want)
	}
}

// esc at any step leaves the binds as they were, on the binds page.
func TestCancellingACustomBind(t *testing.T) {
	a := openBindsPage(t)
	press(a, letter('n'), ctrlAltG, letter('s'), escPress)
	if len(a.config.CustomBinds) != 0 || a.mode != configMode || a.configPanel.bindCapture.custom != nil {
		t.Errorf("binds %+v mode %v after esc", a.config.CustomBinds, a.mode)
	}
}
