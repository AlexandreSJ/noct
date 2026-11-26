package expr

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// function is one an expression calls by its name: name(args).
type function struct {
	// min and max are how many arguments it takes; max -1 is any number.
	min, max int
	// Usage and Doc tell it, for the editor's help: join(list, sep).
	usage, doc string
	call       func(args []any) (any, error)
}

func (f *function) arity() string {
	switch {
	case f.min == f.max:
		return fmt.Sprintf("%d argument%s: %s", f.min, map[bool]string{true: "", false: "s"}[f.min == 1], f.usage)
	case f.max < 0:
		return fmt.Sprintf("%d or more arguments: %s", f.min, f.usage)
	}
	return fmt.Sprintf("%d to %d arguments: %s", f.min, f.max, f.usage)
}

// Function tells a function an expression can call, for the editor to show.
type Function struct {
	Name, Usage, Doc string
}

// Functions are the functions expressions can call, by name.
func Functions() []Function {
	var all []Function
	for _, name := range FunctionNames() {
		f := functions[name]
		all = append(all, Function{Name: name, Usage: f.usage, Doc: f.doc})
	}
	return all
}

// FunctionNames are the names of the functions, in order.
func FunctionNames() []string { return slices.Sorted(maps.Keys(functions)) }

// IsFunction tells a function's name.
func IsFunction(name string) bool {
	_, ok := functions[name]
	return ok
}

var functions map[string]*function

func init() {
	functions = map[string]*function{}
	for _, group := range [][]entry{textFunctions, listFunctions, otherFunctions} {
		for _, e := range group {
			functions[e.name] = &function{min: e.min, max: e.max, usage: e.usage, doc: e.doc, call: e.call}
		}
	}
}

// entry is a function as the tables below write it.
type entry struct {
	name       string
	min, max   int
	usage, doc string
	call       func(args []any) (any, error)
}

// text1 is a function of one text.
func text1(f func(string) any) func([]any) (any, error) {
	return func(args []any) (any, error) { return f(String(args[0])), nil }
}

// text2 is a function of two texts.
func text2(f func(a, b string) any) func([]any) (any, error) {
	return func(args []any) (any, error) { return f(String(args[0]), String(args[1])), nil }
}

var textFunctions = []entry{
	{"len", 1, 1, "len(x)", "how many characters a text has, or items a list or object", length},
	{"lower", 1, 1, "lower(text)", "the text in lower case", text1(func(s string) any { return strings.ToLower(s) })},
	{"upper", 1, 1, "upper(text)", "the text in upper case", text1(func(s string) any { return strings.ToUpper(s) })},
	{"trim", 1, 1, "trim(text)", "the text without the spaces and line breaks around it", text1(func(s string) any { return strings.TrimSpace(s) })},
	{"lines", 1, 1, "lines(text)", "the text's lines, without the empty ones", func(args []any) (any, error) {
		lines, err := List(String(args[0]))
		return orEmpty(lines), err
	}},
	{"split", 2, 2, "split(text, sep)", "the parts of the text between each sep", text2(func(s, sep string) any {
		var parts []any
		for _, part := range strings.Split(s, sep) {
			parts = append(parts, part)
		}
		return parts
	})},
	{"join", 1, 2, "join(list, sep)", "the list's items as one text, sep between them (a space without it)", join},
	{"contains", 2, 2, "contains(x, item)", "whether a text holds a part, a list an item or an object a key: item in x", func(args []any) (any, error) {
		return contains(args[0], args[1]), nil
	}},
	{"startsWith", 2, 2, "startsWith(text, start)", "whether the text starts with start", text2(func(s, p string) any { return strings.HasPrefix(s, p) })},
	{"endsWith", 2, 2, "endsWith(text, end)", "whether the text ends with end", text2(func(s, p string) any { return strings.HasSuffix(s, p) })},
	{"replace", 3, 3, "replace(text, old, new)", "the text with each old as new", func(args []any) (any, error) {
		return strings.ReplaceAll(String(args[0]), String(args[1]), String(args[2])), nil
	}},
	{"match", 2, 2, "match(text, regex)", "whether the regex matches the text, ^ and $ at each line", func(args []any) (any, error) {
		re, err := regexp.Compile("(?m)" + String(args[1]))
		if err != nil {
			return nil, err
		}
		return re.MatchString(String(args[0])), nil
	}},
}

