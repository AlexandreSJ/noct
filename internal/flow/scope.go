package flow

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	tparse "text/template/parse"

	"github.com/AlexandreSJ/noct/internal/script"
)

// Certainty tells whether a variable is set when a point of the flow is reached.
type Certainty int

const (
	// Set is always set: its step ran before the point.
	Set Certainty = iota
	// ThisPass is set by a step earlier in the pass of the repeat around the point; each pass
	// forgets the one before.
	ThisPass
	// Maybe is set only when the branch of an earlier if that holds its step ran.
	Maybe
)

// Variable is a value a template can read at a point of the flow: a parameter, a variable a set
// or prompt step gives, or a field of a step that runs before the point.
type Variable struct {
	// Step is the id of the step, or "" for a variable.
	Step string
	// Path is where the step is; for a variable, the step that last set it, or "" for a
	// parameter's default.
	Path string
	// Field is output, exit, ok, a capture or one of the script's outputs; a variable's name.
	Field     string
	Certainty Certainty
}

// Template is how a template reads the variable.
func (v Variable) Template() string {
	if v.Step == "" {
		return "{{." + v.Field + "}}"
	}
	return "{{.steps." + v.Step + "." + v.Field + "}}"
}

// Scope lists what a template can read at index of parent's branch (parent "" for the flow's own
// steps): the parameters, then the fields of the steps that run before that point, in order.
func (f Flow) Scope(parent, branch string, index int, lookup func(id string) (script.Script, bool)) []Variable {
	return f.scope(parent, branch, index, lookup).vars
}

func (f Flow) scope(parent, branch string, index int, lookup func(id string) (script.Script, bool)) *scope {
	sc := &scope{cleared: map[string]bool{}, lookup: lookup}
	for _, name := range slices.Sorted(maps.Keys(f.Params)) {
		sc.vars = append(sc.vars, Variable{Field: name})
	}
	levels := f.levelsTo(parent, branch, index)
	innermostRepeat := -1
	for i, l := range levels {
		if l.repeatBody {
			innermostRepeat = i
		}
	}
	for i, l := range levels {
		certainty := Set
		if i >= innermostRepeat && innermostRepeat >= 0 {
			certainty = ThisPass
		}
		for j, s := range l.steps[:min(l.index, len(l.steps))] {
			sc.add(s, Path(l.parent, l.branch, j), certainty)
		}
	}
	return sc
}

// scope is what is readable at a point, built step by step, and what unset steps cleared on the
// way there.
type scope struct {
	vars []Variable
	// cleared holds the names and steps.<id> unset before the point and not set again.
	cleared map[string]bool
	lookup  func(id string) (script.Script, bool)
}

// level is one list of steps on the way from the flow's own steps to a point, and the index the
// way goes on at.
type level struct {
	steps          []Step
	parent, branch string
	index          int
	repeatBody     bool
}

func (f Flow) levelsTo(parent, branch string, index int) []level {
	if parent == "" {
		return []level{{steps: f.Steps, index: index}}
	}
	up, upBranch, at := SplitPath(parent)
	levels := f.levelsTo(up, upBranch, at)
	s, ok := f.Step(parent)
	if !ok {
		return levels
	}
	list := map[string][]Step{"then": s.Then, "else": s.Else, "steps": s.Steps}[branch]
	return append(levels, level{steps: list, parent: parent, branch: branch, index: index, repeatBody: branch == "steps"})
}

// add takes in what a step that ran does to the scope, and what the steps inside it do: an if's
// branches may not have run, and a repeat's steps ran at least once.
func (sc *scope) add(s Step, path string, certainty Certainty) {
	if s.ID != "" {
		delete(sc.cleared, stepsPrefix+s.ID)
		for _, field := range s.fields(sc.lookup) {
			sc.vars = append(sc.vars, Variable{Step: s.ID, Path: path, Field: field, Certainty: certainty})
		}
	}
	switch s.Kind() {
	case SetStep:
		for _, name := range slices.Sorted(maps.Keys(s.Set)) {
			sc.set(name, path, certainty)
		}
	case PromptStep:
		sc.set(s.Var, path, certainty)
	case UnsetStep:
		for _, name := range s.Unset {
			sc.unset(name, certainty)
		}
	}
	inner := []struct {
		branch    string
		steps     []Step
		certainty Certainty
	}{{"then", s.Then, Maybe}, {"else", s.Else, Maybe}, {"steps", s.Steps, certainty}}
	for _, list := range inner {
		for i, child := range list.steps {
			sc.add(child, Path(path, list.branch, i), list.certainty)
		}
	}
}

