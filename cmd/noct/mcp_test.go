package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// mcpClient talks to a real noct mcp process as Claude Code does: a JSON message per line.
type mcpClient struct {
	t    *testing.T
	in   io.WriteCloser
	out  *bufio.Scanner
	next int
}

func startMCP(t *testing.T, s *session) *mcpClient {
	t.Helper()
	cmd := exec.Command(s.binary, "mcp")
	cmd.Env = s.env
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { in.Close(); _ = cmd.Wait() })
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 1<<20), 16<<20)
	return &mcpClient{t: t, in: in, out: scanner}
}

func (c *mcpClient) request(method string, params any) map[string]any {
	c.t.Helper()
	c.next++
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.next, "method": method, "params": params})
	fmt.Fprintf(c.in, "%s\n", line)
	if !c.out.Scan() {
		c.t.Fatalf("noct mcp answered nothing to %s", method)
	}
	var response map[string]any
	if err := json.Unmarshal(c.out.Bytes(), &response); err != nil {
		c.t.Fatal(err)
	}
	return response
}

func (c *mcpClient) call(tool string, args any) map[string]any {
	c.t.Helper()
	result := c.request("tools/call", map[string]any{"name": tool, "arguments": args})["result"].(map[string]any)
	if isError, _ := result["isError"].(bool); isError {
		c.t.Fatalf("%s failed: %v", tool, result["content"])
	}
	return result["structuredContent"].(map[string]any)
}

// An AI runs a flow through noct mcp: it opens in the workspace, where the user sees it, and the
// AI reads its structured result and the pane it ran in.
func TestMCPRunsAFlowInTheWorkspace(t *testing.T) {
	s := newSession(t)
	noct, _ := s.attach(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	writeFlow(t, s, "hello", "steps:\n  - {id: greet, run: 'echo hello-from-mcp', capture: {who: 'hello-from-(\\w+)'}}\n")
	c := startMCP(t, s)
	c.request("initialize", map[string]any{"protocolVersion": "2025-06-18"})
	flows := c.call("list_flows", map[string]any{})["flows"].([]any)
	if len(flows) != 1 || flows[0].(map[string]any)["name"] != "hello" {
		t.Fatalf("flows = %v", flows)
	}
	started := c.call("run", map[string]any{"flow": "hello"})
	result := c.call("get_run", map[string]any{"run_id": started["run_id"], "wait_seconds": 20})
	steps := result["steps"].([]any)
	if result["run"].(map[string]any)["status"] != "ok" || len(steps) != 1 || steps[0].(map[string]any)["captures"].(map[string]any)["who"] != "mcp" {
		t.Fatalf("result = %v", result)
	}
	panes := c.call("list_panes", map[string]any{})["panes"].([]any)
	pane := panes[len(panes)-1].(map[string]any)
	screen := c.call("read_pane", map[string]any{"pane": pane["id"]})
	if !strings.Contains(screen["text"].(string), "hello-from-mcp") {
		t.Errorf("pane %v shows %q", pane, screen["text"])
	}
	waitForScreen(t, noct, "the run in the workspace", func(screen string) bool { return strings.Contains(screen, "hello-from-mcp") })
}

// An AI client that initializes noct mcp shows as connected in the MCP popup while it runs.
func TestMCPClientShowsAsConnected(t *testing.T) {
	s := newSession(t)
	noct, _ := s.attach(t)
	waitForScreen(t, noct, "the home screen", showsHome)
	c := startMCP(t, s)
	c.request("initialize", map[string]any{"protocolVersion": "2025-06-18", "clientInfo": map[string]string{"name": "e2e-ai", "version": "1.0"}})
	waitForScreen(t, noct, "the connected notice", func(screen string) bool { return strings.Contains(screen, "e2e-ai connected to noct") })
	noct.SendKey(prefix)
	noct.SendKey(uv.KeyPressEvent{Code: 'y', Text: "y"})
	waitForScreen(t, noct, "the client in the MCP popup", func(screen string) bool {
		return strings.Contains(screen, "● e2e-ai 1.0") && strings.Contains(screen, "claude mcp add")
	})
}
