package flow

import (
	"strings"
	"testing"
)

func TestDollarNamesAreTheFlowsAndEscapedWhereTheyLand(t *testing.T) {
	known := knownNames{vars: map[string]bool{"user": true, "obj": true, "f": true}, steps: map[string]bool{"build": true}}
	for _, c := range []struct {
		kind       textKind
		text, want string
	}{
		{plainText, "hi $user.", "hi {{.user}}."},
		{plainText, "$obj.pass and $build.output", "{{.obj.pass}} and {{.steps.build.output}}"},
		{plainText, "$HOME ${user} $$user $5 {{$x := 1}}", "$HOME ${user} $user $5 {{$x := 1}}"},
		{shellText, `echo $user "at $user" '$user'`, `echo {{quote .user}} "at {{_dq .user}}" '{{_sq .user}}'`},
		{shellText, `echo "it's $user" \$user`, `echo "it's {{_dq .user}}" \$user`},
		{shellText, `for f in *; do echo $f $user; done`, `for f in *; do echo $f {{quote .user}}; done`},
		{jsonText, `{"by": "$user", "obj": $obj}`, `{"by": "{{_js .user}}", "obj": {{json .obj}}}`},
	} {
		got, err := expandDollars(c.text, c.kind, known)
		if err != nil || got != c.want {
			t.Errorf("%q: got %q (%v), want %q", c.text, got, err, c.want)
		}
	}
	both := knownNames{vars: map[string]bool{"build": true}, steps: map[string]bool{"build": true}}
	if _, err := expandDollars("$build.output", plainText, both); err == nil || !strings.Contains(err.Error(), "both") {
		t.Errorf("a name both a variable and a step: %v", err)
	}
}

func TestARunLineReadsDollarNamesSafely(t *testing.T) {
	f := parse(t, `params: {who: {default: 'a "b" $(rm -rf x)'}}
steps:
  - id: greet
    run: printf '%s|' $who "$who" '$who'
  - run: echo "$greet.output"
`)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	r, out := testRunner(t, nil)
	if result := r.Run(f, nil); !result.OK {
		t.Fatalf("run failed:\n%s", out)
	}
	want := `a "b" $(rm -rf x)|a "b" $(rm -rf x)|a "b" $(rm -rf x)|`
	if strings.Count(out.String(), want) != 2 {
		t.Errorf("each $who is the value as typed, and so is the step's output:\n%s", out)
	}
}

func TestAParamDefaultPutsDollarNamesInJSON(t *testing.T) {
	f := parse(t, `params:
  user: {default: 'say "hi"'}
  login: {type: object, default: '{"user": "$user", "again": $user}'}
steps: [{run: 'true'}]
`)
	x, err := (&Runner{}).start(f, nil)
	if want := `{"user": "say \"hi\"", "again": "say \"hi\""}`; err != nil || x.vars["login"] != want {
		t.Errorf("login = %s (%v), want %s", x.vars["login"], err, want)
	}
}
