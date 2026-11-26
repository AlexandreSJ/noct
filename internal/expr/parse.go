package expr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// parser reads an expression's tokens into its tree, each operator over the ones it binds
// tighter. It reads one token ahead.
type parser struct {
	lex lexer
	tok token
	// end is where the token before tok ends.
	end int
}

func newParser(text string) *parser {
	p := &parser{lex: lexer{text: text}}
	p.advance()
	return p
}

func (p *parser) advance() {
	p.end = p.tok.end
	p.tok = p.lex.next()
}

// mark and reset go back to a token, for what is read only if what follows it fits.
func (p *parser) mark() (int, token) { return p.lex.at, p.tok }
func (p *parser) reset(at int, tok token) {
	p.lex.at, p.tok = at, tok
}

// is tells whether the token is the operator or keyword op.
func (p *parser) is(op string) bool {
	return (p.tok.kind == opToken || p.tok.kind == nameToken) && p.tok.text == op
}

// isAny tells which of ops the token is.
func (p *parser) isAny(ops ...string) (string, bool) {
	for _, op := range ops {
		if p.is(op) {
			return op, true
		}
	}
	return "", false
}

// expect reads op, which what comes before it needs.
func (p *parser) expect(op, before string) error {
	if !p.is(op) {
		return p.errorf("%s needs %s", before, op)
	}
	p.advance()
	return nil
}

// errorf is a problem at the token, told with the text there; at text no expression holds, it is
// what that text is.
func (p *parser) errorf(format string, args ...any) error {
	if p.tok.kind == badToken {
		return errors.New(p.tok.text)
	}
	near := p.lex.text[min(p.tok.at, len(p.lex.text)):]
	if near == "" {
		return fmt.Errorf(format+", and the expression ends there", args...)
	}
	if len(near) > 16 {
		near = near[:16] + "…"
	}
	return fmt.Errorf(format+", near %q", append(args, near)...)
}

// expression is a lambda, or the operators of every level over their values.
func (p *parser) expression() (node, error) {
	if lambda, ok, err := p.lambda(); ok || err != nil {
		return lambda, err
	}
	return p.binary(0)
}

// lambda reads x => body or (a, b) => body; at anything else it tells false, having read nothing.
func (p *parser) lambda() (node, bool, error) {
	at, tok := p.mark()
	params, ok := p.lambdaParams()
	if !ok || !p.is("=>") {
		p.reset(at, tok)
		return nil, false, nil
	}
	p.advance()
	body, err := p.expression()
	return lambdaNode{params: params, body: body}, true, err
}

func (p *parser) lambdaParams() ([]string, bool) {
	if p.isParam() {
		name := p.tok.text
		p.advance()
		return []string{name}, true
	}
	if !p.is("(") {
		return nil, false
	}
	p.advance()
	var params []string
	for !p.is(")") {
		if !p.isParam() {
			return nil, false
		}
		params = append(params, p.tok.text)
		p.advance()
		if p.is(",") {
			p.advance()
		}
	}
	p.advance()
	return params, true
}

func (p *parser) isParam() bool { return p.tok.kind == nameToken && !keywords[p.tok.text] }

// keywords are the names an expression keeps for itself.
var keywords = map[string]bool{"and": true, "or": true, "not": true, "in": true, "true": true, "false": true, "null": true}

// levels are the binary operators from the loosest: ??, or, and, the comparisons, the sums and
// the products. not sits between and and the comparisons, as in Python.
var levels = [][]string{
	{"??"},
	{"or", "||"},
	{"and", "&&"},
	nil, // not
	{"==", "!=", "<", "<=", ">", ">=", "in"},
	{"+", "-"},
	{"*", "/", "%"},
}

const (
	notLevel     = 3
	compareLevel = 4
)

func (p *parser) binary(level int) (node, error) {
	switch level {
	case len(levels):
		return p.unary()
	case notLevel:
		return p.not()
	}
	left, err := p.binary(level + 1)
	if err != nil {
		return nil, err
	}
	for {
		op, ok := p.binaryOp(level)
		if !ok {
			return left, nil
		}
		right, err := p.binary(level + 1)
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
		if _, again := p.isAny(levels[compareLevel]...); again && level == compareLevel {
			return nil, p.errorf("one comparison at a time: join them with and")
		}
	}
}

// binaryOp reads an operator of the level, not in among the comparisons.
func (p *parser) binaryOp(level int) (string, bool) {
	if level == compareLevel && p.is("not") {
		at, tok := p.mark()
		if p.advance(); p.is("in") {
			p.advance()
			return "not in", true
		}
		p.reset(at, tok)
		return "", false
	}
	op, ok := p.isAny(levels[level]...)
	if ok {
		p.advance()
	}
	return op, ok
}

