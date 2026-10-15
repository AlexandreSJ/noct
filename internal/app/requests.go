package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"maps"
	"slices"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/script"
)

// PaneInfo describes a pane to the MCP server.
type PaneInfo struct {
	ID       int    `json:"id"`
	Session  string `json:"session"`
	Tab      int    `json:"tab"`
	Title    string `json:"title"`
	Focused  bool   `json:"focused"`
	Floating bool   `json:"floating,omitempty"`
	Cols     int    `json:"cols"`
	Rows     int    `json:"rows"`
}

// answerRequest does what the MCP server asks of the workspace, so that what an AI runs shows
// where the user sees it. Approvals answer later, once the user decides; a client's connection is
// never answered, it stays open while the AI client runs.
func (a *App) answerRequest(c connection) {
	reply := func(data any, err error) {
		exit := ipc.Exit{Reason: ipc.RequestDone}
		if err == nil {
			exit.Data, err = json.Marshal(data)
		}
		if err != nil {
			exit = ipc.Exit{Reason: err.Error()}
		}
		_ = c.conn.SendJSON(ipc.KindExit, exit) // a server that stopped listening learns nothing
		_ = c.conn.Close()
	}
	req := c.hello.Request
	if req == nil {
		reply(nil, errors.New("nothing was asked"))
		return
	}
	var pane paneArgs
	var input inputArgs
	var command commandArgs
	var run runArgs
	var name sessionArgs
	switch req.Action {
	case "list_panes":
		reply(a.paneInfos(), nil)
	case "read_pane":
		reply(decoded(req, &pane, func() (any, error) { return a.readPane(pane) }))
	case "send_input":
		reply(decoded(req, &input, func() (any, error) { return a.sendInput(input) }))
	case "open_command":
		a.openSession(a.session) // what an AI opens shows in the session on screen
		reply(decoded(req, &command, func() (any, error) { return a.openCommand(command) }))
	case "run":
		a.openSession(a.session)
		reply(decoded(req, &run, func() (any, error) { return a.runAsked(run) }))
	case "sessions":
		reply(a.sessionInfos(), nil)
	case "delete_session":
		reply(decoded(req, &name, func() (any, error) { return nil, a.deleteSessionNamed(name.Name) }))
	case "attention":
		reply(decoded(req, &pane, func() (any, error) { return nil, a.showPane(pane) }))
	case "approve":
		var args approvalArgs
		if err := decodeArgs(req, &args); err != nil {
			reply(nil, err)
			return
		}
		a.queueApproval(args, func(approved bool) { reply(map[string]bool{"approved": approved}, nil) })
	case "client":
		var client aiClient
		if err := decodeArgs(req, &client); err != nil {
			reply(nil, err)
			return
		}
		a.connectAIClient(c, &client)
	default:
		reply(nil, fmt.Errorf("the workspace does not know %q", req.Action))
	}
}

// decoded reads a request's arguments into args, then does what it asks.
func decoded(req *ipc.Request, args any, do func() (any, error)) (any, error) {
	if err := decodeArgs(req, args); err != nil {
		return nil, err
	}
	return do()
}

func decodeArgs(req *ipc.Request, into any) error {
	if len(req.Args) == 0 || string(req.Args) == "null" {
		return nil
	}
	if err := json.Unmarshal(req.Args, into); err != nil {
		return fmt.Errorf("arguments of %s: %w", req.Action, err)
	}
	return nil
}

func (a *App) paneInfos() []PaneInfo {
	infos := []PaneInfo{}
	for _, s := range a.sessions {
		for i, t := range s.tabs {
			if t.isEmpty() {
				continue
			}
			for _, id := range slices.Sorted(maps.Keys(t.views)) {
				size := contentSize(t.layoutFrame(id, a.workspaceArea()))
				infos = append(infos, PaneInfo{
					ID: int(id), Session: s.name, Tab: i + 1, Title: a.viewTitle(t, id), Focused: id == t.focused,
					Floating: t.floatingPane(id) != nil, Cols: size.Cols, Rows: size.Rows,
				})
			}
		}
	}
	return infos
}

// paneOf finds a pane by its number, in any tab.
func (a *App) paneOf(id int) (*tab, layout.PaneID, error) {
	t, pane := a.tabOfPane(layout.PaneID(id))
	if t == nil {
		return nil, 0, fmt.Errorf("there is no pane %d; list_panes tells the open ones", id)
	}
	return t, pane, nil
}

type paneArgs struct {
	Pane int `json:"pane"`
}

// readPane gives the text a pane shows now, as it would be drawn at its size.
func (a *App) readPane(args paneArgs) (any, error) {
	t, id, err := a.paneOf(args.Pane)
	if err != nil {
		return nil, err
	}
	size := contentSize(t.layoutFrame(id, a.workspaceArea()))
	cols, rows := max(size.Cols, 1), max(size.Rows, 1)
	buffer := uv.NewScreenBuffer(cols, rows)
	t.views[id].Draw(buffer, image.Rect(0, 0, cols, rows))
	return map[string]string{"text": screenLines(buffer, cols, rows)}, nil
}

