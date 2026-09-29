package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/script"
)

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func libraryTools(d Deps) []Tool {
	return []Tool{
		{
			Name: "list_scripts", Title: "List scripts", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Lists the scripts: id, description, program, parameters (secret ones without values), outputs, and whether each is a draft or broken.",
			Run:         func(json.RawMessage) (any, error) { return d.listScripts(), nil },
		},
		{
			Name: "list_flows", Title: "List flows", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Lists the flows: name, description, params, triggers and whether they are on, draft, problems, how many scope warnings, and the last run.",
			Run:         func(json.RawMessage) (any, error) { return d.listFlows(), nil },
		},
		{
			Name: "get_item", Title: "Read a script or flow", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Reads a flow (its YAML, problems and scope warnings) or a script (its YAML and notes.md). Give flow or script.",
			Params: []Param{
				{Name: "flow", Type: "string", Description: "The flow's name."},
				{Name: "script", Type: "string", Description: "The script's id: name, or folder/name."},
			},
			Run: d.getItem,
		},
		{
			Name: "check", Title: "Check scripts and flows", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Like noct check: every script and flow that cannot be used, and every template that reads a step that may not be there.",
			Run:         func(json.RawMessage) (any, error) { return d.check(), nil },
		},
		{
			Name: "flow_scope", Title: "Variables a step can read", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Lists what templates can read at a step of a flow ({{.param}}, {{.steps.<id>.<field>}}), how sure each is to be set there (set, this_pass, maybe) and its value in the last run. Without a step, the end of the flow.",
			Params: []Param{
				{Name: "flow", Type: "string", Description: "The flow's name.", Required: true},
				{Name: "step", Type: "string", Description: "A step path like 2 or 0/steps/1 (list_flows and get_run show paths); the variables before it runs."},
			},
			Run: d.flowScope,
		},
		{
			Name: "draft_flow", Title: "Write a flow as a draft", Policy: config.PolicyAllow,
			Description: "Creates or replaces a flow from its YAML, marked as a draft: it runs, by hand or by trigger, only after the user reviews and promotes it in noct. Replacing a flow that was not a draft keeps a copy as <name>.yaml.bak. Answers the problems and scope warnings found.",
			Params: []Param{
				{Name: "name", Type: "string", Description: "The flow's name: letters, digits, . - _", Required: true},
				{Name: "yaml", Type: "string", Description: "The whole flow file: description, params, triggers, steps.", Required: true},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "name", "yaml") },
			Run:     d.draftFlow,
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
			Run:     d.draftScript,
		},
		{
			Name: "set_triggers", Title: "Turn a flow's triggers on or off", Destructive: true, Policy: config.PolicyAsk,
			Description: "Turns a flow's triggers (cron, watch, after) on or off on this machine. On, the running noct starts the flow by itself.",
			Params: []Param{
				{Name: "flow", Type: "string", Description: "The flow's name.", Required: true},
				{Name: "enabled", Type: "boolean", Description: "true turns them on, false off.", Required: true},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "flow", "enabled") },
			Run:     d.setTriggers,
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
		info := scriptInfo{ID: s.ID(), Description: s.Description, Program: s.Program, Outputs: s.Outputs, Draft: s.Draft}
		if s.Problem != nil {
			info.Problem = s.Problem.Error()
		}
		for _, p := range s.Params {
			param := paramInfo{Name: p.Name, Prompt: p.Prompt, Choices: p.Choices, Secret: p.Secret, Remember: p.Remember, FromEnv: strings.TrimPrefix(p.From, "env:")}
			if !p.Secret {
				param.Default = p.Default
			}
			info.Params = append(info.Params, param)
		}
		scripts = append(scripts, info)
	}
	return map[string]any{"scripts": scripts}
}

func (d Deps) flows() []flow.Flow {
	flows, _ := flow.LoadAll(flow.Dir(), d.Library().Find) // unreadable flows are listed as none
	return flows
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
	return map[string]any{"flows": flows}
}

func (d Deps) getItem(args json.RawMessage) (any, error) {
	var ask struct{ Flow, Script string }
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	switch {
	case ask.Flow != "":
		f, ok := flow.Find(d.flows(), ask.Flow)
		if !ok {
			return nil, fmt.Errorf("there is no flow %s; list_flows tells them", ask.Flow)
		}
		content, err := os.ReadFile(f.File)
		if err != nil {
			return nil, err
		}
		return map[string]any{"flow": f.Name, "file": f.File, "yaml": string(content), "draft": f.Draft, "problem": problemText(f.Problem), "warnings": warningTexts(f, d.Library().Find)}, nil
	case ask.Script != "":
		s, ok := d.Library().Find(ask.Script)
		if !ok {
			return nil, fmt.Errorf("there is no script %s; list_scripts tells them", ask.Script)
		}
		content, err := os.ReadFile(s.File())
		if err != nil {
			return nil, err
		}
		return map[string]any{"script": s.ID(), "file": s.File(), "yaml": string(content), "notes": s.Notes(), "draft": s.Draft, "problem": problemText(s.Problem)}, nil
	}
	return nil, errors.New("give flow or script")
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
	for _, f := range d.flows() {
		if f.Problem != nil {
			problems = append(problems, f.Problem.Error())
			continue
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
	var ask struct{ Flow, Step string }
	if err := decode(args, &ask); err != nil {
		return nil, err
	}
	f, ok := flow.Find(d.flows(), ask.Flow)
	if !ok {
		return nil, fmt.Errorf("there is no flow %s", ask.Flow)
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
	if !validName.MatchString(ask.Name) {
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
	if err := os.WriteFile(backup, content, 0o644); err != nil {
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
	if len(parts) > 2 || slices.ContainsFunc(parts, func(p string) bool { return !validName.MatchString(p) }) {
		return nil, fmt.Errorf("script name %q: name or folder/name, each letters, digits, . - _", ask.Name)
	}
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(ask.YAML), &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("the YAML does not read as a script: %v", err)
	}
	fields["draft"] = true
	content, err := yaml.Marshal(fields)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(append([]string{d.ScriptsDir()}, parts...)...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	file := filepath.Join(dir, script.FileName)
	backup, err := keepBackup(file, func([]byte) bool { return true })
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(file, content, 0o644); err != nil {
		return nil, err
	}
	if ask.Notes != "" {
		if err := os.WriteFile(filepath.Join(dir, script.NotesName), []byte(ask.Notes), 0o644); err != nil {
			return nil, err
		}
	}
	_, problem := script.Load(dir)
	answer := map[string]any{"script": ask.Name, "file": file, "draft": true, "problem": problemText(problem),
		"next": "ask the user to review the draft in noct's scripts panel (prefix+s) and promote it (P) or run noct promote " + ask.Name}
	if backup != "" {
		answer["backup"] = backup
	}
	return answer, nil
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