// set records a step giving a variable its value. A branch that may not run leaves a variable
// that was surely set as it was.
func (sc *scope) set(name, path string, certainty Certainty) {
	delete(sc.cleared, name)
	i := slices.IndexFunc(sc.vars, func(v Variable) bool { return v.Step == "" && v.Field == name })
	switch {
	case i < 0:
		sc.vars = append(sc.vars, Variable{Path: path, Field: name, Certainty: certainty})
	case certainty != Maybe || sc.vars[i].Certainty == Maybe:
		sc.vars[i].Path, sc.vars[i].Certainty = path, certainty
	}
}

// unset takes a variable, or a step's fields, out of the scope; in a branch that may not run,
// they only may not be there anymore.
func (sc *scope) unset(name string, certainty Certainty) {
	step, isStep := strings.CutPrefix(name, stepsPrefix)
	matches := func(v Variable) bool {
		if isStep {
			return v.Step == step
		}
		return v.Step == "" && v.Field == name
	}
	if certainty == Maybe {
		for i := range sc.vars {
			if matches(sc.vars[i]) {
				sc.vars[i].Certainty = Maybe
			}
		}
		return
	}
	sc.vars = slices.DeleteFunc(sc.vars, matches)
	sc.cleared[name] = true
}

// fields are what later templates read from a step: output, exit and ok, the captures, and the
// outputs its script declares. Only script and run steps have them.
func (s Step) fields(lookup func(id string) (script.Script, bool)) []string {
	kind := s.Kind()
	if kind != ScriptStep && kind != CommandStep {
		return nil
	}
	fields := []string{"output", "exit", "ok"}
	named := slices.Sorted(maps.Keys(s.Capture))
	if lookup != nil && kind == ScriptStep {
		if sc, ok := lookup(s.Script); ok {
			named = append(named, sc.Outputs...)
		}
	}
	for _, name := range named {
		if !slices.Contains(fields, name) {
			fields = append(fields, name)
		}
	}
	return fields
}

// Warning is a template that reads a step it may not find: one that does not exist, has not run
// yet at that point, may not have run, or lacks the field.
type Warning struct {
	// Path is the step whose template reads it.
	Path string
	Text string
}

// Warnings checks what every template of the flow reads from the steps. They are warnings, not
// problems: a flow is built top down, and a condition is often written before the step it reads.
func (f Flow) Warnings(lookup func(id string) (script.Script, bool)) []Warning {
	return f.listWarnings(f.Steps, "", "", lookup)
}

func (f Flow) listWarnings(steps []Step, parent, branch string, lookup func(id string) (script.Script, bool)) []Warning {
	var warnings []Warning
	for i, s := range steps {
		path := Path(parent, branch, i)
		before := f.scope(parent, branch, i, lookup)
		texts := []string{s.Run, s.If, s.Prompt, s.Default}
		for _, values := range []map[string]string{s.With, s.Set} {
			for _, name := range slices.Sorted(maps.Keys(values)) {
				texts = append(texts, values[name])
			}
		}
		for _, text := range texts {
			warnings = append(warnings, f.check(path, text, before)...)
		}
		if s.Repeat != nil {
			warnings = append(warnings, f.check(path, s.Repeat.Until, f.scope(path, "steps", len(s.Steps), lookup))...)
		}
		for _, inner := range []struct {
			branch string
			steps  []Step
		}{{"then", s.Then}, {"else", s.Else}, {"steps", s.Steps}} {
			warnings = append(warnings, f.listWarnings(inner.steps, path, inner.branch, lookup)...)
		}
	}
	return warnings
}

