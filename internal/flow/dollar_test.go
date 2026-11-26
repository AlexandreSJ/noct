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
	// What would run a command is refused: a word in $(( )), which the shell evaluates, and a
	// line that would end a heredoc, after which the shell runs what follows.
	for text, value := range map[string]string{
		`echo $(( $user + 1 ))`:          "HOME",
		"cat <<EOF\n$user\nEOF\n":        "a\nEOF\nrm -rf x",
		"cat <<-'EOF'\n\t$user\n\tEOF\n": "a\n\tEOF\nrm -rf x",
	} {
		vars := map[string]string{"user": value}
		if got, err := renderText(text, shellText, known, vars, nil, nil); err == nil {
			t.Errorf("%q with user %q rendered %q", text, value, got)
		}
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

// A $( ) reads its text as a command line of its own, inside quotes or a heredoc too, so a value
// in it is a word of that line; ` ` ends at the value's first `, so a value there is refused.
func TestAValueInACommandSubstitutionStaysOneWord(t *testing.T) {
	known := knownNames{vars: map[string]bool{"f": true, "n": true}}
	vars := map[string]string{"f": "x; touch PWNED", "n": "2"}
	for text, want := range map[string]string{
		`echo "$(cat $f) at $f"`:         `echo "$(cat 'x; touch PWNED') at x; touch PWNED"`,
		`echo "$(cat "$f" $(ls $f)) $f"`: `echo "$(cat "x; touch PWNED" $(ls 'x; touch PWNED')) x; touch PWNED"`,
		`echo "$(( $n + 1 )) $f"`:        `echo "$(( 2 + 1 )) x; touch PWNED"`,
		"cat <<EOF\n$(cat $f) $f\nEOF\n": "cat <<EOF\n$(cat 'x; touch PWNED') x; touch PWNED\nEOF\n",
		"cat <<EOF\n\\$(cat $f)\nEOF\n":  "cat <<EOF\n\\$(cat x; touch PWNED)\nEOF\n",
		"cat <<'EOF'\n$(cat $f)\nEOF\n":  "cat <<'EOF'\n$(cat x; touch PWNED)\nEOF\n",
		`echo '$(' $f`:                   `echo '$(' 'x; touch PWNED'`,
	} {
		got, err := renderText(text, shellText, known, vars, nil, nil)
		if err != nil || got != want {
			t.Errorf("%q: got %q (%v), want %q", text, got, err, want)
		}
	}
	for _, text := range []string{"echo `cat $f`", "echo \"`cat ${f}`\"", "cat <<EOF\n`cat $f`\nEOF\n"} {
		if got, err := splitText(text, shellText, known); err == nil || !strings.Contains(err.Error(), "$( )") {
			t.Errorf("%q split into %v (%v), want it refused", text, got, err)
		}
	}
}

// Where the shell reads a word as arithmetic, it runs the $( ) and ` of the word's text, quoted or
// not: a number goes in, and so does a text without $ or `, as an array's key; any other is
// refused. In (( )), as in $(( )), only a number.
func TestAValueTheShellReadsAsArithmeticRunsNothing(t *testing.T) {
	known := knownNames{vars: map[string]bool{"n": true, "k": true, "cmd": true, "bad": true}}
	vars := map[string]string{"n": "5", "k": "name", "cmd": "$(touch PWNED)", "bad": "a[$(touch PWNED)]"}
	for text, want := range map[string]string{
		`[[ $n -gt 3 ]]`:                     `[[ '5' -gt 3 ]]`,
		`(( $n > 1 ))`:                       `(( 5 > 1 ))`,
		`for ((i=0; i<$n; i++)); do :; done`: `for ((i=0; i<5; i++)); do :; done`,
		`echo ${arr[$k]}`:                    `echo ${arr['name']}`,
		`let c=$n+1`:                         `let c='5'+1`,
		`[[ $cmd == x ]]`:                    `[[ '$(touch PWNED)' == x ]]`,
		`echo ${x:-$k} ${s:$n}`:              `echo ${x:-'name'} ${s:'5'}`,
		`declare -i c; d=$cmd`:               `declare -i c; d='$(touch PWNED)'`,
	} {
		got, err := renderText(text, shellText, known, vars, nil, nil)
		if err != nil || got != want {
			t.Errorf("%q: got %q (%v), want %q", text, got, err, want)
		}
	}
	for _, text := range []string{
		`[[ $cmd -eq 1 ]]`, `[[ "$cmd" -lt 1 ]]`, `[ 1 -ne $cmd ]`, `(( $k > 1 ))`, `let c=$cmd`,
		`declare -i c=$cmd`, `local -ri c=${cmd}`, `echo ${arr[$cmd]}`, `arr[$cmd]=1`,
		`echo ${s:$cmd}`, `echo "${s:1:$cmd}"`, `arr=([$cmd]=1)`, `echo $[$cmd + 1]`,
		`integer c=$cmd`, `declare -i c; c=$cmd`, `typeset -i c=1; c+=$cmd`, `echo ${x:-$cmd}`,
		`[[ 1 -eq 0$cmd ]]`, `[[ x$cmd -eq 1 ]]`, `declare -i c; c=1$cmd`,
		// a[$( )] runs where the shell reads a name too, so it is refused anywhere
		`[[ -v $bad ]]`, `printf -v $bad x`, `echo "$bad"`, `echo '$bad'`, `echo $bad`,
	} {
		if got, err := renderText(text, shellText, known, vars, nil, nil); err == nil {
			t.Errorf("%q rendered %q, want it refused", text, got)
		}
	}
}
