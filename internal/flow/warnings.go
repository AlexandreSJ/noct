package flow

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/expr"
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
		for _, r := range s.reads() {
			warnings = append(warnings, f.check(path, r, before)...)
		}
		if s.Repeat != nil && s.Repeat.Until != "" {
			warnings = append(warnings, f.check(path, read{text: s.Repeat.Until, expression: true}, f.scope(path, "steps", len(s.Steps), lookup))...)
		}
		for _, inner := range s.Branches() {
			warnings = append(warnings, f.listWarnings(inner.Steps, path, inner.Name, lookup)...)
		}
	}
	return warnings
}

// read is a text of a step that reads values: an expression, or a text with $ references.
type read struct {
	text string
	// expression is a condition or value read as an expression; shell, a run line; items, a
	// print's values apart by commas.
	expression, shell, items bool
}

// reads are the texts of the step read before it runs, in a stable order, without the repeat's
// until, which reads the scope at the end of a pass.
func (s Step) reads() []read {
	before := []string{s.If, s.Switch}
	if s.Repeat != nil {
		before = append(before, s.Repeat.Each)
	}
	var reads []read
	for _, text := range before {
		if text != "" {
			reads = append(reads, read{text: text, expression: true})
		}
	}
	if s.Print != "" {
		reads = append(reads, read{text: s.Print, expression: true, items: true})
	}
	for _, text := range s.texts() {
		reads = append(reads, read{text: text, shell: s.textKind(text) == shellText})
	}
	return reads
}

// check warns about each variable or step a template reads that is not surely in scope.
func (f Flow) check(path string, r read, sc *scope) []Warning {
	var warnings []Warning
	for _, ref := range f.references(r, sc.lookup) {
		i := slices.IndexFunc(sc.vars, func(v Variable) bool { return v.Step == ref.step && v.Field == ref.field })
		var problem string
		switch {
		case i >= 0 && sc.vars[i].Certainty == Maybe && ref.step == "":
			problem = "variable " + ref.field + " may not be set here"
		case i >= 0 && sc.vars[i].Certainty == Maybe:
			problem = "step " + ref.step + " may not have run here"
		case i >= 0 && ref.step == "":
			if problem = sc.ambiguity(ref.field); problem == "" {
				continue
			}
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

// everyStepField are the fields every step that runs something has: the last step's is the one
// meant, so reading them short is never ambiguous.
var everyStepField = []string{"output", "exit", "ok", "answer"}

// ambiguity is the scope warning for a short name that more than one step before the point sets,
// as two steps capturing version: the short name reads the last, and steps.<id> picks another.
func (sc *scope) ambiguity(name string) string {
	if slices.Contains(everyStepField, name) {
		return ""
	}
	var steps []string
	for _, v := range sc.vars {
		if v.Step != "" && v.Field == name && !slices.Contains(steps, v.Step) {
			steps = append(steps, v.Step)
		}
	}
	if len(steps) < 2 {
		return ""
	}
	last := steps[len(steps)-1]
	return fmt.Sprintf("scope: %s is set by %s; %s reads %s's, %s.%s reads %s's", name, strings.Join(steps, " and "), name, last, steps[0], name, steps[0])
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
		return "variable " + name + " was made inside an if, switch or repeat and went when it ended; export it to read it after"
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
		for _, inner := range s.Branches() {
			if f.setsVariable(inner.Steps, name) {
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
		for _, inner := range s.Branches() {
			if found, ok := f.stepWithID(inner.Steps, id); ok {
				return found, true
			}
		}
	}
	return Step{}, false
}

// reference is a step's field a text reads, build.output, or a variable, name with step "".
type reference struct {
	step, field string
}

// references lists the variables and steps' fields a text or expression reads, in order; one
// that does not parse reads none (Validate reports it). A text's $name.field whose name the flow
// does not know is told as a step, for a misspelt id would go to the shell as it is.
func (f Flow) references(r read, lookup func(id string) (script.Script, bool)) []reference {
	known := f.knownNames(lookup)
	var refs []reference
	for _, e := range r.exprs(known) {
		for _, path := range e.Names() {
			if ref, ok := pathReference(path, known); ok {
				refs = append(refs, ref)
			}
		}
	}
	if r.expression {
		return refs
	}
	own := map[string]bool{}
	if r.shell {
		own = shellNames(r.text)
	}
	for _, m := range dottedNames.FindAllStringSubmatch(r.text, -1) {
		if !known.has(m[1]) && !own[m[1]] && strings.ToUpper(m[1]) != m[1] {
			refs = append(refs, reference{step: m[1], field: m[2]})
		}
	}
	return refs
}

// dottedNames are a text's $name.field.
var dottedNames = regexp.MustCompile(`\$([A-Za-z_]\w*)\.([A-Za-z_]\w*)`)

// exprs are the expressions the read holds: itself, or a text's $ references.
func (r read) exprs(known knownNames) []*expr.Expr {
	if r.expression {
		parse := expr.Parse
		if r.items {
			parse = expr.ParseItems
		}
		e, err := parse(r.text)
		if err != nil {
			return nil
		}
		return []*expr.Expr{e}
	}
	kind := plainText
	if r.shell {
		kind = shellText
	}
	pieces, err := splitText(r.text, kind, known)
	if err != nil {
		return nil
	}
	var exprs []*expr.Expr
	for _, p := range pieces {
		if p.ref != nil {
			exprs = append(exprs, p.ref)
		}
	}
	return exprs
}

// pathReference is what a name an expression reads is: a variable, or a step's field.
func pathReference(path []string, known knownNames) (reference, bool) {
	switch {
	case path[0] == "steps" && len(path) > 2:
		return reference{step: path[1], field: path[2]}, true
	case path[0] == "steps":
		return reference{}, false
	case !known.has(path[0]) && len(path) > 1: // an unknown name with a field reads as a step
		return reference{step: path[0], field: path[1]}, true
	case known.vars[path[0]] || !known.steps[path[0]]:
		return reference{field: path[0]}, true
	case len(path) > 1:
		return reference{step: path[0], field: path[1]}, true
	}
	return reference{}, false
}