// check warns about each variable or step a template reads that is not surely in scope.
func (f Flow) check(path, text string, sc *scope) []Warning {
	var warnings []Warning
	for _, ref := range references(text) {
		i := slices.IndexFunc(sc.vars, func(v Variable) bool { return v.Step == ref.step && v.Field == ref.field })
		var problem string
		switch {
		case i >= 0 && sc.vars[i].Certainty == Maybe && ref.step == "":
			problem = "variable " + ref.field + " may not be set here"
		case i >= 0 && sc.vars[i].Certainty == Maybe:
			problem = "step " + ref.step + " may not have run here"
		case i >= 0:
			continue
		case ref.step == "":
			problem = f.missingVariable(ref.field, sc)
		case sc.cleared[stepsPrefix+ref.step]:
			problem = "step " + ref.step + " was unset before here"
		default:
			problem = f.missing(ref, sc.lookup)
		}
		warnings = append(warnings, Warning{Path: path, Text: problem})
	}
	return warnings
}

// missing tells why a step's field is not in scope: no such step, no such field, or the step
// runs later.
func (f Flow) missing(ref reference, lookup func(id string) (script.Script, bool)) string {
	s, found := f.stepWithID(f.Steps, ref.step)
	switch {
	case !found:
		return "there is no step " + ref.step
	case !slices.Contains(s.fields(lookup), ref.field):
		return fmt.Sprintf("step %s has no %s", ref.step, ref.field)
	}
	return "step " + ref.step + " has not run yet here"
}

// missingVariable tells why a variable is not in scope: it was unset, it is set only later, or
// nothing sets it.
func (f Flow) missingVariable(name string, sc *scope) string {
	switch {
	case sc.cleared[name]:
		return "variable " + name + " was unset before here"
	case f.setsVariable(f.Steps, name):
		return "variable " + name + " is not set yet here"
	}
	return "there is no variable " + name
}

// setsVariable tells whether a step among steps, or inside them, sets the variable.
func (f Flow) setsVariable(steps []Step, name string) bool {
	for _, s := range steps {
		if _, set := s.Set[name]; set || s.Kind() == PromptStep && s.Var == name {
			return true
		}
		for _, inner := range [][]Step{s.Then, s.Else, s.Steps} {
			if f.setsVariable(inner, name) {
				return true
			}
		}
	}
	return false
}

func (f Flow) stepWithID(steps []Step, id string) (Step, bool) {
	for _, s := range steps {
		if s.ID == id {
			return s, true
		}
		for _, inner := range [][]Step{s.Then, s.Else, s.Steps} {
			if found, ok := f.stepWithID(inner, id); ok {
				return found, true
			}
		}
	}
	return Step{}, false
}

// reference is a step's field a template reads, {{.steps.<step>.<field>}}, or a variable,
// {{.<field>}} with step "".
type reference struct {
	step, field string
}

// references lists the variables and steps' fields a template reads, in order; a template that does not parse
// reads none (Validate reports it).
func references(text string) []reference {
	if !strings.Contains(text, "{{") {
		return nil
	}
	t, err := parseTemplate(text)
	if err != nil {
		return nil
	}
	var refs []reference
	walkFields(t.Root, func(ident []string) {
		switch {
		case len(ident) >= 3 && ident[0] == "steps":
			refs = append(refs, reference{step: ident[1], field: ident[2]})
		case ident[0] != "steps":
			refs = append(refs, reference{field: ident[0]})
		}
	})
	return refs
}

// walkFields calls visit with the names of each field a template node reads, such as
// [steps pick color] for {{.steps.pick.color}}.
func walkFields(node tparse.Node, visit func(ident []string)) {
	switch n := node.(type) {
	case *tparse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			walkFields(child, visit)
		}
	case *tparse.ActionNode:
		walkFields(n.Pipe, visit)
	case *tparse.PipeNode:
		for _, command := range n.Cmds {
			walkFields(command, visit)
		}
	case *tparse.CommandNode:
		for _, arg := range n.Args {
			walkFields(arg, visit)
		}
	case *tparse.FieldNode:
		visit(n.Ident)
	case *tparse.IfNode:
		walkBranch(&n.BranchNode, visit)
	case *tparse.RangeNode:
		walkBranch(&n.BranchNode, visit)
	case *tparse.WithNode:
		walkBranch(&n.BranchNode, visit)
	}
}

func walkBranch(b *tparse.BranchNode, visit func(ident []string)) {
	walkFields(b.Pipe, visit)
	walkFields(b.List, visit)
	walkFields(b.ElseList, visit)
}
