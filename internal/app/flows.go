package app

import (
	"fmt"
	"image"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"go.yaml.in/yaml/v3"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/script"
)

// stepHints heads the file a step is edited in.
const stepHints = `# One step: script (with, capture, expect, on_fail), run (capture, expect, on_fail),
# if (then, else), repeat (times or until with max, every; steps) or wait (5s, 1m).
# Templates read {{.steps.<id>.output}}, .exit, .ok and captures. See docs/fluxos.md.
`

// reloadFlows reads the flows again, checking their scripts against the library.
func (a *App) reloadFlows() {
	flows, err := flow.LoadAll(flow.Dir(), a.scripts.Find)
	if err != nil {
		a.statusMessage = err.Error()
	}
	a.flows = flows
}

// scriptDescription is what a script does, for the boxes of the steps that run it.
func (a *App) scriptDescription(id string) string {
	s, ok := a.scripts.Find(id)
	if !ok {
		return "✗ not found"
	}
	return s.Description
}

// newFlow asks for a name and creates an empty flow, telling created its name.
func (a *App) newFlow(created func(name string)) {
	a.openPrompt("new flow", "", "enter create  esc cancel  letters, digits, . - _", func(name string) {
		f, err := flow.Create(flow.Dir(), name)
		if err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.reloadFlows()
		created(f.Name)
	})
}

// runFlow runs the flow with noct flow run in a run view beside the focused pane, so it shows
// as it goes and stays in the runs and logs like a script's run.
func (a *App) runFlow(f flow.Flow) {
	if f.Problem != nil {
		a.statusMessage = f.Problem.Error()
		return
	}
	if err := f.DraftError(); err != nil {
		a.statusMessage = err.Error()
		return
	}
	values := a.flowValues(f)
	if f.CheckParams(values) != nil {
		a.openFlowForm(f, values)
		return
	}
	a.startFlow(f, values)
}

// flowValues are the flow's parameters as its last run from here gave them, or their defaults.
func (a *App) flowValues(f flow.Flow) map[string]string {
	values := f.Defaults()
	for name, value := range a.memory.FlowValues(f.Name) {
		if _, still := f.Params[name]; still {
			values[name] = value
		}
	}
	return values
}

// openFlowForm asks for the flow's parameters (p in the inspect); enter checks them, remembers
// them and runs the flow.
func (a *App) openFlowForm(f flow.Flow, values map[string]string) {
	if len(f.Params) == 0 {
		a.statusMessage = "flow " + f.Name + " has no parameters"
		return
	}
	var fields []script.Param
	for _, name := range slices.Sorted(maps.Keys(f.Params)) {
		p := f.Params[name]
		label := p.Label(name)
		if p.Type == flow.NumberParam {
			label += " (number)"
		}
		fields = append(fields, script.Param{Name: name, Prompt: label})
	}
	a.form = paramsForm{
		title: "run " + f.Name, fields: fields, values: maps.Clone(values),
		preview: func(values map[string]string) string {
			if err := f.CheckParams(values); err != nil {
				return "⚠ " + err.Error()
			}
			return "$ noct flow run " + f.Name + " " + strings.Join(paramArgs(values), " ")
		},
		submit: func(values map[string]string) error {
			if err := f.CheckParams(values); err != nil {
				return err
			}
			a.startFlow(f, values)
			return nil
		},
	}
	a.mode = formMode
}

// startFlow runs the flow with noct flow run and the values, which it remembers for the next run.
func (a *App) startFlow(f flow.Flow, values map[string]string) {
	if len(values) > 0 {
		if err := a.memory.RememberFlow(f.Name, values); err != nil {
			a.statusMessage = err.Error() // the run goes on; only the memory is lost
		}
	}
	argv := append([]string{orDefault(a.binary.path, "noct"), "flow", "run", f.Name}, paramArgs(values)...)
	a.openTrackedRun("flow "+f.Name, script.Run{Flow: f.Name}, program{argv: argv, dir: a.workDir})
}

