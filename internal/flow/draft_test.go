package flow

import (
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/script"
)

// A step that calls a draft script fails: a draft runs only after the user promotes it.
func TestDraftScriptsDoNotRun(t *testing.T) {
	r, out := testRunner(t, map[string]script.Script{"new": {Name: "new", Run: "echo ran", Draft: true}})
	result := r.Run(parse(t, "steps: [{script: new}]\n"), nil)
	if result.OK || strings.Contains(out.String(), "ran\n") || !strings.Contains(result.Steps["0"].Message, "draft") {
		t.Errorf("ok %v, step %+v, output %q", result.OK, result.Steps["0"], out)
	}
}

func TestDraftFlowsKeepTheMark(t *testing.T) {
	f := parse(t, "draft: true\nsteps: [{run: 'true'}]\n")
	if !f.Draft {
		t.Fatal("draft not read")
	}
	f.Draft = false
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if again, _ := Load(f.File); again.Draft {
		t.Error("promoting did not clear the mark")
	}
}
