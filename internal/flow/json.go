package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/AlexandreSJ/noct/internal/expr"
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

// jsonParams are the params expressions read as JSON, with their types.
func (f Flow) jsonParams() map[string]string {
	kinds := map[string]string{}
	for name, p := range f.Params {
		if p.Type == ListParam || p.Type == ObjectParam {
			kinds[name] = p.Type
		}
	}
	return kinds
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
