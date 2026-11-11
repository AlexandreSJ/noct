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

	"github.com/AlexandreSJ/noct/internal/naming"
)

// Types a parameter or a prompt's answer can have; a parameter can also be yes or no, one of its
// choices, or a path.
const (
	TextParam   = "text"
	NumberParam = "number"
	BoolParam   = "bool"
	ChoiceParam = "choice"
	PathParam   = "path"
)

// ParamTypes are the types a parameter can have, in the order the inspect offers them.
var ParamTypes = []string{TextParam, NumberParam, BoolParam, ChoiceParam, PathParam}

// BoolValues are what a yes or no parameter holds.
var BoolValues = []string{"true", "false"}

const maxPromptTries = 3

// Param is a flow's parameter. In YAML it is its default alone (env: dev), or a mapping with the
// rest (a: {default: "3", type: number, prompt: First number, required: true}).
type Param struct {
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
	// Type is one of ParamTypes; empty is TextParam.
	Type string `yaml:"type,omitempty" json:"type,omitempty"`
	// Choices are the values a ChoiceParam can take.
	Choices []string `yaml:"choices,omitempty" json:"choices,omitempty"`
	// Prompt is what the params form shows instead of the name.
	Prompt string `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	// Required refuses to run with the parameter empty.
	Required bool `yaml:"required,omitempty" json:"required,omitempty"`
	// From reads the value from the environment (env:NAME) when the variable is set, without
	// asking; unset, the parameter is asked like any other.
	From string `yaml:"from,omitempty" json:"from,omitempty"`
	// Secret values are masked in the form, never remembered and never on the command line.
	Secret bool `yaml:"secret,omitempty" json:"secret,omitempty"`
}

// EnvName is the variable an env parameter reads.
func (p Param) EnvName() (string, bool) {
	return strings.CutPrefix(p.From, "env:")
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
	if p.onlyDefault() {
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

// Values are the parameters' values before anything is given: the environment's for env
// parameters whose variable is set, else their defaults.
func (f Flow) Values(getenv func(string) string) map[string]string {
	values := f.Defaults()
	for name, p := range f.Params {
		if env, ok := p.EnvName(); ok && getenv(env) != "" {
			values[name] = getenv(env)
		}
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
		if p.Required && strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if problem := p.fits(value); problem != "" {
			return fmt.Errorf("%s %s", name, problem)
		}
	}
	return nil
}

// onlyDefault tells a parameter that is its default alone, which saves as plain text.
func (p Param) onlyDefault() bool {
	return p.Type == "" && p.Prompt == "" && !p.Required && p.From == "" && !p.Secret && len(p.Choices) == 0
}

// Values are what the parameter can take, for the types that have a few; nil for any.
func (p Param) Values() []string {
	switch p.Type {
	case BoolParam:
		return BoolValues
	case ChoiceParam:
		return p.Choices
	}
	return nil
}

// fits tells why value is not one the parameter takes, or "".
func (p Param) fits(value string) string {
	switch {
	case value == "":
		return ""
	case p.Type == NumberParam && !fitsType(NumberParam, value):
		return fmt.Sprintf("must be a number, not %q", value)
	case p.Values() != nil && !slices.Contains(p.Values(), value):
		return fmt.Sprintf("must be one of %s, not %q", strings.Join(p.Values(), ", "), value)
	}
	return ""
}

// fitsType tells whether value is of the type: any text, or a number (like 3, -2 or 1.5).
func fitsType(kind, value string) bool {
	if kind != NumberParam {
		return true
	}
	_, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil
}

// checkParam reports a parameter that cannot work: an unknown type, a choice without choices,
// or a default it would refuse.
func checkParam(p Param) error {
	if p.Type != "" && !slices.Contains(ParamTypes, p.Type) {
		return fmt.Errorf("type %q is one of %s", p.Type, strings.Join(ParamTypes, ", "))
	}
	if p.Type == ChoiceParam && len(p.Choices) == 0 {
		return errors.New("a choice needs its choices")
	}
	if problem := p.fits(p.Default); problem != "" {
		return errors.New("the default " + problem)
	}
	return nil
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
	return naming.IsIdentifier(name) && name != "steps"
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
		return validateUnsetNames(s.Unset)
	}
	return nil
}

func validateUnsetNames(names []string) error {
	for _, name := range names {
		step, isStep := strings.CutPrefix(name, stepsPrefix)
		if isStep && naming.IsIdentifier(step) || !isStep && isVariable(name) {
			continue
		}
		return fmt.Errorf("unset %q is a variable's name or %s<id>", name, stepsPrefix)
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