// screenLines is a screen as text: its rows without trailing spaces, without empty rows at the end.
func screenLines(buffer uv.ScreenBuffer, cols, rows int) string {
	lines := make([]string, rows)
	for y := range rows {
		var line strings.Builder
		for x := range cols {
			if cell := buffer.CellAt(x, y); cell != nil && cell.Width > 0 {
				line.WriteString(cell.Content)
			} else if cell == nil {
				line.WriteByte(' ')
			}
		}
		lines[y] = strings.TrimRight(line.String(), " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

type inputArgs struct {
	Pane  int    `json:"pane"`
	Text  string `json:"text"`
	Enter bool   `json:"enter"`
}

// sendInput types text into a pane as keys, a newline as enter, and enter after it when asked.
func (a *App) sendInput(args inputArgs) (any, error) {
	t, id, err := a.paneOf(args.Pane)
	if err != nil {
		return nil, err
	}
	typeInView(t.views[id], args.Text, args.Enter)
	return map[string]int{"pane": args.Pane}, nil
}

type commandArgs struct {
	Command string `json:"command"`
	Dir     string `json:"dir"`
}

// openCommand runs a command line in a new pane of the current tab, held open with its exit code.
func (a *App) openCommand(args commandArgs) (any, error) {
	if strings.TrimSpace(args.Command) == "" {
		return nil, errors.New("the command line is empty")
	}
	p := program{argv: append(slices.Clone(a.shellArgv), "-c", args.Command), dir: orDefault(args.Dir, a.workDir), logName: "ai-command"}
	pane, log := a.openTrackedRun("ai: "+args.Command, script.Run{Script: "ai-command"}, p)
	if log == "" {
		return nil, errors.New(orDefault(a.statusMessage, "the pane could not open"))
	}
	return map[string]any{"pane": int(pane), "run_id": log}, nil
}

type runArgs struct {
	Script string            `json:"script"`
	Flow   string            `json:"flow"`
	Params map[string]string `json:"params"`
	Steps  string            `json:"steps"`
}

// runAsked runs a script or a flow where the user sees it, as r does in the TUI, and tells its
// run id: the path of its log.
func (a *App) runAsked(args runArgs) (any, error) {
	var pane layout.PaneID
	var log string
	var err error
	switch {
	case args.Flow != "":
		pane, log, err = a.runFlowAsked(args)
	case args.Script != "":
		pane, log, err = a.runScriptAsked(args)
	default:
		err = errors.New("run needs a script or a flow")
	}
	if err == nil && log == "" {
		err = errors.New(orDefault(a.statusMessage, "the run's pane could not open"))
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"run_id": log, "pane": int(pane)}, nil
}

func (a *App) runFlowAsked(args runArgs) (layout.PaneID, string, error) {
	a.reloadScripts()
	a.reloadFlows()
	f, ok := flow.Find(a.flows, args.Flow)
	switch {
	case !ok:
		return 0, "", fmt.Errorf("there is no flow %s", args.Flow)
	case f.Problem != nil:
		return 0, "", f.Problem
	case f.DraftError() != nil:
		return 0, "", f.DraftError()
	}
	argv := []string{orDefault(a.binary.path, "noct"), "flow", "run", f.Name}
	if args.Steps != "" {
		argv = append(argv, "--steps", args.Steps)
	}
	for _, name := range slices.Sorted(maps.Keys(args.Params)) {
		argv = append(argv, name+"="+args.Params[name])
	}
	pane, log := a.openTrackedRun("flow "+f.Name, script.Run{Flow: f.Name}, program{argv: argv, dir: a.workDir})
	return pane, log, nil
}

func (a *App) runScriptAsked(args runArgs) (layout.PaneID, string, error) {
	a.reloadScripts()
	s, ok := a.scripts.Find(args.Script)
	switch {
	case !ok:
		return 0, "", fmt.Errorf("there is no script %s", args.Script)
	case s.Problem != nil:
		return 0, "", s.Problem
	case s.Draft:
		return 0, "", fmt.Errorf("script %s is a draft: the user promotes it first", s.ID())
	}
	values := s.StartingValues(a.memory.Values(s), a.getenv)
	maps.Copy(values, args.Params)
	if err := s.CheckValues(values); err != nil {
		return 0, "", err
	}
	command, err := s.Command(values)
	if err != nil {
		return 0, "", err
	}
	p := program{argv: []string{a.shellArgv[0], "-c", command}, dir: s.WorkingDir(a.workDir)}
	pane, log := a.openTrackedRun(s.Name, script.Run{Script: s.ID()}, p)
	return pane, log, nil
}
