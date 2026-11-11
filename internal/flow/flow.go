// Package flow chains scripts into flows: steps with conditions, repetitions, waits and captures
// of their output, kept as YAML (see docs/fluxos.md).
package flow

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"go.yaml.in/yaml/v3"

	"github.com/AlexandreSJ/noct/internal/atomicfile"
	"github.com/AlexandreSJ/noct/internal/naming"
	"github.com/AlexandreSJ/noct/internal/script"
)

// Ext is the extension of a flow's file; the name before it is the flow's name.
const Ext = ".yaml"

// TrackedEnv is set for a flow run from noct's workspace, which keeps the run's record itself.
const TrackedEnv = "NOCT_FLOW_TRACKED"

// ResultEnv names a file where a flow run also keeps its result: the one beside the run's log,
// so the calendar can show how that run went.
const ResultEnv = "NOCT_FLOW_RESULT"

// Flow is a sequence of steps, run top to bottom.
type Flow struct {
	Name string `yaml:"-"`
	File string `yaml:"-"`
	// Problem is why the flow's file cannot be used, for a flow listed anyway.
	Problem     error  `yaml:"-"`
	Description string `yaml:"description,omitempty"`
	// Draft marks a flow an AI wrote: it does not run, by hand or by trigger, until the user
	// reviews and promotes it.
	Draft bool `yaml:"draft,omitempty"`
	// Params are values the steps' templates read as {{.name}}, with their defaults.
	Params map[string]Param `yaml:"params,omitempty"`
	// Triggers start the flow on their own, once enabled on this machine.
	Triggers []Trigger `yaml:"triggers,omitempty"`
	// Timeout bounds the whole run, as a duration like 1h30m; empty is no limit.
	Timeout string `yaml:"timeout,omitempty"`
	Steps   []Step `yaml:"steps"`
}

// Step does one thing: runs a script or a command line, branches, repeats steps, waits, asks the
// user, or sets or clears variables.
type Step struct {
	// ID names the step for the templates of the steps after it: {{.steps.<id>.output}}.
	ID     string            `yaml:"id,omitempty"`
	Script string            `yaml:"script,omitempty"`
	With   map[string]string `yaml:"with,omitempty"`
	Run    string            `yaml:"run,omitempty"`
	// Capture extracts values from the output: name to a regex whose first group, or whole
	// match, becomes {{.steps.<id>.<name>}}.
	Capture map[string]string `yaml:"capture,omitempty"`
	Expect  *Expect           `yaml:"expect,omitempty"`
	// OnFail is what a failure does: OnFailStop (the default), OnFailContinue or OnFailAsk.
	OnFail string `yaml:"on_fail,omitempty"`
	// Pane runs the step in a pane of its own (PaneNew); Background does not wait for it.
	Pane       string `yaml:"pane,omitempty"`
	Background bool   `yaml:"background,omitempty"`
	// Timeout bounds a script or run step, as a duration like 10m; past it the step is stopped
	// and fails. Empty is no limit.
	Timeout string `yaml:"timeout,omitempty"`

	If   string `yaml:"if,omitempty"`
	Then []Step `yaml:"then,omitempty"`
	Else []Step `yaml:"else,omitempty"`

	Repeat *Repeat `yaml:"repeat,omitempty"`
	// Steps are what a repeat runs on each pass.
	Steps []Step `yaml:"steps,omitempty"`

	Wait string `yaml:"wait,omitempty"`

	// Prompt asks whoever watches the run, and keeps the answer in the variable Var; an empty
	// answer keeps Default. Type number asks again until the answer is a number.
	Prompt  string `yaml:"prompt,omitempty"`
	Var     string `yaml:"var,omitempty"`
	Default string `yaml:"default,omitempty"`
	Type    string `yaml:"type,omitempty"`

	// Set gives variables, new or not, the values of its templates: {{.name}} after it.
	Set map[string]string `yaml:"set,omitempty"`
	// Unset clears variables (name) and steps' results (steps.<id>).
	Unset []string `yaml:"unset,omitempty"`

	// Export keeps variables an if or repeat made, which go when it ends, for the steps after it:
	// each inner name to the name it has outside, or "" to keep its name.
	Export map[string]string `yaml:"export,omitempty"`
}

// Expect is what makes a script or command step a success; without it, exit code 0.
type Expect struct {
	Exit          *int   `yaml:"exit,omitempty"`
	OutputMatches string `yaml:"output_matches,omitempty"`
}

// Repeat runs its steps Times times, or until Until is true, at most Max times, waiting Every
// between passes.
type Repeat struct {
	Times int    `yaml:"times,omitempty"`
	Until string `yaml:"until,omitempty"`
	Max   int    `yaml:"max,omitempty"`
	Every string `yaml:"every,omitempty"`
}

// What a failing step does to the flow.
const (
	OnFailStop     = "stop"
	OnFailContinue = "continue"
	// OnFailAsk asks whoever watches the run whether to retry the step, go on or stop.
	OnFailAsk = "ask"
	// onFailRetry is the answer that runs the step again.
	onFailRetry = "retry"
)

// Kind is what a step does.
type Kind string

