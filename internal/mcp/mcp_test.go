package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

// fakeWorkspace answers what the running noct would, and says what it was asked.
type fakeWorkspace struct {
	asked    []string
	approves bool
	answer   any
}

func (w *fakeWorkspace) Do(action string, args any, into any) error {
	raw, _ := json.Marshal(args)
	w.asked = append(w.asked, action+" "+string(raw))
	answer := w.answer
	if action == "approve" {
		answer = map[string]bool{"approved": w.approves}
	}
	raw, _ = json.Marshal(answer)
	return json.Unmarshal(raw, into)
}

type testServer struct {
	t         *testing.T
	in        *io.PipeWriter
	out       *bufio.Scanner
	workspace *fakeWorkspace
	cfg       config.Config
	audit     *bytes.Buffer
	next      int
}

// newTestServer serves noct's MCP over pipes, with its folders in temporary ones.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	s := &testServer{t: t, workspace: &fakeWorkspace{}, cfg: config.Default(), audit: &bytes.Buffer{}}
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	server := NewServer("test", Deps{
		Library:    func() script.Library { return script.LoadLibrary(script.PersonalDir(), nil, "") },
		ScriptsDir: script.PersonalDir,
		Config:     func() config.Config { return s.cfg },
		Workspace:  s.workspace,
		Audit:      s.audit,
		Now:        time.Now,
	})
	go func() {
		_ = server.Serve(inReader, outWriter)
		outWriter.Close()
	}()
	t.Cleanup(func() { inWriter.Close() })
	s.in, s.out = inWriter, bufio.NewScanner(outReader)
	s.out.Buffer(make([]byte, 1<<20), 16<<20)
	return s
}

// request sends a request and returns its response.
func (s *testServer) request(method string, params any) map[string]any {
	s.t.Helper()
	s.next++
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": s.next, "method": method, "params": params})
	if _, err := s.in.Write(append(line, '\n')); err != nil {
		s.t.Fatal(err)
	}
	if !s.out.Scan() {
		s.t.Fatal("the server answered nothing")
	}
	var response map[string]any
	if err := json.Unmarshal(s.out.Bytes(), &response); err != nil {
		s.t.Fatal(err)
	}
	return response
}