// paramArgs are values as noct flow run takes them, name=value, by name.
func paramArgs(values map[string]string) []string {
	var args []string
	for _, name := range slices.Sorted(maps.Keys(values)) {
		args = append(args, name+"="+values[name])
	}
	return args
}

// testFlowSteps tests the steps first..last of the flow (first alone when last is "") with noct
// flow run --steps, in a run view like a whole run.
func (a *App) testFlowSteps(f flow.Flow, first, last string) {
	if f.Problem != nil {
		a.statusMessage = f.Problem.Error()
		return
	}
	if err := f.DraftError(); err != nil {
		a.statusMessage = err.Error()
		return
	}
	steps := first
	if last != "" && last != first {
		steps += ".." + last
	}
	binary := orDefault(a.binary.path, "noct")
	run := program{argv: []string{binary, "flow", "run", f.Name, "--steps", steps}, dir: a.workDir}
	a.openTrackedRun("test "+f.Name+" "+steps, script.Run{Flow: f.Name}, run)
}

// promoteFlow lets a draft flow run, after asking: the user has reviewed what an AI wrote.
func (a *App) promoteFlow(f flow.Flow) {
	if !f.Draft {
		a.notice = "flow " + f.Name + " is not a draft"
		return
	}
	a.askToConfirm("promote draft flow "+f.Name+"? it can run and trigger afterwards", func() {
		if err := a.changeFlow(f, func(f *flow.Flow) error { f.Draft = false; return nil }); err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.notice = "flow " + f.Name + " promoted"
	})
}

// promoteScript lets a draft script run, after asking.
func (a *App) promoteScript(s script.Script) {
	if !s.Draft {
		a.notice = "script " + s.ID() + " is not a draft"
		return
	}
	a.askToConfirm("promote draft script "+s.ID()+"? it can run afterwards", func() {
		if err := script.Promote(s); err != nil {
			a.statusMessage = err.Error()
			return
		}
		a.reloadScripts()
		a.notice = "script " + s.ID() + " promoted"
	})
}

// editFlowFile opens the flow's file in the editor; when the editor exits, the flows are read
// again and a problem the edit made is shown.
func (a *App) editFlowFile(f flow.Flow) {
	a.openEditor(f.File, "edit-flow-"+f.Name, func() {
		a.reloadFlows()
		if edited, ok := flow.Find(a.flows, f.Name); ok && edited.Problem != nil {
			a.statusMessage = edited.Problem.Error()
		}
	})
}

// openEditor edits a file in a pane beside the focused one and calls done when the editor exits.
func (a *App) openEditor(file, logName string, done func()) {
	argv, ok := commandArgv(a.config.Editor, file)
	if !ok {
		a.statusMessage = "editor " + a.config.Editor + " not found; choose another in the config"
		return
	}
	a.openPanel(func(id layout.PaneID, frame image.Rectangle) (view, error) {
		return a.launch(program{
			argv:    argv,
			dir:     filepath.Dir(file),
			logName: logName,
			onExit: func(int) {
				a.detachView(id)
				done()
			},
		}, contentSize(frame))
	})
}

// editFlowStep opens one step, with the steps inside it, alone in the editor as YAML. When the
// editor exits, the step takes what was saved, unless it makes the flow invalid.
func (a *App) editFlowStep(f flow.Flow, path string) {
	step, ok := f.Step(path)
	if !ok {
		return
	}
	content, err := yaml.Marshal(step)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	file := filepath.Join(os.TempDir(), fmt.Sprintf("noct-%s-step-%s.yaml", f.Name, strings.ReplaceAll(path, "/", "-")))
	if err := os.WriteFile(file, append([]byte(stepHints), content...), 0o600); err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.openEditor(file, "edit-step", func() {
		content, err := os.ReadFile(file)
		var edited flow.Step
		if err == nil {
			err = yaml.Unmarshal(content, &edited)
		}
		if err == nil {
			err = a.changeFlow(f, func(f *flow.Flow) error { return f.Replace(path, edited) })
		}
		if err != nil {
			a.statusMessage = "step not saved: " + err.Error() + "; the edit is in " + file
			return
		}
		_ = os.Remove(file) // saved; a leftover in the temp folder harms nothing
	})
}

