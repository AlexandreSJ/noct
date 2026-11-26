package app

import (
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// Functions show in the inspect's list after the flows, marked ƒ, and are built like a flow: their
// params card also lists what they return. F makes one; a call step runs one, with a value for each
// of its params from its step form (callForm).

// newFunction asks a new function's name, starting with folder, and makes it, empty.
func (a *App) newFunction(folder string, created func(key string)) {
	a.openPrompt("new function", folder, "{enter} create  esc cancel  letters, digits, . - _  / for folders", func(name string) {
		f, err := flow.CreateFunction(flow.FunctionsDir(), name)
		if err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.reloadFlows()
		created(entryKey(f))
	})
}

// functionItems are the functions a call step can run, for the step picker to list.
func (a *App) functionItems(picked func(fn flow.Flow)) []finderItem {
	var items []finderItem
	for _, fn := range a.functions {
		if fn.Problem == nil {
			items = append(items, finderItem{label: fn.Name, kind: fn.Description, open: func() { picked(fn) }})
		}
	}
	if len(items) == 0 {
		a.statusMessage = a.hint("flows", "no functions yet: {F} in the list of flows makes one")
	}
	return items
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
		lines = append(lines, "↩ "+name+" = "+strings.TrimSpace(f.Returns[name]))
	}
	return lines
}

// changedFunctionNotice tells which flows a change to a function reaches.
func (a *App) changedFunctionNotice(f flow.Flow) string {
	callers := flow.Callers(append(slices.Clip(a.flows), a.functions...), f.Name)
	if len(callers) == 0 {
		return "function " + f.Name + " saved; nothing calls it yet"
	}
	return "function " + f.Name + " saved; it changes " + strings.Join(callers, ", ")
}
