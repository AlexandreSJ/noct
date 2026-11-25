package flow

import (
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/AlexandreSJ/noct/internal/script"
)

// A template reads a name as $name, and a step's field or an object's as $build.output or
// $obj.pass, besides the {{ }} of Go's templates, which stay for logic: {{if}}, {{range}},
// {{json .x}}. Only the names the flow knows are its own, so the shell keeps $HOME, $PATH and $1;
// ${name} and $$name are always the shell's, and so is a name a run line sets itself. Each value is escaped for where it lands: in a run
// line, one shell word outside quotes and the text of a quoted string inside them; in a list or
// object param's default, JSON. expandDollars rewrites the $ names as the {{ }} that read them.

// textKind is what a template's text is, which says how a value goes into it.
type textKind int

const (
	plainText textKind = iota
	// shellText is a run line, read by the shell.
	shellText
	// jsonText is a list or object param's default, read as JSON.
	jsonText
)

// knownNames are the names a flow's templates can read with $: its variables and its steps' ids.
type knownNames struct {
	vars, steps map[string]bool
}

// knownNames are the names the flow makes anywhere: its params, the variables its steps set,
// prompt, capture or export, the fields every step gives, and its steps' ids. A name a step makes
// reads empty before that step runs, but it is the flow's, never the shell's.
func (f Flow) knownNames(lookup func(id string) (script.Script, bool)) knownNames {
	known := knownNames{vars: map[string]bool{}, steps: map[string]bool{}}
	for _, name := range everyStepField {
		known.vars[name] = true
	}
	for name := range f.Params {
		known.vars[name] = true
	}
	walkSteps(f.Steps, "", "", func(s Step, _ string) {
		if s.ID != "" {
			known.steps[s.ID] = true
		}
		for _, field := range s.fields(lookup, f.functions) {
			known.vars[field] = isVariable(field)
		}
		for name := range s.Set {
			known.vars[name] = true
		}
		for _, outer := range s.Export {
			known.vars[outer] = true
		}
		if s.Var != "" {
			known.vars[s.Var] = true
		}
		if s.Repeat != nil && s.Repeat.Each != "" {
			known.vars[s.Repeat.EachVar()] = true
		}
	})
	maps.DeleteFunc(known.vars, func(_ string, ok bool) bool { return !ok })
	return known
}

// with are the names known and the ones in vars and steps besides, as a run has set them.
func (k knownNames) with(vars map[string]string, steps map[string]map[string]string) knownNames {
	both := knownNames{vars: maps.Clone(k.vars), steps: maps.Clone(k.steps)}
	if both.vars == nil {
		both.vars = map[string]bool{}
	}
	if both.steps == nil {
		both.steps = map[string]bool{}
	}
	for name := range vars {
		both.vars[name] = true
	}
	for id := range steps {
		both.steps[id] = true
	}
	return both
}

// quoting is where a value lands in a run line or a JSON text: outside quotes, or inside single
// or double quotes.
type quoting int

const (
	unquoted quoting = iota
	singleQuoted
	doubleQuoted
)

// readers are the template actions that put a value where it lands, for each kind of text and
// quoting.
var readers = map[textKind]map[quoting]string{
	plainText: {unquoted: "{{%s}}"},
	shellText: {unquoted: "{{quote %s}}", singleQuoted: "{{_sq %s}}", doubleQuoted: "{{_dq %s}}"},
	jsonText:  {unquoted: "{{json %s}}", doubleQuoted: "{{_js %s}}"},
}

// expandDollars rewrites each $name of text the names know, outside {{ }}, as the {{ }} that
// reads it, escaped for where it lands. It tells a name that is both a variable and a step's id.
func expandDollars(text string, kind textKind, known knownNames) (string, error) {
	if !strings.Contains(text, "$") {
		return text, nil
	}
	if kind == shellText { // a name the line sets itself, as for f in or n=, is the shell's there
		own := shellNames(text)
		known = knownNames{vars: maps.Clone(known.vars), steps: known.steps}
		maps.DeleteFunc(known.vars, func(name string, _ bool) bool { return own[name] })
	}
	e := expansion{text: []rune(text), kind: kind, known: known}
	return e.run()
}

// expansion is expandDollars under way: the text read up to at, written to out, and the quotes
// open there.
type expansion struct {
	text  []rune
	at    int
	out   strings.Builder
	kind  textKind
	known knownNames
	quote quoting
}

func (e *expansion) run() (string, error) {
	for e.at < len(e.text) {
		r := e.text[e.at]
		switch {
		case r == '{' && e.peek(1) == '{':
			e.copyAction()
		case r == '\\' && e.quote != singleQuoted && e.kind != plainText:
			e.copy(2) // an escaped character never opens or closes quotes
		case r == '$':
			if err := e.dollar(); err != nil {
				return "", err
			}
		default:
			e.track(r)
			e.copy(1)
		}
	}
	return e.out.String(), nil
}