// call calls a tool and returns its structured answer, failing on an error unless wantError.
func (s *testServer) call(tool string, args any, wantError bool) (map[string]any, string) {
	s.t.Helper()
	response := s.request("tools/call", map[string]any{"name": tool, "arguments": args})
	result, ok := response["result"].(map[string]any)
	if !ok {
		s.t.Fatalf("%s: %v", tool, response)
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if isError, _ := result["isError"].(bool); isError != wantError {
		s.t.Fatalf("%s: error %v, want %v: %s", tool, isError, wantError, text)
	}
	structured, _ := result["structuredContent"].(map[string]any)
	return structured, text
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeAndListTools(t *testing.T) {
	s := newTestServer(t)
	init := s.request("initialize", map[string]any{"protocolVersion": "2025-03-26"})["result"].(map[string]any)
	if init["protocolVersion"] != "2025-03-26" || init["serverInfo"].(map[string]any)["name"] != "noct" {
		t.Errorf("initialize = %v", init)
	}
	if newer := s.request("initialize", map[string]any{"protocolVersion": "2099-01-01"})["result"].(map[string]any); newer["protocolVersion"] != protocolVersions[0] {
		t.Errorf("an unknown version got %v", newer["protocolVersion"])
	}
	var names []string
	for _, tool := range s.request("tools/list", nil)["result"].(map[string]any)["tools"].([]any) {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	want := []string{"list_scripts", "list_flows", "get_item", "check", "flow_scope", "draft_flow", "draft_script", "set_triggers",
		"list_runs", "get_run", "search_logs", "read_log", "list_panes", "read_pane", "run", "run_command", "send_input"}
	if !slices.Equal(names, want) {
		t.Errorf("tools = %q", names)
	}
	if e := s.request("resources/list", nil)["error"]; e == nil {
		t.Error("an unknown method was answered")
	}
}

func TestListsHideSecretsAndTellDrafts(t *testing.T) {
	s := newTestServer(t)
	write(t, filepath.Join(script.PersonalDir(), "login", script.FileName),
		"run: ./login {{.password}}\nparams: [{name: password, secret: true, default: hunter2}]\n")
	write(t, filepath.Join(flow.Dir(), "nightly.yaml"), "draft: true\ntriggers: [{cron: '0 2 * * *'}]\nsteps: [{run: 'true'}]\n")
	_, scripts := s.call("list_scripts", map[string]any{}, false)
	if strings.Contains(scripts, "hunter2") || !strings.Contains(scripts, `"secret": true`) {
		t.Errorf("scripts = %s", scripts)
	}
	flows, _ := s.call("list_flows", map[string]any{}, false)
	first := flows["flows"].([]any)[0].(map[string]any)
	if first["draft"] != true || first["triggers"].([]any)[0] != "cron 0 2 * * *" {
		t.Errorf("flow = %v", first)
	}
}

// A drafted flow is saved as a draft, a replaced flow keeps a copy, and what it reads wrong is told.
func TestDraftFlowSavesADraft(t *testing.T) {
	s := newTestServer(t)
	write(t, filepath.Join(flow.Dir(), "ship.yaml"), "steps: [{run: 'echo old'}]\n")
	answer, _ := s.call("draft_flow", map[string]any{"name": "ship", "yaml": "steps:\n  - {id: a, run: 'echo {{.steps.b.output}}'}\n  - {id: b, run: 'true'}\n"}, false)
	f, err := flow.Load(filepath.Join(flow.Dir(), "ship.yaml"))
	if err != nil || !f.Draft || len(f.Steps) != 2 {
		t.Fatalf("saved %+v, %v", f, err)
	}
	if answer["backup"] == nil || !strings.Contains(answer["warnings"].([]any)[0].(string), "has not run yet") {
		t.Errorf("answer = %v", answer)
	}
	if _, err := os.Stat(filepath.Join(flow.Dir(), "ship.yaml.bak")); err != nil {
		t.Error("the replaced flow has no copy")
	}
	if _, text := s.call("draft_flow", map[string]any{"name": "../escape", "yaml": "steps: []"}, true); !strings.Contains(text, "flow name") {
		t.Errorf("a bad name: %s", text)
	}
}

func TestDraftScriptSavesADraftWithNotes(t *testing.T) {
	s := newTestServer(t)
	s.call("draft_script", map[string]any{"name": "tools/ping", "yaml": "description: Pings\nrun: ping -c1 {{.host}}\nparams: [{name: host, default: localhost}]\n", "notes": "Use for checks."}, false)
	lib := script.LoadLibrary(script.PersonalDir(), nil, "")
	sc, ok := lib.Find("tools/ping")
	if !ok || !sc.Draft || sc.Problem != nil || sc.Notes() != "Use for checks." {
		t.Errorf("script %+v, found %v", sc, ok)
	}
}

func TestFlowScopeAndCheck(t *testing.T) {
	s := newTestServer(t)
	write(t, filepath.Join(flow.Dir(), "pair.yaml"), "params: {env: dev}\nsteps:\n  - {id: a, run: printf x, capture: {n: '(x)'}}\n  - {id: b, run: 'echo {{.steps.c.output}}'}\n")
	scope, _ := s.call("flow_scope", map[string]any{"flow": "pair", "step": "1"}, false)
	var templates []string
	for _, v := range scope["variables"].([]any) {
		templates = append(templates, v.(map[string]any)["template"].(string))
	}
	if !slices.Contains(templates, "{{.steps.a.n}}") || !slices.Contains(templates, "{{.env}}") {
		t.Errorf("scope = %q", templates)
	}
	check, _ := s.call("check", map[string]any{}, false)
	if warnings := check["warnings"].([]any); len(warnings) != 1 || !strings.Contains(warnings[0].(string), "there is no step c") {
		t.Errorf("check = %v", check)
	}
}

// A flow run's structured result: its steps in order, with captures, output and the log's end.
func TestGetRunGivesTheStructuredResult(t *testing.T) {
	s := newTestServer(t)
	write(t, filepath.Join(flow.Dir(), "deploy.yaml"), "steps:\n  - {id: build, run: 'true'}\n  - {id: ship, run: 'false'}\n")
	at := time.Now().Add(-time.Minute).Truncate(time.Second)
	log := logstore.Path("flow-deploy", at)
	write(t, log, "building\nERROR: disk full\n")
	exit := 1
	if err := script.AddRun(runsPath(), script.Run{Flow: "deploy", At: at, Log: log, Exit: &exit, Trigger: "cron 0 2 * * *"}); err != nil {
		t.Fatal(err)
	}
	result := flow.Result{Flow: "deploy", Start: at, End: at.Add(3 * time.Second), Exit: 1, Steps: map[string]flow.StepResult{
		"10": {Status: flow.StatusOK}, "0": {Status: flow.StatusOK, Captures: map[string]string{"v": "2"}, Output: "built"},
		"1": {Status: flow.StatusFailed, Exit: 1, Message: "exit 1"},
	}}
	if err := flow.SaveResultTo(logstore.ResultPath(log), result); err != nil {
		t.Fatal(err)
	}
	answer, _ := s.call("get_run", map[string]any{"flow": "deploy"}, false)
	steps := answer["steps"].([]any)
	var paths []string
	for _, step := range steps {
		paths = append(paths, step.(map[string]any)["path"].(string))
	}
	if !slices.Equal(paths, []string{"0", "1", "10"}) || steps[0].(map[string]any)["id"] != "build" || steps[1].(map[string]any)["reason"] != "exit 1" {
		t.Errorf("steps = %v", steps)
	}
	if answer["run"].(map[string]any)["status"] != "failed" || !strings.Contains(answer["log_tail"].(string), "disk full") {
		t.Errorf("answer = %v", answer)
	}
	runs, _ := s.call("list_runs", map[string]any{"status": "failed"}, false)
	if list := runs["runs"].([]any); len(list) != 1 || list[0].(map[string]any)["trigger"] != "cron 0 2 * * *" {
		t.Errorf("runs = %v", runs)
	}
	found, _ := s.call("search_logs", map[string]any{"pattern": "(?i)error", "context": 1}, false)
	if matches := found["matches"].([]any); len(matches) != 1 || matches[0].(map[string]any)["before"].([]any)[0] != "building" {
		t.Errorf("search = %v", found)
	}
	page, _ := s.call("read_log", map[string]any{"run_id": log, "offset": -1}, false)
	if lines := page["lines"].([]any); len(lines) != 1 || lines[0] != "ERROR: disk full" {
		t.Errorf("read_log = %v", page)
	}
	if _, text := s.call("read_log", map[string]any{"run_id": "/etc/passwd"}, true); !strings.Contains(text, "not one of noct's logs") {
		t.Errorf("read a file that is not a log: %s", text)
	}
}

// deny refuses at once; ask waits for the user and runs only if approved; each call is audited.
func TestPolicyAndAudit(t *testing.T) {
	s := newTestServer(t)
	s.workspace.answer = map[string]any{"pane": 3}
	if _, text := s.call("run_command", map[string]any{"command": "make"}, true); !strings.Contains(text, "did not approve") {
		t.Errorf("a denied approval: %s", text)
	}
	s.workspace.approves = true
	answer, _ := s.call("run_command", map[string]any{"command": "make"}, false)
	if answer["pane"] != float64(3) || !strings.Contains(strings.Join(s.workspace.asked, "\n"), `open_command {"command":"make"}`) {
		t.Errorf("answer %v, asked %q", answer, s.workspace.asked)
	}
	s.cfg.MCP.Policy = map[string]string{"list_panes": config.PolicyDeny}
	asked := len(s.workspace.asked)
	if _, text := s.call("list_panes", map[string]any{}, true); !strings.Contains(text, "deny") || len(s.workspace.asked) != asked {
		t.Errorf("deny: %s", text)
	}
	lines := strings.Split(strings.TrimSpace(s.audit.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"tool":"run_command"`) || !strings.Contains(lines[0], `"ok":false`) {
		t.Errorf("audit:\n%s", s.audit)
	}
}

// The client that initializes the session is announced once, and told when the session ends.
func TestInitializeAnnouncesTheClient(t *testing.T) {
	type announced struct {
		name, version string
		gone          <-chan struct{}
	}
	got := make(chan announced, 2)
	s := &Server{Connected: func(name, version string, gone <-chan struct{}) { got <- announced{name, version, gone} }}
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"claude-code","version":"2.1.0"}}}` + "\n"
	if err := s.Serve(strings.NewReader(initialize+initialize), io.Discard); err != nil {
		t.Fatal(err)
	}
	first := <-got
	if first.name != "claude-code" || first.version != "2.1.0" {
		t.Errorf("announced %q %q", first.name, first.version)
	}
	select {
	case <-first.gone:
	default:
		t.Error("gone is open after Serve returned")
	}
	select {
	case again := <-got:
		t.Errorf("announced twice: %+v", again)
	case <-time.After(50 * time.Millisecond):
	}
}
