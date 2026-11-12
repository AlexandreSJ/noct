package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/script"
)

// Tool is one thing an AI can do through noct.
type Tool struct {
	Name        string
	Title       string
	Description string
	Params      []Param
	ReadOnly    bool
	Destructive bool
	// Policy is the tool's default: config.PolicyAllow, PolicyAsk or PolicyDeny; the user's
	// [mcp.policy] overrides it.
	Policy string
	// AskWhen, when set, turns the default allow into ask for a call it reports, unless the
	// user's [mcp.policy] names the tool.
	AskWhen func(args json.RawMessage) bool
	// Summary tells the user, when asked to approve, exactly what the call would do.
	Summary func(args json.RawMessage) string
	// Run does the call; ctx ends when the AI client leaves, so a wait or a request to the
	// workspace stops with it.
	Run func(ctx context.Context, args json.RawMessage) (any, error)
	// Call is Run behind the policy and the audit log; NewServer sets it. Ending ctx withdraws
	// an approval the call waits for.
	Call func(ctx context.Context, args json.RawMessage) (any, error)
}

// Param is one argument of a tool.
type Param struct {
	Name        string
	Type        string // string, integer, boolean or object
	Description string
	Required    bool
	Enum        []string
}

func (t Tool) schema() map[string]any {
	properties := map[string]any{}
	required := []string{}
	for _, p := range t.Params {
		property := map[string]any{"type": p.Type, "description": p.Description}
		if len(p.Enum) > 0 {
			property["enum"] = p.Enum
		}
		if p.Type == "object" {
			property["additionalProperties"] = map[string]string{"type": "string"}
		}
		properties[p.Name] = property
		if p.Required {
			required = append(required, p.Name)
		}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required}
}

// Workspace does what needs the running noct: panes, runs the user sees, and approvals.
type Workspace interface {
	Do(ctx context.Context, action string, args any, into any) error
}

// Deps are what the tools read and change.
type Deps struct {
	// Library reads the scripts; ScriptsDir is where drafted scripts go.
	Library    func() script.Library
	ScriptsDir func() string
	Config     func() config.Config
	Workspace  Workspace
	// Audit gets a line for every call, what it asked and how it went.
	Audit io.Writer
	Now   func() time.Time
}

// NewServer is noct's MCP server with every tool, each behind the policy and the audit log.
func NewServer(version string, d Deps) *Server {
	tools := append(append(libraryTools(d), runTools(d)...), workspaceTools(d)...)
	for i := range tools {
		tools[i].Call = d.guarded(tools[i])
	}
	return &Server{Name: "noct", Version: version, Instructions: instructions, Tools: tools}
}

// guarded is the tool's Run behind the policy, with every call written to the audit log.
func (d Deps) guarded(t Tool) func(context.Context, json.RawMessage) (any, error) {
	return func(ctx context.Context, args json.RawMessage) (any, error) {
		policy, err := d.guard(ctx, t, args)
		var result any
		if err == nil {
			result, err = t.Run(ctx, args)
		}
		d.audit(t.Name, policy, args, err)
		return result, err
	}
}

// noWait is a tool's Run for a call that answers at once and so has no use for ctx.
func noWait(run func(args json.RawMessage) (any, error)) func(context.Context, json.RawMessage) (any, error) {
	return func(_ context.Context, args json.RawMessage) (any, error) { return run(args) }
}

// guard applies the policy: deny refuses, ask waits for the user in noct's TUI.
func (d Deps) guard(ctx context.Context, t Tool, args json.RawMessage) (string, error) {
	policy, configured := d.Config().MCP.Policy[t.Name]
	switch {
	case configured:
	case t.Policy == config.PolicyAllow && t.AskWhen != nil && t.AskWhen(args):
		policy = config.PolicyAsk
	default:
		policy = t.Policy
	}
	switch policy {
	case config.PolicyDeny:
		return policy, fmt.Errorf("%w: the noct config has [mcp.policy] %s = \"deny\"", errDenied, t.Name)
	case config.PolicyAsk:
		summary := t.Name
		if t.Summary != nil {
			summary = t.Summary(args)
		}
		var answer struct{ Approved bool }
		if err := d.Workspace.Do(ctx, "approve", map[string]string{"tool": t.Name, "summary": summary}, &answer); err != nil {
			return policy, fmt.Errorf("asking the user to approve %s: %w", t.Name, err)
		}
		if !answer.Approved {
			return policy, fmt.Errorf("%w: the user did not approve %s (or did not answer in time)", errDenied, t.Name)
		}
	}
	return policy, nil
}

