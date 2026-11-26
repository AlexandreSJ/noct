package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDollarNamesAreTheFlowsAndEscapedWhereTheyLand(t *testing.T) {
	known := knownNames{vars: map[string]bool{"user": true, "obj": true, "f": true, "hosts": true, "n": true}, steps: map[string]bool{"build": true}}
	vars := map[string]string{"user": `it's "me"`, "obj": `{"pass": "x"}`, "f": "file", "hosts": `["a","b"]`, "n": "2"}
	jsonVars := map[string]string{"obj": ObjectParam, "hosts": ListParam}
	steps := map[string]map[string]string{"build": {"output": "done"}}
	for _, c := range []struct {
		kind       textKind
		text, want string
	}{
		{plainText, "hi $user.", `hi it's "me".`},
		{plainText, "$obj.pass and $build.output", "x and done"},
		{plainText, "$HOME ${HOME} $$user $${user} $5 ${1} {{$x := 1}} {{.Names}}", "$HOME ${HOME} $user ${user} $5 ${1} {{$x := 1}} {{.Names}}"},
		{plainText, "${user}s ${n + 1} ${upper(f)}", `it's "me"s 3 FILE`},
		{shellText, `echo key=$n ${n}0`, `echo key='2' '2'0`},
		{plainText, `$hosts[1] $len(hosts) $join(map(hosts, h => upper(h)), "+")`, "b 2 A+B"},
		{plainText, "$steps.build.output", "done"},
		{shellText, `echo $user "at $user" '$user'`, `echo 'it'\''s "me"' "at it's \"me\"" 'it'\''s "me"'`},
		{shellText, `echo "it's $f" \$user`, `echo "it's file" \$user`},
		{shellText, `for f in *; do echo $f $n; done`, `for f in *; do echo $f '2'; done`},
		{shellText, `echo $(( $n + 3 )) $n`, `echo $(( 2 + 3 )) '2'`},
		{shellText, `echo $user;`, `echo 'it'\''s "me"';`},
		{shellText, "cat <<'EOF' | wc -l\n$user $n\nEOF\necho $n", "cat <<'EOF' | wc -l\nit's \"me\" 2\nEOF\necho '2'"},
		{shellText, "cat <<-EOF\n\t$f costs \\$5 ${n}\n\tEOF\n", "cat <<-EOF\n\tfile costs \\$5 2\n\tEOF\n"},
		{jsonText, `{"by": "$user", "obj": $obj, "n": $n}`, `{"by": "it's \"me\"", "obj": {"pass":"x"}, "n": "2"}`},
	} {
		got, err := renderText(c.text, c.kind, known, vars, jsonVars, steps)
		if err != nil || got != c.want {
			t.Errorf("%q: got %q (%v), want %q", c.text, got, err, c.want)
		}
	}
	both := knownNames{vars: map[string]bool{"build": true}, steps: map[string]bool{"build": true}}
	if _, err := splitText("$build.output", plainText, both); err == nil || !strings.Contains(err.Error(), "both") {
		t.Errorf("a name both a variable and a step: %v", err)
	}
	if _, err := splitText("$join(hosts", plainText, known); err == nil {
		t.Error("a call left open is told")
	}
}

func TestOldTemplatesAreTold(t *testing.T) {
	f := parse(t, `variables: {who: {default: x}}
steps:
  - run: echo {{quote .who}}
`)
	if err := f.Validate(nil); err == nil || !strings.Contains(err.Error(), "write $who") {
		t.Errorf("an old template is told: %v", err)
	}
	f = parse(t, `steps:
  - run: docker ps --format '{{.Names}}'
`)
	if err := f.Validate(nil); err != nil {
		t.Errorf("a program's own {{ }} is text: %v", err)
	}
	f = parse(t, `variables: {ok2: {default: x}}
steps:
  - if: '{{eq .ok2 "x"}}'
    then: [{run: 'true'}]
`)
	if err := f.Validate(nil); err == nil || !strings.Contains(err.Error(), "{{ }} is gone") {
		t.Errorf("an old condition is told: %v", err)
	}
	file := filepath.Join(t.TempDir(), "old"+Ext)
	if err := os.WriteFile(file, []byte("params: {a: {default: x}}\nsteps: [{run: 'true'}]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil || !strings.Contains(err.Error(), "variables") {
		t.Errorf("params is told to be variables: %v", err)
	}
}

func TestARunLineReadsDollarNamesSafely(t *testing.T) {
	f := parse(t, `variables: {who: {default: 'a "b" $(rm -rf x)'}}
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
	f := parse(t, `variables:
  user: {default: 'say "hi"'}
  login: {type: object, default: '{"user": "$user", "again": $user}'}
steps: [{run: 'true'}]
`)
	x, err := (&Runner{}).start(f, nil)
	if want := `{"user": "say \"hi\"", "again": "say \"hi\""}`; err != nil || x.vars["login"] != want {
		t.Errorf("login = %s (%v), want %s", x.vars["login"], err, want)
	}
}
