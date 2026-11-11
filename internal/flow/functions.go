package flow

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/script"
)

// A function is steps any flow can call: a file of its own in the functions folder, with params,
// steps and returns, the values it gives back. A call step runs it in a scope of its own, with
// the params its with gives, and the returns become the call's captures. Changing a function
// changes every flow that calls it, and Callers lists them.

// maxCallDepth bounds functions calling functions, which also stops one calling itself forever.
const maxCallDepth = 16

// FunctionsDir is where the user's functions are kept, beside the flows.
func FunctionsDir() string {
	return filepath.Join(filepath.Dir(script.PersonalDir()), "functions")
}

// LoadFunctions reads the functions in dir as LoadAll reads flows.
func LoadFunctions(dir string, lookup func(id string) (script.Script, bool)) ([]Flow, error) {
	functions, err := LoadAll(dir, lookup)
	for i := range functions {
		functions[i].Function = true
	}
	return functions, err
}

// CreateFunction writes a new, empty function called name in dir.
func CreateFunction(dir, name string) (Flow, error) {
	if !naming.IsValid(name) {
		return Flow{}, fmt.Errorf("function name %q: use letters, digits, . - _", name)
	}
	f := Flow{Name: name, File: filepath.Join(dir, name+Ext), Function: true}
	if _, err := os.Stat(f.File); err == nil {
		return Flow{}, fmt.Errorf("function %s already exists", name)
	}
	return f, f.Save()
}

// WithFunctions lets the flow know the functions its call steps name, for its scope and checks.
func (f *Flow) WithFunctions(lookup func(name string) (Flow, bool)) {
	f.functions = lookup
}

// CheckCalls reports a call to a function that is not there, a with for a param it does not
// have, or a required param left out.
func (f Flow) CheckCalls(functions []Flow) error {
	var problem error
	walkSteps(f.Steps, "", "", func(s Step, path string) {
		if problem != nil || s.Kind() != CallStep {
			return
		}
		fn, ok := Find(functions, s.Call)
		switch {
		case !ok:
			problem = fmt.Errorf("step %s calls %s, which is not a function", s.Label(), s.Call)
			return
		case fn.Name == f.Name && f.Function:
			problem = fmt.Errorf("function %s calls itself", f.Name)
			return
		}
		for name := range s.With {
			if _, known := fn.Params[name]; !known {
				problem = fmt.Errorf("step %s gives %s, which function %s has no param for", s.Label(), name, fn.Name)
				return
			}
		}
		for _, name := range slices.Sorted(maps.Keys(fn.Params)) {
			if p := fn.Params[name]; p.Required && p.Default == "" && s.With[name] == "" {
				problem = fmt.Errorf("step %s leaves out %s, which function %s needs", s.Label(), name, fn.Name)
				return
			}
		}
	})
	return problem
}

// Callers are the names of the flows and functions whose steps call the function name.
func Callers(flows []Flow, name string) []string {
	var callers []string
	for _, f := range flows {
		calls := false
		walkSteps(f.Steps, "", "", func(s Step, _ string) { calls = calls || s.Call == name })
		if calls {
			callers = append(callers, f.Name)
		}
	}
	return callers
}

// walkSteps visits each step of the list at parent's branch and the ones inside them, with its
// path.
func walkSteps(steps []Step, parent, branch string, visit func(s Step, path string)) {
	for i, s := range steps {
		path := Path(parent, branch, i)
		visit(s, path)
		walkSteps(s.Then, path, "then", visit)
		walkSteps(s.Else, path, "else", visit)
		walkSteps(s.Steps, path, "steps", visit)
	}
}

// runCall runs the function the step names in a scope of its own: its params from the step's
// with, rendered here, and its own steps and variables. What it returns becomes the step's
// captures, and its short variables here.
func (x *running) runCall(s Step, path string) bool {
	fn, ok := x.lookupFunction(s.Call)
	switch {
	case !ok:
		return x.stop(path, "there is no function "+s.Call, 1)
	case x.depth >= maxCallDepth:
		return x.stop(path, fmt.Sprintf("functions call each other more than %d deep", maxCallDepth), 1)
	}
	values := fn.Values(x.getenv)
	for _, name := range slices.Sorted(maps.Keys(s.With)) {
		value, err := x.render(s.With[name])
		if err != nil {
			return x.stop(path, "with "+name+": "+err.Error(), 1)
		}
		values[name] = value
	}
	if err := fn.CheckParams(values); err != nil {
		return x.stop(path, "function "+fn.Name+": "+err.Error(), 1)
	}
	x.header("ƒ", s.Label(), "call "+fn.Name)
	start := time.Now()
	caller := *x
	x.vars, x.steps, x.blocks = values, map[string]map[string]string{}, nil
	x.depth++
	ran := x.runSteps(fn.Steps, path, "call")
	returns := map[string]string{}
	var problem error
	for _, name := range slices.Sorted(maps.Keys(fn.Returns)) {
		if !ran {
			break
		}
		if returns[name], problem = x.render(fn.Returns[name]); problem != nil {
			break
		}
	}
	x.vars, x.steps, x.blocks, x.depth = caller.vars, caller.steps, caller.blocks, caller.depth
	if !ran {
		x.result.Steps[path] = StepResult{Status: StatusFailed, Exit: x.result.Exit, Duration: time.Since(start), Message: "function " + fn.Name + " stopped"}
		return false
	}
	if problem != nil {
		return x.stop(path, "returns of "+fn.Name+": "+problem.Error(), 1)
	}
	result := StepResult{Status: StatusOK, Duration: time.Since(start), Captures: returns}
	x.result.Steps[path] = result
	x.keepStepValues(s.ID, result, "", true)
	return true
}

func (x *running) lookupFunction(name string) (Flow, bool) {
	if x.Function == nil {
		return Flow{}, false
	}
	return x.Function(name)
}

// LoadWithFunctions reads the flows in flowsDir and the functions in functionsDir, each knowing
// the functions, and gives a problem to one whose calls do not fit them.
func LoadWithFunctions(flowsDir, functionsDir string, lookup func(id string) (script.Script, bool)) (flows, functions []Flow, err error) {
	flows, err = LoadAll(flowsDir, lookup)
	functions, ferr := LoadFunctions(functionsDir, lookup)
	find := func(name string) (Flow, bool) { return Find(functions, name) }
	for _, list := range [][]Flow{flows, functions} {
		for i := range list {
			list[i].WithFunctions(find)
			if err := list[i].CheckCalls(functions); err != nil && list[i].Problem == nil {
				list[i].Problem = fmt.Errorf("%s %s: %w", map[bool]string{true: "function", false: "flow"}[list[i].Function], list[i].Name, err)
			}
		}
	}
	return flows, functions, errors.Join(err, ferr)
}
