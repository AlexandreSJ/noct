package flow

import (
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/expr"
)

func TestDecodeJSONKeepsNestedValues(t *testing.T) {
	for _, c := range []struct {
		kind, text, want string
	}{
		{ListParam, `["a", 2, true]`, `["a",2,true]`},
		{ListParam, `[]`, `[]`},
		{ListParam, `[[1, 2], {"k": [3]}]`, `[[1,2],{"k":[3]}]`},
		{ObjectParam, `{"host": "x", "db": {"port": 5432, "tags": ["a"]}}`, `{"db":{"port":5432,"tags":["a"]},"host":"x"}`},
		{ObjectParam, `{"big": 12345678901234567890, "pi": 3.14}`, `{"big":12345678901234567890,"pi":3.14}`},
		{ObjectParam, `{"none": null}`, `{"none":null}`},
	} {
		value, err := DecodeJSON(c.kind, c.text)
		if err != nil {
			t.Errorf("%s %s: %v", c.kind, c.text, err)
			continue
		}
		if got := expr.String(value); got != c.want {
			t.Errorf("%s %s printed %s, want %s", c.kind, c.text, got, c.want)
		}
	}
}

func TestDecodeJSONRefusesTheOtherShapes(t *testing.T) {
	for _, c := range []struct{ kind, text, want string }{
		{ListParam, `{"a": 1}`, "a JSON list"},
		{ListParam, `"a"`, "a JSON list"},
		{ListParam, `[1, 2`, "a JSON list"},
		{ListParam, `[1] [2]`, "a JSON list"},
		{ObjectParam, `[1]`, "a JSON object"},
		{ObjectParam, `host: x`, "a JSON object"},
		{ObjectParam, ``, "a JSON object"},
	} {
		if _, err := DecodeJSON(c.kind, c.text); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %q: %v, want %q", c.kind, c.text, err, c.want)
		}
	}
}

// $ reaches inside list and object variables, at any depth, and prints them whole as JSON.
func TestTextsReadListAndObjectVariables(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `variables:
  hosts: {type: list, default: '["a", "b"]'}
  cfg: {type: object, default: '{"db": {"port": 5432, "tags": ["x", "y"]}, "name": "noct"}'}
steps:
  - run: echo "$hosts[1] $cfg.name $cfg.db.port $cfg.db.tags[0]"
  - run: echo "$join(map(hosts, h => "<" + h + ">"), "") $cfg.db"
  - run: echo $hosts
  - repeat: {each: hosts, as: host}
    steps: [{run: 'echo "host $host"'}]
`)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	if result := r.Run(f, nil); !result.OK {
		t.Fatalf("run failed: %s", out)
	}
	for _, want := range []string{"b noct 5432 x", `<a><b> {"port":5432,"tags":["x","y"]}`, `["a","b"]`, "host a", "host b"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestListAndObjectParamsCheckTheirValues(t *testing.T) {
	f := parse(t, `variables:
  hosts: {type: list}
  cfg: {type: object}
steps: [{run: 'true'}]
`)
	for values, problem := range map[string]string{
		`hosts=["a"]`:   "",
		`hosts=a, b`:    `hosts must be a JSON list`,
		`cfg={"a": {}}`: "",
		`cfg=[]`:        `cfg must be a JSON object`,
	} {
		name, value, _ := strings.Cut(values, "=")
		err := f.CheckParams(map[string]string{name: value})
		if (problem == "") != (err == nil) || err != nil && !strings.Contains(err.Error(), problem) {
			t.Errorf("%s: %v, want %q", values, err, problem)
		}
	}
	if err := parse(t, "variables:\n  a: {type: list, default: 'x'}\nsteps: [{run: 'true'}]\n").Validate(nil); err == nil || !strings.Contains(err.Error(), "the default must be a JSON list") {
		t.Errorf("a list with a default that is not one: %v", err)
	}
}

// Flows written with the names before ADR-071 read as the names now.
func TestLegacyParamTypesReadAsTheNewNames(t *testing.T) {
	f := parse(t, "variables:\n  a: {type: bool}\n  b: {type: choice, choices: [x]}\nsteps: [{run: 'true'}]\n")
	if f.Params["a"].Type != BooleanParam || f.Params["b"].Type != OptionParam {
		t.Errorf("types %q and %q", f.Params["a"].Type, f.Params["b"].Type)
	}
}

// A function's list param is JSON inside it, and the caller's variables are as they were after.
func TestFunctionsReadTheirOwnJSONParams(t *testing.T) {
	r, out := testRunner(t, nil)
	fn := parse(t, "variables:\n  items: {type: list}\nsteps:\n  - run: echo \"first $items[0]\"\n")
	fn.Name, fn.Function = "firstof", true
	r.Function = func(name string) (Flow, bool) { return fn, name == "firstof" }
	f := parse(t, `variables:
  items: '["not", "json"]'
steps:
  - call: firstof
    with: {items: '["z"]'}
  - run: echo "after $items"
`)
	f.WithFunctions(r.Function)
	if result := r.Run(f, nil); !result.OK {
		t.Fatalf("run failed: %s", out)
	}
	if !strings.Contains(out.String(), "first z") || !strings.Contains(out.String(), `after ["not","json"]`) {
		t.Errorf("output:\n%s", out)
	}
}

// A lambda's params belong to it, so they warn about nothing.
func TestALambdasParamsAreNotVariables(t *testing.T) {
	f := parse(t, `variables:
  servers: {type: list, default: '[{"name": "a"}]'}
steps:
  - run: echo $join(map(servers, s => s.name + host), " ")
  - set: {host: x}
`)
	var texts []string
	for _, w := range f.Warnings(nil) {
		texts = append(texts, w.Text)
	}
	if len(texts) != 1 || texts[0] != "variable host is not set yet here" {
		t.Errorf("warnings %q, want only host's", texts)
	}
}

// A list or object param without a value reads as an empty one, so range and fields still work.
func TestAnEmptyListOrObjectReadsAsEmpty(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, "variables:\n  hosts: {type: list}\n  cfg: {type: object}\nsteps:\n  - run: echo \"[$len(hosts)] $hosts $cfg\"\n")
	if result := r.Run(f, nil); !result.OK {
		t.Fatalf("run failed: %s", out)
	}
	if !strings.Contains(out.String(), "[0] [] {}") {
		t.Errorf("output:\n%s", out)
	}
}

func TestListItemsRoundTrip(t *testing.T) {
	items, ok := ListItems(`["a", 3, "3", {"k": [1, 2]}, true]`)
	want := []string{"a", "3", `"3"`, `{"k":[1,2]}`, "true"}
	if !ok || !slices.Equal(items, want) {
		t.Fatalf("items %q, %v; want %q", items, ok, want)
	}
	if got := ItemsJSON(items); got != `["a",3,"3",{"k":[1,2]},true]` {
		t.Errorf("joined %s", got)
	}
	if _, ok := ListItems(`{"a": 1}`); ok {
		t.Error("an object read as a list")
	}
	if items, ok := ListItems(""); !ok || items != nil || ItemsJSON(nil) != "" {
		t.Error("an empty list is no default")
	}
	if got := ItemsJSON([]string{"two words", "{not json"}); got != `["two words","{not json"]` {
		t.Errorf("text items joined as %s", got)
	}
}