// changeFlow changes the flow as it is on disk and saves it, unless the change brings a problem
// the flow did not have.
func (a *App) changeFlow(f flow.Flow, change func(*flow.Flow) error) error {
	fresh, err := flow.Load(f.File)
	if err != nil {
		return err
	}
	before := fresh.Validate(a.scripts.Find)
	if err := change(&fresh); err != nil {
		return err
	}
	if after := fresh.Validate(a.scripts.Find); after != nil && (before == nil || after.Error() != before.Error()) {
		return after
	}
	if err := fresh.Save(); err != nil {
		return err
	}
	a.reloadFlows()
	return nil
}

// addStep asks what kind of step to add, and its main setting, and puts it where the selected
// block is: at a gap or into an empty list, or right after a step. added gets the new step's path.
func (a *App) addStep(f flow.Flow, at chartBlock, added func(path string)) {
	parent, branch, index := at.slot.parent, at.slot.branch, at.index
	if !at.isInsertion() {
		parent, branch, index = flow.SplitPath(at.path)
		index++
	}
	insert := func(step flow.Step) {
		var path string
		err := a.changeFlow(f, func(f *flow.Flow) error {
			if step.ID == "" && (step.Script != "" || step.Run != "") {
				step.ID = f.UniqueID(orDefault(step.Script, "step"))
			}
			var err error
			path, err = f.Insert(parent, branch, index, step)
			return err
		})
		if err != nil {
			a.statusMessage = err.Error()
			return
		}
		added(path)
	}
	// Templates typed here read what runs before the new step; ctrl+v searches it.
	vars := f.Scope(parent, branch, index, a.scripts.Find)
	templatePrompt := func(title, initial, hints string, submit func(text string)) {
		a.openPrompt(title, initial, hints+"  ctrl+v variable", submit)
		a.prompt.pick = func() { a.pickVariable(f, vars) }
	}
	a.openChoice("add step",
		choiceOption{key: "s", label: "script", pick: func() { a.pickScript(insert) }},
		choiceOption{key: "c", label: "command line", pick: func() {
			templatePrompt("command line", "", "enter add  esc cancel", func(text string) {
				insert(flow.Step{Run: text})
			})
		}},
		choiceOption{key: "i", label: "if / else", pick: func() {
			templatePrompt("if", `{{eq .steps.<id>.<capture> "value"}}`, "enter add  esc cancel  true runs then", func(text string) {
				insert(flow.Step{If: text})
			})
		}},
		choiceOption{key: "l", label: "repeat", pick: func() {
			templatePrompt("repeat", "3", "enter add  esc cancel  times, or until", func(text string) {
				insert(repeatStep(text))
			})
		}},
		choiceOption{key: "w", label: "wait", pick: func() {
			a.openPrompt("wait", "5s", "enter add  esc cancel  like 5s, 1m30s", func(text string) {
				insert(flow.Step{Wait: text})
			})
		}},
		choiceOption{key: "p", label: "prompt the user", pick: func() {
			templatePrompt("prompt", "", "enter next  esc cancel  the question asked", func(question string) {
				a.openPrompt("into variable", "", "enter add  esc cancel  new, or one to change", func(name string) {
					insert(flow.Step{Prompt: question, Var: strings.TrimSpace(name)})
				})
			})
		}},
		choiceOption{key: "v", label: "set variable", pick: func() {
			templatePrompt("set", "name = ", "enter add  esc cancel  name = value", func(text string) {
				name, value, _ := strings.Cut(text, "=")
				insert(flow.Step{Set: map[string]string{strings.TrimSpace(name): strings.TrimSpace(value)}})
			})
		}},
		choiceOption{key: "u", label: "unset variables", pick: func() {
			a.openPrompt("unset", "", "enter add  esc cancel  names, steps.<id>", func(text string) {
				insert(flow.Step{Unset: strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' })})
			})
		}},
	)
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// repeatStep repeats a number of times, or until a condition, at most 10 times.
func repeatStep(text string) flow.Step {
	if times, err := strconv.Atoi(text); err == nil {
		return flow.Step{Repeat: &flow.Repeat{Times: times}}
	}
	return flow.Step{Repeat: &flow.Repeat{Until: text, Max: 10}}
}

// pickScript searches the scripts for the one a step runs.
func (a *App) pickScript(picked func(flow.Step)) {
	var items []finderItem
	for _, s := range a.scripts.All() {
		if s.Problem == nil {
			items = append(items, finderItem{label: s.ID(), kind: s.Description, open: func() { picked(flow.Step{Script: s.ID()}) }})
		}
	}
	if len(items) == 0 {
		a.statusMessage = "no scripts yet: create one in the scripts panel (prefix+s)"
		return
	}
	a.openFinderWith("script for the step", items)
}

func (a *App) removeStep(f flow.Flow, path string) {
	step, _ := f.Step(path)
	question := "remove step " + step.Label() + "?"
	if inner := countSteps(step.Then) + countSteps(step.Else) + countSteps(step.Steps); inner > 0 {
		question = fmt.Sprintf("remove step %s and the %d steps inside it?", step.Label(), inner)
	}
	a.askToConfirm(question, func() {
		if err := a.changeFlow(f, func(f *flow.Flow) error { return f.Remove(path) }); err != nil {
			a.statusMessage = err.Error()
		}
	})
}

func countSteps(steps []flow.Step) int {
	n := len(steps)
	for _, s := range steps {
		n += countSteps(s.Then) + countSteps(s.Else) + countSteps(s.Steps)
	}
	return n
}

func (a *App) moveStep(f flow.Flow, path string, step int, moved func(path string)) {
	var to string
	err := a.changeFlow(f, func(f *flow.Flow) error {
		to = f.Move(path, step)
		return nil
	})
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	moved(to)
}

// aboutStep tells what a step does: what it runs, with which values, what it captures, and how
// it went in the flow's last run.
func (a *App) aboutStep(f flow.Flow, path string, result *flow.Result) {
	step, ok := f.Step(path)
	if !ok {
		return
	}
	pal := &a.palette
	lines := a.stepLines(step)
	if result != nil {
		lines = append(lines, styledLine{"", pal.panelText}, styledLine{"run of " + result.Start.Format("Jan 2 15:04"), pal.panelHeading})
		lines = append(lines, stepResultLines(*result, path, pal)...)
	}
	a.openInfo(step.Label(), lines)
}

func (a *App) stepLines(step flow.Step) []styledLine {
	pal := &a.palette
	var lines []styledLine
	add := func(text string, style uv.Style) { lines = append(lines, styledLine{text, style}) }
	switch step.Kind() {
	case flow.ScriptStep:
		lines = append(lines, a.scriptLines(step.Script)...)
	case flow.CommandStep:
		add("Runs a command line:", pal.panelHeading)
		add("$ "+step.Run, pal.panelText)
	case flow.IfStep:
		add("Chooses a branch: then when this is true, else when false.", pal.panelHeading)
		add(step.If, pal.panelText)
		add(fmt.Sprintf("then: %d steps · else: %d steps", countSteps(step.Then), countSteps(step.Else)), pal.panelMuted)
	case flow.RepeatStep:
		add("Repeats the steps inside it.", pal.panelHeading)
		add(repeatText(step.Repeat), pal.panelText)
		add("Each pass forgets the results of the pass before.", pal.panelMuted)
	case flow.WaitStep:
		add("Waits "+step.Wait+" before the next step.", pal.panelHeading)
	case flow.PromptStep:
		add("Asks the user, and waits for the answer:", pal.panelHeading)
		add(step.Prompt, pal.panelText)
		add("the answer goes to {{."+step.Var+"}}", pal.panelText)
		if step.Default != "" {
			add("an empty answer keeps "+step.Default, pal.panelMuted)
		}
		if step.Type == flow.NumberParam {
			add("asks again until the answer is a number", pal.panelMuted)
		}
		add("a run nobody watches (a trigger) fails here", pal.panelMuted)
	case flow.SetStep:
		add("Sets variables for the steps after it:", pal.panelHeading)
		for _, text := range withText(step.Set) {
			add(text, pal.panelText)
		}
	case flow.UnsetStep:
		add("Clears variables and steps' results; they read as empty after it:", pal.panelHeading)
		add(strings.Join(step.Unset, ", "), pal.panelText)
	}
	for _, text := range withText(step.With) {
		add("with "+text, pal.panelText)
	}
	for _, name := range slices.Sorted(maps.Keys(step.Capture)) {
		add(fmt.Sprintf("captures %s from %s", name, step.Capture[name]), pal.panelText)
	}
	if step.ID != "" && (step.Kind() == flow.ScriptStep || step.Kind() == flow.CommandStep) {
		add("later steps read {{.steps."+step.ID+".output}}, .exit, .ok and the captures", pal.panelMuted)
	}
	if step.OnFail == flow.OnFailContinue {
		add("a failure does not stop the flow", pal.panelMuted)
	}
	return lines
}

// scriptLines tell what a script does: its description, command line and notes.
func (a *App) scriptLines(id string) []styledLine {
	pal := &a.palette
	s, ok := a.scripts.Find(id)
	if !ok {
		return []styledLine{{"Runs script " + id + ", which is not found.", pal.status}}
	}
	lines := []styledLine{{"Runs script " + s.ID(), pal.panelHeading}}
	if s.Description != "" {
		lines = append(lines, styledLine{s.Description, pal.panelText})
	}
	for _, line := range strings.Split(strings.TrimRight(s.Run, "\n"), "\n") {
		lines = append(lines, styledLine{"$ " + line, pal.panelMuted})
	}
	if notes := strings.TrimSpace(s.Notes()); notes != "" {
		lines = append(lines, styledLine{"", pal.panelText})
		for _, line := range strings.Split(notes, "\n") {
			lines = append(lines, styledLine{line, pal.panelText})
		}
	}
	return lines
}

func repeatText(r *flow.Repeat) string {
	text := fmt.Sprintf("%d times", r.Times)
	if r.Until != "" {
		text = fmt.Sprintf("until %s, at most %d times", r.Until, mostPasses(r.Times, r.Max))
	}
	if r.Every != "" {
		text += ", every " + r.Every
	}
	return text
}

// mostPasses is times when set, else max: the most passes a repeat runs.
func mostPasses(times, most int) int {
	if times > 0 {
		return times
	}
	return most
}

func stepResultLines(r flow.Result, path string, pal *palette) []styledLine {
	step, ran := r.Steps[path]
	if !ran {
		return []styledLine{{"did not run (" + r.Start.Format("Jan 2 15:04") + ")", pal.panelMuted}}
	}
	style, text := pal.panelText, "✓ ok"
	if step.Status == flow.StatusFailed {
		style, text = pal.status, "✗ failed"
	}
	text += fmt.Sprintf(" · exit %d · %s · %s", step.Exit, step.Duration.Round(1e6), r.Start.Format("Jan 2 15:04"))
	if step.Tried {
		text += " · tried alone"
	}
	lines := []styledLine{{text, style}}
	if step.Message != "" {
		lines = append(lines, styledLine{step.Message, style})
	}
	for _, name := range slices.Sorted(maps.Keys(step.Captures)) {
		lines = append(lines, styledLine{name + " = " + step.Captures[name], pal.panelText})
	}
	if step.Output != "" {
		lines = append(lines, styledLine{"", pal.panelText}, styledLine{"output", pal.panelHeading})
		for _, line := range strings.Split(step.Output, "\n") {
			lines = append(lines, styledLine{line, pal.panelMuted})
		}
	}
	return lines
}
