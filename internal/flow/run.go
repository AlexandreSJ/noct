package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

// maxCaptured bounds how much of a step's output is kept for captures and templates.
const maxCaptured = 1 << 20

// Status is how a step went in a run.
type Status string

const (
	StatusOK     Status = "ok"
	StatusFailed Status = "failed"
)

// StepResult is how a step went, the last time the run passed through it.
type StepResult struct {
	Status   Status            `json:"status"`
	Exit     int               `json:"exit,omitempty"`
	Duration time.Duration     `json:"duration"`
	Captures map[string]string `json:"captures,omitempty"`
	// Output is the step's output as templates read it, cut to the runner's KeepOutput.
	Output string `json:"output,omitempty"`
	// Tried marks a step run alone, as a test, after the flow's last run.
	Tried bool `json:"tried,omitempty"`
	// Message says why the step failed, or what an if or repeat decided.
	Message string `json:"message,omitempty"`
}

// Result is how a run went, each step by its Path.
type Result struct {
	Flow  string                `json:"flow"`
	Start time.Time             `json:"start"`
	End   time.Time             `json:"end"`
	OK    bool                  `json:"ok"`
	Exit  int                   `json:"exit"`
	Steps map[string]StepResult `json:"steps"`
}

// Path names a step by where it is: "2" is the third step of the flow, "2/then/0" the first
// step of its then, "0/steps/1" the second step repeated by the first.
func Path(parent, branch string, index int) string {
	if parent == "" {
		return strconv.Itoa(index)
	}
	return parent + "/" + branch + "/" + strconv.Itoa(index)
}

// Runner runs flows in the terminal it writes to: each step's output goes to Out as it comes.
type Runner struct {
	Lookup func(id string) (script.Script, bool)
	// Shell runs command lines with -c.
	Shell string
	// Dir is where steps run, unless their script sets its own workdir.
	Dir    string
	Getenv func(string) string
	Stdin  io.Reader
	Out    io.Writer
	Sleep  func(time.Duration)
	// KeepOutput is how many bytes of each step's output the result keeps; 0 keeps all of it.
	KeepOutput int
	// Ask puts a question to whoever watches the run and returns the answer, for on_fail: ask;
	// nil when nobody can answer, and then a failure stops the flow.
	Ask func(question string) string
	// OpenPane asks noct's workspace for a pane that runs a step; nil outside noct.
	OpenPane func(PaneRequest) (int, error)
	// Attention asks noct's workspace to show the run, before a prompt waits for the user; nil
	// outside noct.
	Attention func()
}

// running is the state of one run: the values templates read and the results so far.
type running struct {
	*Runner
	// vars are the flow's variables: its parameters, and what set and prompt steps gave.
	vars map[string]string
	// steps holds, by step id, output, exit, ok and the captures, for the templates.
	steps  map[string]map[string]string
	result Result
}

// Run runs the flow with the parameters given over its defaults. Result.Exit is the exit code of
// the step that stopped it, or 1 for a stop that was not a program's.
func (r *Runner) Run(f Flow, params map[string]string) Result {
	x := r.start(f, params)
	x.runSteps(f.Steps, "", "")
	x.result.End = time.Now()
	if x.result.OK {
		fmt.Fprintf(r.Out, "\n\x1b[1m✓ flow %s done\x1b[0m in %s\n", f.Name, x.result.End.Sub(x.result.Start).Round(time.Millisecond))
	}
	return x.result
}

// start begins a run of the flow with the parameters given over its defaults.
func (r *Runner) start(f Flow, params map[string]string) *running {
	x := &running{
		Runner: r,
		vars:   f.Defaults(),
		steps:  map[string]map[string]string{},
		result: Result{Flow: f.Name, Start: time.Now(), Steps: map[string]StepResult{}, OK: true},
	}
	maps.Copy(x.vars, params)
	return x
}

// runSteps runs a list of steps, reporting false when the flow must stop.
func (x *running) runSteps(steps []Step, parent, branch string) bool {
	for i, s := range steps {
		if !x.runStep(s, Path(parent, branch, i)) {
			return false
		}
	}
	return true
}

