package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestSwitchRunsTheFirstCaseThatHoldsTheValue(t *testing.T) {
	f := parse(t, `
params: { kind: "" }
steps:
  - switch: '{{.kind}}'
    cases:
      - is: flaky
        steps: [ { run: echo ran-flaky } ]
      - is: [bug, regression]
        steps: [ { run: echo ran-bug } ]
    else: [ { run: echo ran-else } ]
`)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	for kind, want := range map[string]string{"flaky": "ran-flaky", "regression": "ran-bug", " bug\n": "ran-bug", "env": "ran-else"} {
		r, out := testRunner(t, nil)
		result := r.Run(f, map[string]string{"kind": kind})
		if !result.OK || strings.Count(out.String(), "\nran-") != 1 || !strings.Contains(out.String(), want) {
			t.Errorf("kind %q: want only %s:\n%s", kind, want, out)
		}
	}
	r, _ := testRunner(t, nil)
	result := r.Run(f, map[string]string{"kind": "bug"})
	if _, ran := result.Steps["0/case1/0"]; !ran {
		t.Errorf("the second case's step has no result: %v", result.Steps)
	}
	if got := result.Steps["0"].Message; got != `"bug"` {
		t.Errorf("switch message = %q", got)
	}
}

func TestSwitchValidation(t *testing.T) {
	bad := map[string]string{
		"no cases":         `steps: [ { switch: x } ]`,
		"empty is":         `steps: [ { switch: x, cases: [ { steps: [ { run: ls } ] } ] } ]`,
		"a value twice":    `steps: [ { switch: x, cases: [ { is: a }, { is: [b, a] } ] } ]`,
		"cases on an if":   `steps: [ { if: 'true', cases: [ { is: a } ] } ]`,
		"then on a switch": `steps: [ { switch: x, cases: [ { is: a } ], then: [ { run: ls } ] } ]`,
		"bad step inside":  `steps: [ { switch: x, cases: [ { is: a, steps: [ { wait: soon } ] } ] } ]`,
	}
	for name, text := range bad {
		if err := parse(t, text).Validate(nil); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// A single value is written alone, and the flow saved reads the same.
func TestSwitchAndRetryKeepTheirShortFormsWhenSaved(t *testing.T) {
	f := parse(t, `
steps:
  - { run: ls, retry: 2 }
  - { run: ls, retry: { max: 3, every: 1s } }
  - switch: x
    cases:
      - is: a
      - is: [b, c]
`)
	if f.Steps[0].Retry == nil || f.Steps[0].Retry.Max != 2 || f.Steps[1].Retry.Every != "1s" {
		t.Fatalf("retry = %+v, %+v", f.Steps[0].Retry, f.Steps[1].Retry)
	}
	f.File = filepath.Join(t.TempDir(), "saved"+Ext)
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	text, _ := os.ReadFile(f.File)
	for _, want := range []string{"retry: 2\n", "every: 1s", "is: a\n", "- b\n"} {
		if !strings.Contains(string(text), want) {
			t.Errorf("saved flow lacks %q:\n%s", want, text)
		}
	}
	var again Flow
	if err := yaml.Unmarshal(text, &again); err != nil || len(again.Steps[2].Cases[1].Is) != 2 {
		t.Errorf("saved flow reads back as %+v, %v", again.Steps, err)
	}
}

func TestRetryRunsAFailedStepAgainBeforeOnFail(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
steps:
  - id: flaky
    run: 'n=$(($(cat count 2>/dev/null || echo 0) + 1)); echo $n > count; [ $n -ge 3 ]'
    retry: { max: 3, every: 1s }
  - { run: 'exit 5', retry: 1, on_fail: continue }
  - { run: echo after }
`)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	var slept []time.Duration
	r.Sleep = func(d time.Duration) { slept = append(slept, d) }
	result := r.Run(f, nil)
	if len(slept) != 2 || slept[0] != time.Second {
		t.Errorf("retry waits every before each try: %v", slept)
	}
	if !result.OK || result.Steps["0"].Status != StatusOK {
		t.Fatalf("the third try passes, so the flow goes on:\n%s", out)
	}
	if got := strings.Count(out.String(), "retry 1 of 1"); got != 1 {
		t.Errorf("the second step retries once, then continues: %d\n%s", got, out)
	}
	if !strings.Contains(out.String(), "retry 2 of 3") || strings.Contains(out.String(), "retry 3 of 3") {
		t.Errorf("want two retries of the first step:\n%s", out)
	}
	if !strings.Contains(out.String(), "after") {
		t.Errorf("on_fail continue after the last retry:\n%s", out)
	}
}

func TestRetryGivesUpAfterMax(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
steps:
  - { run: 'echo try >> tries; exit 2', retry: 2 }
`)
	result := r.Run(f, nil)
	tries, _ := os.ReadFile(filepath.Join(r.Dir, "tries"))
	if result.OK || result.Exit != 2 || strings.Count(string(tries), "try") != 3 {
		t.Fatalf("want 3 tries and a stop with exit 2, got %+v, tries %q:\n%s", result, tries, out)
	}
	for _, text := range []string{`steps: [ { run: ls, retry: 0 } ]`, `steps: [ { wait: 1s, retry: 2 } ]`, `steps: [ { run: ls, retry: { max: 1, every: soon } } ]`} {
		if err := parse(t, text).Validate(nil); err == nil {
			t.Errorf("%s: no error", text)
		}
	}
}

func TestRepeatEachRunsOncePerLine(t *testing.T) {
	r, out := testRunner(t, nil)
	f := parse(t, `
params: { file: outer }
steps:
  - id: list
    run: printf 'a.txt\n\n  b.txt \nc.txt\n'
  - repeat: { each: '{{.steps.list.output}}', as: file, until: '{{eq .file "b.txt"}}' }
    steps:
      - run: 'echo got-{{.file}}'
  - run: 'echo then-{{.file}}'
  - repeat: { each: '{{.steps.list.output}}' }
    steps: [ { run: 'echo item-{{.item}}' } ]
`)
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	if w := f.Warnings(nil); len(w) > 0 {
		t.Errorf("warnings: %v", w)
	}
	result := r.Run(f, nil)
	text := out.String()
	if !result.OK {
		t.Fatalf("run failed:\n%s", text)
	}
	for _, want := range []string{"got-a.txt", "got-b.txt", "then-outer", "item-a.txt", "item-b.txt", "item-c.txt"} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %s:\n%s", want, text)
		}
	}
	if strings.Contains(text, "got-c.txt") {
		t.Errorf("until ends the repeat each early:\n%s", text)
	}
	if strings.Contains(text, "item-\n") {
		t.Errorf("an empty line runs no pass:\n%s", text)
	}
}

