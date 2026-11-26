// Package expr reads and evaluates noct's expressions: the conditions of a flow's if, until and
// switch, a repeat's each, and what a $ reads in a text. A name alone reads a variable or a
// step (login, build.output, hosts[0]); a name with ( ) calls a function (len(hosts)); and, or and
// not (&&, || and !) join conditions; == compares; x => body is a lambda for map, filter and the
// rest. Every value a flow keeps is text, so a text that reads as a number is a number to
// compare and to count with (exit == 0, count + 1). See ADR-074 in docs/decisoes.md.
package expr

import (
	"fmt"
	"strings"
)

// Expr is an expression read and checked, ready to evaluate.
type Expr struct {
	root node
	text string
}

// Scope finds the value of a name the expression reads: a variable's text or a list's or
// object's value, or a step's fields as a map. It tells false for a name it does not have.
type Scope func(name string) (any, bool)

// Parse reads text as one expression. It tells what does not parse, a function that does not
// exist and one given too many or too few arguments.
func Parse(text string) (*Expr, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("the expression is empty")
	}
	p := newParser(text)
	root, err := p.expression()
	if err != nil {
		return nil, err
	}
	if p.tok.kind != endToken {
		return nil, p.errorf("the expression goes on after its end")
	}
	return &Expr{root: root, text: text}, nil
}

// ParseRef reads the expression a $ in a text starts, without the $: a name and the fields and
// indexes right after it, $build.output or $hosts[0], or a function's call with its arguments,
// $join(hosts, ", "). A dot that no name follows ends it, as at the end of a sentence. It tells
// how many bytes of text the expression takes.
func ParseRef(text string) (*Expr, int, error) {
	p := newParser(text)
	if p.tok.kind != nameToken || p.tok.at != 0 {
		return nil, 0, fmt.Errorf("a $ reads a name")
	}
	root, err := p.name()
	if err != nil {
		return nil, 0, err
	}
	for {
		rest := text[p.end:]
		switch {
		case strings.HasPrefix(rest, ".") && len(rest) > 1 && IsNameStart(rune(rest[1])):
			root, err = p.field(root)
		case strings.HasPrefix(rest, "["):
			root, err = p.index(root)
		default:
			return &Expr{root: root, text: text[:p.end]}, p.end, nil
		}
		if err != nil {
			return nil, 0, err
		}
	}
}

// Eval evaluates the expression with the names in scope.
func (e *Expr) Eval(scope Scope) (any, error) {
	return e.root.eval(&env{scope: scope})
}

func (e *Expr) String() string { return e.text }

// Names are the names the expression reads, each with the fields read right after it: login is
// [login], build.output is [build output]. A lambda's own params are not among them.
func (e *Expr) Names() [][]string {
	var names [][]string
	walk(e.root, map[string]bool{}, func(path []string) { names = append(names, path) })
	return names
}

// walk visits each name a node reads that bound does not hold, with its fields.
func walk(n node, bound map[string]bool, visit func([]string)) {
	if path, ok := staticPath(n); ok {
		if !bound[path[0]] {
			visit(path)
		}
		return
	}
	if l, ok := n.(lambdaNode); ok {
		inner := map[string]bool{}
		for name := range bound {
			inner[name] = true
		}
		for _, name := range l.params {
			inner[name] = true
		}
		walk(l.body, inner, visit)
		return
	}
	for _, child := range children(n) {
		walk(child, bound, visit)
	}
}

// staticPath is a name and the fields after it, when n is only that.
func staticPath(n node) ([]string, bool) {
	switch n := n.(type) {
	case nameNode:
		return []string{n.name}, true
	case fieldNode:
		if path, ok := staticPath(n.of); ok {
			return append(path, n.name), true
		}
	}
	return nil, false
}

func children(n node) []node {
	switch n := n.(type) {
	case fieldNode:
		return []node{n.of}
	case indexNode:
		return []node{n.of, n.index}
	case callNode:
		return n.args
	case unaryNode:
		return []node{n.operand}
	case binaryNode:
		return []node{n.left, n.right}
	case listNode:
		return n.items
	case objectNode:
		return n.values
	}
	return nil
}
