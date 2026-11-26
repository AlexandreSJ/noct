package flow

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/AlexandreSJ/noct/internal/expr"
	"github.com/AlexandreSJ/noct/internal/script"
)

// A flow reads its values two ways (ADR-074). A condition, if, until, switch and a repeat's each,
// is an expression: names alone, login == expected and not ok. A text, a run line, a prompt, a
// with or set value, reads them with $: $name, $build.output, $hosts[0], a function's call,
// $join(hosts, ", "), whose arguments are expressions, or any expression in braces, ${count + 1}.
// Only the names the flow knows are its own, so the shell keeps $HOME, ${HOME}, $PATH and $1;
// $$name and $${name} are always the shell's, and so is a name a run line sets itself. Each value is escaped for where it lands: in a run line, one shell
// word outside quotes and the text of a quoted string inside them; in a list or object param's
// default, JSON.

// textKind is what a text is, which says how a value goes into it.
type textKind int

const (
	plainText textKind = iota
	// shellText is a run line, read by the shell.
	shellText
	// jsonText is a list or object param's default, read as JSON.
	jsonText
)

// knownNames are the names a flow reads: its variables and its steps' ids.
type knownNames struct {
	vars, steps map[string]bool
}

// knownNames are the names the flow makes anywhere: its variables, the ones its steps set,
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

// has tells a name the flow reads: a variable, a step, or steps, which holds every step by its
// id, for a step whose id is also a variable's name.
func (k knownNames) has(name string) bool { return k.vars[name] || k.steps[name] || name == "steps" }

// check tells a name an expression reads with a field that is both a variable and a step's id,
// which could be either. A name the flow does not know is a warning (Warnings), as a condition is
// often written before the step it reads, and a problem only when the run reads it.
func (k knownNames) check(e *expr.Expr) error {
	for _, path := range e.Names() {
		if k.vars[path[0]] && k.steps[path[0]] && len(path) > 1 {
			return fmt.Errorf("%s is both a variable and a step's id: rename one, or read the step's as steps.%s", strings.Join(path, "."), strings.Join(path, "."))
		}
	}
	return nil
}

// knowsAll tells that the flow knows every name the expression reads.
func (k knownNames) knowsAll(e *expr.Expr) bool {
	for _, path := range e.Names() {
		if !k.has(path[0]) {
			return false
		}
	}
	return true
}

// parseExpression reads a condition, or a switch's or each's value, and checks the names it reads.
func parseExpression(text string, known knownNames) (*expr.Expr, error) {
	if strings.Contains(text, "{{") {
		return nil, errors.New("{{ }} is gone from conditions: write the expression alone, as login == expected and not ok")
	}
	e, err := expr.Parse(text)
	if err != nil {
		return nil, err
	}
	return e, known.check(e)
}

// oldActions are the {{ }} of a text, and the .names inside them.
var (
	oldActions = regexp.MustCompile(`\{\{.*?\}\}`)
	oldNames   = regexp.MustCompile(`\.([A-Za-z_]\w*)`)
)

// oldTemplate tells a {{ }} in a text that reads one of the flow's names, as the templates before
// ADR-074 did: that text is no template now, and the shell would get it as it is. A {{ }} of
// names the flow does not know is the text of a program's own, as docker's {{.Names}}.
func oldTemplate(text string, known knownNames) error {
	for _, action := range oldActions.FindAllString(text, -1) {
		for _, name := range oldNames.FindAllStringSubmatch(action, -1) {
			if known.has(name[1]) || name[1] == "steps" {
				return fmt.Errorf("%s is no template now: write $%s", action, strings.TrimPrefix(name[0], "."))
			}
		}
	}
	return nil
}

// quoting is where a value lands in a run line or a JSON text: outside quotes, or inside single
// or double quotes.
type quoting int

