package app

import (
	"cmp"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/script"
	"github.com/AlexandreSJ/noct/internal/secret"
)

// The forms of each kind of step: what each asks, and how its answers become the step. Each
// starts from the step it changes, the zero step to add one, so what the form does not ask (an
// id, a capture, the steps inside a block) stays as it was.

// stepFormFor is the form that changes s; false for the kinds changed as YAML, as a script step.
func (a *App) stepFormFor(f flow.Flow, path string, s flow.Step, vars []flow.Variable) (stepFormSpec, bool) {
	typeOf := func(name string) string { return f.VariableTypeOutside(name, path) }
	switch s.Kind() {
	case flow.ScriptStep:
		if sc, ok := a.scripts.Find(s.Script); ok && len(sc.Params) > 0 {
			return scriptStepForm(sc, s), true
		}
	case flow.CommandStep:
		return commandForm(s), true
	case flow.IfStep:
		return ifForm(s, vars), true
	case flow.SwitchStep:
		return switchForm(s), true
	case flow.RepeatStep:
		if s.Repeat.Each != "" {
			return eachForm(s), true
		}
		return repeatForm(s), true
	case flow.WaitStep:
		return waitForm(s), true
	case flow.PromptStep:
		return promptForm(s, typeOf), true
	case flow.SetStep:
		return setForm(s, typeOf), true
	case flow.UnsetStep:
		return unsetForm(s), true
	case flow.PrintStep:
		return printForm(s), true
	case flow.AgentStep:
		return a.agentForm(s), true
	case flow.CallStep:
		if fn, ok := flow.Find(a.functions, s.Call); ok && len(fn.Params) > 0 {
			return callForm(fn, s), true
		}
	}
	return stepFormSpec{}, false
}

func needed(what string) func(string) string {
	return func(value string) string {
		if value == "" {
			return what + " is needed"
		}
		return ""
	}
}

func checkDuration(value string) string {
	if _, err := time.ParseDuration(value); value != "" && err != nil {
		return "a duration, as 30s, 5m or 1h30m"
	}
	return ""
}

func checkCount(value string) string {
	if n, err := strconv.Atoi(value); value != "" && (err != nil || n < 1) {
		return "a whole number, 1 or more"
	}
	return ""
}

func checkVariable(value string) string {
	if !naming.IsIdentifier(value) || value == "steps" {
		return "a variable's name is letters, digits and _, and not steps"
	}
	return ""
}

