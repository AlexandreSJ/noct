package flow

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/script"
)

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
		for _, text := range s.orderedTemplates() {
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

// orderedTemplates are the texts of the step rendered before it runs, in a stable order, without
// the repeat's until, which reads the scope at the end of a pass.
func (s Step) orderedTemplates() []string {
	texts := []string{s.Run, s.If, s.Prompt, s.Default}
	if s.Agent != nil {
		texts = append(texts, s.Agent.Ask, s.Agent.Use)
	}
	for _, values := range []map[string]string{s.With, s.Set} {
		for _, name := range slices.Sorted(maps.Keys(values)) {
			texts = append(texts, values[name])
		}
	}
	return texts
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
	case !slices.Contains(s.fields(lookup, f.functions), ref.field):
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
	case sc.ended[name]:
		return "variable " + name + " was made inside an if or repeat and went when it ended; export it to read it after"
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
	script.WalkFields(t.Root, func(ident []string) {
		switch {
		case len(ident) >= 3 && ident[0] == "steps":
			refs = append(refs, reference{step: ident[1], field: ident[2]})
		case ident[0] != "steps":
			refs = append(refs, reference{field: ident[0]})
		}
	})
	return refs
}
