package flow

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const greetFunction = `description: Greets someone
params:
  who: {required: true}
steps:
  - {id: say, run: 'echo hello {{.who}}'}
  - set: {shout: 'HELLO {{.who}}'}
returns:
  greeting: '{{.steps.say.output}}'
  loud: '{{.shout}}'
`

func functionsIn(t *testing.T, files map[string]string) []Flow {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name+Ext), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	functions, err := LoadFunctions(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	return functions
}

func TestACallRunsTheFunctionInAScopeOfItsOwn(t *testing.T) {
	functions := functionsIn(t, map[string]string{"greet": greetFunction})
	f := parse(t, `steps:
  - set: {shout: mine}
  - {id: hi, call: greet, with: {who: ana}}
  - {run: 'echo "[{{.greeting}}] [{{.steps.hi.loud}}] [{{.shout}}] [{{.who}}]"'}
`)
	lookup := func(name string) (Flow, bool) { return Find(functions, name) }
	f.WithFunctions(lookup)
	if err := f.CheckCalls(functions); err != nil {
		t.Fatal(err)
	}
	r, out := testRunner(t, nil)
	r.Function = lookup
	if result := r.Run(f, nil); !result.OK {
		t.Fatalf("run failed:\n%s", out)
	}
	if !strings.Contains(out.String(), "[hello ana] [HELLO ana] [mine] []") {
		t.Errorf("after the call:\n%s\nwant the returns, the caller's shout kept and no who", out)
	}
	scope := f.Scope("", "", 2, nil)
	if !slices.ContainsFunc(scope, func(v Variable) bool { return v.Step == "hi" && v.Field == "greeting" }) {
		t.Errorf("the scope after the call lacks its returns: %+v", scope)
	}
}

func TestCallsAreChecked(t *testing.T) {
	functions := functionsIn(t, map[string]string{"greet": greetFunction})
	for content, problem := range map[string]string{
		"steps: [{call: nope}]":                            "nope, which is not a function",
		"steps: [{call: greet, with: {who: a, extra: b}}]": "has no param for",
		"steps: [{call: greet}]":                           "leaves out who",
	} {
		if err := parse(t, content).CheckCalls(functions); err == nil || !strings.Contains(err.Error(), problem) {
			t.Errorf("%s: %v, want %q", content, err, problem)
		}
	}
}

func TestCallersAreTheFlowsThatCallAFunction(t *testing.T) {
	flows := []Flow{
		parse(t, "steps: [{call: greet}]"),
		parse(t, "steps: [{if: 'true', then: [{call: greet}]}]"),
		parse(t, "steps: [{run: 'true'}]"),
	}
	flows[0].Name, flows[1].Name, flows[2].Name = "a", "b", "c"
	if got := Callers(flows, "greet"); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("callers %v, want a and b", got)
	}
}

func TestAFunctionCallingItselfStops(t *testing.T) {
	functions := functionsIn(t, map[string]string{"loop": "steps: [{call: loop}]\n"})
	r, out := testRunner(t, nil)
	r.Function = func(name string) (Flow, bool) { return Find(functions, name) }
	if result := r.Run(parse(t, "steps: [{call: loop}]"), nil); result.OK || !strings.Contains(out.String(), "more than 16 deep") {
		t.Errorf("ok %v:\n%s", result.OK, out)
	}
}
