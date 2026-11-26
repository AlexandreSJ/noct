package flow

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParamsKeepTheirOrder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "order"+Ext)
	if err := os.WriteFile(file, []byte("variables:\n  zone: eu\n  app: {type: number}\n  mode: dev\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.ParamNames(); !slices.Equal(got, []string{"zone", "app", "mode"}) {
		t.Fatalf("read order %v", got)
	}
	f.ParamOrder = []string{"mode", "zone"}
	f.Params["extra"] = Param{}
	if got := f.ParamNames(); !slices.Equal(got, []string{"mode", "zone", "app", "extra"}) {
		t.Fatalf("set order %v: the rest go after it, by name", got)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(file)
	text := string(content)
	if !(strings.Index(text, "mode:") < strings.Index(text, "zone:") && strings.Index(text, "zone:") < strings.Index(text, "app:")) {
		t.Fatalf("saved out of order:\n%s", text)
	}
	again, err := Load(file)
	if err != nil || !slices.Equal(again.ParamNames(), []string{"mode", "zone", "app", "extra"}) {
		t.Fatalf("reloaded %v, %v", again.ParamNames(), err)
	}
}