const (
	ScriptStep  Kind = "script"
	CommandStep Kind = "run"
	IfStep      Kind = "if"
	RepeatStep  Kind = "repeat"
	WaitStep    Kind = "wait"
	PromptStep  Kind = "prompt"
	SetStep     Kind = "set"
	UnsetStep   Kind = "unset"
)

// Kind is what the step does, or "" when it sets none or more than one kind.
func (s Step) Kind() Kind {
	var kinds []Kind
	if s.Script != "" {
		kinds = append(kinds, ScriptStep)
	}
	if s.Run != "" {
		kinds = append(kinds, CommandStep)
	}
	if s.If != "" {
		kinds = append(kinds, IfStep)
	}
	if s.Repeat != nil {
		kinds = append(kinds, RepeatStep)
	}
	if s.Wait != "" {
		kinds = append(kinds, WaitStep)
	}
	if s.Prompt != "" {
		kinds = append(kinds, PromptStep)
	}
	if len(s.Set) > 0 {
		kinds = append(kinds, SetStep)
	}
	if len(s.Unset) > 0 {
		kinds = append(kinds, UnsetStep)
	}
	if len(kinds) != 1 {
		return ""
	}
	return kinds[0]
}

// Label is how the step is named on screen: its id, or what it runs.
func (s Step) Label() string {
	switch {
	case s.ID != "":
		return s.ID
	case s.Script != "":
		return s.Script
	case s.Run != "":
		return "run"
	case s.If != "":
		return "if"
	case s.Repeat != nil:
		return "repeat"
	case s.Wait != "":
		return "wait"
	}
	return string(s.Kind())
}

// Dir is where the user's flows are kept, beside the standard scripts folder.
func Dir() string {
	return filepath.Join(filepath.Dir(script.PersonalDir()), "flows")
}

func Load(file string) (Flow, error) {
	f := Flow{Name: strings.TrimSuffix(filepath.Base(file), Ext), File: file}
	content, err := os.ReadFile(file)
	if err != nil {
		return f, fmt.Errorf("reading flow %s: %w", f.Name, err)
	}
	if err := yaml.Unmarshal(content, &f); err != nil {
		return f, fmt.Errorf("flow %s: %w", f.Name, err)
	}
	return f, nil
}

// LoadAll reads every flow in dir, by name. A flow that cannot be read or is not valid comes with
// its Problem; lookup finds the scripts the steps name.
func LoadAll(dir string, lookup func(id string) (script.Script, bool)) ([]Flow, error) {
	files, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the flows: %w", err)
	}
	var flows []Flow
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != Ext {
			continue
		}
		f, err := Load(filepath.Join(dir, file.Name()))
		if err == nil {
			if err = f.Validate(lookup); err != nil {
				err = fmt.Errorf("flow %s: %w", f.Name, err)
			}
		}
		f.Problem = err
		flows = append(flows, f)
	}
	markAfterLoops(flows)
	return flows, nil
}

func Find(flows []Flow, name string) (Flow, bool) {
	i := slices.IndexFunc(flows, func(f Flow) bool { return f.Name == name })
	if i < 0 {
		return Flow{}, false
	}
	return flows[i], true
}

const exactlyOne = "a step sets exactly one of script, run, if, repeat, wait, prompt, set and unset"

// Create writes a new, empty flow called name in dir.
func Create(dir, name string) (Flow, error) {
	if !naming.IsValid(name) {
		return Flow{}, fmt.Errorf("flow name %q: use letters, digits, . - _", name)
	}
	f := Flow{Name: name, File: filepath.Join(dir, name+Ext)}
	if _, err := os.Stat(f.File); err == nil {
		return Flow{}, fmt.Errorf("flow %s already exists", name)
	}
	return f, f.Save()
}

// Save writes the flow to its file, atomically.
func (f Flow) Save() error {
	content, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("saving flow %s: %w", f.Name, err)
	}
	if err := os.MkdirAll(filepath.Dir(f.File), 0o755); err != nil {
		return fmt.Errorf("saving flow %s: %w", f.Name, err)
	}
	if err := atomicfile.Write(f.File, content, 0o644); err != nil {
		return fmt.Errorf("saving flow %s: %w", f.Name, err)
	}
	return nil
}

// DraftError tells why a draft flow does not run, and how to let it.
func (f Flow) DraftError() error {
	if !f.Draft {
		return nil
	}
	return fmt.Errorf("flow %s is a draft: review it in the inspect (prefix+i) and promote it (P), or run noct promote %s", f.Name, f.Name)
}

func (f Flow) Delete() error {
	if err := os.Remove(f.File); err != nil {
		return fmt.Errorf("deleting flow %s: %w", f.Name, err)
	}
	return nil
}

func parseTemplate(text string) (*template.Template, error) {
	return template.New("").Option("missingkey=zero").Funcs(templateFuncs).Parse(text)
}

// templateFuncs are the functions templates can call besides Go's own.
var templateFuncs = template.FuncMap{
	// match tells whether text matches a regex: {{if match .steps.mail.output "sent"}}.
	"match": func(text, pattern string) (bool, error) {
		return regexp.MatchString(pattern, text)
	},
	"quote": script.Quote,
}
