package app

import (
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// Functions show in the inspect's list after the flows, marked ƒ, and are built like a flow: their
// params card also lists what they return. F makes one; a call step runs one, asking a value for
// each of its params.

// newFunction asks a new function's name and makes it, empty.
func (a *App) newFunction(created func(key string)) {
	a.openPrompt("new function", "", "{enter} create  esc cancel  letters, digits, . - _", func(name string) {
		f, err := flow.CreateFunction(flow.FunctionsDir(), name)
		if err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.reloadFlows()
		created(entryKey(f))
	})
}

// pickFunction asks which function a call step runs, then the value of each of its params.
func (a *App) pickFunction(picked func(flow.Step)) {
	var items []finderItem
	for _, fn := range a.functions {
		if fn.Problem == nil {
			items = append(items, finderItem{label: fn.Name, kind: fn.Description, open: func() { a.askCallValues(fn, picked) }})
		}
	}
	if len(items) == 0 {
		a.statusMessage = a.hint("flows", "no functions yet: {F} in the list of flows makes one")
		return
	}
	a.openFinderWith("function to call", items)
}

// askCallValues asks a call step's value for each param of fn in turn, offering its default.
func (a *App) askCallValues(fn flow.Flow, picked func(flow.Step)) {
	names := slices.Sorted(maps.Keys(fn.Params))
	step := flow.Step{Call: fn.Name, With: map[string]string{}}
	var ask func(i int)
	ask = func(i int) {
		if i == len(names) {
			if len(step.With) == 0 {
				step.With = nil
			}
			picked(step)
			return
		}
		p := fn.Params[names[i]]
		a.openPrompt(fn.Name+": "+p.Label(names[i]), p.Default, "{enter} next  esc cancel  a template, like {{.env}}", func(value string) {
			if value != "" && value != p.Default {
				step.With[names[i]] = value
			}
			ask(i + 1)
		})
	}
	ask(0)
}

// returnLines are what a function's params card lists of its returns.
func returnLines(f flow.Flow) []string {
	if !f.Function {
		return nil
	}
	if len(f.Returns) == 0 {
		return []string{"returns nothing · r adds a return"}
	}
	var lines []string
	for _, name := range slices.Sorted(maps.Keys(f.Returns)) {
		lines = append(lines, "↩ "+name+" = "+shortTemplate(f.Returns[name]))
	}
	return lines
}

// editReturn asks a function's return as name = template; a name alone removes it.
func (a *App) editReturn(f flow.Flow) {
	vars := f.Scope("", "", len(f.Steps), a.scripts.Find)
	a.openPrompt("return of "+f.Name, "", "{enter} save  esc cancel  name = {{.template}}; a name alone removes it", func(text string) {
		name, value, _ := strings.Cut(text, "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		err := a.changeFlow(f, func(f *flow.Flow) error {
			if f.Returns == nil {
				f.Returns = map[string]string{}
			}
			if value == "" {
				delete(f.Returns, name)
				return nil
			}
			f.Returns[name] = value
			return nil
		})
		if err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.notice = a.changedFunctionNotice(f)
	})
	a.withVariables(vars)
}

// changedFunctionNotice tells which flows a change to a function reaches.
func (a *App) changedFunctionNotice(f flow.Flow) string {
	callers := flow.Callers(append(slices.Clip(a.flows), a.functions...), f.Name)
	if len(callers) == 0 {
		return "function " + f.Name + " saved; nothing calls it yet"
	}
	return "function " + f.Name + " saved; it changes " + strings.Join(callers, ", ")
}
