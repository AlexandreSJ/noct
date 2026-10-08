package app

import (
	"encoding/json"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// Each AI client gets a command with the full path of this noct, quoted for the shell when it
// needs it, or the JSON for its config file.
func TestMCPSetupsUseThisNoct(t *testing.T) {
	setups := mcpSetups("/home/me/my tools/noct")
	byClient := map[string]mcpSetup{}
	for _, s := range setups {
		byClient[s.client] = s
	}
	want := map[string]string{
		"Claude Code": "claude mcp add --scope user noct -- '/home/me/my tools/noct' mcp",
		"Codex":       "codex mcp add noct -- '/home/me/my tools/noct' mcp",
		"Gemini CLI":  "gemini mcp add -s user noct '/home/me/my tools/noct' mcp",
		"VS Code":     `code --add-mcp '{"args":["mcp"],"command":"/home/me/my tools/noct","name":"noct"}'`,
	}
	for client, text := range want {
		if got := byClient[client].text; got != text {
			t.Errorf("%s:\n got %s\nwant %s", client, got, text)
		}
	}
	var opencode struct {
		MCP map[string]struct {
			Type    string   `json:"type"`
			Command []string `json:"command"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(byClient["opencode"].text), &opencode); err != nil {
		t.Fatal(err)
	}
	if noct := opencode.MCP["noct"]; noct.Type != "local" || !slices.Equal(noct.Command, []string{"/home/me/my tools/noct", "mcp"}) {
		t.Errorf("opencode: %s", byClient["opencode"].text)
	}
	if byClient["opencode"].file == "" || byClient["Claude Code"].file != "" {
		t.Error("only a client without an add command names a config file")
	}
	if shellWord("/usr/bin/noct") != "/usr/bin/noct" || shellWord("it's") != `'it'\''s'` {
		t.Errorf("shellWord: %s, %s", shellWord("/usr/bin/noct"), shellWord("it's"))
	}
}

// prefix+y lists the setups; enter copies the selected one and closes the popup.
func TestMCPPopupCopiesASetup(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.binary.path = "/usr/bin/noct"
	press(a, prefixPress, letter('y'))
	screen := screenText(a)
	if a.mode != mcpMode || !strings.Contains(screen, "Claude Code") || !strings.Contains(screen, "no AI client yet") {
		t.Fatalf("mode %v, screen:\n%s", a.mode, screen)
	}
	if !strings.Contains(screen, "claude mcp add --scope user noct -- /usr/bin/noct mcp") {
		t.Errorf("the first setup's command is not shown:\n%s", screen)
	}
	press(a, letter('j'), enterPress)
	if a.mode != normalMode || a.notice != "copied: add noct to Codex" {
		t.Errorf("mode %v, notice %q", a.mode, a.notice)
	}
}

// An AI client is listed while its noct mcp keeps the connection open, and not after.
func TestAIClientsStayListedWhileConnected(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.loopTasks, a.stopped = make(chan func()), make(chan struct{})
	server, client := net.Pipe()
	t.Cleanup(func() { server.Close(); client.Close() })
	hello := ipc.Hello{Version: ipc.Version, Purpose: ipc.Ask, Request: &ipc.Request{Action: "client", Args: json.RawMessage(`{"name":"claude-code","version":"2.1.0"}`)}}
	a.handleConnection(connection{conn: ipc.NewConn(server), hello: hello})
	if a.notice != "claude-code connected to noct" {
		t.Errorf("notice = %q", a.notice)
	}
	a.openMCP()
	if screen := screenText(a); !strings.Contains(screen, "● claude-code 2.1.0") {
		t.Errorf("the client is not listed:\n%s", screen)
	}
	client.Close()
	runLoopTask(t, a)
	if len(a.aiClients) != 0 {
		t.Errorf("still listed after it left: %+v", a.aiClients)
	}
}

// Multiple connections from the same client (same name and version) appear as a single entry.
func TestDuplicateAIClientsAppearOnce(t *testing.T) {
	a := testApp(t, newTab(1, &fakeView{}))
	a.loopTasks, a.stopped = make(chan func()), make(chan struct{})
	servers := make([]net.Conn, 3)
	clients := make([]net.Conn, 3)
	for i := range 3 {
		servers[i], clients[i] = net.Pipe()
	}
	t.Cleanup(func() {
		for i := range 3 {
			servers[i].Close()
			clients[i].Close()
		}
	})
	for i := range 3 {
		hello := ipc.Hello{Version: ipc.Version, Purpose: ipc.Ask, Request: &ipc.Request{Action: "client", Args: json.RawMessage(`{"name":"claude-code","version":"2.1.0"}`)}}
		a.handleConnection(connection{conn: ipc.NewConn(servers[i]), hello: hello})
	}
	if len(a.aiClients) != 1 {
		t.Errorf("got %d entries, want 1: %+v", len(a.aiClients), a.aiClients)
	}
	if a.aiClients[0].count != 3 {
		t.Errorf("count = %d, want 3", a.aiClients[0].count)
	}
	clients[0].Close()
	runLoopTask(t, a)
	if len(a.aiClients) != 1 || a.aiClients[0].count != 2 {
		t.Errorf("after one disconnect: count = %d, want 2", a.aiClients[0].count)
	}
	clients[1].Close()
	clients[2].Close()
	runLoopTask(t, a)
	runLoopTask(t, a)
	if len(a.aiClients) != 0 {
		t.Errorf("still listed after all disconnect: %+v", a.aiClients)
	}
}
