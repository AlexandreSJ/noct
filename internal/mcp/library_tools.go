package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/AlexandreSJ/noct/internal/atomicfile"
	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/script"
)

func libraryTools(d Deps) []Tool {
	return []Tool{
		{
			Name: "list_scripts", Title: "List scripts", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Lists the scripts: id, description, program, parameters (secret ones without values), outputs, and whether each is a draft or broken.",
			Run:         func(context.Context, json.RawMessage) (any, error) { return d.listScripts(), nil },
		},
		{
			Name: "list_flows", Title: "List flows", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Lists the flows (name, description, params with their types, triggers and whether they are on, draft, problems, how many scope warnings, the last run), the functions flows call (params, returns, which flows call each), and the AI agents an agent step can ask.",
			Run:         func(context.Context, json.RawMessage) (any, error) { return d.listFlows(), nil },
		},
		{
			Name: "get_item", Title: "Read a script, flow or function", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Reads a flow or a function (its YAML, problems, scope warnings and, for a function, the flows that call it) or a script (its YAML and notes.md). Give flow, function or script.",
			Params: []Param{
				{Name: "flow", Type: "string", Description: "The flow's name."},
				{Name: "function", Type: "string", Description: "The function's name."},
				{Name: "script", Type: "string", Description: "The script's id: name, or folder/name."},
			},
			Run: noWait(d.getItem),
		},
		{
			Name: "check", Title: "Check scripts and flows", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Like noct check: every script and flow that cannot be used, and every template that reads a step that may not be there.",
			Run:         func(context.Context, json.RawMessage) (any, error) { return d.check(), nil },
		},
		{
			Name: "flow_scope", Title: "Variables a step can read", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Lists what templates can read at a step of a flow or function ({{.param}}, short names like {{.output}} and captures, {{.steps.<id>.<field>}}), how sure each is to be set there (set, this_pass, maybe) and its value in the last run. Variables an if or repeat makes go when it ends unless it exports them. Without a step, the end.",
			Params: []Param{
				{Name: "flow", Type: "string", Description: "The flow's name, or the function's with function."},
				{Name: "function", Type: "string", Description: "A function's name, in place of flow."},
				{Name: "step", Type: "string", Description: "A step path like 2 or 0/steps/1 (list_flows and get_run show paths); the variables before it runs."},
			},
			Run: noWait(d.flowScope),
		},
		{
			Name: "draft_flow", Title: "Write a flow as a draft", Policy: config.PolicyAllow,
			Description: "Creates or replaces a flow from its YAML, marked as a draft: it runs, by hand or by trigger, only after the user reviews and promotes it in noct. Replacing a flow that was not a draft keeps a copy as <name>.yaml.bak. Answers the problems and scope warnings found.",
			Params: []Param{
				{Name: "name", Type: "string", Description: "The flow's name: letters, digits, . - _", Required: true},
				{Name: "yaml", Type: "string", Description: "The whole flow file: description, params, triggers, steps.", Required: true},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "name", "yaml") },
			Run:     noWait(d.draftFlow),
		},
		{
			Name: "draft_function", Title: "Write a function as a draft", Policy: config.PolicyAllow,
			Description: "Creates or replaces a function (params, steps, returns) from its YAML, marked as a draft: no flow runs it, and flows that call it have a problem, until the user reviews and promotes it in noct. Replacing a function that was not a draft keeps a copy as <name>.yaml.bak; the answer names the flows that call it.",
			Params: []Param{
				{Name: "name", Type: "string", Description: "The function's name: letters, digits, . - _", Required: true},
				{Name: "yaml", Type: "string", Description: "The whole function file: description, params, steps, returns.", Required: true},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "name", "yaml") },
			Run:     noWait(d.draftFunction),
		},
		{
			Name: "draft_script", Title: "Write a script as a draft", Policy: config.PolicyAllow,
			Description: "Creates or replaces one of the user's scripts from its script.yaml (and notes.md), marked as a draft: it runs only after the user promotes it. Replacing a script keeps a copy of its script.yaml as script.yaml.bak.",
			Params: []Param{
				{Name: "name", Type: "string", Description: "The script's name, or folder/name for a folder of scripts.", Required: true},
				{Name: "yaml", Type: "string", Description: "The script.yaml: description, run, params, outputs.", Required: true},
				{Name: "notes", Type: "string", Description: "notes.md: how and why to use it; left as it is when omitted."},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "name", "yaml") },
			Run:     noWait(d.draftScript),
		},
		{
			Name: "set_triggers", Title: "Turn a flow's triggers on or off", Destructive: true, Policy: config.PolicyAsk,
			Description: "Turns a flow's triggers (cron, watch, after) on or off on this machine. On, the running noct starts the flow by itself.",
			Params: []Param{
				{Name: "flow", Type: "string", Description: "The flow's name.", Required: true},
				{Name: "enabled", Type: "boolean", Description: "true turns them on, false off.", Required: true},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "flow", "enabled") },
			Run:     noWait(d.setTriggers),
		},
	}
}

