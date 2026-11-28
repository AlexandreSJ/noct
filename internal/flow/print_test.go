package flow

import (
	"strings"
	"testing"
)

func TestAPrintWritesItsValuesAsAProgramWould(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
variables:
  count: {default: "3"}
  ok: {type: boolean, default: "true"}
  pin: {default: "1234", secret: true}
steps:
  - print: '"1" + "1"'
  - print: 'count + 1, "is", count + "1"'
  - print: '''it\''s'', "a \"quote\"", ok, not ok'
  - print: '"pin:", pin'
  - print: '["a", "b"], {"n": 1}'
`)
	result := r.Run(f, nil)
	if !result.OK {
		t.Fatalf("the run failed:\n%s", out)
	}
	for _, line := range []string{"\n11\n", "\n4 is 31\n", "\nit's a \"quote\" true false\n", "\npin: ••••\n", "\n[\"a\",\"b\"] {\"n\":1}\n"} {
		if !strings.Contains(out.String(), line) {
			t.Errorf("the output has no line %q:\n%s", line, out)
		}
	}
	if got := result.Steps["0"].Message; got != "11" {
		t.Errorf("the print's result tells %q, want 11", got)
	}
	if strings.Contains(out.String(), "1234") {
		t.Errorf("a secret shows:\n%s", out)
	}
}

func TestAPrintOfANameNothingGivesStopsTheFlow(t *testing.T) {
	r, out := testRunner(t, nil)
	result := r.Run(parse(t, `steps: [{print: 'nobody'}, {print: '"after"'}]`), nil)
	if result.OK || !strings.Contains(result.Steps["0"].Message, "nobody") || strings.Contains(out.String(), "after") {
		t.Errorf("ok %v, message %q:\n%s", result.OK, result.Steps["0"].Message, out)
	}
}

func TestPrintValidation(t *testing.T) {
	cases := map[string]string{
		`steps: [{print: '"hi"'}]`:                  "",
		`steps: [{print: '"a" "b"'}]`:               "commas",
		`steps: [{print: '"hi"', capture: {x: .}}]`: "capture",
		`steps: [{print: '{{.x}}'}]`:                "{{ }}",
		`steps: [{print: '"hi"', run: 'true'}]`:     "exactly one",
	}
	for text, problem := range cases {
		err := parse(t, text).Validate(nil)
		if problem == "" {
			if err != nil {
				t.Errorf("%s: %v", text, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), problem) {
			t.Errorf("%s: err %v, want %q", text, err, problem)
		}
	}
}
