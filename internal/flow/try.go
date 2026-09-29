package flow

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"
)

// TriedOutput is how much of each step's output a test of some steps keeps.
const TriedOutput = 4 << 10

// RunSteps tests the steps from first to last, which are in the same list, as if the flow had
// run up to them: templates read the values of previous, the flow's last result. The result is
// previous with those steps, and the steps inside them, replaced and marked as tried.
func (r *Runner) RunSteps(f Flow, params map[string]string, first, last string, previous Result) (Result, error) {
	parent, branch, from := SplitPath(first)
	lastParent, lastBranch, to := SplitPath(last)
	steps := f.list(parent, branch)
	switch {
	case lastParent != parent || lastBranch != branch:
		return Result{}, fmt.Errorf("steps %s and %s are not in the same list", first, last)
	case steps == nil || from < 0 || to >= len(*steps) || from > to:
		return Result{}, fmt.Errorf("no steps from %s to %s", first, last)
	}
	tried := *r
	tried.KeepOutput = TriedOutput
	x := tried.start(f, params)
	x.remember(f, previous)
	x.rememberVariables(f, parent, branch, from, previous)
	for i := from; i <= to; i++ {
		if !x.runStep((*steps)[i], Path(parent, branch, i)) {
			break
		}
	}
	x.result.End = time.Now()
	return merge(previous, x.result, parent, branch, from, to), nil
}

// remember gives the templates the values the steps had in a result.
func (x *running) remember(f Flow, r Result) {
	for path, step := range r.Steps {
		s, ok := f.Step(path)
		if !ok || s.ID == "" || (s.Kind() != ScriptStep && s.Kind() != CommandStep) {
			continue
		}
		values := map[string]string{"output": step.Output, "exit": strconv.Itoa(step.Exit), "ok": strconv.FormatBool(step.Status == StatusOK)}
		maps.Copy(values, step.Captures)
		x.steps[s.ID] = values
	}
}

// rememberVariables gives the variables the values that the steps before index set in a result,
// and clears the ones those steps unset.
func (x *running) rememberVariables(f Flow, parent, branch string, index int, r Result) {
	inScope := map[string]bool{}
	for _, v := range f.Scope(parent, branch, index, x.Lookup) {
		if v.Step != "" {
			continue
		}
		inScope[v.Field] = true
		if value, set := r.Steps[v.Path].Captures[v.Field]; v.Path != "" && set {
			x.vars[v.Field] = value
		}
	}
	for name := range x.vars {
		if !inScope[name] {
			delete(x.vars, name)
		}
	}
}

// merge puts the steps a test ran into the previous result, in place of what they, and the steps
// inside them, did before. A flow that never ran takes the test's result as its own.
func merge(previous, tried Result, parent, branch string, from, to int) Result {
	for path, step := range tried.Steps {
		step.Tried = true
		tried.Steps[path] = step
	}
	if previous.Steps == nil {
		return tried
	}
	merged := previous
	merged.Steps = maps.Clone(previous.Steps)
	for i := from; i <= to; i++ {
		ran := Path(parent, branch, i)
		for path := range merged.Steps {
			if path == ran || strings.HasPrefix(path, ran+"/") {
				delete(merged.Steps, path)
			}
		}
	}
	maps.Copy(merged.Steps, tried.Steps)
	return merged
}
