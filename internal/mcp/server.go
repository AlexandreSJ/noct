// Package mcp is noct's MCP server: it lets an AI list, read, draft, run and follow scripts and
// flows, and see and use the workspace's panes, within the user's policy (see docs/mcp.md).
package mcp

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
)

// protocolVersions are the MCP versions the server speaks, the newest first.
var protocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// maxMessage bounds one message, so a runaway client cannot make the server hold gigabytes.
const maxMessage = 16 << 20

// JSON-RPC error codes the server answers with.
const (
	parseError     = -32700
	methodNotFound = -32601
	invalidParams  = -32602
)

// message is a JSON-RPC request, notification or response.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server answers an MCP client over a pair of streams, a JSON message per line.
type Server struct {
	Name         string
	Version      string
	Instructions string
	Tools        []Tool
	// Connected, when set, learns the client that initialized the session; gone closes when
	// Serve returns.
	Connected func(name, version string, gone <-chan struct{})

	out       io.Writer
	gone      chan struct{}
	greeted   sync.Once
	writeLock sync.Mutex
	calls     sync.WaitGroup
}

// Serve reads messages until in ends. Tool calls run at once, each on its own, so a call that
// waits (for the user's approval, or for a run to end) does not hold the others.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	s.out = out
	s.gone = make(chan struct{})
	defer close(s.gone)
	defer s.calls.Wait()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64<<10), maxMessage)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m message
		if err := json.Unmarshal(line, &m); err != nil {
			s.send(message{ID: json.RawMessage("null"), Error: &rpcError{parseError, err.Error()}})
			continue
		}
		s.handle(m)
	}
	return scanner.Err()
}

func (s *Server) handle(m message) {
	if len(m.ID) == 0 {
		return // a notification, such as notifications/initialized: nothing to answer
	}
	switch m.Method {
	case "initialize":
		s.reply(m, s.initialize(m.Params), nil)
	case "ping":
		s.reply(m, map[string]any{}, nil)
	case "tools/list":
		s.reply(m, map[string]any{"tools": s.toolList()}, nil)
	case "tools/call":
		s.calls.Add(1)
		go func() {
			defer s.calls.Done()
			result, err := s.callTool(m.Params)
			s.reply(m, result, err)
		}()
	default:
		s.reply(m, nil, &rpcError{methodNotFound, "noct does not serve " + m.Method})
	}
}

func (s *Server) initialize(params json.RawMessage) map[string]any {
	var asked struct {
		ProtocolVersion string `json:"protocolVersion"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}
	_ = json.Unmarshal(params, &asked) // without a version asked, the newest is offered
	if s.Connected != nil {
		name := cmp.Or(asked.ClientInfo.Name, "an AI client")
		s.greeted.Do(func() { go s.Connected(name, asked.ClientInfo.Version, s.gone) })
	}
	version := protocolVersions[0]
	if slices.Contains(protocolVersions, asked.ProtocolVersion) {
		version = asked.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		"instructions":    s.Instructions,
	}
}

func (s *Server) toolList() []map[string]any {
	var list []map[string]any
	for _, t := range s.Tools {
		list = append(list, map[string]any{
			"name":        t.Name,
			"title":       t.Title,
			"description": t.Description,
			"inputSchema": t.schema(),
			"annotations": map[string]any{
				"title":           t.Title,
				"readOnlyHint":    t.ReadOnly,
				"destructiveHint": t.Destructive,
				"openWorldHint":   false,
			},
		})
	}
	return list
}

// callTool runs a tool. A tool's own failure is a result the AI reads (isError), not a protocol
// error, so it can correct itself; only an unknown tool or unreadable arguments are errors.
func (s *Server) callTool(params json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &rpcError{invalidParams, err.Error()}
	}
	i := slices.IndexFunc(s.Tools, func(t Tool) bool { return t.Name == call.Name })
	if i < 0 {
		return nil, &rpcError{invalidParams, fmt.Sprintf("noct has no tool %q", call.Name)}
	}
	if len(call.Arguments) == 0 {
		call.Arguments = json.RawMessage("{}")
	}
	result, err := s.Tools[i].Call(call.Arguments)
	return toolResult(result, err), nil
}

// toolResult puts a tool's answer both as JSON text, which every client shows, and as structured
// content for clients that read it.
func toolResult(result any, err error) map[string]any {
	if err != nil {
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		}
	}
	text, marshalErr := json.MarshalIndent(result, "", "  ")
	if marshalErr != nil {
		return toolResult(nil, marshalErr)
	}
	answer := map[string]any{"content": []map[string]any{{"type": "text", "text": string(text)}}}
	var object map[string]any
	if json.Unmarshal(text, &object) == nil {
		answer["structuredContent"] = object
	}
	return answer
}

func (s *Server) reply(request message, result any, err *rpcError) {
	answer := message{ID: request.ID, Result: result, Error: err}
	if err == nil && result == nil {
		answer.Result = map[string]any{}
	}
	s.send(answer)
}

func (s *Server) send(m message) {
	m.JSONRPC = "2.0"
	line, err := json.Marshal(m)
	if err != nil {
		line, _ = json.Marshal(message{JSONRPC: "2.0", ID: m.ID, Error: &rpcError{-32603, err.Error()}}) // plain fields always encode
	}
	s.writeLock.Lock()
	defer s.writeLock.Unlock()
	_, _ = s.out.Write(append(line, '\n')) // a client that closed its input has gone
}

// errDenied is what a tool answers when the policy or the user says no.
var errDenied = errors.New("denied")
