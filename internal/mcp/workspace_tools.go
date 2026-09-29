package mcp

import (
	"encoding/json"

	"github.com/AlexandreSJ/noct/internal/config"
)

// workspaceTools act through the running noct, so what they do shows where the user sees it.
func workspaceTools(d Deps) []Tool {
	return []Tool{
		{
			Name: "list_panes", Title: "List panes", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "Lists the panes open in noct's workspace: id, tab, title, size, and which has the focus in its tab.",
			Run:         d.forward("list_panes", func(v any) any { return map[string]any{"panes": v} }),
		},
		{
			Name: "read_pane", Title: "Read a pane", ReadOnly: true, Policy: config.PolicyAllow,
			Description: "The text a pane shows right now: a terminal's screen, a run's output, a panel.",
			Params:      []Param{{Name: "pane", Type: "integer", Description: "A pane id from list_panes.", Required: true}},
			Run:         d.forward("read_pane", nil),
		},
		{
			Name: "run", Title: "Run a script or flow", Policy: config.PolicyAllow,
			Description: "Runs a script or a flow (not a draft) in a new pane of noct's workspace, where the user sees it, and answers its run_id right away; get_run with wait_seconds follows it. steps tests only some steps of a flow (2, or 0/steps/1..0/steps/3), reading the values of its last run.",
			Params: []Param{
				{Name: "script", Type: "string", Description: "A script's id."},
				{Name: "flow", Type: "string", Description: "A flow's name."},
				{Name: "params", Type: "object", Description: "Parameter values by name, over the defaults. Secret parameters come from the environment; do not pass them."},
				{Name: "steps", Type: "string", Description: "For a flow, the steps to test alone: first..last in one list, or one path."},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "script", "flow", "params", "steps") },
			Run:     d.forward("run", nil),
		},
		{
			Name: "run_command", Title: "Run a command line", Destructive: true, Policy: config.PolicyAsk,
			Description: "Runs any shell command line in a new pane of noct's workspace, held open with its exit code; answers the pane and run_id. Usually needs the user's approval.",
			Params: []Param{
				{Name: "command", Type: "string", Description: "The command line, run with the user's shell -c.", Required: true},
				{Name: "dir", Type: "string", Description: "Where it runs; where noct was opened by default."},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "command", "dir") },
			Run:     d.forward("open_command", nil),
		},
		{
			Name: "send_input", Title: "Type into a pane", Destructive: true, Policy: config.PolicyAsk,
			Description: "Types text into a pane, as keys (a newline is enter), and enter after it when asked: to answer a program's question or run something in a shell pane. Usually needs the user's approval.",
			Params: []Param{
				{Name: "pane", Type: "integer", Description: "A pane id from list_panes.", Required: true},
				{Name: "text", Type: "string", Description: "What to type.", Required: true},
				{Name: "enter", Type: "boolean", Description: "Press enter after the text."},
			},
			Summary: func(args json.RawMessage) string { return argText(args, "pane", "text", "enter") },
			Run:     d.forward("send_input", nil),
		},
	}
}

// forward asks the workspace to do a tool's action with the tool's arguments; wrap, when set,
// shapes the answer into an object.
func (d Deps) forward(action string, wrap func(any) any) func(json.RawMessage) (any, error) {
	return func(args json.RawMessage) (any, error) {
		var answer any
		if err := d.Workspace.Do(action, args, &answer); err != nil {
			return nil, err
		}
		if wrap != nil {
			return wrap(answer), nil
		}
		return answer, nil
	}
}