func (x *running) runStep(s Step, path string) bool {
	switch s.Kind() {
	case ScriptStep, CommandStep:
		return x.runCommand(s, path)
	case IfStep:
		return x.runIf(s, path)
	case RepeatStep:
		return x.runRepeat(s, path)
	case PromptStep:
		return x.runPrompt(s, path)
	case SetStep:
		return x.runSet(s, path)
	case UnsetStep:
		x.runUnset(s, path)
		return true
	case WaitStep:
		d, _ := time.ParseDuration(s.Wait) // checked by Validate
		x.header("⏱", "wait "+s.Wait, "")
		x.Sleep(d)
		x.result.Steps[path] = StepResult{Status: StatusOK, Duration: d}
		return true
	}
	return x.stop(path, exactlyOne, 1)
}

func (x *running) runIf(s Step, path string) bool {
	yes, err := x.condition(s.If)
	if err != nil {
		return x.stop(path, "if: "+err.Error(), 1)
	}
	branch, steps, answer := "then", s.Then, "yes"
	if !yes {
		branch, steps, answer = "else", s.Else, "no"
	}
	x.header("◇", "if "+s.If, "→ "+answer)
	x.result.Steps[path] = StepResult{Status: StatusOK, Message: answer}
	return x.runSteps(steps, path, branch)
}

// runRepeat runs the steps pass after pass. Each pass starts without the results of the steps
// inside from the pass before, so a step that did not run this time reads as empty.
func (x *running) runRepeat(s Step, path string) bool {
	r := s.Repeat
	passes := r.Times
	if passes == 0 {
		passes = r.Max
	}
	every, _ := time.ParseDuration(r.Every) // checked by Validate; "" is no wait
	start := time.Now()
	for pass := 1; pass <= passes; pass++ {
		if pass > 1 && every > 0 {
			x.Sleep(every)
		}
		x.forget(s.Steps)
		x.header("↻", fmt.Sprintf("%s pass %d of %d", s.Label(), pass, passes), "")
		if !x.runSteps(s.Steps, path, "steps") {
			return false
		}
		if r.Until == "" {
			continue
		}
		done, err := x.condition(r.Until)
		if err != nil {
			return x.stop(path, "until: "+err.Error(), 1)
		}
		if done {
			x.result.Steps[path] = StepResult{Status: StatusOK, Duration: time.Since(start), Message: fmt.Sprintf("%d passes", pass)}
			return true
		}
	}
	if r.Until != "" && r.Times == 0 {
		return x.stop(path, fmt.Sprintf("until was still false after %d passes (max)", passes), 1)
	}
	x.result.Steps[path] = StepResult{Status: StatusOK, Duration: time.Since(start), Message: fmt.Sprintf("%d passes", passes)}
	return true
}

func (x *running) forget(steps []Step) {
	for _, s := range steps {
		delete(x.steps, s.ID)
		for _, inner := range [][]Step{s.Then, s.Else, s.Steps} {
			x.forget(inner)
		}
	}
}

func (x *running) runCommand(s Step, path string) bool {
	command, dir, err := x.commandLine(s)
	if err != nil {
		return x.stop(path, err.Error(), 1)
	}
	detail := s.Script
	if s.Script == "" {
		detail = "$ " + strings.Join(strings.Fields(command), " ")
	}
	x.header("▶", s.Label(), detail)
	start := time.Now()
	output, exit := "", 0
	if s.Pane != "" {
		output, exit = x.runInPane(s, command, dir)
	} else {
		output, exit = x.execute(command, dir)
	}
	result := StepResult{Status: StatusOK, Exit: exit, Duration: time.Since(start), Captures: capture(x.capturePatterns(s), output), Output: truncate(output, x.KeepOutput)}
	ok := succeeded(s.Expect, exit, output)
	if s.ID != "" {
		values := map[string]string{"output": output, "exit": strconv.Itoa(exit), "ok": strconv.FormatBool(ok)}
		for name, value := range result.Captures {
			values[name] = value
		}
		x.steps[s.ID] = values
	}
	if ok {
		x.result.Steps[path] = result
		return true
	}
	result.Status, result.Message = StatusFailed, failure(s.Expect, exit)
	x.result.Steps[path] = result
	switch x.onFail(s, result.Message) {
	case OnFailContinue:
		fmt.Fprintf(x.Out, "\x1b[2m✗ %s: %s; going on\x1b[0m\n", s.Label(), result.Message)
		return true
	case onFailRetry:
		return x.runCommand(s, path)
	}
	return x.stop(path, result.Message, max(exit, 1))
}

