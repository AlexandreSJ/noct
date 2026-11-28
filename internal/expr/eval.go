package expr

import (
	"cmp"
	"fmt"
	"math"
	"strings"
)

// node is a part of an expression's tree, which evaluates to a value.
type node interface {
	eval(e *env) (any, error)
}

type (
	literal  struct{ value any }
	nameNode struct{ name string }
	// fieldNode reads a field of a value: an object's key, a step's output.
	fieldNode struct {
		of   node
		name string
	}
	// indexNode reads an item of a list, from 0 and from the end below it, or an object's key.
	indexNode struct{ of, index node }
	callNode  struct {
		name string
		fn   *function
		args []node
	}
	unaryNode struct {
		op      string
		operand node
	}
	// text is a + that joins as texts, as one side is a quoted text: "1" + "1" is 11.
	binaryNode struct {
		op          string
		left, right node
		text        bool
	}
	listNode   struct{ items []node }
	objectNode struct {
		keys   []string
		values []node
	}
	lambdaNode struct {
		params []string
		body   node
	}
)

// env is where an expression evaluates: the scope's names and the params of the lambdas it is in.
type env struct {
	scope  Scope
	locals map[string]any
	outer  *env
}

func (e *env) lookup(name string) (any, error) {
	for at := e; at != nil; at = at.outer {
		if value, ok := at.locals[name]; ok {
			return value, nil
		}
	}
	if e.scope != nil {
		if value, ok := e.scope(name); ok {
			return value, nil
		}
	}
	return nil, fmt.Errorf("there is no variable or step %s: write %q for a text", name, name)
}

func (n literal) eval(*env) (any, error)    { return n.value, nil }
func (n nameNode) eval(e *env) (any, error) { return e.lookup(n.name) }

func (n fieldNode) eval(e *env) (any, error) {
	of, err := n.of.eval(e)
	if err != nil {
		return nil, err
	}
	return fieldOf(of, n.name)
}

func (n indexNode) eval(e *env) (any, error) {
	of, err := n.of.eval(e)
	if err != nil {
		return nil, err
	}
	at, err := n.index.eval(e)
	if err != nil {
		return nil, err
	}
	return indexOf(of, at)
}

func (n callNode) eval(e *env) (any, error) {
	args := make([]any, len(n.args))
	for i, arg := range n.args {
		value, err := arg.eval(e)
		if err != nil {
			return nil, err
		}
		args[i] = value
	}
	value, err := n.fn.call(args)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", n.name, err)
	}
	return value, nil
}

func (n unaryNode) eval(e *env) (any, error) {
	value, err := n.operand.eval(e)
	if err != nil {
		return nil, err
	}
	if n.op != "-" {
		return !Truthy(value), nil
	}
	number, ok := Number(value)
	if !ok {
		return nil, fmt.Errorf("- needs a number, not %s", describe(value))
	}
	return -number, nil
}

func (n binaryNode) eval(e *env) (any, error) {
	left, err := n.left.eval(e)
	if err != nil {
		return nil, err
	}
	switch n.op { // these may not need the right side
	case "and", "&&", "nand":
		if !Truthy(left) {
			return n.op == "nand", nil
		}
	case "or", "||", "nor":
		if Truthy(left) {
			return n.op != "nor", nil
		}
	case "??":
		if !isEmpty(left) {
			return left, nil
		}
	}
	right, err := n.right.eval(e)
	if err != nil {
		return nil, err
	}
	if n.text {
		return String(left) + String(right), nil
	}
	return operate(n.op, left, right)
}

// operate gives the value of a binary operator over its two sides.
func operate(op string, left, right any) (any, error) {
	switch op {
	case "and", "&&", "or", "||":
		return Truthy(right), nil
	case "nand", "nor": // the left side alone did not decide it
		return !Truthy(right), nil
	case "xor":
		return Truthy(left) != Truthy(right), nil
	case "??":
		return right, nil
	case "==":
		return Equal(left, right), nil
	case "!=":
		return !Equal(left, right), nil
	case "<", "<=", ">", ">=":
		return compare(op, left, right), nil
	case "in":
		return contains(right, left), nil
	case "not in":
		return !contains(right, left), nil
	case "+":
		return add(left, right), nil
	}
	return arithmetic(op, left, right)
}

// compare orders two values: as numbers when both are, and as texts otherwise.
func compare(op string, left, right any) bool {
	order := strings.Compare(String(left), String(right))
	if a, ok := Number(left); ok {
		if b, ok := Number(right); ok {
			order = cmp.Compare(a, b)
		}
	}
	switch op {
	case "<":
		return order < 0
	case "<=":
		return order <= 0
	case ">":
		return order > 0
	}
	return order >= 0
}

// add sums two numbers, joins two lists, and joins any other two values as texts.
func add(left, right any) any {
	if a, ok := Number(left); ok {
		if b, ok := Number(right); ok {
			return a + b
		}
	}
	a, leftList := left.([]any)
	b, rightList := right.([]any)
	if leftList && rightList {
		return append(append([]any{}, a...), b...)
	}
	return String(left) + String(right)
}

func arithmetic(op string, left, right any) (any, error) {
	a, ok := Number(left)
	if !ok {
		return nil, fmt.Errorf("%s needs numbers, not %s", op, describe(left))
	}
	b, ok := Number(right)
	if !ok {
		return nil, fmt.Errorf("%s needs numbers, not %s", op, describe(right))
	}
	switch op {
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	}
	if b == 0 {
		return nil, fmt.Errorf("%s %s 0: no number divides by 0", String(left), op)
	}
	if op == "/" {
		return a / b, nil
	}
	return math.Mod(a, b), nil
}

func (n listNode) eval(e *env) (any, error) {
	list := make([]any, len(n.items))
	for i, item := range n.items {
		value, err := item.eval(e)
		if err != nil {
			return nil, err
		}
		list[i] = value
	}
	return list, nil
}

func (n objectNode) eval(e *env) (any, error) {
	object := make(map[string]any, len(n.keys))
	for i, key := range n.keys {
		value, err := n.values[i].eval(e)
		if err != nil {
			return nil, err
		}
		object[key] = value
	}
	return object, nil
}

// Lambda is a function an expression writes, x => x.name, for map, filter and the others.
type Lambda struct {
	params []string
	body   node
	env    *env
}

func (n lambdaNode) eval(e *env) (any, error) {
	return &Lambda{params: n.params, body: n.body, env: e}, nil
}

// call runs the lambda with args for its params, in order; a param left without one is null.
func (l *Lambda) call(args ...any) (any, error) {
	locals := make(map[string]any, len(l.params))
	for i, name := range l.params {
		if i < len(args) {
			locals[name] = args[i]
		} else {
			locals[name] = nil
		}
	}
	return l.body.eval(&env{scope: l.env.scope, locals: locals, outer: l.env})
}
