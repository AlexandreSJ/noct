package flow

import (
	"fmt"
	"maps"
	"strings"
)

// A variable's default can read the variables written before it, as {"user": "$user"}, with the
// values they start the run with. Only a default renders: a value given in its place is taken as
// it is, so text typed in the form or passed to a call never reads a $. A value given that is the
// default's own text, as the form sends it untouched, is the default.

// isTemplate tells text that may read a value: one with a $.
func isTemplate(text string) bool { return strings.Contains(text, "$") }

// withDefaults are the values with each variable missing from them at its default, and each one
// at a default that reads a $ at that default rendered over the variables before it. On an error
// the params before the one that failed are rendered and the rest are as they were.
func (f Flow) withDefaults(values map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(values)+len(f.Params))
	maps.Copy(resolved, values)
	kinds := map[string]string{}
	before := map[string]string{}
	for _, name := range f.ParamNames() {
		p := f.Params[name]
		value, given := resolved[name]
		if !given {
			value = p.Default
		}
		if value == p.Default && isTemplate(p.Default) {
			rendered, err := renderText(p.Default, p.textKind(), knownNames{}, before, kinds, nil)
			if err != nil {
				return resolved, fmt.Errorf("the default of %s: %w", name, err)
			}
			value = rendered
		}
		resolved[name], before[name] = value, value
		if kind := p.jsonKind(value); kind != "" {
			kinds[name] = kind
		}
	}
	return resolved, nil
}

// DefaultProblem tells why the variable's default cannot be, or "": a $ call that does not parse,
// or a value the variable would refuse. What a $ reads is told when the run renders it.
func (p Param) DefaultProblem() string {
	if isTemplate(p.Default) {
		if _, err := splitText(p.Default, p.textKind(), knownNames{}); err != nil {
			return err.Error()
		}
		return ""
	}
	return p.Fits(p.Default)
}

// textKind is how the variable's default reads values: as JSON for a list or object.
func (p Param) textKind() textKind {
	if p.Type == ListParam || p.Type == ObjectParam {
		return jsonText
	}
	return plainText
}
