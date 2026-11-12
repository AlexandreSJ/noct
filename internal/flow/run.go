package flow

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"maps"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/script"
)

// maxCaptured bounds how much of a step's output is kept for captures and templates.
const maxCaptured = 1 << 20

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
	// Progress hears of each step as it starts, by its path; nil when nobody follows the run.
	Progress func(path string)
	// Function finds a function a call step names.
	Function func(name string) (Flow, bool)
	// Agents are the config's commands of agents by name, over AgentPresets; DefaultAgent is the
	// one a step without use asks, or "" for AutoAgent.
	Agents       map[string]string
	DefaultAgent string
}

// running is the state of one run: the values templates read and the results so far.
type running struct {
	*Runner
	// vars are the flow's variables: its parameters, and what set and prompt steps gave.
	vars map[string]string
	// steps holds, by step id, output, exit, ok and the captures, for the templates.
	steps  map[string]map[string]string
	result Result
	// blocks are, for each if and repeat running, the variables made inside it, which go when it
	// ends.
	blocks []map[string]bool
	// depth is how many functions call each other down to the step running.
	depth int
	// deadline is when the flow's timeout, as written in timeout, ends the run; zero without one.
	deadline time.Time
	timeout  string
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

func (r *Runner) getenv(name string) string {
	if r.Getenv == nil {
		return ""
	}
	return r.Getenv(name)
}

// start begins a run of the flow with the parameters given over its defaults.
func (r *Runner) start(f Flow, params map[string]string) *running {
	x := &running{
		Runner: r,
		vars:   f.Values(r.getenv),
		steps:  map[string]map[string]string{},
		result: Result{Flow: f.Name, Start: time.Now(), Steps: map[string]StepResult{}, OK: true},
	}
	maps.Copy(x.vars, params)
	x.startDeadline(f)
	return x
}

// runSteps runs a list of steps, reporting false when the flow must stop.
func (x *running) runSteps(steps []Step, parent, branch string) bool {
	for i, s := range steps {
		path := Path(parent, branch, i)
		if x.runTimedOut() {
			return x.stopTimedOut(path)
		}
		if !x.runStep(s, path) {
			return false
		}
	}
	return true
}

func (x *running) runStep(s Step, path string) bool {
	if x.Progress != nil {
		x.Progress(path)
	}
	switch s.Kind() {
	case ScriptStep, CommandStep:
		return x.runCommand(s, path)
	case CallStep:
		return x.runCall(s, path)
	case AgentStep:
		return x.runAgent(s, path)
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
		return x.runWait(s, path)
	}
	return x.stop(path, exactlyOne, 1)
}

func (x *running) runWait(s Step, path string) bool {
	d, _ := time.ParseDuration(s.Wait) // checked by Validate
	x.header("⏱", "wait "+s.Wait, "")
	x.sleep(d)
	if x.runTimedOut() {
		return x.stopTimedOut(path)
	}
	x.result.Steps[path] = StepResult{Status: StatusOK, Duration: d}
	return true
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
	x.openBlock()
	defer x.closeBlock(s.Export)
	return x.runSteps(steps, path, branch)
}

// runRepeat runs the steps pass after pass. Each pass starts without the results of the steps
// inside from the pass before, so a step that did not run this time reads as empty.
func (x *running) runRepeat(s Step, path string) bool {
	r := s.Repeat
	passes := r.passes()
	every, _ := time.ParseDuration(r.Every) // checked by Validate; "" is no wait
	start := time.Now()
	x.openBlock()
	defer x.closeBlock(s.Export)
	for pass := 1; pass <= passes; pass++ {
		if pass > 1 && every > 0 {
			x.sleep(every)
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

func (r *Repeat) passes() int {
	if r.Times == 0 {
		return r.Max
	}
	return r.Times
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
	deadline := x.stepDeadline(s)
	output, exit := x.launch(s, command, dir, deadline)
	// A step that ended well right at its deadline finished on its own; only a failure past it
	// was the stop.
	timedOut := exit != 0 && passed(deadline)
	if timedOut {
		exit = timeoutExit
	}
	result := StepResult{Status: StatusOK, Exit: exit, Duration: time.Since(start), Captures: capture(x.capturePatterns(s), output), Output: truncate(output, x.KeepOutput)}
	ok := !timedOut && succeeded(s.Expect, exit, output)
	x.keepStepValues(s.ID, result, output, ok)
	if ok {
		x.result.Steps[path] = result
		return true
	}
	result.Status, result.Message = StatusFailed, failure(s.Expect, exit)
	if timedOut {
		result.Message = "timed out after " + s.Timeout
	}
	x.result.Steps[path] = result
	if x.runTimedOut() {
		return x.stopTimedOut(path)
	}
	return x.afterFailure(s, path, result.Message, exit)
}

// launch runs a step's command line in a pane of its own when it asks for one, else here, and
// stops it at the deadline.
func (x *running) launch(s Step, command, dir string, deadline time.Time) (string, int) {
	if s.Pane != "" {
		return x.runInPane(s, command, dir, deadline)
	}
	return x.execute(command, dir, deadline)
}

// keepStepValues makes a step's output, exit, ok and captures readable by the templates after
// it.
// keepStepValues keeps what a script or run step gave: as {{.steps.<id>.<name>}}, and as the
// variables of the same names, {{.output}}, {{.exit}}, {{.ok}} and its captures, which the next
// such step gives again.
func (x *running) keepStepValues(id string, result StepResult, output string, ok bool) {
	values := stepValues(output, result.Exit, ok, result.Captures)
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if isVariable(name) {
			x.assign(name, values[name])
		}
	}
	if id != "" {
		x.steps[id] = values
	}
}

// assign gives a variable its value: one that exists keeps its place, wherever it was made, and a
// new one belongs to the innermost if or repeat running, which takes it away when it ends.
func (x *running) assign(name, value string) {
	if _, exists := x.vars[name]; !exists && len(x.blocks) > 0 {
		x.blocks[len(x.blocks)-1][name] = true
	}
	x.vars[name] = value
}

func (x *running) openBlock() {
	x.blocks = append(x.blocks, map[string]bool{})
}

// closeBlock takes away the variables the block made, keeping the ones it exports under their
// names outside.
func (x *running) closeBlock(export map[string]string) {
	made := x.blocks[len(x.blocks)-1]
	x.blocks = x.blocks[:len(x.blocks)-1]
	kept := map[string]string{}
	for inner, outer := range export {
		if value, ok := x.vars[inner]; ok {
			kept[cmp.Or(outer, inner)] = value
		}
	}
	for name := range made {
		delete(x.vars, name)
	}
	for _, name := range slices.Sorted(maps.Keys(kept)) {
		x.assign(name, kept[name])
	}
}

// stepValues are what templates read from a script or run step: {{.steps.<id>.<name>}}.
func stepValues(output string, exit int, ok bool, captures map[string]string) map[string]string {
	values := map[string]string{"output": output, "exit": strconv.Itoa(exit), "ok": strconv.FormatBool(ok)}
	maps.Copy(values, captures)
	return values
}

// afterFailure does what the failed step's on_fail says: go on, run it again or stop the flow.
func (x *running) afterFailure(s Step, path, reason string, exit int) bool {
	switch x.onFail(s, reason) {
	case OnFailContinue:
		fmt.Fprintf(x.Out, "\x1b[2m✗ %s: %s; going on\x1b[0m\n", s.Label(), reason)
		return true
	case onFailRetry:
		return x.runStep(s, path)
	}
	return x.stop(path, reason, max(exit, 1))
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
// keeping it, without colors, for captures and templates. At the deadline it is stopped.
func (x *running) execute(command, dir string, deadline time.Time) (string, int) {
	cmd, cancel := x.commandUntil(deadline, command)
	defer cancel()
	cmd.Dir, cmd.Stdin = dir, x.Stdin
	var kept limitedBuffer
	cmd.Stdout = io.MultiWriter(x.Out, &kept)
	cmd.Stderr = cmd.Stdout
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err != nil && passed(deadline):
		return kept.text(), timeoutExit // stopped at the deadline; how it ended says nothing more
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
	script.WalkFields(t.Root, func(ident []string) {
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
