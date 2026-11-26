package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/expr"
	"github.com/AlexandreSJ/noct/internal/naming"
)

var (
	errNotList   = errors.New(`a JSON list, as ["a", "b"]`)
	errNotObject = errors.New(`a JSON object, as {"key": "value"}`)
)

// DecodeJSON reads the text of a list or object param as expressions read it, keeping numbers as
// written. It tells what the text must be otherwise.
func DecodeJSON(kind, text string) (any, error) {
	want := map[string]error{ListParam: errNotList, ObjectParam: errNotObject}[kind]
	value, err := expr.FromJSON(text)
	if err != nil {
		return nil, want
	}
	_, isList := value.([]any)
	_, isObject := value.(map[string]any)
	if kind == ListParam && !isList || kind == ObjectParam && !isObject {
		return nil, want
	}
	return value, nil
}

// jsonParams are the params expressions read as JSON, with their kinds: a list or an object
// by its type, and a var when its value is one.
func (f Flow) jsonParams(values map[string]string) map[string]string {
	kinds := map[string]string{}
	for name, p := range f.Params {
		if kind := p.jsonKind(values[name]); kind != "" {
			kinds[name] = kind
		}
	}
	return kinds
}

func (p Param) jsonKind(value string) string {
	switch p.Type {
	case ListParam, ObjectParam:
		return p.Type
	case "", VarType:
		kind, _ := setKind(VarType, value)
		return kind
	}
	return ""
}

// readAs is a variable's value as an expression reads it: a list or object param decoded, empty
// when it has no value, or its text when something set it to text that is not JSON.
func readAs(kind, value string) any {
	switch {
	case kind == "":
		return value
	case strings.TrimSpace(value) == "" && kind == ListParam:
		return []any{}
	case strings.TrimSpace(value) == "":
		return map[string]any{}
	}
	decoded, err := DecodeJSON(kind, value)
	if err != nil {
		return value
	}
	return decoded
}

// ListItems splits a list param's JSON into its items as the inspect edits them, one each: a
// string as its text, unless its text reads as JSON, and anything else as its JSON. It tells
// whether text is a JSON list; an empty text is an empty list.
func ListItems(text string) ([]string, bool) {
	if strings.TrimSpace(text) == "" {
		return nil, true
	}
	if _, err := DecodeJSON(ListParam, text); err != nil {
		return nil, false
	}
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, false
	}
	items := make([]string, len(raw))
	for i, item := range raw {
		var s string
		if json.Unmarshal(item, &s) == nil && !json.Valid([]byte(s)) {
			items[i] = s
			continue
		}
		items[i] = compactRaw(item)
	}
	return items, true
}

// ItemsJSON joins items into a list param's JSON, reading each as ListItems writes it; no items
// is no default.
func ItemsJSON(items []string) string {
	if len(items) == 0 {
		return ""
	}
	raw := make([]json.RawMessage, len(items))
	for i, item := range items {
		if json.Valid([]byte(item)) {
			raw[i] = json.RawMessage(compactRaw([]byte(item)))
			continue
		}
		raw[i], _ = json.Marshal(item) // a string always encodes
	}
	text, _ := expr.JSON(raw) // raw messages always encode
	return text
}

func compactRaw(raw []byte) string {
	var out bytes.Buffer
	if err := json.Compact(&out, raw); err != nil {
		return string(raw)
	}
	return out.String()
}

var dollarRef = regexp.MustCompile(`\$\{[^}]*\}|\$[A-Za-z_][A-Za-z0-9_.]*`)

// ObjectPaths are the fields, at any depth, of the object a variable is: its default as a flow
// variable, or the value the last set in the flow gives it, read with each $ in it as null.
func (f Flow) ObjectPaths(name string) []string {
	text := ""
	if p, ok := f.Params[name]; ok {
		text = p.Default
	}
	walkSteps(f.Steps, "", "", func(s Step, _ string) {
		if value, ok := s.Set[name]; ok {
			text = value
		}
	})
	value, err := expr.FromJSON(dollarRef.ReplaceAllString(text, "null"))
	if err != nil {
		return nil
	}
	var paths []string
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		object, ok := v.(map[string]any)
		if !ok {
			return
		}
		for _, key := range slices.Sorted(maps.Keys(object)) {
			if !naming.IsIdentifier(key) {
				continue
			}
			paths = append(paths, prefix+"."+key)
			walk(prefix+"."+key, object[key])
		}
	}
	walk(name, value)
	return paths
}

// WithObjectPaths are vars and the fields of the objects among them, as obj.isOnline, which an
// expression reads and a form completes.
func (f Flow) WithObjectPaths(vars []Variable) []Variable {
	out := slices.Clone(vars)
	for _, v := range vars {
		if v.Step != "" {
			continue
		}
		for _, path := range f.ObjectPaths(v.Field) {
			out = append(out, Variable{Path: v.Path, Field: path, Certainty: v.Certainty})
		}
	}
	return out
}