func (e *expansion) peek(by int) rune {
	if e.at+by < len(e.text) {
		return e.text[e.at+by]
	}
	return 0
}

func (e *expansion) copy(n int) {
	end := min(e.at+n, len(e.text))
	e.out.WriteString(string(e.text[e.at:end]))
	e.at = end
}

// copyAction copies a {{ }} as it is: what is inside is the template's.
func (e *expansion) copyAction() {
	end := strings.Index(string(e.text[e.at:]), "}}")
	if end < 0 {
		e.copy(len(e.text))
		return
	}
	e.copy(len([]rune(string(e.text[e.at:])[:end+2])))
}

// track follows the quotes a character opens or closes: the shell's ' and ", JSON's ".
func (e *expansion) track(r rune) {
	switch {
	case e.kind == shellText && r == '\'' && e.quote != doubleQuoted:
		e.quote = map[quoting]quoting{unquoted: singleQuoted, singleQuoted: unquoted}[e.quote]
	case e.kind != plainText && r == '"' && e.quote != singleQuoted:
		e.quote = map[quoting]quoting{unquoted: doubleQuoted, doubleQuoted: unquoted}[e.quote]
	}
}

// dollar reads the $ at the cursor: a name the flow knows, $$name for the shell's $name, or a $
// left as it is.
func (e *expansion) dollar() error {
	if e.peek(1) == '$' && isNameStart(e.peek(2)) {
		e.out.WriteRune('$') // $$name is the shell's $name
		e.at += 2
		e.copyName()
		return nil
	}
	path := e.name(e.at + 1)
	if len(path) == 0 {
		e.copy(1)
		return nil
	}
	first, length := path[0], 1+len([]rune(strings.Join(path, ".")))
	isVar, isStep := e.known.vars[first], e.known.steps[first]
	switch {
	case isVar && isStep && len(path) > 1:
		return fmt.Errorf("$%s is both a variable and a step's id: rename one, or write {{.%s}} or {{.steps.%s}}", strings.Join(path, "."), first, first)
	case isStep && !isVar:
		path = append([]string{"steps"}, path...)
	case !isVar:
		e.copy(1 + len([]rune(first))) // not the flow's: the shell's, or text
		return nil
	}
	field := "." + strings.Join(path, ".")
	reader := readers[e.kind][e.quote]
	if reader == "" {
		reader = readers[e.kind][unquoted]
	}
	e.out.WriteString(fmt.Sprintf(reader, field))
	e.at += length
	return nil
}

// name reads the name at from, with the fields after it: obj.pass is [obj pass]. A dot that no
// name follows ends it, as at the end of a sentence.
func (e *expansion) name(from int) []string {
	var path []string
	at := from
	for {
		if at >= len(e.text) || !isNameStart(e.text[at]) {
			return path
		}
		end := at
		for end < len(e.text) && isNameRune(e.text[end]) {
			end++
		}
		path = append(path, string(e.text[at:end]))
		if end+1 >= len(e.text) || e.text[end] != '.' || !isNameStart(e.text[end+1]) {
			return path
		}
		at = end + 1
	}
}

// copyName copies the name at the cursor as text.
func (e *expansion) copyName() {
	for e.at < len(e.text) && isNameRune(e.text[e.at]) {
		e.copy(1)
	}
}

func isNameStart(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isNameRune(r rune) bool {
	return isNameStart(r) || r >= '0' && r <= '9'
}

// Escapes for a value inside quotes: a shell's double or single quotes, a JSON string.
func inDoubleQuotes(value any) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(fmt.Sprint(value))
}

func inSingleQuotes(value any) string {
	return strings.ReplaceAll(fmt.Sprint(value), "'", `'\''`)
}

func inJSONString(value any) (string, error) {
	quoted, err := toJSON(fmt.Sprint(value))
	return strings.TrimSuffix(strings.TrimPrefix(quoted, `"`), `"`), err
}

// shellSets finds the names a shell line gives a value: for name in, name=, read name.
var shellSets = regexp.MustCompile(`\bfor\s+([A-Za-z_]\w*)\s+in\b|(?:^|[\s;&|(])([A-Za-z_]\w*)=|\bread\s+(?:-\w+\s+)*([A-Za-z_]\w*)`)

// shellNames are the names a run line gives a value itself, whose $name is the shell's.
func shellNames(run string) map[string]bool {
	names := map[string]bool{}
	for _, match := range shellSets.FindAllStringSubmatch(run, -1) {
		names[match[1]+match[2]+match[3]] = true
	}
	return names
}
