package expr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// An expression's values are texts, numbers (float64, or json.Number as a list or object param
// writes them), true and false, null (nil), lists ([]any), objects (map[string]any) and
// lambdas. Every value has a text, which String gives.

// String is a value as text: a number without needless zeros, a list or object as compact JSON,
// null as nothing.
func String(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case json.Number:
		return v.String()
	case []any, map[string]any:
		text, err := JSON(v)
		if err != nil {
			return err.Error()
		}
		return text
	case *Lambda:
		return "a function"
	}
	return fmt.Sprint(value)
}

// JSON is a value as compact JSON: a text in quotes, with what it holds escaped.
func JSON(value any) (string, error) {
	if _, ok := value.(*Lambda); ok {
		return "", errors.New("a function has no JSON")
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false) // <, > and & stay as typed
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

// FromJSON reads a JSON text as an expression's value, keeping numbers as written.
func FromJSON(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("%s is no JSON", describe(text))
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("%s is no JSON: something follows its value", describe(text))
	}
	return value, nil
}

// numberText is a text that reads as a number: digits, a fraction, an exponent.
var numberText = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// Number is a value as a number: a number, or a text that reads as one, as every exit and count a
// flow keeps is.
func Number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		text := strings.TrimSpace(v)
		if !numberText.MatchString(text) {
			return 0, false
		}
		n, err := strconv.ParseFloat(text, 64)
		return n, err == nil
	}
	return 0, false
}

// Truthy tells whether a value holds as a condition. False are false, null, 0, an empty list or
// object, and a text that is empty, false or 0; everything else is true.
func Truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		text := strings.TrimSpace(v)
		if n, ok := Number(text); ok {
			return n != 0
		}
		return text != "" && text != "false"
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	}
	if n, ok := Number(value); ok {
		return n != 0
	}
	return true
}

// Equal tells two values the same: as numbers when both are, and as texts otherwise, so "1.0"
// equals 1 and "true" equals true.
func Equal(a, b any) bool {
	if x, ok := Number(a); ok {
		if y, ok := Number(b); ok {
			return x == y
		}
	}
	return String(a) == String(b)
}

// isEmpty tells a value ?? passes over: null or an empty text.
func isEmpty(value any) bool {
	return value == nil || value == ""
}

// contains tells whether item is in container: an item of a list, a key of an object, a part of
// a text.
func contains(container, item any) bool {
	switch c := container.(type) {
	case []any:
		for _, each := range c {
			if Equal(each, item) {
				return true
			}
		}
		return false
	case map[string]any:
		_, ok := c[String(item)]
		return ok
	case nil:
		return false
	}
	return strings.Contains(String(container), String(item))
}

// structured is a value with fields or items: a list or object, or a text of JSON read as one.
func structured(value any) (any, error) {
	text, ok := value.(string)
	if !ok {
		return value, nil
	}
	decoded, err := FromJSON(text)
	if err != nil {
		return nil, fmt.Errorf("%s has no fields: it is no JSON object or list", describe(text))
	}
	return decoded, nil
}

// fieldOf is an object's field, or null when it has none; a text of a JSON object is read first.
func fieldOf(value any, name string) (any, error) {
	if value == nil {
		return nil, nil
	}
	value, err := structured(value)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s has no field %s: it is no object", describe(value), name)
	}
	return object[name], nil
}

// indexOf is a list's item, from 0, or from the end at -1 and below, or an object's key; null
// past the end.
func indexOf(value, at any) (any, error) {
	if value == nil {
		return nil, nil
	}
	value, err := structured(value)
	if err != nil {
		return nil, err
	}
	if object, ok := value.(map[string]any); ok {
		return object[String(at)], nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s has no items: it is no list", describe(value))
	}
	n, ok := Number(at)
	if !ok || n != float64(int(n)) {
		return nil, fmt.Errorf("a list's index is a whole number, not %s", describe(at))
	}
	i := int(n)
	if i < 0 {
		i += len(list)
	}
	if i < 0 || i >= len(list) {
		return nil, nil
	}
	return list[i], nil
}

// List is a value as the items a repeat each or a list function goes through: a list's items, or
// a text's lines without the empty ones; null is no items.
func List(value any) ([]any, error) {
	switch v := value.(type) {
	case []any:
		return v, nil
	case nil:
		return nil, nil
	case map[string]any:
		return nil, fmt.Errorf("%s is an object, not a list: keys() or values() give its parts", describe(v))
	}
	var lines []any
	for _, line := range strings.Split(String(value), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

// describe tells a value in a problem's message, cut short when long.
func describe(value any) string {
	switch value.(type) {
	case nil:
		return "nothing"
	case string:
		return "the text " + shorten(strconv.Quote(value.(string)))
	case []any:
		return "the list " + shorten(String(value))
	case map[string]any:
		return "the object " + shorten(String(value))
	}
	return shorten(String(value))
}

func shorten(text string) string {
	if len([]rune(text)) > 40 {
		return string([]rune(text)[:40]) + "…"
	}
	return text
}
