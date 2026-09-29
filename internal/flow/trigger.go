package flow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/AlexandreSJ/noct/internal/logstore"
)

// Trigger starts a flow on its own: on a schedule, when files change, or after another flow
// ends. Manual only says the flow is also run by hand. A flow's triggers do nothing until the
// user enables them on this machine (see Enabled), so a flow received through git never starts
// running by itself.
type Trigger struct {
	Manual bool   `yaml:"-"`
	Cron   string `yaml:"cron,omitempty"`
	Watch  *Watch `yaml:"watch,omitempty"`
	After  *After `yaml:"after,omitempty"`
}

// Watch starts the flow when files matching Paths change, once they settle for Debounce. Paths
// are globs, ** for any folders, relative to where noct was opened.
type Watch struct {
	Paths    []string `yaml:"paths"`
	Debounce string   `yaml:"debounce,omitempty"`
}

// After starts the flow when another flow ends: with Status ok, failed, or "" for either.
type After struct {
	Flow   string `yaml:"flow"`
	Status string `yaml:"status,omitempty"`
}

// Statuses an after trigger can wait for.
const (
	AfterOK     = "ok"
	AfterFailed = "failed"
)

// DefaultDebounce is how long watched files must stay unchanged before the flow starts.
const DefaultDebounce = time.Second

// manualTrigger is how a manual trigger is written: a word instead of a setting.
const manualTrigger = "manual"

// trigger has the same fields as Trigger without its YAML methods, to decode them.
type trigger Trigger

func (t *Trigger) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		if node.Value != manualTrigger {
			return fmt.Errorf("line %d: trigger %q is not manual, cron, watch or after", node.Line, node.Value)
		}
		*t = Trigger{Manual: true}
		return nil
	}
	return node.Decode((*trigger)(t))
}

func (t Trigger) MarshalYAML() (any, error) {
	if t.Manual {
		return manualTrigger, nil
	}
	return trigger(t), nil
}

// Label says in a few words what starts the flow.
func (t Trigger) Label() string {
	switch {
	case t.Cron != "":
		return "cron " + t.Cron
	case t.Watch != nil:
		return "watch " + strings.Join(t.Watch.Paths, " ")
	case t.After != nil && t.After.Status != "":
		return fmt.Sprintf("after %s (%s)", t.After.Flow, t.After.Status)
	case t.After != nil:
		return "after " + t.After.Flow
	}
	return manualTrigger
}

// TriggerLabels say what starts the flow by itself, leaving out manual.
func (f Flow) TriggerLabels() []string {
	var labels []string
	for _, t := range f.Triggers {
		if !t.Manual {
			labels = append(labels, t.Label())
		}
	}
	return labels
}

// DebounceTime is how long a watch waits for its files to settle.
func (w Watch) DebounceTime() time.Duration {
	d, err := time.ParseDuration(w.Debounce)
	if err != nil || d <= 0 {
		return DefaultDebounce
	}
	return d
}

func (t Trigger) validate(flowName string) error {
	kinds := 0
	for _, set := range []bool{t.Manual, t.Cron != "", t.Watch != nil, t.After != nil} {
		if set {
			kinds++
		}
	}
	if kinds != 1 {
		return errors.New("a trigger is exactly one of manual, cron, watch and after")
	}
	switch {
	case t.Cron != "":
		_, err := ParseCron(t.Cron)
		return err
	case t.Watch != nil:
		return t.Watch.validate()
	case t.After != nil:
		return t.After.validate(flowName)
	}
	return nil
}

func (w Watch) validate() error {
	if len(w.Paths) == 0 {
		return errors.New("watch needs paths")
	}
	for _, p := range w.Paths {
		if _, err := path.Match(strings.ReplaceAll(p, "**", "*"), ""); err != nil {
			return fmt.Errorf("watch path %q: %w", p, err)
		}
	}
	if w.Debounce != "" {
		if d, err := time.ParseDuration(w.Debounce); err != nil || d <= 0 {
			return fmt.Errorf("watch debounce %q is not a duration like 2s", w.Debounce)
		}
	}
	return nil
}