const (
	unquoted quoting = iota
	singleQuoted
	doubleQuoted
	// arithmetic is the shell's $(( )), where a value goes in as in double quotes.
	arithmetic
	// heredocText is the body of a quoted heredoc, <<'EOF', which the shell keeps as it is: a
	// value goes in as it is. heredocShell is an unquoted one's, <<EOF, where the shell expands
	// $, ` and \, so those are escaped.
	heredocText
	heredocShell
)

// piece is a part of a text: text as it is, or a $ reference and the quotes it is in.
type piece struct {
	text  string
	ref   *expr.Expr
	quote quoting
}

// splitText reads a text into its parts: what stays as written, and each $ reference to a name
// the flow knows, or to a function. It tells a reference that does not parse.
func splitText(text string, kind textKind, known knownNames) ([]piece, error) {
	if !strings.Contains(text, "$") {
		return []piece{{text: text}}, nil
	}
	if kind == shellText { // a name the line sets itself, as for f in or n=, is the shell's there
		own := shellNames(text)
		known = knownNames{vars: maps.Clone(known.vars), steps: known.steps}
		maps.DeleteFunc(known.vars, func(name string, _ bool) bool { return own[name] })
	}
	s := splitter{text: text, kind: kind, known: known}
	return s.run()
}

// splitter is splitText under way: the text read up to at, the pieces made, and the quotes open
// there.
type splitter struct {
	text  string
	at    int
	kind  textKind
	known knownNames
	quote quoting
	// depth counts the parentheses open in a $(( )).
	depth int
	// heredoc is the word that ends a heredoc the line opened, whose body starts at the next line;
	// dash is <<-, whose end may be indented with tabs.
	heredoc       string
	heredocQuoted bool
	dash          bool
	plain         strings.Builder
	pieces        []piece
}

func (s *splitter) run() ([]piece, error) {
	for s.at < len(s.text) {
		c := s.text[s.at]
		switch {
		case c == '\\' && s.quote != singleQuoted && s.kind != plainText:
			s.copy(2) // an escaped character never opens or closes quotes
		case c == '$':
			if err := s.dollar(); err != nil {
				return nil, err
			}
		case c == '<' && s.opensHeredoc():
		case c == '\n' && s.heredoc != "":
			s.copy(1)
			if err := s.heredocBody(); err != nil {
				return nil, err
			}
		default:
			s.track(c)
			s.copy(1)
		}
	}
	s.flush()
	return s.pieces, nil
}

func (s *splitter) copy(n int) {
	end := min(s.at+n, len(s.text))
	s.plain.WriteString(s.text[s.at:end])
	s.at = end
}

func (s *splitter) flush() {
	if s.plain.Len() > 0 {
		s.pieces = append(s.pieces, piece{text: s.plain.String()})
		s.plain.Reset()
	}
}

// heredocStart is a heredoc's start: <<, <<- and the word that ends it, bare or quoted.
var heredocStart = regexp.MustCompile(`^<<(-?)[ \t]*(?:'([A-Za-z_]\w*)'|"([A-Za-z_]\w*)"|([A-Za-z_]\w*))`)

// opensHeredoc copies the << of a heredoc at the cursor, outside quotes in a run line, and keeps
// the word that ends its body.
func (s *splitter) opensHeredoc() bool {
	m := heredocStart.FindStringSubmatch(s.text[s.at:])
	if s.kind != shellText || s.quote != unquoted || m == nil {
		return false
	}
	s.heredoc, s.heredocQuoted, s.dash = m[2]+m[3]+m[4], m[4] == "", m[1] == "-"
	s.copy(len(m[0]))
	return true
}

// heredocBody reads the lines of a heredoc up to the one that ends it, a $ in them going in as
// the heredoc keeps it.
func (s *splitter) heredocBody() error {
	quote := map[bool]quoting{true: heredocText, false: heredocShell}[s.heredocQuoted]
	for s.at < len(s.text) {
		end := strings.IndexByte(s.text[s.at:], '\n')
		if end < 0 {
			end = len(s.text) - s.at
		}
		line := s.text[s.at : s.at+end]
		if line == s.heredoc || s.dash && strings.TrimLeft(line, "\t") == s.heredoc {
			s.heredoc = ""
			return nil // the end's line, and what follows it, is the shell's again
		}
		for stop := s.at + end; s.at < stop; {
			if s.text[s.at] != '$' {
				s.copy(1)
				continue
			}
			s.quote = quote
			err := s.dollar()
			s.quote = unquoted
			if err != nil {
				return err
			}
		}
		s.copy(1)
	}
	return nil
}

