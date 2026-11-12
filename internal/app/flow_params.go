package app

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/naming"
)

// A flow's params are made and changed in the inspect, on the params card above the start: a adds
// one, offering every type a param can have, and enter changes or removes one.

// paramKind is a way a param can be added: its type, and whether it is secret or read from the
// environment.
type paramKind struct {
	key, label, kind string
	secret, env      bool
}

var paramKinds = []paramKind{
	{"t", "text", flow.TextParam, false, false},
	{"n", "number", flow.NumberParam, false, false},
	{"b", "yes or no", flow.BoolParam, false, false},
	{"c", "one of a few choices", flow.ChoiceParam, false, false},
	{"p", "path of a file or folder", flow.PathParam, false, false},
	{"s", "secret, never remembered", flow.TextParam, true, false},
	{"e", "from the environment", flow.TextParam, false, true},
}

// addFlowParam asks a new param's name, its type, what the type needs and its default.
func (a *App) addFlowParam(f flow.Flow) {
	a.openPrompt("new param", "", "{enter} name it  esc cancel  letters, digits and _", func(name string) {
		switch {
		case !naming.IsIdentifier(name) || name == "steps":
			a.statusMessage = fmt.Sprintf("%q cannot be a param's name: letters, digits and _, not steps", name)
			return
		case hasParam(f, name):
			a.statusMessage = "flow " + f.Name + " already has a param " + name
			return
		}
		var options []choiceOption
		for _, k := range paramKinds {
			options = append(options, choiceOption{key: k.key, label: k.label, pick: func() {
				a.askParamDetails(f, name, flow.Param{Type: k.kind, Secret: k.secret}, k.env)
			}})
		}
		a.openChoice("type of "+name, options...)
	})
}

func hasParam(f flow.Flow, name string) bool {
	_, ok := f.Params[name]
	return ok
}

// askParamDetails asks what the param's type needs, its variable for env, its choices for a
// choice, then its default, and saves it.
func (a *App) askParamDetails(f flow.Flow, name string, p flow.Param, env bool) {
	switch {
	case env && p.From == "":
		a.openPrompt("variable of "+name, strings.ToUpper(name), "{enter} the environment variable it reads  esc cancel", func(variable string) {
			p.From = "env:" + strings.TrimSpace(variable)
			a.askParamDetails(f, name, p, env)
		})
	case p.Type == flow.ChoiceParam && len(p.Choices) == 0:
		a.openPrompt("choices of "+name, "", "{enter} the choices, a space between  esc cancel", func(text string) {
			p.Choices = strings.Fields(text)
			if len(p.Choices) == 0 {
				a.statusMessage = "a choice needs its choices"
				return
			}
			a.askParamDetails(f, name, p, env)
		})
	default:
		a.askParamDefault(f, name, p, func(p flow.Param) { a.saveFlowParam(f, name, p) })
	}
}

// askParamDefault asks the param's default as its type has it: one of its values, or typed.
func (a *App) askParamDefault(f flow.Flow, name string, p flow.Param, done func(flow.Param)) {
	if values := p.Values(); values != nil {
		var options []choiceOption
		for i, value := range values {
			options = append(options, choiceOption{key: fmt.Sprint(i + 1), label: value, pick: func() {
				p.Default = value
				done(p)
			}})
		}
		a.openChoice("default of "+name, options...)
		return
	}
	a.openPrompt("default of "+name, p.Default, "{enter} save  esc cancel  empty has no default", func(value string) {
		p.Default = value
		done(p)
	})
}

func (a *App) saveFlowParam(f flow.Flow, name string, p flow.Param) {
	err := a.changeFlow(f, func(f *flow.Flow) error {
		if f.Params == nil {
			f.Params = map[string]flow.Param{}
		}
		f.Params[name] = p
		return nil
	})
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.notice = "param " + name + " saved in flow " + f.Name
}

// editFlowParams asks which param to change, then what to do with it.
func (a *App) editFlowParams(f flow.Flow) {
	names := slices.Sorted(maps.Keys(f.Params))
	if len(names) == 0 {
		a.notice = a.hint("flows", "flow "+f.Name+" has no params; {a} adds one")
		return
	}
	var options []choiceOption
	for i, name := range names {
		options = append(options, choiceOption{key: choiceKey(i), label: name + "  " + paramText(f.Params[name]), pick: func() { a.editFlowParam(f, name) }})
	}
	a.openChoice("params of "+f.Name, options...)
}

func (a *App) editFlowParam(f flow.Flow, name string) {
	p := f.Params[name]
	a.openChoice(name+"  "+paramText(p),
		choiceOption{key: "d", label: "default", pick: func() {
			a.askParamDefault(f, name, p, func(p flow.Param) { a.saveFlowParam(f, name, p) })
		}},
		choiceOption{key: "r", label: map[bool]string{true: "not required", false: "required"}[p.Required], pick: func() {
			p.Required = !p.Required
			a.saveFlowParam(f, name, p)
		}},
		choiceOption{key: "l", label: "label in the form", pick: func() {
			a.openPrompt("label of "+name, p.Prompt, "{enter} save  esc cancel  empty shows the name", func(label string) {
				p.Prompt = label
				a.saveFlowParam(f, name, p)
			})
		}},
		choiceOption{key: "x", label: "remove", pick: func() {
			if err := a.changeFlow(f, func(f *flow.Flow) error { delete(f.Params, name); return nil }); err != nil {
				a.statusMessage = err.Error()
			}
		}},
	)
}

// choiceKey is the key of the i-th option of a long list: 1 to 9, then a to z.
func choiceKey(i int) string {
	if i < 9 {
		return fmt.Sprint(i + 1)
	}
	return string(rune('a' + i - 9))
}

// paramText tells a param's type and default, as its card and menus show it.
func paramText(p flow.Param) string {
	kind := orDefault(p.Type, flow.TextParam)
	switch {
	case p.From != "":
		kind += " from $" + strings.TrimPrefix(p.From, "env:")
	case p.Secret:
		kind = "secret " + kind
	case p.Type == flow.ChoiceParam:
		kind += " " + strings.Join(p.Choices, "|")
	}
	text := kind
	if p.Default != "" && !p.Secret {
		text += " = " + p.Default
	}
	if p.Required {
		text += " required"
	}
	return text
}
