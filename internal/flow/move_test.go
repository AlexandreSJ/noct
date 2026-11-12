package flow

import (
	"strings"
	"testing"
)

// ids lists a flow's step ids as they nest: if(a,b|c) is an if with a and b in then, c in else.
func ids(steps []Step) string {
	var parts []string
	for _, s := range steps {
		switch {
		case s.Kind() == IfStep:
			parts = append(parts, s.ID+"("+ids(s.Then)+"|"+ids(s.Else)+")")
		case s.Kind() == RepeatStep:
			parts = append(parts, s.ID+"["+ids(s.Steps)+"]")
		default:
			parts = append(parts, s.ID)
		}
	}
	return strings.Join(parts, ",")
}

const shiftedFlow = `steps:
  - {id: a, run: 'true'}
  - id: when
    if: 'true'
    then: [{id: b, run: 'true'}]
    else: [{id: c, run: 'true'}]
  - id: loop
    repeat: {times: 2}
    steps: [{id: d, run: 'true'}]
  - {id: e, run: 'true'}
`

// Moving a step down goes through the flow as a line goes through code: into the if's yes branch,
// on to its no branch, out after it, into the loop, out of it.
func TestShiftingAStepGoesInAndOutOfBlocks(t *testing.T) {
	f := parse(t, shiftedFlow)
	path := "0"
	want := []string{
		"when(a,b|c),loop[d],e",
		"when(b,a|c),loop[d],e",
		"when(b|a,c),loop[d],e",
		"when(b|c,a),loop[d],e",
		"when(b|c),a,loop[d],e",
		"when(b|c),loop[a,d],e",
		"when(b|c),loop[d,a],e",
		"when(b|c),loop[d],a,e",
		"when(b|c),loop[d],e,a",
		"when(b|c),loop[d],e,a",
	}
	for i, w := range want {
		path = f.Shift(path, true)
		if got := ids(f.Steps); got != w {
			t.Fatalf("down %d: %s, want %s", i+1, got, w)
		}
		if s, _ := f.Step(path); s.ID != "a" {
			t.Fatalf("down %d: the path %s leads to %q, want a", i+1, path, s.ID)
		}
	}
	for i := len(want) - 3; i >= 0; i-- {
		path = f.Shift(path, false)
		if got := ids(f.Steps); got != want[i] {
			t.Fatalf("up back to %d: %s, want %s", i, got, want[i])
		}
	}
	path = f.Shift(path, false)
	if got := ids(f.Steps); got != "a,when(b|c),loop[d],e" {
		t.Errorf("up to the top: %s", got)
	}
}

func TestMovingAStepToAPlace(t *testing.T) {
	f := parse(t, shiftedFlow)
	to, err := f.MoveTo("0", "2", "steps", 1)
	if err != nil || ids(f.Steps) != "when(b|c),loop[d,a],e" || to != "1/steps/1" {
		t.Fatalf("moved into the loop: %s at %s, %v", ids(f.Steps), to, err)
	}
	if to, err = f.MoveTo("2", "", "", 0); err != nil || ids(f.Steps) != "e,when(b|c),loop[d,a]" || to != "0" {
		t.Fatalf("moved to the top: %s at %s, %v", ids(f.Steps), to, err)
	}
	if _, err := f.MoveTo("2", "2", "steps", 0); err == nil {
		t.Error("moved the loop into itself")
	}
}