type paramInfo struct {
	Name     string   `json:"name"`
	Prompt   string   `json:"prompt,omitempty"`
	Default  string   `json:"default,omitempty"`
	Choices  []string `json:"choices,omitempty"`
	Secret   bool     `json:"secret,omitempty"`
	FromEnv  string   `json:"from_env,omitempty"`
	Remember bool     `json:"remember,omitempty"`
}

type scriptInfo struct {
	ID          string      `json:"id"`
	Description string      `json:"description,omitempty"`
	Program     string      `json:"program,omitempty"`
	Params      []paramInfo `json:"params,omitempty"`
	Outputs     []string    `json:"outputs,omitempty"`
	Draft       bool        `json:"draft,omitempty"`
	Problem     string      `json:"problem,omitempty"`
}

func (d Deps) listScripts() map[string]any {
	scripts := []scriptInfo{}
	for _, s := range d.Library().All() {
		scripts = append(scripts, describeScript(s))
	}
	return map[string]any{"scripts": scripts}
}

func describeScript(s script.Script) scriptInfo {
	info := scriptInfo{ID: s.ID(), Description: s.Description, Program: s.Program, Outputs: s.Outputs, Draft: s.Draft, Problem: problemText(s.Problem)}
	for _, p := range s.Params {
		info.Params = append(info.Params, describeParam(p))
	}
	return info
}

// describeParam tells a parameter, leaving out a secret one's default.
func describeParam(p script.Param) paramInfo {
	param := paramInfo{Name: p.Name, Prompt: p.Prompt, Choices: p.Choices, Secret: p.Secret, Remember: p.Remember, FromEnv: strings.TrimPrefix(p.From, "env:")}
	if !p.Secret {
		param.Default = p.Default
	}
	return param
}

func (d Deps) flows() []flow.Flow {
	flows, _ := d.flowsAndFunctions()
	return flows
}

// flowsAndFunctions are the flows and the functions they call; unreadable ones are left out.
func (d Deps) flowsAndFunctions() ([]flow.Flow, []flow.Flow) {
	flows, functions, _ := flow.LoadWithFunctions(flow.Dir(), flow.FunctionsDir(), d.Library().Find)
	return flows, functions
}

type functionInfo struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Params      map[string]flow.Param `json:"params,omitempty"`
	Returns     map[string]string     `json:"returns,omitempty"`
	CalledBy    []string              `json:"called_by"`
	Draft       bool                  `json:"draft,omitempty"`
	Problem     string                `json:"problem,omitempty"`
}

// agentNames are the agents an agent step can ask, and the default one.
func (d Deps) agentNames() map[string]any {
	names := slices.Sorted(maps.Keys(flow.AgentPresets))
	cfg := d.Config()
	for name := range cfg.Agents {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return map[string]any{"names": names, "default": cfg.Agent}
}

type flowInfo struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Params      map[string]flow.Param `json:"params,omitempty"`
	Triggers    []string              `json:"triggers,omitempty"`
	TriggersOn  bool                  `json:"triggers_on,omitempty"`
	Draft       bool                  `json:"draft,omitempty"`
	Problem     string                `json:"problem,omitempty"`
	Warnings    int                   `json:"warnings,omitempty"`
	LastRun     map[string]any        `json:"last_run,omitempty"`
}

