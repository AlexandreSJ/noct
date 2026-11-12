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
	"strconv"
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
	// Returns are what a function gives back to the step that calls it, as templates read at its
	// end; only functions have them.
	Returns map[string]string `yaml:"returns,omitempty"`
	// Function marks steps kept in the functions folder, which flows call.
	Function bool `yaml:"-"`
	// functions finds the functions the call steps name.
	functions func(name string) (Flow, bool)
}

// Step does one thing: runs a script or a command line, branches, repeats steps, waits, asks the
// user, or sets or clears variables.
type Step struct {
	// ID names the step for the templates of the steps after it: {{.steps.<id>.output}}.
	ID     string `yaml:"id,omitempty"`
	Script string `yaml:"script,omitempty"`
	// Call runs a function by its name, with the params With gives.
	Call string `yaml:"call,omitempty"`
	// Agent asks an AI agent and keeps its answer.
	Agent *Agent            `yaml:"agent,omitempty"`
	With  map[string]string `yaml:"with,omitempty"`
	Run   string            `yaml:"run,omitempty"`
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
	// Retry runs a script, run or agent step that failed again, before its on_fail.
	Retry *Retry `yaml:"retry,omitempty"`

	// Switch is a template; the first of Cases whose is holds its value runs, or Else with none.
	Switch string `yaml:"switch,omitempty"`
	Cases  []Case `yaml:"cases,omitempty"`

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

	// Export keeps variables an if, a switch or a repeat made, which go when it ends, for the steps after it:
	// each inner name to the name it has outside, or "" to keep its name.
	Export map[string]string `yaml:"export,omitempty"`
}

// Expect is what makes a script or command step a success; without it, exit code 0.
type Expect struct {
	Exit          *int   `yaml:"exit,omitempty"`
	OutputMatches string `yaml:"output_matches,omitempty"`
}

// Repeat runs its steps Times times, or until Until is true, at most Max times, waiting Every
// between passes. With Each, it runs them once for each line of Each's text, the line in the
// variable As (item without it), stopping early when Until is true or after Max passes.
type Repeat struct {
	Times int    `yaml:"times,omitempty"`
	Each  string `yaml:"each,omitempty"`
	As    string `yaml:"as,omitempty"`
	Until string `yaml:"until,omitempty"`
	Max   int    `yaml:"max,omitempty"`
	Every string `yaml:"every,omitempty"`
}

// DefaultEachVar is the variable a repeat each gives each line in, without as.
const DefaultEachVar = "item"

// EachVar is the variable each line of a repeat each is in.
func (r *Repeat) EachVar() string {
	if r.As == "" {
		return DefaultEachVar
	}
	return r.As
}

// Retry runs a failed step again, at most Max times, waiting Every before each try. In YAML it is
// also a number alone: retry: 3.
type Retry struct {
	Max   int    `yaml:"max"`
	Every string `yaml:"every,omitempty"`
}

func (r *Retry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		return node.Decode(&r.Max)
	}
	type plain Retry
	return node.Decode((*plain)(r))
}

func (r Retry) MarshalYAML() (any, error) {
	if r.Every == "" {
		return r.Max, nil
	}
	type plain Retry
	return plain(r), nil
}

// Case is one way of a switch: its steps run when the switch's value is one of Is.
type Case struct {
	Is    Values `yaml:"is"`
	Steps []Step `yaml:"steps,omitempty"`
}

// Values are one value or more; in YAML a single one is written alone: is: bug.
type Values []string

func (v *Values) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*v = Values{node.Value}
		return nil
	}
	var list []string
	if err := node.Decode(&list); err != nil {
		return err
	}
	*v = list
	return nil
}

func (v Values) MarshalYAML() (any, error) {
	if len(v) == 1 {
		return v[0], nil
	}
	return []string(v), nil
}

// CaseBranch names the list of steps of a switch's case at index, in a path: "2/case0/1" is the
// second step of the first case of the third step.
func CaseBranch(index int) string {
	return "case" + strconv.Itoa(index)
}

// caseIndex is the index of a case in its branch's name, or false for another branch.
func caseIndex(branch string) (int, bool) {
	digits, ok := strings.CutPrefix(branch, "case")
	if !ok {
		return 0, false
	}
	index, err := strconv.Atoi(digits)
	return index, err == nil && index >= 0
}

// Branch is a list of steps inside a step, by the name its paths use: an if's then and else, a
// switch's cases and else, a repeat's steps.
type Branch struct {
	Name  string
	Steps []Step
}

// Branches are the lists of steps inside the step, in the order they are drawn and moved through.
func (s Step) Branches() []Branch {
	switch s.Kind() {
	case IfStep:
		return []Branch{{"then", s.Then}, {"else", s.Else}}
	case SwitchStep:
		var branches []Branch
		for i, c := range s.Cases {
			branches = append(branches, Branch{CaseBranch(i), c.Steps})
		}
		return append(branches, Branch{"else", s.Else})
	case RepeatStep:
		return []Branch{{"steps", s.Steps}}
	}
	return nil
}

// branch is the list of steps named branch inside the step, or nil when it has none of that name.
func (s *Step) branch(name string) *[]Step {
	switch name {
	case "then":
		return &s.Then
	case "else":
		return &s.Else
	case "steps":
		return &s.Steps
	}
	if i, ok := caseIndex(name); ok && i < len(s.Cases) {
		return &s.Cases[i].Steps
	}
	return nil
}

// IsBlock tells whether the step holds steps of its own: an if, a switch or a repeat, which
// takes away the variables made inside it when it ends, unless it exports them.
func (s Step) IsBlock() bool {
	kind := s.Kind()
	return kind == IfStep || kind == SwitchStep || kind == RepeatStep
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
	CallStep    Kind = "call"
	AgentStep   Kind = "agent"
	CommandStep Kind = "run"
	IfStep      Kind = "if"
	SwitchStep  Kind = "switch"
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
	if s.Call != "" {
		kinds = append(kinds, CallStep)
	}
	if s.Agent != nil {
		kinds = append(kinds, AgentStep)
	}
	if s.Run != "" {
		kinds = append(kinds, CommandStep)
	}
	if s.If != "" {
		kinds = append(kinds, IfStep)
	}
	if s.Switch != "" {
		kinds = append(kinds, SwitchStep)
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
	case s.Call != "":
		return s.Call
	case s.Agent != nil:
		return "agent"
	case s.Run != "":
		return "run"
	case s.If != "":
		return "if"
	case s.Switch != "":
		return "switch"
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

const exactlyOne = "a step sets exactly one of script, call, agent, run, if, repeat, wait, prompt, set and unset"

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