var listFunctions = []entry{
	{"map", 2, 2, "map(list, x => ...)", "the list with each item made into what the lambda gives", mapList},
	{"filter", 2, 2, "filter(list, x => ...)", "the items the lambda holds for", filterList},
	{"any", 2, 2, "any(list, x => ...)", "whether the lambda holds for an item", anyItem},
	{"all", 2, 2, "all(list, x => ...)", "whether the lambda holds for every item", allItems},
	{"find", 2, 2, "find(list, x => ...)", "the first item the lambda holds for, or null", findItem},
	{"first", 1, 1, "first(list)", "the list's first item, or null", func(args []any) (any, error) { return at(args[0], 0) }},
	{"last", 1, 1, "last(list)", "the list's last item, or null", func(args []any) (any, error) { return at(args[0], -1) }},
	{"sort", 1, 1, "sort(list)", "the items in order: as numbers when all are, as texts otherwise", sortList},
	{"unique", 1, 1, "unique(list)", "the items without the repeated ones", uniqueItems},
	{"reverse", 1, 1, "reverse(list)", "the items from the last", func(args []any) (any, error) {
		list, err := List(args[0])
		list = slices.Clone(list)
		slices.Reverse(list)
		return orEmpty(list), err
	}},
	{"keys", 1, 1, "keys(object)", "an object's keys, in order", func(args []any) (any, error) { return objectParts(args[0], true) }},
	{"values", 1, 1, "values(object)", "an object's values, in the order of its keys", func(args []any) (any, error) { return objectParts(args[0], false) }},
}

var otherFunctions = []entry{
	{"json", 1, 1, "json(x)", "the value as JSON: a text in quotes, a list or object as it is", func(args []any) (any, error) { return JSON(args[0]) }},
	{"fromJSON", 1, 1, "fromJSON(text)", "the value a JSON text holds, to read its fields", func(args []any) (any, error) { return FromJSON(String(args[0])) }},
	{"number", 1, 1, "number(x)", "the value as a number", func(args []any) (any, error) {
		n, ok := Number(args[0])
		if !ok {
			return nil, fmt.Errorf("%s is no number", describe(args[0]))
		}
		return n, nil
	}},
	{"string", 1, 1, "string(x)", "the value as text", func(args []any) (any, error) { return String(args[0]), nil }},
	{"if", 3, 3, "if(condition, yes, no)", "yes when the condition holds, no otherwise", func(args []any) (any, error) {
		if Truthy(args[0]) {
			return args[1], nil
		}
		return args[2], nil
	}},
	{"min", 1, -1, "min(a, b, ...)", "the smallest number, of the arguments or of one list", func(args []any) (any, error) { return extreme(args, -1) }},
	{"max", 1, -1, "max(a, b, ...)", "the largest number, of the arguments or of one list", func(args []any) (any, error) { return extreme(args, 1) }},
	{"round", 1, 2, "round(x, digits)", "the number rounded, to digits after the point (none without it)", round},
	{"abs", 1, 1, "abs(x)", "the number without its sign", func(args []any) (any, error) {
		n, err := number(args[0])
		return math.Abs(n), err
	}},
}

// orEmpty is a list that is [] rather than null when it has no items.
func orEmpty(list []any) []any {
	if list == nil {
		return []any{}
	}
	return list
}

func number(value any) (float64, error) {
	n, ok := Number(value)
	if !ok {
		return 0, fmt.Errorf("%s is no number", describe(value))
	}
	return n, nil
}

func length(args []any) (any, error) {
	switch v := args[0].(type) {
	case []any:
		return float64(len(v)), nil
	case map[string]any:
		return float64(len(v)), nil
	}
	return float64(utf8.RuneCountInString(String(args[0]))), nil
}