// onFail is what a failed step does: what its on_fail says or, for ask, what the answer says.
func (x *running) onFail(s Step, reason string) string {
	if s.OnFail != OnFailAsk {
		return s.OnFail
	}
	if x.Ask == nil {
		return OnFailStop
	}
	answer := x.Ask(fmt.Sprintf("✗ %s: %s. Retry, continue or stop? [r/c/s] ", s.Label(), reason))
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "r", "retry":
		return onFailRetry
	case "c", "continue":
		return OnFailContinue
	}
	return OnFailStop
}

// capturePatterns are the step's captures, and a name=value line for each output its script
// declares that the step does not capture itself.
func (x *running) capturePatterns(s Step) map[string]string {
	sc, ok := x.Lookup(s.Script)
	if s.Script == "" || !ok || len(sc.Outputs) == 0 {
		return s.Capture
	}
	patterns := map[string]string{}
	for _, name := range sc.Outputs {
		patterns[name] = "^" + regexp.QuoteMeta(name) + "=(.*)$"
	}
	maps.Copy(patterns, s.Capture)
	return patterns
}

// commandLine is the shell line a script or run step runs, and where.
func (x *running) commandLine(s Step) (string, string, error) {
	if s.Script == "" {
		command, err := x.render(s.Run)
		return command, x.Dir, err
	}
	sc, ok := x.Lookup(s.Script)
	if !ok {
		return "", "", fmt.Errorf("script %s not found", s.Script)
	}
	if sc.Draft {
		return "", "", fmt.Errorf("script %s is a draft: promote it before it runs", s.Script)
	}
	values := sc.StartingValues(nil, x.Getenv)
	for name, text := range s.With {
		value, err := x.render(text)
		if err != nil {
			return "", "", fmt.Errorf("with %s: %w", name, err)
		}
		values[name] = value
	}
	if err := sc.CheckValues(values); err != nil {
		return "", "", err
	}
	command, err := sc.Command(values)
	return command, sc.WorkingDir(x.Dir), err
}

// execute runs a command line with the terminal's input, showing its output as it comes and
// keeping it, without colors, for captures and templates.
func (x *running) execute(command, dir string) (string, int) {
	cmd := exec.Command(x.Shell, "-c", command)
	cmd.Dir, cmd.Stdin = dir, x.Stdin
	var kept limitedBuffer
	cmd.Stdout = io.MultiWriter(x.Out, &kept)
	cmd.Stderr = cmd.Stdout
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		return kept.text(), exit.ExitCode()
	case err != nil:
		fmt.Fprintln(x.Out, err)
		return kept.text(), 127
	}
	return kept.text(), 0
}

// header announces a step in the output: a symbol, the step in bold, a detail dimmed.
func (x *running) header(symbol, title, detail string) {
	fmt.Fprintf(x.Out, "\n\x1b[1m%s %s\x1b[0m", symbol, title)
	if detail != "" {
		fmt.Fprintf(x.Out, " \x1b[2m%s\x1b[0m", detail)
	}
	fmt.Fprintln(x.Out)
}

// stop ends the run at the step at path, which failed for reason.
func (x *running) stop(path, reason string, exit int) bool {
	step := x.result.Steps[path]
	step.Status, step.Message = StatusFailed, reason
	x.result.Steps[path] = step
	x.result.OK, x.result.Exit = false, exit
	fmt.Fprintf(x.Out, "\n\x1b[1m✗ flow %s stopped: %s\x1b[0m\n", x.result.Flow, reason)
	return false
}