func (d Deps) listFlows() map[string]any {
	enabled := flow.Enabled(flow.TriggersFile())
	flows := []flowInfo{}
	for _, f := range d.flows() {
		info := flowInfo{Name: f.Name, Description: f.Description, Params: f.Params, Triggers: f.TriggerLabels(), TriggersOn: enabled[f.Name], Draft: f.Draft}
		if f.Problem != nil {
			info.Problem = f.Problem.Error()
		} else {
			info.Warnings = len(f.Warnings(d.Library().Find))
		}
		if r, ran := flow.LastResult(flow.ResultsDir(), f.Name); ran {
			info.LastRun = map[string]any{"ok": r.OK, "exit": r.Exit, "started_at": r.Start, "ended_at": r.End}
		}
		flows = append(flows, info)
	}
	all, functions := d.flowsAndFunctions()
	described := []functionInfo{}
	for _, fn := range functions {
		described = append(described, functionInfo{
			Name: fn.Name, Description: fn.Description, Params: fn.Params, Returns: fn.Returns, Draft: fn.Draft,
			CalledBy: callers(append(all, functions...), fn.Name), Problem: problemText(fn.Problem),
		})
	}
	return map[string]any{"flows": flows, "functions": described, "agents": d.agentNames()}
}

func callers(flows []flow.Flow, name string) []string {
	if names := flow.Callers(flows, name); names != nil {
		return names
	}
	return []string{}
}

func (d Deps) getItem(args json.RawMessage) (any, error) {
	var ask struct{ Flow, Function, Script string }
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	switch {
	case ask.Function != "":
		return d.getFunction(ask.Function)
	case ask.Flow != "":
		return d.getFlow(ask.Flow)
	case ask.Script != "":
		return d.getScript(ask.Script)
	}
	return nil, errors.New("give flow, function or script")
}

func (d Deps) getFunction(name string) (any, error) {
	flows, functions := d.flowsAndFunctions()
	fn, ok := flow.Find(functions, name)
	if !ok {
		return nil, fmt.Errorf("there is no function %s; list_flows tells them", name)
	}
	content, err := os.ReadFile(fn.File)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"function": fn.Name, "file": fn.File, "yaml": string(content), "draft": fn.Draft, "problem": problemText(fn.Problem),
		"warnings": warningTexts(fn, d.Library().Find), "called_by": callers(append(flows, functions...), fn.Name),
	}, nil
}

func (d Deps) getFlow(name string) (any, error) {
	f, ok := flow.Find(d.flows(), name)
	if !ok {
		return nil, fmt.Errorf("there is no flow %s; list_flows tells them", name)
	}
	content, err := os.ReadFile(f.File)
	if err != nil {
		return nil, err
	}
	return map[string]any{"flow": f.Name, "file": f.File, "yaml": string(content), "draft": f.Draft, "problem": problemText(f.Problem), "warnings": warningTexts(f, d.Library().Find)}, nil
}

func (d Deps) getScript(id string) (any, error) {
	s, ok := d.Library().Find(id)
	if !ok {
		return nil, fmt.Errorf("there is no script %s; list_scripts tells them", id)
	}
	content, err := os.ReadFile(s.File())
	if err != nil {
		return nil, err
	}
	return map[string]any{"script": s.ID(), "file": s.File(), "yaml": string(content), "notes": s.Notes(), "draft": s.Draft, "problem": problemText(s.Problem)}, nil
}

func problemText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func warningTexts(f flow.Flow, lookup func(string) (script.Script, bool)) []string {
	texts := []string{}
	if f.Problem != nil {
		return texts
	}
	for _, w := range f.Warnings(lookup) {
		step, _ := f.Step(w.Path)
		texts = append(texts, fmt.Sprintf("step %s (%s): %s", step.Label(), w.Path, w.Text))
	}
	return texts
}

func (d Deps) check() map[string]any {
	lib := d.Library()
	problems := []string{}
	for _, err := range append(lib.Broken(), lib.Problems...) {
		problems = append(problems, err.Error())
	}
	warnings := []string{}
	flows, functions := d.flowsAndFunctions()
	for _, f := range append(flows, functions...) {
		if f.Problem != nil {
			problems = append(problems, f.Problem.Error())
			continue
		}
		if warning := f.TimeoutWarning(); warning != "" {
			warnings = append(warnings, "flow "+f.Name+": "+warning)
		}
		for _, w := range warningTexts(f, lib.Find) {
			warnings = append(warnings, "flow "+f.Name+", "+w)
		}
	}
	return map[string]any{"problems": problems, "warnings": warnings, "scripts": len(lib.All())}
}

