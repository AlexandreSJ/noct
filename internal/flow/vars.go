package flow

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Types a parameter or a prompt's answer can have.
const (
	TextParam   = "text"
	NumberParam = "number"
)

// maxPromptTries is how many answers a prompt takes before its step fails.
const maxPromptTries = 3

// Param is a flow's parameter. In YAML it is its default alone (env: dev), or a mapping with the
// rest (a: {default: "3", type: number, prompt: First number, required: true}).
type Param struct {
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
	// Type is TextParam (when empty) or NumberParam.
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
	// Prompt is what the params form shows instead of the name.
	Prompt string `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	// Required refuses to run with the parameter empty.
	Required bool `yaml:"required,omitempty" json:"required,omitempty"`
}

// paramFields is Param without its YAML methods, to decode and encode the mapping form.
type paramFields Param

func (p *Param) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*p = Param{Default: node.Value}
		return nil
	}
	return node.Decode((*paramFields)(p))
}

func (p Param) MarshalYAML() (any, error) {
	if p == (Param{Default: p.Default}) {
		return p.Default, nil
	}
	return paramFields(p), nil
}

// Label is what the params form shows for the parameter.
func (p Param) Label(name string) string {
	if p.Prompt != "" {
		return p.Prompt
	}
	return name
}

// Defaults are the parameters' values before anything is given.
func (f Flow) Defaults() map[string]string {
	values := make(map[string]string, len(f.Params))
	for name, p := range f.Params {
		values[name] = p.Default
	}
	return values
}

// CheckParams reports values the flow cannot start with: names it has no parameter for,
// numbers that are not, and required parameters left empty. Parameters missing from values
// take their defaults.
func (f Flow) CheckParams(values map[string]string) error {
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if _, ok := f.Params[name]; !ok {
			return fmt.Errorf("flow %s has no parameter %s", f.Name, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(f.Params)) {
		p := f.Params[name]
		value, given := values[name]
		if !given {
			value = p.Default
		}
		switch {
		case p.Required && strings.TrimSpace(value) == "":
			return fmt.Errorf("%s is required", name)
		case value != "" && !fitsType(p.Type, value):
			return fmt.Errorf("%s must be a number, not %q", name, value)
		}
	}
	return nil
}

// fitsType tells whether value is of the type: any text, or a number (like 3, -2 or 1.5).
func fitsType(kind, value string) bool {
	if kind != NumberParam {
		return true
	}
	_, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil
}

func checkType(kind string) error {
	if kind != "" && kind != TextParam && kind != NumberParam {
		return fmt.Errorf("type %q is %s or %s", kind, TextParam, NumberParam)
	}
	return nil
}

// isVariable tells whether name can be a variable: an identifier other than steps, which holds
// the steps' results.
func isVariable(name string) bool {
	return identifier.MatchString(name) && name != "steps"
}

// stepsPrefix starts what unset names to clear a step's results instead of a variable.
const stepsPrefix = "steps."

func (s Step) validateVariables(kind Kind) error {
	switch kind {
	case PromptStep:
		if s.Var == "" {
			return errors.New("a prompt needs var, the variable its answer goes to")
		}
		if !isVariable(s.Var) {
			return fmt.Errorf("var %q must be letters, digits and _ only, and not steps", s.Var)
		}
		return checkType(s.Type)
	case SetStep:
		for name := range s.Set {
			if !isVariable(name) {
				return fmt.Errorf("set name %q must be letters, digits and _ only, and not steps", name)
			}
		}
	case UnsetStep:
		for _, name := range s.Unset {
			step, isStep := strings.CutPrefix(name, stepsPrefix)
			if isStep && identifier.MatchString(step) || !isStep && isVariable(name) {
				continue
			}
			return fmt.Errorf("unset %q is a variable's name or %s<id>", name, stepsPrefix)
		}
	}
	return nil
}

// runPrompt asks whoever watches the run and keeps the answer. Nobody may be there, as in a run
// by a trigger: then the step fails at once rather than wait.
func (x *running) runPrompt(s Step, path string) bool {
	question, err := x.render(s.Prompt)
	if err != nil {
		return x.stop(path, "prompt: "+err.Error(), 1)
	}
	fallback, err := x.render(s.Default)
	if err != nil {
		return x.stop(path, "default: "+err.Error(), 1)
	}
	x.header("?", s.Label(), "→ ."+s.Var)
	if x.Ask == nil {
		return x.stop(path, "prompt: nobody is at the terminal to answer", 1)
	}
	if x.Attention != nil {
		x.Attention()
	}
	fmt.Fprint(x.Out, "\a")
	if fallback != "" {
		question += " [" + fallback + "]"
	}
	start := time.Now()
	for range maxPromptTries {
		answer := strings.TrimSpace(x.Ask("\x1b[1m" + question + ":\x1b[0m "))
		if answer == "" {
			answer = fallback
		}
		if s.Type == NumberParam && (answer == "" || !fitsType(NumberParam, answer)) {
			fmt.Fprintf(x.Out, "\x1b[2m%q is not a number\x1b[0m\n", answer)
			continue
		}
		x.vars[s.Var] = answer
		x.result.Steps[path] = StepResult{Status: StatusOK, Duration: time.Since(start), Captures: map[string]string{s.Var: answer}, Message: s.Var + " = " + answer}
		return true
	}
	return x.stop(path, fmt.Sprintf("prompt: no number after %d answers", maxPromptTries), 1)
}

// runSet renders every value first and then sets them all, so each reads the variables as they
// were before the step.
func (x *running) runSet(s Step, path string) bool {
	names := slices.Sorted(maps.Keys(s.Set))
	values := make(map[string]string, len(names))
	for _, name := range names {
		value, err := x.render(s.Set[name])
		if err != nil {
			return x.stop(path, "set "+name+": "+err.Error(), 1)
		}
		values[name] = value
	}
	x.header("=", s.Label(), strings.Join(names, ", "))
	var shown []string
	for _, name := range names {
		x.vars[name] = values[name]
		shown = append(shown, name+" = "+values[name])
		fmt.Fprintf(x.Out, "\x1b[2m  %s = %s\x1b[0m\n", name, values[name])
	}
	x.result.Steps[path] = StepResult{Status: StatusOK, Captures: values, Message: strings.Join(shown, ", ")}
	return true
}

// runUnset clears variables and steps' results; a template reads them as empty after it.
func (x *running) runUnset(s Step, path string) {
	for _, name := range s.Unset {
		if step, isStep := strings.CutPrefix(name, stepsPrefix); isStep {
			delete(x.steps, step)
		} else {
			delete(x.vars, name)
		}
	}
	x.header("∅", s.Label(), strings.Join(s.Unset, ", "))
	x.result.Steps[path] = StepResult{Status: StatusOK, Message: "cleared " + strings.Join(s.Unset, ", ")}
}