func TestRepeatEachValidationAndScope(t *testing.T) {
	for _, text := range []string{
		`steps: [ { repeat: { each: x, times: 2 }, steps: [ { run: ls } ] } ]`,
		`steps: [ { repeat: { times: 2, as: x }, steps: [ { run: ls } ] } ]`,
		`steps: [ { repeat: { each: x, as: 'not-a-name' }, steps: [ { run: ls } ] } ]`,
	} {
		if err := parse(t, text).Validate(nil); err == nil {
			t.Errorf("%s: no error", text)
		}
	}
	f := parse(t, `
steps:
  - repeat: { each: 'a', as: host }
    steps: [ { run: 'echo {{.host}}' } ]
  - run: 'echo {{.host}}'
`)
	warnings := f.Warnings(nil)
	if len(warnings) != 1 || warnings[0].Path != "1" {
		t.Errorf("only the read after the repeat warns: %v", warnings)
	}
	vars := f.Scope("0", "steps", 0, nil)
	if !containsVariable(vars, "host") {
		t.Errorf("host is in scope inside: %v", vars)
	}
}

func containsVariable(vars []Variable, name string) bool {
	for _, v := range vars {
		if v.Step == "" && v.Field == name {
			return true
		}
	}
	return false
}

// A variable every case and the else set is set after the switch, as after an if.
func TestSwitchScope(t *testing.T) {
	f := parse(t, `
steps:
  - switch: x
    cases:
      - is: a
        steps: [ { set: { note: one, only: a } } ]
      - is: b
        steps: [ { set: { note: two } } ]
    else: [ { set: { note: three } } ]
    export: { note: "", only: "" }
  - run: 'echo {{.note}}'
  - run: 'echo {{.only}}'
`)
	warnings := f.Warnings(nil)
	if len(warnings) != 1 || warnings[0].Path != "2" || !strings.Contains(warnings[0].Text, "may not") {
		t.Errorf("want one warning, only may not be set: %v", warnings)
	}
}

// J and K move a step through a switch's cases one after the other, as through an if's branches.
func TestShiftGoesThroughTheCases(t *testing.T) {
	f := parse(t, `
steps:
  - { id: m, run: ls }
  - switch: x
    cases:
      - is: a
      - is: b
        steps: [ { id: inside, run: ls } ]
`)
	path := "0"
	var visited []string
	for range 6 {
		path = f.Shift(path, true)
		visited = append(visited, path)
	}
	want := []string{"0/case0/0", "0/case1/0", "0/case1/1", "0/else/0", "1", "1"}
	if strings.Join(visited, " ") != strings.Join(want, " ") {
		t.Errorf("down: %v, want %v", visited, want)
	}
	path = f.Shift(path, false)
	if path != "0/else/0" {
		t.Errorf("up from below goes into the else: %s", path)
	}
}