type variableInfo struct {
	Template  string `json:"template"`
	Step      string `json:"step,omitempty"`
	Path      string `json:"path,omitempty"`
	Certainty string `json:"certainty"`
	LastValue string `json:"last_value,omitempty"`
}

var certaintyNames = map[flow.Certainty]string{flow.Set: "set", flow.ThisPass: "this_pass", flow.Maybe: "maybe"}

func (d Deps) flowScope(args json.RawMessage) (any, error) {
	var ask struct{ Flow, Function, Step string }
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	flows, functions := d.flowsAndFunctions()
	f, ok := flow.Find(flows, ask.Flow)
	if ask.Function != "" {
		f, ok = flow.Find(functions, ask.Function)
	}
	if !ok {
		return nil, fmt.Errorf("there is no flow %s or function %s", ask.Flow, ask.Function)
	}
	parent, branch, index := "", "", len(f.Steps)
	if ask.Step != "" {
		if _, found := f.Step(ask.Step); !found {
			return nil, fmt.Errorf("flow %s has no step %s", f.Name, ask.Step)
		}
		parent, branch, index = flow.SplitPath(ask.Step)
	}
	last, ran := flow.LastResult(flow.ResultsDir(), f.Name)
	vars := []variableInfo{}
	for _, v := range f.Scope(parent, branch, index, d.Library().Find) {
		info := variableInfo{Template: v.Template(), Step: v.Step, Path: v.Path, Certainty: certaintyNames[v.Certainty]}
		if ran {
			info.LastValue = lastValue(f, v, last)
		}
		vars = append(vars, info)
	}
	return map[string]any{"flow": f.Name, "step": ask.Step, "variables": vars}, nil
}

// lastValue is a variable's value in a result, or its default for a parameter.
func lastValue(f flow.Flow, v flow.Variable, r flow.Result) string {
	if v.Step == "" && v.Path == "" {
		return f.Params[v.Field].Default
	}
	step, ran := r.Steps[v.Path]
	if !ran {
		return ""
	}
	switch v.Field {
	case "output":
		return tail(step.Output, 500)
	case "exit":
		return fmt.Sprint(step.Exit)
	case "ok":
		return fmt.Sprint(step.Status == flow.StatusOK)
	}
	return step.Captures[v.Field]
}

func (d Deps) draftFlow(args json.RawMessage) (any, error) {
	var ask struct{ Name, YAML string }
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	if !naming.IsValid(ask.Name) {
		return nil, fmt.Errorf("flow name %q: use letters, digits, . - _", ask.Name)
	}
	var f flow.Flow
	if err := yaml.Unmarshal([]byte(ask.YAML), &f); err != nil {
		return nil, fmt.Errorf("the YAML does not read as a flow: %w", err)
	}
	f.Name, f.File, f.Draft = ask.Name, filepath.Join(flow.Dir(), ask.Name+flow.Ext), true
	backup, err := keepBackup(f.File, func(content []byte) bool {
		var old flow.Flow
		return yaml.Unmarshal(content, &old) == nil && !old.Draft
	})
	if err != nil {
		return nil, err
	}
	if err := f.Save(); err != nil {
		return nil, err
	}
	problem := f.Validate(d.Library().Find)
	answer := map[string]any{"flow": f.Name, "file": f.File, "draft": true, "problem": problemText(problem), "warnings": []string{}}
	if problem == nil {
		answer["warnings"] = warningTexts(f, d.Library().Find)
	}
	if backup != "" {
		answer["backup"] = backup
	}
	answer["next"] = "ask the user to review the draft in noct (prefix+i) and promote it (P) or run noct promote " + f.Name
	return answer, nil
}

