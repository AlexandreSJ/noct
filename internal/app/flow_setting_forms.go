package app

import (
	"maps"
	"slices"
	"strconv"

	"github.com/AlexandreSJ/noct/internal/flow"
)

// Forms for what a step or a function has besides its kind: the variables a block exports, a
// step's retry and a function's returns, in place of the chart like a step's form.

func exportForm(s flow.Step) stepFormSpec {
	row := stepField{
		note: "a variable the block makes, then its name after it; empty keeps the name",
		check: func(outer string) string {
			if outer == "" {
				return ""
			}
			return checkVariable(outer)
		},
	}
	spec := stepFormSpec{title: "export from " + s.Label(), named: true, rowField: row, adds: "export"}
	for _, inner := range slices.Sorted(maps.Keys(s.Export)) {
		field := row
		field.label, field.cell = inner, textCellOf(s.Export[inner])
		spec.fields = append(spec.fields, field)
	}
	spec.build = func(v []formValue) (flow.Step, error) {
		s.Export = map[string]string{}
		for _, pair := range v {
			if pair.name != "" {
				s.Export[pair.name] = pair.value
			}
		}
		if len(s.Export) == 0 {
			s.Export = nil
		}
		return s, nil
	}
	return spec
}

func retryForm(s flow.Step) stepFormSpec {
	retry := flow.Retry{Max: 3}
	if s.Retry != nil {
		retry = *s.Retry
	}
	return stepFormSpec{
		title: "retry " + s.Label(),
		fields: []stepField{
			{label: "times", cell: textCellOf(countText(retry.Max)), check: checkCount, note: "how many times a failure runs it again, before on_fail; empty runs it once"},
			{label: "every", cell: textCellOf(retry.Every), check: checkDuration, note: "a wait before each try, as 10s; empty is none"},
		},
		build: func(v []formValue) (flow.Step, error) {
			times, _ := strconv.Atoi(v[0].value)
			s.Retry = nil
			if times > 0 {
				s.Retry = &flow.Retry{Max: times, Every: v[1].value}
			}
			return s, nil
		},
	}
}

// returnsForm asks a function's returns, a name and its value a row, and saves them.
func (a *App) returnsForm(f flow.Flow) stepFormSpec {
	row := stepField{note: "a name the caller reads, then its value: text with $names, read at the end", template: true}
	spec := stepFormSpec{title: "returns of " + f.Name, named: true, rowField: row, adds: "return"}
	for _, name := range slices.Sorted(maps.Keys(f.Returns)) {
		field := row
		field.label, field.cell = name, textCellOf(f.Returns[name])
		spec.fields = append(spec.fields, field)
	}
	spec.saveValues = func(v []formValue) error {
		return a.changeFlow(f, func(f *flow.Flow) error {
			f.Returns = map[string]string{}
			for _, pair := range v {
				if pair.name != "" {
					f.Returns[pair.name] = pair.value
				}
			}
			if len(f.Returns) == 0 {
				f.Returns = nil
			}
			return nil
		})
	}
	return spec
}

// exportStep edits what the block at path keeps for the steps after it; the help lists what its
// first list makes.
func (p *inspectPanel) exportStep(f flow.Flow, path string) {
	s, ok := f.Step(path)
	if !ok || !s.IsBlock() {
		p.app.statusMessage = "only an if, a switch or a repeat exports variables"
		return
	}
	inside := s.Branches()[0]
	vars := f.Scope(path, inside.Name, len(inside.Steps), p.app.scripts.Find)
	p.openEditor(p.app.newStepForm(exportForm(s), vars, p.lastValues(f, vars), false, p.replaceStep(f, path, s)))
}

func (p *inspectPanel) retryStep(f flow.Flow, path string) {
	s, ok := f.Step(path)
	if kind := s.Kind(); !ok || kind != flow.ScriptStep && kind != flow.CommandStep && kind != flow.AgentStep {
		p.app.statusMessage = "only a script, command or agent step retries"
		return
	}
	p.openEditor(p.app.newStepForm(retryForm(s), nil, nil, false, p.replaceStep(f, path, s)))
}

func (p *inspectPanel) editReturns(f flow.Flow) {
	vars := f.Scope("", "", len(f.Steps), p.app.scripts.Find)
	p.openEditor(p.app.newStepForm(p.app.returnsForm(f), vars, p.lastValues(f, vars), false, nil))
}
