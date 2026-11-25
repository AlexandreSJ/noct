package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
)

// A param's default can be a template, as {"user": {{json .user}}}: it reads the params written
// before it, with the values they start the run with. Only a default renders: a value given in its
// place is taken as it is, so text typed in the form or passed to a call never runs as a template.
// A value given that is the default's own text, as the form sends it untouched, is the default.

// isTemplate tells text a template would change.
func isTemplate(text string) bool { return strings.Contains(text, "{{") }

// withDefaults are the values with each param missing from them at its default, and each param
// at a default that is a template at that default rendered over the params before it. On an error
// the params before the one that failed are rendered and the rest are as they were.
func (f Flow) withDefaults(values map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(values)+len(f.Params))
	maps.Copy(resolved, values)
	kinds := f.jsonParams()
	before := map[string]string{}
	for _, name := range f.ParamNames() {
		p := f.Params[name]
		value, given := resolved[name]
		if !given {
			value = p.Default
		}
		if value == p.Default && isTemplate(p.Default) {
			rendered, err := renderVars(p.Default, before, kinds, nil)
			if err != nil {
				return resolved, fmt.Errorf("the default of %s: %w", name, err)
			}
			value = rendered
		}
		resolved[name], before[name] = value, value
	}
	return resolved, nil
}

// DefaultProblem tells why the param's default cannot be, or "": a template that does not parse,
// or a value the param would refuse. A template's value is told when the run renders it.
func (p Param) DefaultProblem() string {
	if isTemplate(p.Default) {
		if _, err := parseTemplate(p.Default); err != nil {
			return "is no template: " + err.Error()
		}
		return ""
	}
	return p.Fits(p.Default)
}

// toJSON is a value as JSON, for a template to put in JSON: text in quotes with what it holds
// escaped, and a list or object param, or a part of one, as it is.
func toJSON(value any) (string, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false) // <, > and & stay as typed
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}