// not reads not x, which holds the comparison after it: not a == b is not (a == b).
func (p *parser) not() (node, error) {
	if !p.is("not") {
		return p.binary(notLevel + 1)
	}
	p.advance()
	operand, err := p.not()
	return unaryNode{op: "not", operand: operand}, err
}

// unary reads -x and !x, which hold only the value after them: !a == b is (!a) == b.
func (p *parser) unary() (node, error) {
	op, ok := p.isAny("-", "!")
	if !ok {
		return p.postfix()
	}
	p.advance()
	operand, err := p.unary()
	return unaryNode{op: op, operand: operand}, err
}

// postfix reads a value and the fields and indexes after it: deploy.output, hosts[0].
func (p *parser) postfix() (node, error) {
	value, err := p.primary()
	for err == nil {
		switch {
		case p.is("."):
			value, err = p.field(value)
		case p.is("["):
			value, err = p.index(value)
		default:
			return value, nil
		}
	}
	return nil, err
}

// field reads .name after a value, the token being the dot.
func (p *parser) field(of node) (node, error) {
	p.advance()
	if p.tok.kind != nameToken {
		return nil, p.errorf("a . needs the name of a field after it")
	}
	name := p.tok.text
	p.advance()
	return fieldNode{of: of, name: name}, nil
}

// index reads [expression] after a value, the token being the [.
func (p *parser) index(of node) (node, error) {
	p.advance()
	at, err := p.expression()
	if err != nil {
		return nil, err
	}
	return indexNode{of: of, index: at}, p.expect("]", "an index")
}

func (p *parser) primary() (node, error) {
	tok := p.tok
	switch {
	case tok.kind == numberToken:
		n, _ := strconv.ParseFloat(tok.text, 64) // the lexer read digits
		p.advance()
		return literal{value: n}, nil
	case tok.kind == stringToken:
		p.advance()
		return literal{value: tok.text}, nil
	case tok.kind == nameToken:
		return p.name()
	case p.is("("):
		return p.group()
	case p.is("["):
		return p.list()
	case p.is("{"):
		return p.object()
	case tok.kind == endToken:
		return nil, p.errorf("a value is missing")
	}
	return nil, p.errorf("%s needs a value before it", tok.text)
}

// name reads a keyword's value, a variable or a function's call.
func (p *parser) name() (node, error) {
	tok := p.tok
	if value, ok := map[string]any{"true": true, "false": false, "null": nil}[tok.text]; ok {
		p.advance()
		return literal{value: value}, nil
	}
	if keywords[tok.text] {
		return nil, p.errorf("%s needs a value before it", tok.text)
	}
	p.advance()
	if !p.is("(") || p.tok.at != tok.at+len(tok.text) {
		return nameNode{name: tok.text}, nil
	}
	return p.call(tok.text)
}

// call reads a function's arguments, the token being the (, and checks how many they are.
func (p *parser) call(name string) (node, error) {
	fn, ok := functions[name]
	if !ok {
		return nil, fmt.Errorf("there is no function %s: the functions are %s", name, strings.Join(FunctionNames(), ", "))
	}
	args, err := p.items(")", "the arguments of "+name)
	if err != nil {
		return nil, err
	}
	if len(args) < fn.min || fn.max >= 0 && len(args) > fn.max {
		return nil, fmt.Errorf("%s takes %s", name, fn.arity())
	}
	return callNode{name: name, fn: fn, args: args}, nil
}

// items reads expressions apart by commas up to close, the token being what opens them.
func (p *parser) items(close, of string) ([]node, error) {
	p.advance()
	var items []node
	for !p.is(close) {
		item, err := p.expression()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if err := p.separator(close, of); err != nil {
			return nil, err
		}
	}
	p.advance()
	return items, nil
}

// separator reads the comma between items, or sees the end of them.
func (p *parser) separator(close, of string) error {
	switch {
	case p.is(","):
		p.advance()
	case !p.is(close):
		return p.errorf("%s are apart by commas and end with %s", of, close)
	}
	return nil
}

func (p *parser) group() (node, error) {
	p.advance()
	inner, err := p.expression()
	if err != nil {
		return nil, err
	}
	return inner, p.expect(")", "a (")
}

func (p *parser) list() (node, error) {
	items, err := p.items("]", "a list's items")
	return listNode{items: items}, err
}

// object reads {key: value, "other key": value}, the token being the {.
func (p *parser) object() (node, error) {
	var object objectNode
	p.advance()
	for !p.is("}") {
		if p.tok.kind != nameToken && p.tok.kind != stringToken {
			return nil, p.errorf("an object's key is a name or a text")
		}
		object.keys = append(object.keys, p.tok.text)
		p.advance()
		if err := p.expect(":", "a key"); err != nil {
			return nil, err
		}
		value, err := p.expression()
		if err != nil {
			return nil, err
		}
		object.values = append(object.values, value)
		if err := p.separator("}", "an object's keys"); err != nil {
			return nil, err
		}
	}
	p.advance()
	return object, nil
}
