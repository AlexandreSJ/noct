package flow

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/script"
)

// Validate reports the first problem that would make the flow fail before running its steps.
// lookup, when not nil, checks that each script exists and loads.
func (f Flow) Validate(lookup func(id string) (script.Script, bool)) error {
	for name, p := range f.Params {
		if !isVariable(name) {
			return fmt.Errorf("parameter name %q must be letters, digits and _ only, and not steps", name)
		}
		if err := checkParam(p); err != nil {
			return fmt.Errorf("parameter %s: %w", name, err)
		}
		if _, ok := p.EnvName(); p.From != "" && !ok {
			return fmt.Errorf("parameter %s: from must look like env:NAME", name)
		}
	}
	for i, t := range f.Triggers {
		if err := t.validate(f.Name); err != nil {
			return fmt.Errorf("trigger %d: %w", i+1, err)
		}
	}
	if _, err := parseTimeout(f.Timeout); err != nil {
		return err
	}
	ids := map[string]bool{}
	return validateSteps(f.Steps, "", ids, lookup)
}

func validateSteps(steps []Step, where string, ids map[string]bool, lookup func(id string) (script.Script, bool)) error {
	for i, s := range steps {
		at := fmt.Sprintf("%sstep %d", where, i+1)
		if s.ID != "" {
			at += " (" + s.ID + ")"
		}
		if err := s.validate(ids, lookup); err != nil {
			return fmt.Errorf("%s: %w", at, err)
		}
		for _, inner := range s.Branches() {
			if err := validateSteps(inner.Steps, at+" > ", ids, lookup); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s Step) validate(ids map[string]bool, lookup func(id string) (script.Script, bool)) error {
	if err := s.validateID(ids); err != nil {
		return err
	}
	kind := s.Kind()
	if kind == "" {
		return errors.New(exactlyOne)
	}
	if err := s.validateFieldsBelong(kind); err != nil {
		return err
	}
	if err := s.validatePane(kind); err != nil {
		return err
	}
	if err := s.validateTimeout(kind); err != nil {
		return err
	}
	if err := s.validateTemplates(); err != nil {
		return err
	}
	return s.validateKind(kind, lookup)
}

// validateFieldsBelong refuses fields that only another kind of step uses.
func (s Step) validateFieldsBelong(kind Kind) error {
	if (s.Var != "" || s.Default != "" || s.Type != "") && kind != PromptStep {
		return errors.New("var, default and type belong to a prompt")
	}
	if len(s.Then) > 0 && kind != IfStep {
		return errors.New("then belongs to an if")
	}
	if len(s.Else) > 0 && kind != IfStep && kind != SwitchStep {
		return errors.New("else belongs to an if or a switch")
	}
	if len(s.Cases) > 0 && kind != SwitchStep {
		return errors.New("cases belong to a switch")
	}
	if len(s.Steps) > 0 && kind != RepeatStep {
		return errors.New("steps belong to a repeat")
	}
	if (len(s.Capture) > 0 || s.Expect != nil || s.OnFail != "" || s.Retry != nil) && kind != ScriptStep && kind != CommandStep && kind != AgentStep {
		return errors.New("capture, expect, on_fail and retry belong to a script, run or agent step")
	}
	if len(s.With) > 0 && kind != ScriptStep && kind != CallStep {
		return errors.New("with belongs to a script or call step")
	}
	if len(s.Export) > 0 && !s.IsBlock() {
		return errors.New("export belongs to an if, a switch or a repeat")
	}
	for inner, outer := range s.Export {
		if !isVariable(inner) || outer != "" && !isVariable(outer) {
			return fmt.Errorf("export %s: %s are variable names", inner, outer)
		}
	}
	if s.OnFail != "" && s.OnFail != OnFailStop && s.OnFail != OnFailContinue && s.OnFail != OnFailAsk {
		return fmt.Errorf("on_fail is %s, %s or %s", OnFailStop, OnFailContinue, OnFailAsk)
	}
	return s.Retry.validate()
}

func (r *Retry) validate() error {
	if r == nil {
		return nil
	}
	if r.Max < 1 {
		return errors.New("retry is how many times to run the step again: 1 or more")
	}
	if r.Every != "" {
		if _, err := time.ParseDuration(r.Every); err != nil {
			return fmt.Errorf("retry every %q is not a duration like 10s", r.Every)
		}
	}
	return nil
}

// validateSwitch checks that each case has values, and that no value is in two cases, where only
// the first would ever run.
func (s Step) validateSwitch() error {
	if len(s.Cases) == 0 {
		return errors.New("a switch needs cases: - is: value, with its steps")
	}
	seen := map[string]int{}
	for i, c := range s.Cases {
		if len(c.Is) == 0 {
			return fmt.Errorf("case %d: is is empty; give it a value", i+1)
		}
		for _, value := range c.Is {
			value = strings.TrimSpace(value)
			if first, twice := seen[value]; twice {
				return fmt.Errorf("case %d: %q is also in case %d, which runs first", i+1, value, first)
			}
			seen[value] = i + 1
		}
	}
	return nil
}

func (s Step) validateID(ids map[string]bool) error {
	if s.ID == "" {
		return nil
	}
	if !naming.IsIdentifier(s.ID) {
		return fmt.Errorf("id %q must be letters, digits and _ only", s.ID)
	}
	if ids[s.ID] {
		return fmt.Errorf("id %q is used twice", s.ID)
	}
	ids[s.ID] = true
	return nil
}

func (s Step) validateKind(kind Kind, lookup func(id string) (script.Script, bool)) error {
	switch kind {
	case ScriptStep:
		if err := validateScript(s.Script, s.With, lookup); err != nil {
			return err
		}
	case RepeatStep:
		return s.Repeat.validate()
	case SwitchStep:
		return s.validateSwitch()
	case AgentStep:
		if strings.TrimSpace(s.Agent.Ask) == "" {
			return errors.New("an agent step asks something: ask is empty")
		}
		if s.Pane != "" || s.Background {
			return errors.New("an agent step runs in the flow's own terminal: no pane or background")
		}
		return s.validatePatterns()
	case CallStep:
		if !naming.IsValid(s.Call) {
			return fmt.Errorf("call %q is not a function's name", s.Call)
		}
		for name := range s.With {
			if !isVariable(name) {
				return fmt.Errorf("with %q is not a param's name", name)
			}
		}
		return nil
	case WaitStep:
		if _, err := time.ParseDuration(s.Wait); err != nil {
			return fmt.Errorf("wait %q is not a duration like 5s or 1m30s", s.Wait)
		}
	case PromptStep, SetStep, UnsetStep:
		return s.validateVariables(kind)
	}
	return s.validatePatterns()
}

func (s Step) validatePatterns() error {
	for name, pattern := range s.Capture {
		if !naming.IsIdentifier(name) {
			return fmt.Errorf("capture name %q must be letters, digits and _ only", name)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("capture %s: %w", name, err)
		}
	}
	if s.Expect != nil && s.Expect.OutputMatches != "" {
		if _, err := regexp.Compile(s.Expect.OutputMatches); err != nil {
			return fmt.Errorf("expect output_matches: %w", err)
		}
	}
	return nil
}

// validateScript checks that the script loads and takes every value with gives it, so a misspelt
// name does not leave the script running with its default.
func validateScript(id string, with map[string]string, lookup func(id string) (script.Script, bool)) error {
	if lookup == nil {
		return nil
	}
	s, ok := lookup(id)
	if !ok {
		return fmt.Errorf("script %s not found", id)
	}
	if s.Problem != nil {
		return fmt.Errorf("script %s is broken: %w", id, s.Problem)
	}
	var names []string
	for _, p := range s.Params {
		names = append(names, p.Name)
	}
	for _, name := range slices.Sorted(maps.Keys(with)) {
		if !slices.Contains(names, name) {
			return fmt.Errorf("with %s is not a parameter of script %s (%s)", name, id, cmp.Or(strings.Join(names, ", "), "it has none"))
		}
	}
	return nil
}

func (r *Repeat) validate() error {
	if r.Times < 0 || r.Max < 0 {
		return errors.New("repeat times and max cannot be negative")
	}
	switch {
	case r.Each != "" && r.Times > 0:
		return errors.New("repeat runs each line of each or times, not both")
	case r.As != "" && r.Each == "":
		return errors.New("as names the variable of each: it needs each")
	case r.As != "" && !isVariable(r.As):
		return fmt.Errorf("as %q must be letters, digits and _ only", r.As)
	case r.Each != "":
		// the lines end it; until and max may end it sooner
	case r.Times == 0 && r.Until == "":
		return errors.New("repeat needs times, each or until")
	case r.Times == 0 && r.Max == 0:
		return errors.New("repeat until needs max, so it cannot run forever")
	}
	if r.Every != "" {
		if _, err := time.ParseDuration(r.Every); err != nil {
			return fmt.Errorf("repeat every %q is not a duration like 10s", r.Every)
		}
	}
	return nil
}

// templates are every text of the step that is rendered: those before it runs and the repeat's
// until.
func (s Step) templates() []string {
	texts := s.orderedTemplates()
	if s.Repeat != nil {
		texts = append(texts, s.Repeat.Until)
	}
	return texts
}

func (s Step) validateTemplates() error {
	for _, text := range s.templates() {
		if _, err := parseTemplate(text); err != nil {
			return err
		}
	}
	return nil
}