func countText(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func commandForm(s flow.Step) stepFormSpec {
	return stepFormSpec{
		title: "command",
		fields: []stepField{{
			label: "run", cell: textCellOf(s.Run), template: true, check: needed("a command"),
			note:     "a command line the shell runs",
			examples: []string{"echo $name: one shell word, quoted for you", "echo ${count + 1} $join(hosts, \",\")"},
		}},
		build: func(v []formValue) (flow.Step, error) {
			s.Run = v[0].value
			return s, nil
		},
	}
}

func ifForm(s flow.Step, vars []flow.Variable) stepFormSpec {
	return stepFormSpec{
		title: "if",
		fields: []stepField{{
			label: "if", cell: textCellOf(s.If), expression: true, check: needed("a condition"),
			note:     "yes runs when it holds; no when false, 0, empty or nothing",
			examples: conditionExamples(flow.Readable(vars)),
		}},
		build: func(v []formValue) (flow.Step, error) {
			s.If = v[0].value
			return s, nil
		},
	}
}

// conditionExamples are conditions written with the variables in scope, so they read as the flow
// does.
func conditionExamples(vars []flow.Variable) []string {
	has := func(name string) bool {
		return slices.ContainsFunc(vars, func(v flow.Variable) bool { return v.Step == "" && v.Field == name })
	}
	var examples []string
	if has("ok") {
		examples = append(examples, "ok: the step before passed; not ok, when it failed")
	}
	for _, v := range vars {
		if v.Step == "" && !slices.Contains([]string{"output", "exit", "ok", "answer"}, v.Field) {
			examples = append(examples, v.Field+` == "value" and exit == 0`)
			break
		}
	}
	if has("output") {
		examples = append(examples, `match(output, "error"): a regex; contains(output, "done")`)
	}
	return append(examples,
		"and, or, not (&&, ||, !), nand, nor, xor; == != < <= > >=; + - * / %; x in [a, b]",
		`len(x), lower(x), any(list, x => startsWith(x, "web")): ? lists every function`)
}

func switchForm(s flow.Step) stepFormSpec {
	return stepFormSpec{
		title: "switch",
		fields: []stepField{
			{label: "on", cell: textCellOf(s.Switch), expression: true, check: needed("the value"), note: "the value the cases compare", examples: []string{"answer", "lower(env)"}},
			{label: "cases", cell: listCellOf(caseItems(s.Cases), -1, false, "case"), check: needed("a case"), note: "the values each case runs on; | puts two in one: bug|regression"},
		},
		build: func(v []formValue) (flow.Step, error) {
			s.Switch, s.Cases = v[0].value, switchCases(s.Cases, v[1].items)
			return s, nil
		},
	}
}

func caseItems(cases []flow.Case) []string {
	var items []string
	for _, c := range cases {
		items = append(items, strings.Join(c.Is, "|"))
	}
	return items
}

// switchCases makes a case for each of words, keeping the steps of the cases already there: a
// word kept keeps its case's steps, and a word changed takes those of the case it replaced, in
// the same place.
func switchCases(old []flow.Case, words []string) []flow.Case {
	kept := map[string]bool{}
	for _, word := range words {
		kept[word] = true
	}
	steps := map[string][]flow.Step{}
	for _, c := range old {
		steps[strings.Join(c.Is, "|")] = c.Steps
	}
	cases := make([]flow.Case, len(words))
	for i, word := range words {
		cases[i] = flow.Case{Is: strings.Split(word, "|"), Steps: steps[word]}
		if _, known := steps[word]; !known && i < len(old) && !kept[strings.Join(old[i].Is, "|")] {
			cases[i].Steps = old[i].Steps
		}
	}
	return cases
}

// defaultRepeatMax bounds a repeat until that sets no max, so it cannot run forever.
const defaultRepeatMax = 10

func repeatForm(s flow.Step) stepFormSpec {
	r := flow.Repeat{}
	if s.Repeat != nil {
		r = *s.Repeat
	}
	if s.Repeat == nil {
		r.Times = 3
	}
	return stepFormSpec{
		title: "repeat",
		fields: []stepField{
			{label: "times", cell: textCellOf(countText(r.Times)), check: checkCount, note: "how many passes; empty repeats until"},
			{label: "until", cell: textCellOf(r.Until), expression: true, note: "stops after the pass where it holds", examples: []string{"ok", `status.output == "ready"`}},
			{label: "max", cell: textCellOf(countText(r.Max)), check: checkCount, note: "the most passes an until runs; empty is 10"},
			{label: "every", cell: textCellOf(r.Every), check: checkDuration, note: "a wait between passes, as 30s; empty is none"},
		},
		build: func(v []formValue) (flow.Step, error) {
			times, _ := strconv.Atoi(v[0].value)
			most, _ := strconv.Atoi(v[2].value)
			if times == 0 && v[1].value == "" {
				return s, errors.New("times or until: how many passes, or when it stops")
			}
			if v[1].value != "" && most == 0 {
				most = defaultRepeatMax
			}
			s.Repeat = &flow.Repeat{Times: times, Until: v[1].value, Max: most, Every: v[3].value}
			return s, nil
		},
	}
}

func eachForm(s flow.Step) stepFormSpec {
	r := flow.Repeat{Each: "lines(output)"}
	if s.Repeat != nil {
		r = *s.Repeat
	}
	return stepFormSpec{
		title: "for each line",
		fields: []stepField{
			{label: "items of", cell: textCellOf(r.Each), expression: true, check: needed("a list or a text"), note: "its steps run once for each item of a list, or line of a text", examples: []string{"hosts: a list variable", "lines(output)", `filter(hosts, h => startsWith(h, "web"))`}},
			{label: "as", cell: textCellOf(r.EachVar()), check: checkVariable, note: "the variable each item is in"},
		},
		build: func(v []formValue) (flow.Step, error) {
			r.Each, r.As = v[0].value, v[1].value
			if r.As == flow.DefaultEachVar {
				r.As = ""
			}
			s.Repeat = &r
			return s, nil
		},
	}
}

func waitForm(s flow.Step) stepFormSpec {
	return stepFormSpec{
		title:  "wait",
		fields: []stepField{{label: "for", cell: textCellOf(orDefault(s.Wait, "5s")), check: func(v string) string { return needed("a duration")(v) + checkDuration(v) }, note: "as 5s, 1m30s"}},
		build: func(v []formValue) (flow.Step, error) {
			s.Wait = v[0].value
			return s, nil
		},
	}
}

// printRules tell, in the print form's help, how a print reads what it writes.
var printRules = []string{
	`• "hello, " + name: a text goes in quotes, "..." or '...'`,
	`• "1" + "1" is 11: + joins texts; 1 + 1 is 2: + adds numbers`,
	`• count + 1: a variable that reads as a number counts as one`,
	`• "total:", count: commas write each value, a space between`,
	`• \" \' \\ write the quote or the \ itself; \n a new line, \t a tab`,
	`• true and false as words; a list or an object as JSON`,
	`• names alone, no $: name, build.output, hosts[0], len(hosts)`,
}

func printForm(s flow.Step) stepFormSpec {
	return stepFormSpec{
		title: "print",
		fields: []stepField{{
			label: "print", cell: textCellOf(s.Print), expression: true, check: needed("what to print"),
			note: "writes values on a line of the run's output, as print does in a program", examples: printRules,
		}},
		build: func(v []formValue) (flow.Step, error) {
			s.Print = v[0].value
			return s, nil
		},
	}
}

func promptForm(s flow.Step, typeOf func(string) string) stepFormSpec {
	kind := cmp.Or(s.Type, defaultType(s.Var, typeOf, flow.TextParam))
	return stepFormSpec{
		title:  "prompt",
		typeOf: typeOf,
		changed: func(g *editGrid) {
			name, cell := strings.TrimSpace(g.at(1, stepFormValueColumn).value()), g.at(2, stepFormValueColumn)
			if name != cell.autoFor {
				cell.autoFor, cell.field = name, newTextField(defaultType(name, typeOf, flow.TextParam))
			}
		},
		problem: func(v []formValue, row int) string {
			if row != 2 {
				return ""
			}
			return retyped(v[1].value, v[2].value, typeOf)
		},
		fields: []stepField{
			{label: "ask", cell: escapedCellOf(s.Prompt), template: true, check: needed("a question"), note: `shown as written to whoever watches the run, as login:; \n is a new line, \t a tab`},
			{label: "into", cell: textCellOf(s.Var), check: checkVariable, note: "the variable the answer goes to, new or not"},
			{label: "type", cell: autoCell(optionCellOf(kind, flow.PromptTypes), s.Var), note: "a number asks again until the answer is one; inherit keeps the variable's type"},
			{label: "default", cell: textCellOf(s.Default), template: true, note: "what an empty answer gives"},
			{label: "secret", cell: gridCell{kind: toggleCell, on: s.Secret}, note: "the answer shows as • while typed, and the run never shows it"},
		},
		build: func(v []formValue) (flow.Step, error) {
			s.Prompt, s.Var, s.Default = v[0].value, v[1].value, v[3].value
			s.Type, s.Secret = writtenType(s.Var, v[2].value, typeOf, flow.TextParam), v[4].value != ""
			return s, nil
		},
	}
}

func setForm(s flow.Step, typeOf func(name string) string) stepFormSpec {
	row := stepField{note: "a name, then its value: text, $name reads others; $name reads it after; a var is what its value is", template: true,
		examples: []string{`{"user": "$login", "online": true}: braces, brackets and quotes close themselves, enter adds the comma, $ completes the variables`}}
	spec := stepFormSpec{title: "set", named: true, rowField: row, adds: "variable", types: flow.SetTypes, typeOf: typeOf}
	for _, name := range slices.Sorted(maps.Keys(s.Set)) {
		field := row
		field.label, field.cell, field.kind = name, textCellOf(s.Set[name]), cmp.Or(s.Types[name], defaultType(name, typeOf, flow.VarType))
		spec.fields = append(spec.fields, field)
	}
	if len(spec.fields) == 0 {
		row.kind = flow.VarType
		spec.fields = []stepField{row}
	}
	spec.build = func(v []formValue) (flow.Step, error) { return setStep(s, v, typeOf) }
	return spec
}

// defaultType is the type a set or prompt row starts with: inherit for a name the flow has
// already, fallback for a new one.
func defaultType(name string, typeOf func(string) string, fallback string) string {
	if name != "" && typeOf(name) != "" {
		return flow.InheritType
	}
	return fallback
}

// writtenType is the type a row keeps in the flow: none for inherit, nor for a new name's
// fallback, which is what no type gives.
func writtenType(name, kind string, typeOf func(string) string, fallback string) string {
	if kind == flow.InheritType || kind == fallback && typeOf(name) == "" {
		return ""
	}
	return kind
}

// retyped tells why kind cannot be a variable's, when the flow gives it another already.
func retyped(name, kind string, typeOf func(string) string) string {
	had := typeOf(name)
	if had == "" || had == flow.VarType || kind == flow.InheritType || kind == had {
		return ""
	}
	return name + " is a " + had + " already: keep inherit, or give the new value another name"
}

func setStep(s flow.Step, rows []formValue, typeOf func(string) string) (flow.Step, error) {
	s.Set, s.Types = map[string]string{}, map[string]string{}
	for _, row := range rows {
		if row.name == "" {
			continue
		}
		s.Set[row.name] = row.value
		if kind := writtenType(row.name, row.kind, typeOf, flow.VarType); kind != "" {
			s.Types[row.name] = kind
		}
	}
	if len(s.Types) == 0 {
		s.Types = nil
	}
	if len(s.Set) == 0 {
		return s, errors.New("a variable to set: its name, then its value")
	}
	return s, nil
}

func unsetForm(s flow.Step) stepFormSpec {
	return stepFormSpec{
		title:  "unset",
		fields: []stepField{{label: "names", cell: listCellOf(s.Unset, -1, false, "name"), check: needed("a name"), note: "the variables it clears; steps.build clears what step build gave"}},
		build: func(v []formValue) (flow.Step, error) {
			s.Unset = v[0].items
			return s, nil
		},
	}
}

// scriptStepForm asks the script step's with: a value for each of the script's params, templates
// read where the step is.
func scriptStepForm(sc script.Script, s flow.Step) stepFormSpec {
	spec := stepFormSpec{title: "script " + sc.ID()}
	for _, p := range sc.Params {
		spec.fields = append(spec.fields, stepField{
			label: p.Name, cell: textCellOf(s.With[p.Name]), template: true,
			note:  scriptParamNote(p),
			check: func(value string) string { return scriptValueProblem(p, value) },
		})
	}
	spec.build = func(v []formValue) (flow.Step, error) {
		s.Script, s.With = sc.ID(), map[string]string{}
		for i, p := range sc.Params {
			if v[i].value != "" {
				s.With[p.Name] = v[i].value
			}
		}
		if len(s.With) == 0 {
			s.With = nil
		}
		return s, nil
	}
	return spec
}

// scriptParamNote tells what a script's param takes when a flow step gives it.
func scriptParamNote(p script.Param) string {
	note := orDefault(p.Prompt, p.Name)
	switch {
	case len(p.Choices) > 0:
		note += ": one of " + strings.Join(p.Choices, ", ")
	case p.From != "":
		note += "; empty reads $" + strings.TrimPrefix(p.From, "env:")
	case p.Default != "":
		note += "; empty takes " + secret.Show(p.Default, p.Secret)
	}
	return note
}

// scriptValueProblem is why a step's value cannot be the script's param; a step runs without
// asking, so a param without a default needs one, and a value with a $ is known only when it runs.
func scriptValueProblem(p script.Param, value string) string {
	switch {
	case strings.Contains(value, "$"):
		return ""
	case value == "" && p.Default == "" && p.From == "":
		return "the script needs it: a step runs without asking"
	case value != "" && len(p.Choices) > 0 && !slices.Contains(p.Choices, value):
		return "one of " + strings.Join(p.Choices, ", ")
	}
	return ""
}

// callForm asks a value for each of the function's params, templates read where the call is.
func callForm(fn flow.Flow, s flow.Step) stepFormSpec {
	spec := stepFormSpec{title: "call " + fn.Name}
	names := fn.ParamNames()
	for _, name := range names {
		p := fn.Params[name]
		spec.fields = append(spec.fields, stepField{
			label: name, cell: textCellOf(s.With[name]), template: true,
			note:  paramText(p) + "; empty takes the default",
			check: func(value string) string { return callValueProblem(p, value) },
		})
	}
	spec.build = func(v []formValue) (flow.Step, error) {
		s.Call, s.With = fn.Name, map[string]string{}
		for i, name := range names {
			if v[i].value != "" {
				s.With[name] = v[i].value
			}
		}
		if len(s.With) == 0 {
			s.With = nil
		}
		return s, nil
	}
	return spec
}

// callValueProblem is why a call's value cannot be the param's; a value with a $ is known only
// when it runs.
func callValueProblem(p flow.Param, value string) string {
	switch {
	case strings.Contains(value, "$"):
		return ""
	case value == "" && p.Required && p.Default == "":
		return "the function needs it"
	}
	return p.Fits(value)
}

// agentForm asks what an agent step asks, the answers it may give, which agent and its timeout;
// custom agent also asks the name and command of an agent of the user's own, which it adds to the
// config.
func (a *App) agentForm(s flow.Step) stepFormSpec {
	agent := flow.Agent{Use: flow.DefaultAgent}
	if s.Agent != nil {
		agent = *s.Agent
	}
	if s.Agent == nil {
		s.Timeout = agentStepTimeout
	}
	all := readAgents(agentConfigOf(a.config))
	names := []string{flow.DefaultAgent, flow.AutoAgent}
	for _, choice := range agentChoices(all) {
		names = append(names, choice.Name)
	}
	if !slices.Contains(names, agent.Use) {
		names = append(names, agent.Use)
	}
	notes := agentNotes(a.config, all)
	custom := func(v []formValue) bool { return v[2].value == customAgent }
	var adding bool
	var newAgent, command string
	return stepFormSpec{
		title: "AI agent",
		fields: []stepField{
			{label: "ask", cell: textCellOf(agent.Ask), template: true, check: needed("a question"), note: "what the agent is asked; $answer is its answer after"},
			{label: "answers", cell: listCellOf(agent.Choices, -1, false, "answer"), note: "the answers it may give; none takes any"},
			{label: "agent", cell: optionCellOf(agent.Use, append(names, customAgent)), describe: func(name string) string { return orDefault(notes[name], "the agent that answers") }},
			{label: "timeout", cell: textCellOf(s.Timeout), check: checkDuration, note: "as 30s or 5m; empty waits as long as it takes"},
			{label: "name", cell: textCellOf(""), when: custom, check: customAgentProblem, note: "your agent's name, as mymodel; the config keeps it in [agents]"},
			{label: "command", cell: textCellOf(""), when: custom, check: checkAgentCommand, note: "the command it runs; {prompt} is the question, or it goes as its input"},
		},
		build: func(v []formValue) (flow.Step, error) {
			agent.Ask, agent.Choices, agent.Use = v[0].value, v[1].items, v[2].value
			adding, newAgent, command = agent.Use == customAgent, v[4].value, v[5].value
			if adding {
				agent.Use = newAgent
			}
			s.Agent, s.Timeout = &agent, v[3].value
			return s, nil
		},
		finish: func(step flow.Step, save func(flow.Step) error) error {
			if adding {
				if err := a.saveAgentCommand(newAgent, command, ""); err != nil {
					return err
				}
			}
			return save(step)
		},
	}
}

func checkAgentCommand(command string) string {
	if _, _, err := flow.AgentArgv(command, "question"); err != nil {
		return err.Error()
	}
	return ""
}

func escapedCellOf(text string) gridCell {
	cell := textCellOf(text)
	cell.escapes = true
	return cell
}

func autoCell(cell gridCell, name string) gridCell {
	cell.autoFor = name
	return cell
}
