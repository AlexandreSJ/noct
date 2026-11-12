package flow

import (
	"slices"
	"strings"
	"testing"
)

// A block works as in a language: it changes a variable made before it, and the ones it makes go
// when it ends, unless it exports them.
const scopedFlow = `params:
  env: dev
steps:
  - {id: build, run: 'echo built; echo version=1.2'}
  - if: 'true'
    then:
      - set: {env: prod, inner: here}
      - {id: probe, run: 'echo probed'}
    export: {inner: kept}
  - {run: 'echo "{{.env}} {{.output}} {{.kept}} [{{.inner}}] {{.steps.build.output}}"'}
`

func TestABlockChangesWhatExistsAndDropsWhatItMade(t *testing.T) {
	f := parse(t, scopedFlow)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	r, out := testRunner(t, nil)
	if result := r.Run(f, nil); !result.OK {
		t.Fatalf("run failed:\n%s", out)
	}
	if !strings.Contains(out.String(), "prod probed here [] built") {
		t.Errorf("the last step read:\n%s\nwant env changed, output of the last step, inner kept as kept and gone as inner", out)
	}
}

func TestTheScopeFollowsTheBlocks(t *testing.T) {
	f := parse(t, scopedFlow)
	var names []string
	for _, v := range f.Scope("", "", 2, nil) {
		if v.Step == "" {
			names = append(names, v.Field)
		}
	}
	for _, want := range []string{"env", "output", "exit", "ok", "kept"} {
		if !slices.Contains(names, want) {
			t.Errorf("after the if the scope lacks %s: %v", want, names)
		}
	}
	if slices.Contains(names, "inner") {
		t.Errorf("after the if the scope still has inner: %v", names)
	}
}

func TestOnlyIfsAndRepeatsExport(t *testing.T) {
	f := parse(t, "steps:\n  - {run: 'true', export: {a: b}}\n")
	if err := f.Validate(nil); err == nil || !strings.Contains(err.Error(), "export belongs") {
		t.Errorf("err = %v", err)
	}
}
