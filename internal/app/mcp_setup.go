package app

import (
	"encoding/json"
	"strings"
)

// mcpSetup is how one AI client adds noct as an MCP server: a command to paste in a terminal or,
// for a client without one, the JSON to put in its config file (file).
type mcpSetup struct {
	client string
	file   string
	text   string
}

// mcpSetups are the ways to add noct to the AI clients it knows, for the noct binary at binary.
// The full path lets a client find noct even when its PATH does not have it.
func mcpSetups(binary string) []mcpSetup {
	noct := shellWord(binary)
	vscode, _ := json.Marshal(map[string]any{"name": "noct", "command": binary, "args": []string{"mcp"}})
	opencode, _ := json.Marshal(map[string]any{"mcp": map[string]any{"noct": map[string]any{"type": "local", "command": []string{binary, "mcp"}}}})
	cursor, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"noct": map[string]any{"command": binary, "args": []string{"mcp"}}}})
	return []mcpSetup{
		{client: "Claude Code", text: "claude mcp add --scope user noct -- " + noct + " mcp"},
		{client: "Codex", text: "codex mcp add noct -- " + noct + " mcp"},
		{client: "Gemini CLI", text: "gemini mcp add -s user noct " + noct + " mcp"},
		{client: "VS Code", text: "code --add-mcp " + shellWord(string(vscode))},
		{client: "opencode", file: "~/.config/opencode/opencode.json", text: string(opencode)},
		{client: "Cursor and others", file: "~/.cursor/mcp.json, or the client's MCP config", text: string(cursor)},
	}
}

// shellWord quotes s for a POSIX shell when it needs it.
func shellWord(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+=:,@") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