// condition renders a template that must give true or false.
func (x *running) condition(text string) (bool, error) {
	value, err := x.render(text)
	if err != nil {
		return false, err
	}
	switch strings.TrimSpace(value) {
	case "true":
		return true, nil
	case "false", "":
		return false, nil
	}
	return false, fmt.Errorf("gave %q, not true or false", value)
}

func (x *running) render(text string) (string, error) {
	t, err := parseTemplate(text)
	if err != nil {
		return "", err
	}
	data := map[string]any{"steps": x.steps}
	for name, value := range x.vars {
		data[name] = value
	}
	// A variable not set, or unset, reads as empty rather than as <no value>.
	walkFields(t.Root, func(ident []string) {
		if _, set := data[ident[0]]; !set {
			data[ident[0]] = ""
		}
	})
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

// capture applies each regex to the output line by line (^ and $ match at each line); the first
// match wins, and its first group, when it has one, is the value.
func capture(patterns map[string]string, output string) map[string]string {
	if len(patterns) == 0 {
		return nil
	}
	values := map[string]string{}
	for name, pattern := range patterns {
		re, err := regexp.Compile("(?m)" + pattern)
		if err != nil {
			continue // checked by Validate
		}
		if m := re.FindStringSubmatch(output); m != nil {
			values[name] = m[min(1, len(m)-1)]
		}
	}
	return values
}

func succeeded(expect *Expect, exit int, output string) bool {
	if expect == nil {
		return exit == 0
	}
	if expect.Exit != nil && exit != *expect.Exit {
		return false
	}
	if expect.Exit == nil && exit != 0 {
		return false
	}
	if expect.OutputMatches != "" {
		matched, _ := regexp.MatchString("(?m)"+expect.OutputMatches, output)
		return matched
	}
	return true
}

func failure(expect *Expect, exit int) string {
	wanted := 0
	if expect != nil && expect.Exit != nil {
		wanted = *expect.Exit
	}
	if exit != wanted {
		return fmt.Sprintf("exit %d", exit)
	}
	return "output did not match " + strconv.Quote(expect.OutputMatches)
}

// truncate keeps the first limit bytes of text, without cutting a character; 0 keeps it all.
func truncate(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return text[:limit]
}

// limitedBuffer keeps the first maxCaptured bytes written to it and drops the rest.
type limitedBuffer struct {
	bytes.Buffer
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := maxCaptured - b.Len(); room > 0 {
		b.Buffer.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

// text is the output as templates see it: without colors or carriage returns, and without the
// line break at its end.
func (b *limitedBuffer) text() string {
	text := strings.ReplaceAll(ansi.Strip(b.String()), "\r", "")
	return strings.TrimRight(text, "\n")
}

// ResultsDir keeps the last result of each flow.
func ResultsDir() string {
	return filepath.Join(logstore.Dir(), "flows")
}

// SaveResult keeps a run's result as its flow's last.
func SaveResult(dir string, r Result) error {
	return SaveResultTo(filepath.Join(dir, r.Flow+".json"), r)
}

// SaveResultTo writes a run's result to a file of its own.
func SaveResultTo(path string, r Result) error {
	content, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("saving the result: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("saving the result: %w", err)
	}
	if err := os.WriteFile(path+".tmp", content, 0o600); err != nil {
		return fmt.Errorf("saving the result: %w", err)
	}
	return os.Rename(path+".tmp", path)
}

// ReadResult reads a result kept in a file.
func ReadResult(path string) (Result, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Result{}, false
	}
	var r Result
	if json.Unmarshal(content, &r) != nil {
		return Result{}, false
	}
	return r, true
}

// LastResult is the result of the flow's last run; ok is false when it never ran.
func LastResult(dir, name string) (Result, bool) {
	return ReadResult(filepath.Join(dir, name+".json"))
}