// track follows the quotes a character opens or closes: the shell's ' and ", JSON's ", and the
// shell's $(( )).
func (s *splitter) track(c byte) {
	if s.quote == arithmetic {
		s.depth += map[byte]int{'(': 1, ')': -1}[c]
		if s.depth == 0 {
			s.quote = unquoted
		}
		return
	}
	switch {
	case s.kind == shellText && c == '\'' && s.quote != doubleQuoted:
		s.quote = map[quoting]quoting{unquoted: singleQuoted, singleQuoted: unquoted}[s.quote]
	case s.kind != plainText && c == '"' && s.quote != singleQuoted:
		s.quote = map[quoting]quoting{unquoted: doubleQuoted, doubleQuoted: unquoted}[s.quote]
	}
}

// dollar reads the $ at the cursor: a reference the flow knows, $$name for the shell's $name, or
// a $ left as it is.
func (s *splitter) dollar() error {
	rest := s.text[s.at+1:]
	name := leadingName(rest)
	switch {
	case s.kind == shellText && s.quote == unquoted && strings.HasPrefix(rest, "(("):
		s.quote, s.depth = arithmetic, 2
		s.copy(3)
		return nil
	case strings.HasPrefix(rest, "$") && (leadingName(rest[1:]) != "" || strings.HasPrefix(rest[1:], "{")):
		s.plain.WriteByte('$') // $$name and $${name} are the shell's $name and ${name}
		s.at += 2
		s.copy(max(len(leadingName(rest[1:])), 1))
		return nil
	case strings.HasPrefix(rest, "{"):
		s.braces(rest)
		return nil
	case name == "" || !s.known.has(name) && !(expr.IsFunction(name) && strings.HasPrefix(rest[len(name):], "(")):
		s.copy(1 + len(name)) // not the flow's: the shell's, or text
		return nil
	}
	ref, n, err := expr.ParseRef(rest)
	if err != nil {
		return fmt.Errorf("$%s: %w", name, err)
	}
	if err := s.known.check(ref); err != nil {
		return fmt.Errorf("$%s: %w", ref, err)
	}
	s.flush()
	s.pieces = append(s.pieces, piece{ref: ref, quote: s.quote})
	s.at += 1 + n
	return nil
}

// braces reads ${expression}, rest being what follows the $: the flow's when the expression
// reads names and the flow knows them all, as ${count + 1} or ${name}s, and the shell's
// otherwise, as ${HOME} or ${1}.
func (s *splitter) braces(rest string) {
	end := closingBrace(rest)
	if end > 0 {
		e, err := expr.Parse(rest[1:end])
		if err == nil && len(e.Names()) > 0 && s.known.knowsAll(e) && s.known.check(e) == nil {
			s.flush()
			s.pieces = append(s.pieces, piece{ref: e, quote: s.quote})
			s.at += 1 + end + 1
			return
		}
	}
	s.copy(2)
}