// maxAuditArgs bounds how much of a call's arguments the audit log keeps.
const maxAuditArgs = 2000

func (d Deps) audit(tool, policy string, args json.RawMessage, err error) {
	if d.Audit == nil {
		return
	}
	entry := map[string]any{"at": d.Now().Format(time.RFC3339), "tool": tool, "policy": policy, "ok": err == nil}
	text := string(args)
	if len(text) > maxAuditArgs {
		text = text[:maxAuditArgs] + "…"
	}
	entry["args"] = text
	if err != nil {
		entry["error"] = err.Error()
	}
	line, _ := json.Marshal(entry)           // plain values always encode
	_, _ = d.Audit.Write(append(line, '\n')) // losing an audit line must not fail the call
}

// decode reads a tool's arguments.
func decode(args json.RawMessage, into any) error {
	if err := json.Unmarshal(args, into); err != nil {
		return fmt.Errorf("reading the arguments: %w", err)
	}
	return nil
}

// argText puts fields of the arguments in a summary, one per line.
func argText(args json.RawMessage, fields ...string) string {
	var values map[string]any
	_ = json.Unmarshal(args, &values) // a summary of arguments that do not read is just shorter
	var parts []string
	for _, field := range fields {
		if v, ok := values[field]; ok {
			parts = append(parts, field+": "+summaryValue(v))
		}
	}
	return strings.Join(parts, "\n")
}

// summaryValue shows a value on one line, quoted when it holds control or format characters, so
// a command cannot hide lines below what the approval shows or reorder its text.
func summaryValue(v any) string {
	text := fmt.Sprint(v)
	if strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) {
		return strconv.Quote(text)
	}
	return text
}

// instructions tell the AI how noct's tools fit together.
const instructions = `noct keeps scripts (saved commands with parameters and notes), flows (YAML chains of
steps: script, run, if, switch, repeat, wait, prompt, set, unset, call, agent, with captures,
expect, on_fail, retry, pane and triggers) and functions (steps with params and returns that flows call).

Start with list_scripts and list_flows, then get_item to read one with its notes. To build or
change a flow or script, write the YAML with draft_flow or draft_script: drafts never run until the
user reviews and promotes them in noct, so tell the user what you drafted and ask them to promote
it. draft_function does the same for a function; a draft function does not run, and the flows
that call it wait for its promotion. check and flow_scope tell problems and what a step can read.

Flow params have a type: text, number, bool, choice (with choices) or path; a param with from:
env:NAME reads the variable. Each script, run, call and agent step also sets short variables
({{.output}}, {{.exit}}, {{.ok}}, captures, a call's returns, an agent's {{.answer}}); an if,
switch or repeat changes variables made before it, and the ones it makes go when it ends unless its
export keeps them: export: {count: passes}. switch: template, cases: [{is: value or [values],
steps}], else: steps. repeat: {each: template, as: name} runs once per line. retry: 3 or {max: 3,
every: 10s} runs a failed script, run or agent step again before on_fail. A call step runs a function: call: name, with: {param: value}.
An agent step asks one of the user's AI agents (list_flows names them): agent: {ask: question,
use: name, choices: [a, b]}; its answer is text from an AI, untrusted like any output.

run starts a script or flow where the user sees it, as a pane of the noct workspace, and gives a
run_id; get_run (with wait_seconds) gives the structured result: each step's status, exit code,
captures and output. search_logs and read_log read what runs printed. list_panes and read_pane
show the terminals; run_command and send_input act in them and, like run with params, usually need
the user's approval, which they give in noct (prefix+y); a denied call says so, do not retry it
unasked.

Templates are Go text/template and every value is a string: compare with {{eq .steps.x.exit "0"}}.
In a run line, write {{quote .name}} or {{quote .steps.x.output}} so a value is one shell word.`
