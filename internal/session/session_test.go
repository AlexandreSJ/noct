package session

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/layout"
)

func TestSaveThenLoadKeepsEverything(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "sessions.json")
	when := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	saved := File{Sessions: []Session{{
		Name: "dev", Created: when, LastAccess: when.Add(time.Hour), ActiveTab: 1,
		Tabs: []Tab{{
			Name:    "api",
			Layout:  layout.Shape{Children: []layout.Shape{{Pane: 1, Share: 0.5}, {Pane: 2, Share: 0.5}}, Share: 1},
			Focused: 2,
			Panes: []Pane{
				{ID: 1, Kind: Terminal, Dir: "/tmp", Name: "server"},
				{ID: 2, Kind: Scripts},
				{ID: 3, Kind: Terminal, Floating: &Frame{X: 5, Y: 3, Width: 40, Height: 10}},
			},
		}},
	}}}
	if err := Save(path, saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, saved) {
		t.Errorf("loaded %+v, saved %+v", loaded, saved)
	}
}

func TestLoadWithoutAFileHasNoSessions(t *testing.T) {
	f, err := Load(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil || len(f.Sessions) != 0 {
		t.Errorf("got %+v, %v", f, err)
	}
}

func TestCheckName(t *testing.T) {
	for name, ok := range map[string]bool{"dev": true, "api-2": true, "": false, "my dev": false, "a\tb": false} {
		if err := CheckName(name); (err == nil) != ok {
			t.Errorf("CheckName(%q) = %v", name, err)
		}
	}
}

func TestFreeNameSkipsTakenNumbers(t *testing.T) {
	taken := map[string]bool{"1": true, "2": true, "4": true}
	if name := FreeName(func(name string) bool { return taken[name] }); name != "3" {
		t.Errorf("FreeName = %q, want 3", name)
	}
}