// closingBrace is where the } that closes the { text starts with is, outside quotes, or -1.
func closingBrace(text string) int {
	depth, quote := 0, byte(0)
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case quote != 0 && c == '\\':
			i++
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '{':
			depth++
		case c == '}':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

// leadingName is the name text starts with, or "".
func leadingName(text string) string {
	if text == "" || !expr.IsNameStart(rune(text[0])) {
		return ""
	}
	end := 1
	for end < len(text) && expr.IsNameRune(rune(text[end])) {
		end++
	}
	return text[:end]
}

// render writes the text's parts with each reference's value escaped for where it lands.
func render(pieces []piece, kind textKind, scope expr.Scope) (string, error) {
	var out strings.Builder
	for _, p := range pieces {
		if p.ref == nil {
			out.WriteString(p.text)
			continue
		}
		value, err := p.ref.Eval(scope)
		if err != nil {
			return "", fmt.Errorf("$%s: %w", p.ref, err)
		}
		text, err := escape(value, kind, p.quote)
		if err != nil {
			return "", fmt.Errorf("$%s: %w", p.ref, err)
		}
		out.WriteString(text)
	}
	return out.String(), nil
}

// escape is a value as it goes into a kind of text, inside its quoting: one shell word outside
// quotes, the text of a shell's quoted string inside them, JSON in a JSON text.
func escape(value any, kind textKind, quote quoting) (string, error) {
	switch {
	case kind == shellText && quote == unquoted:
		return script.Quote(expr.String(value)), nil
	case kind == shellText && (quote == doubleQuoted || quote == arithmetic):
		return inDoubleQuotes(expr.String(value)), nil
	case kind == shellText && quote == heredocText:
		return expr.String(value), nil
	case kind == shellText && quote == heredocShell:
		return strings.NewReplacer(`\`, `\\`, "$", `\$`, "`", "\\`").Replace(expr.String(value)), nil
	case kind == shellText:
		return inSingleQuotes(expr.String(value)), nil
	case kind == jsonText && quote == doubleQuoted:
		return inJSONString(expr.String(value))
	case kind == jsonText:
		return expr.JSON(value)
	}
	return expr.String(value), nil
}

// Escapes for a value inside quotes: a shell's double or single quotes, a JSON string.
func inDoubleQuotes(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(value)
}

func inSingleQuotes(value string) string {
	return strings.ReplaceAll(value, "'", `'\''`)
}

func inJSONString(value string) (string, error) {
	quoted, err := expr.JSON(value)
	return strings.TrimSuffix(strings.TrimPrefix(quoted, `"`), `"`), err
}

// shellSets finds the names a shell line gives a value: for name in, read name, and name= where
// a command starts or after export, local and the like; echo key=$key sets nothing.
var shellSets = regexp.MustCompile(`\bfor\s+([A-Za-z_]\w*)\s+in\b|(?:^|[;&|({]|\b(?:export|local|declare|readonly|typeset)\s)\s*([A-Za-z_]\w*)=|\bread\s+(?:-\w+\s+)*([A-Za-z_]\w*)`)

// shellNames are the names a run line gives a value itself, whose $name is the shell's.
func shellNames(run string) map[string]bool {
	names := map[string]bool{}
	for _, match := range shellSets.FindAllStringSubmatch(run, -1) {
		names[match[1]+match[2]+match[3]] = true
	}
	return names
}

// scopeOf is what expressions read in a run: the variables, a list or object param decoded, the
// steps' fields, and empty for a name the flow knows that is not set.
func scopeOf(known knownNames, vars, jsonVars map[string]string, steps map[string]map[string]string) expr.Scope {
	return func(name string) (any, bool) {
		if value, ok := vars[name]; ok {
			return readAs(jsonVars[name], value), true
		}
		if fields, ok := steps[name]; ok {
			return stepValue(fields), true
		}
		switch {
		case known.vars[name]:
			return "", true
		case known.steps[name]:
			return nil, true // a step that has not run: each field reads as nothing
		case name == "steps":
			all := make(map[string]any, len(steps))
			for id, fields := range steps {
				all[id] = stepValue(fields)
			}
			return all, true
		}
		return nil, false
	}
}

func stepValue(fields map[string]string) map[string]any {
	step := make(map[string]any, len(fields))
	for field, value := range fields {
		step[field] = value
	}
	return step
}

// renderText renders a text of a kind over the variables and the steps' results.
func renderText(text string, kind textKind, known knownNames, vars, jsonVars map[string]string, steps map[string]map[string]string) (string, error) {
	known = known.with(vars, steps)
	pieces, err := splitText(text, kind, known)
	if err != nil {
		return "", err
	}
	return render(pieces, kind, scopeOf(known, vars, jsonVars, steps))
}