func (d Deps) draftFunction(args json.RawMessage) (any, error) {
	var ask struct{ Name, YAML string }
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	if !naming.IsValid(ask.Name) {
		return nil, fmt.Errorf("function name %q: use letters, digits, . - _", ask.Name)
	}
	var fn flow.Flow
	if err := yaml.Unmarshal([]byte(ask.YAML), &fn); err != nil {
		return nil, fmt.Errorf("the YAML does not read as a function: %w", err)
	}
	fn.Name, fn.File, fn.Draft, fn.Function = ask.Name, filepath.Join(flow.FunctionsDir(), ask.Name+flow.Ext), true, true
	backup, err := keepBackup(fn.File, func(content []byte) bool {
		var old flow.Flow
		return yaml.Unmarshal(content, &old) == nil && !old.Draft
	})
	if err != nil {
		return nil, err
	}
	if err := fn.Save(); err != nil {
		return nil, err
	}
	flows, functions := d.flowsAndFunctions()
	fn.WithFunctions(func(name string) (flow.Flow, bool) { return flow.Find(functions, name) })
	problem := errors.Join(fn.Validate(d.Library().Find), fn.CheckCalls(functions))
	answer := map[string]any{"function": fn.Name, "file": fn.File, "draft": true, "problem": problemText(problem), "warnings": []string{}, "called_by": callers(append(flows, functions...), fn.Name)}
	if problem == nil {
		answer["warnings"] = warningTexts(fn, d.Library().Find)
	}
	if backup != "" {
		answer["backup"] = backup
	}
	answer["next"] = "ask the user to review the draft in noct (prefix+i) and promote it (P) or run noct promote " + fn.Name + "; until then the flows that call it do not run"
	return answer, nil
}

// keepBackup copies a file before it is replaced, when it exists and wanted says so, and tells
// where the copy went.
func keepBackup(path string, wanted func(content []byte) bool) (string, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !wanted(content)) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	backup := path + ".bak"
	if err := atomicfile.Write(backup, content, 0o644); err != nil {
		return "", fmt.Errorf("keeping a copy of %s: %w", path, err)
	}
	return backup, nil
}

func (d Deps) draftScript(args json.RawMessage) (any, error) {
	var ask struct{ Name, YAML, Notes string }
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	parts := strings.Split(ask.Name, "/")
	if len(parts) > 2 || slices.ContainsFunc(parts, func(p string) bool { return !naming.IsValid(p) }) {
		return nil, fmt.Errorf("script name %q: name or folder/name, each letters, digits, . - _", ask.Name)
	}
	content, err := markedDraft(ask.YAML)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(append([]string{d.ScriptsDir()}, parts...)...)
	file := filepath.Join(dir, script.FileName)
	backup, err := keepBackup(file, func([]byte) bool { return true })
	if err != nil {
		return nil, err
	}
	if err := script.WriteFiles(dir, content, ask.Notes); err != nil {
		return nil, err
	}
	_, problem := script.Load(dir)
	answer := map[string]any{"script": ask.Name, "file": file, "draft": true, "problem": problemText(problem),
		"next": "ask the user to review the draft in noct's scripts panel (prefix+s) and promote it (P) or run noct promote " + ask.Name}
	if backup != "" {
		answer["backup"] = backup
	}
	return answer, nil
}

// markedDraft is a script.yaml with draft: true set in it.
func markedDraft(scriptYAML string) ([]byte, error) {
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(scriptYAML), &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("the YAML does not read as a script: %v", err)
	}
	fields["draft"] = true
	return yaml.Marshal(fields)
}

func (d Deps) setTriggers(args json.RawMessage) (any, error) {
	var ask struct {
		Flow    string
		Enabled bool
	}
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	f, ok := flow.Find(d.flows(), ask.Flow)
	switch {
	case !ok:
		return nil, fmt.Errorf("there is no flow %s", ask.Flow)
	case ask.Enabled && f.Draft:
		return nil, f.DraftError()
	case len(f.TriggerLabels()) == 0:
		return nil, fmt.Errorf("flow %s has no triggers; draft them in its YAML first", f.Name)
	}
	if err := flow.SetEnabled(flow.TriggersFile(), f.Name, ask.Enabled); err != nil {
		return nil, err
	}
	return map[string]any{"flow": f.Name, "triggers": f.TriggerLabels(), "enabled": ask.Enabled}, nil
}
