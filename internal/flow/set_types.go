package flow

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/expr"
	"github.com/AlexandreSJ/noct/internal/naming"
)

// NoneValue is what a set gives a boolean for no value: null in an object, empty for a variable.
const NoneValue = "none"

func splitField(name string) (string, []string) {
	parts := strings.Split(name, ".")
	return parts[0], parts[1:]
}

func isSetName(name string) bool {
	root, fields := splitField(name)
	return isVariable(root) && !slices.ContainsFunc(fields, func(f string) bool { return !naming.IsIdentifier(f) })
}

// objectValue is the JSON the flow gives a variable as written, its default or the value of the
// last set of it, with each $ in it as null.
func (f Flow) objectValue(name string) any {
	text := ""
	if p, ok := f.Params[name]; ok {
		text = p.Default
	}
	walkSteps(f.Steps, "", "", func(s Step, _ string) {
		if value, ok := s.Set[name]; ok {
			text = value
		}
	})
	value, _ := expr.FromJSON(dollarRef.ReplaceAllString(text, "null"))
	return value
}

func fieldOf(value any, fields []string) any {
	for _, field := range fields {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[field]
	}
	return value
}

// jsonKindOf is the type a JSON value is, var for null or nothing.
func jsonKindOf(value any) string {
	switch value.(type) {
	case bool:
		return BooleanParam
	case json.Number, float64:
		return NumberParam
	case string:
		return TextParam
	case []any:
		return ListParam
	case map[string]any:
		return ObjectParam
	}
	return VarType
}

// typeOf is the type a set or prompt gives name: the one written, or the variable's own when it
// inherits, as none written does for a variable the run has.
func (x *running) typeOf(name, written string) string {
	if kind := given(written); kind != "" {
		return kind
	}
	return cmp.Or(x.types[name], VarType)
}

// setValue gives a variable, or a field of an object variable, a set's value; kind is the JSON
// kind setKind found for a variable's.
func (x *running) setValue(name, written, value, kind string) error {
	root, fields := splitField(name)
	if len(fields) == 0 {
		if x.typeOf(name, written) == BooleanParam && value == NoneValue {
			value = ""
		}
		x.assign(name, value)
		if kind != "" {
			x.jsonVars[name] = kind
		} else {
			delete(x.jsonVars, name)
		}
		if given(written) != "" || x.types[name] == "" {
			x.types[name] = x.typeOf(name, written)
		}
		return nil
	}
	current, err := expr.FromJSON(cmp.Or(strings.TrimSpace(x.vars[root]), "{}"))
	object, ok := current.(map[string]any)
	if err != nil || !ok {
		return fmt.Errorf("%s is no object", root)
	}
	field, err := fieldValue(cmp.Or(given(written), jsonKindOf(fieldOf(object, fields))), value)
	if err != nil {
		return err
	}
	setPath(object, fields, field)
	text, err := expr.JSON(object)
	if err != nil {
		return err
	}
	x.assign(root, text)
	x.jsonVars[root] = ObjectParam
	return nil
}

func setPath(object map[string]any, fields []string, value any) {
	for _, field := range fields[:len(fields)-1] {
		inner, ok := object[field].(map[string]any)
		if !ok {
			inner = map[string]any{}
			object[field] = inner
		}
		object = inner
	}
	object[fields[len(fields)-1]] = value
}

// fieldValue is a set's text as a JSON value of the type.
func fieldValue(kind, text string) (any, error) {
	trimmed := strings.TrimSpace(text)
	switch kind {
	case BooleanParam:
		switch trimmed {
		case "true", "false":
			return trimmed == "true", nil
		case NoneValue, "":
			return nil, nil
		}
		return nil, fmt.Errorf("%q is not true, false or none", text)
	case NumberParam:
		if !fitsType(NumberParam, trimmed) {
			return nil, fmt.Errorf("%q is no number", text)
		}
		return json.Number(trimmed), nil
	case TextParam:
		return text, nil
	case ListParam, ObjectParam:
		return DecodeJSON(kind, text)
	}
	if value, err := expr.FromJSON(trimmed); err == nil {
		return value, nil
	}
	return text, nil
}

// validateRetyping refuses a set or a prompt that gives a variable a type other than the one it
// has already: a name has one type in a flow, and a set changing it would make two. A var takes
// any, as it is what its value is.
func (f Flow) validateRetyping() error {
	declared := map[string]string{}
	for name, p := range f.Params {
		declared[name] = cmp.Or(p.Type, VarType)
	}
	var problem error
	check := func(path, name, written, fallback string) {
		kind, had := given(written), declared[name]
		switch {
		case problem != nil, strings.Contains(name, "."):
		case had == "":
			declared[name] = cmp.Or(kind, fallback)
		case kind != "" && kind != had && had != VarType:
			problem = fmt.Errorf("step %s: %s is a %s already; leave its type to inherit, or give the new value another name", path, name, had)
		}
	}
	walkSteps(f.Steps, "", "", func(s Step, path string) {
		for _, name := range slices.Sorted(maps.Keys(s.Set)) {
			check(path, name, s.Types[name], VarType)
		}
		if s.Kind() == PromptStep {
			check(path, s.Var, s.Type, TextParam)
		}
	})
	return problem
}

func (f Flow) paramTypes() map[string]string {
	types := map[string]string{}
	for name, p := range f.Params {
		types[name] = cmp.Or(p.Type, VarType)
	}
	return types
}
