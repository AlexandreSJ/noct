package flow

import (
	"strings"
	"testing"
)

const objectFromParams = `variables:
  user: {from: "env:DEPLOY_USER"}
  password: {default: 'say "hi"', secret: true}
  hosts: {type: list, default: '["a"]'}
  login:
    type: object
    default: '{"user": $user, "pass": $password, "hosts": $hosts}'
steps:
  - run: printf '%s %s|' $login.user $login.pass
`

func TestAParamDefaultReadsTheParamsBeforeIt(t *testing.T) {
	f := parse(t, objectFromParams)
	if err := f.Validate(nil); err != nil {
		t.Fatalf("a default that is a template: %v", err)
	}
	r, out := testRunner(t, nil)
	r.Getenv = func(name string) string { return map[string]string{"DEPLOY_USER": "ana"}[name] }
	if result := r.Run(f, map[string]string{"password": "typed"}); !result.OK {
		t.Fatalf("run failed:\n%s", out)
	}
	if !strings.Contains(out.String(), "ana typed|") {
		t.Errorf("the object read %q, want the env param and the password typed", out)
	}
	x, err := r.start(f, nil)
	if err != nil {
		t.Fatal(err)
	}
	// hosts comes before login, which reads it as JSON; the quotes in the password are escaped.
	if want := `{"user": "ana", "pass": "say \"hi\"", "hosts": ["a"]}`; x.vars["login"] != want {
		t.Errorf("login = %s, want %s", x.vars["login"], want)
	}
}

func TestAValueGivenForATemplateDefaultIsNotRendered(t *testing.T) {
	f := parse(t, objectFromParams)
	given := `{"user": "$password"}`
	x, err := (&Runner{}).start(f, map[string]string{"login": given})
	if err != nil || x.vars["login"] != given {
		t.Errorf("login = %q (%v), want the value given as it is", x.vars["login"], err)
	}
}

func TestADefaultThatRendersWrongIsTold(t *testing.T) {
	f := parse(t, "variables:\n  a: {type: object, default: '{\"x\": $b}'}\nsteps: [{run: 'true'}]\n")
	if err := f.Validate(nil); err != nil {
		t.Fatalf("a template default is checked when it renders: %v", err)
	}
	if err := f.CheckParams(nil); err == nil || !strings.Contains(err.Error(), "JSON object") {
		t.Errorf("CheckParams = %v, want the rendered default refused", err)
	}
	broken := parse(t, "variables:\n  a: '$len(b'\nsteps: [{run: 'true'}]\n")
	if err := broken.Validate(nil); err == nil || !strings.Contains(err.Error(), "$len") {
		t.Errorf("Validate = %v, want a default that does not parse refused", err)
	}
}