func join(args []any) (any, error) {
	list, err := List(args[0])
	if err != nil {
		return nil, err
	}
	sep := " "
	if len(args) > 1 {
		sep = String(args[1])
	}
	texts := make([]string, len(list))
	for i, item := range list {
		texts[i] = String(item)
	}
	return strings.Join(texts, sep), nil
}

// each calls the lambda, the second argument, with each item of the list, the first, and its
// index; visit takes what it gives and tells whether to go on.
func each(args []any, visit func(item, given any) bool) error {
	list, err := List(args[0])
	if err != nil {
		return err
	}
	fn, ok := args[1].(*Lambda)
	if !ok {
		return fmt.Errorf("the second argument is a lambda, as x => x.name, not %s", describe(args[1]))
	}
	for i, item := range list {
		given, err := fn.call(item, float64(i))
		if err != nil {
			return err
		}
		if !visit(item, given) {
			return nil
		}
	}
	return nil
}

func mapList(args []any) (any, error) {
	out := []any{}
	err := each(args, func(_, given any) bool { out = append(out, given); return true })
	return out, err
}

func filterList(args []any) (any, error) {
	out := []any{}
	err := each(args, func(item, given any) bool {
		if Truthy(given) {
			out = append(out, item)
		}
		return true
	})
	return out, err
}

func anyItem(args []any) (any, error) {
	found := false
	err := each(args, func(_, given any) bool { found = Truthy(given); return !found })
	return found, err
}

func allItems(args []any) (any, error) {
	held := true
	err := each(args, func(_, given any) bool { held = Truthy(given); return held })
	return held, err
}

func findItem(args []any) (any, error) {
	var found any
	err := each(args, func(item, given any) bool {
		if Truthy(given) {
			found = item
			return false
		}
		return true
	})
	return found, err
}

func at(value any, i int) (any, error) {
	list, err := List(value)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return list[(i+len(list))%len(list)], nil
}

func sortList(args []any) (any, error) {
	list, err := List(args[0])
	if err != nil {
		return nil, err
	}
	list = slices.Clone(list)
	numbers := !slices.ContainsFunc(list, func(item any) bool { _, ok := Number(item); return !ok })
	slices.SortStableFunc(list, func(a, b any) int {
		if numbers {
			x, _ := Number(a)
			y, _ := Number(b)
			return cmp.Compare(x, y)
		}
		return strings.Compare(String(a), String(b))
	})
	return orEmpty(list), nil
}

func uniqueItems(args []any) (any, error) {
	list, err := List(args[0])
	out := []any{}
	for _, item := range list {
		if !slices.ContainsFunc(out, func(seen any) bool { return Equal(seen, item) }) {
			out = append(out, item)
		}
	}
	return out, err
}

func objectParts(value any, keys bool) (any, error) {
	value, err := structured(value)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is no object", describe(value))
	}
	parts := []any{}
	for _, key := range slices.Sorted(maps.Keys(object)) {
		if keys {
			parts = append(parts, key)
		} else {
			parts = append(parts, object[key])
		}
	}
	return parts, nil
}

// extreme is the smallest number of args, at sign -1, or the largest, at 1; one list argument
// gives its items.
func extreme(args []any, sign int) (any, error) {
	if list, ok := args[0].([]any); ok && len(args) == 1 {
		args = list
	}
	if len(args) == 0 {
		return nil, nil
	}
	best, err := number(args[0])
	for _, arg := range args[1:] {
		n, problem := number(arg)
		if problem != nil {
			return nil, problem
		}
		if cmp.Compare(n, best) == sign {
			best = n
		}
	}
	return best, err
}

func round(args []any) (any, error) {
	n, err := number(args[0])
	if err != nil {
		return nil, err
	}
	digits := 0.0
	if len(args) > 1 {
		if digits, err = number(args[1]); err != nil {
			return nil, err
		}
	}
	scale := math.Pow(10, digits)
	return math.Round(n*scale) / scale, nil
}