func (a After) validate(flowName string) error {
	switch {
	case a.Flow == "":
		return errors.New("after needs the flow it waits for")
	case a.Flow == flowName:
		return errors.New("a flow cannot start after itself")
	case a.Status != "" && a.Status != AfterOK && a.Status != AfterFailed:
		return fmt.Errorf("after status is %s or %s, or left out for either", AfterOK, AfterFailed)
	}
	return nil
}

// Waits tells whether an after trigger starts its flow when a run of the other flow ends so.
func (a After) Waits(finished string, ok bool) bool {
	if a.Flow != finished {
		return false
	}
	return a.Status == "" || (a.Status == AfterOK) == ok
}

// MatchPath tells whether a slash-separated path matches a glob in which ** stands for any
// number of folders.
func MatchPath(pattern, name string) bool {
	return matchParts(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchParts(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			for skip := 0; skip <= len(name); skip++ {
				if matchParts(pattern[1:], name[skip:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, _ := path.Match(pattern[0], name[0]); !ok {
			return false
		}
		pattern, name = pattern[1:], name[1:]
	}
	return len(name) == 0
}

// markAfterLoops gives a problem to each flow in a loop of after triggers, which would start
// each other forever.
func markAfterLoops(flows []Flow) {
	starts := map[string][]string{} // a flow's name to the flows its end starts
	for _, f := range flows {
		for _, t := range f.Triggers {
			if t.After != nil {
				starts[t.After.Flow] = append(starts[t.After.Flow], f.Name)
			}
		}
	}
	for i, f := range flows {
		if loop := afterLoop(starts, f.Name, []string{f.Name}); loop != nil && flows[i].Problem == nil {
			flows[i].Problem = fmt.Errorf("flow %s: after triggers make a loop: %s", f.Name, strings.Join(loop, " → "))
		}
	}
}

// afterLoop follows the flows started from the last one in the way, and returns the way back to
// the first, if there is one.
func afterLoop(starts map[string][]string, first string, way []string) []string {
	for _, next := range starts[way[len(way)-1]] {
		if next == first {
			return append(way, first)
		}
		if slices.Contains(way, next) {
			continue
		}
		if loop := afterLoop(starts, first, append(slices.Clone(way), next)); loop != nil {
			return loop
		}
	}
	return nil
}

// TriggersFile keeps which flows have their triggers enabled on this machine, apart from the
// flows' files.
func TriggersFile() string {
	return filepath.Join(logstore.Dir(), "triggers.json")
}

type enabledFlows struct {
	Enabled []string `json:"enabled"`
}

// Enabled are the flows whose triggers run; a missing or unreadable file is none.
func Enabled(path string) map[string]bool {
	content, err := os.ReadFile(path)
	var stored enabledFlows
	if err != nil || json.Unmarshal(content, &stored) != nil {
		return map[string]bool{}
	}
	on := map[string]bool{}
	for _, name := range stored.Enabled {
		on[name] = true
	}
	return on
}

// SetEnabled turns a flow's triggers on or off.
func SetEnabled(path, name string, on bool) error {
	enabled := Enabled(path)
	if on {
		enabled[name] = true
	} else {
		delete(enabled, name)
	}
	var stored enabledFlows
	for name := range enabled {
		stored.Enabled = append(stored.Enabled, name)
	}
	slices.Sort(stored.Enabled)
	content, err := json.Marshal(stored)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o700)
	}
	if err == nil {
		err = os.WriteFile(path+".tmp", content, 0o600)
	}
	if err == nil {
		err = os.Rename(path+".tmp", path)
	}
	if err != nil {
		return fmt.Errorf("saving the enabled triggers: %w", err)
	}
	return nil
}
