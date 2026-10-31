package app

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// The MCP popup (prefix+y) shows, in this order, the actions AIs wait for the user to decide on,
// the AI clients connected to noct right now, and how to add noct to each AI client it knows.
// Its rows are the approvals and then the setups; the selected row decides what the keys do.

const (
	approvalHints = "{y} approve  {n} deny  {up}{down} choose  esc close"
	setupHints    = "{enter} copy  {up}{down} choose  esc close"
	mcpPanelWidth = 76
)

// approval is an action an AI asked for that the policy wants the user to decide on. It waits,
// holding the MCP call, until the user decides or the timeout denies it.
type approval struct {
	tool    string
	summary string
	decide  func(approved bool)
	timer   *time.Timer
}

type approvalArgs struct {
	Tool    string `json:"tool"`
	Summary string `json:"summary"`
}

// aiClient is an AI client running noct mcp, connected while its request stays open.
// count tracks how many connections this client has (when it reconnects or opens multiple sessions).
type aiClient struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	count   int
}

// queueApproval waits for the user to approve an action, telling them how to see it.
func (a *App) queueApproval(args approvalArgs, decide func(approved bool)) {
	ap := &approval{tool: args.Tool, summary: args.Summary, decide: decide}
	timeout := time.Duration(a.config.MCP.ApprovalSeconds) * time.Second
	ap.timer = time.AfterFunc(timeout, func() {
		a.onLoop(func() { a.decideApproval(ap, false) })
	})
	a.approvals = append(a.approvals, ap)
	a.notice = fmt.Sprintf("AI asks to %s: prefix+y to review", args.Tool)
	a.notifyScreenChanged()
}

// decideApproval answers an approval once; a timeout after the user decided does nothing.
func (a *App) decideApproval(ap *approval, approved bool) {
	i := slices.Index(a.approvals, ap)
	if i < 0 {
		return
	}
	a.approvals = slices.Delete(a.approvals, i, i+1)
	ap.timer.Stop()
	ap.decide(approved)
	if a.mcpSelected > i {
		a.mcpSelected--
	}
}

// connectAIClient lists an AI client as connected until its noct mcp closes the connection, which
// it keeps open for as long as it runs. Multiple connections from the same client (name+version)
// are counted but shown as a single entry.
func (a *App) connectAIClient(c connection, client *aiClient) {
	for _, existing := range a.aiClients {
		if existing.Name == client.Name && existing.Version == client.Version {
			existing.count++
			go func() {
				for {
					if _, err := c.conn.Receive(); err != nil {
						break
					}
				}
				_ = c.conn.Close()
				a.onLoop(func() {
					existing.count--
					if existing.count <= 0 {
						a.aiClients = slices.DeleteFunc(a.aiClients, func(other *aiClient) bool { return other == existing })
					}
				})
			}()
			return
		}
	}
	client.count = 1
	a.aiClients = append(a.aiClients, client)
	a.notice = client.Name + " connected to noct"
	go func() {
		for {
			if _, err := c.conn.Receive(); err != nil {
				break
			}
		}
		_ = c.conn.Close()
		a.onLoop(func() {
			client.count--
			if client.count <= 0 {
				a.aiClients = slices.DeleteFunc(a.aiClients, func(other *aiClient) bool { return other == client })
			}
		})
	}()
}

// openMCP shows the MCP popup, on the oldest approval when one waits.
func (a *App) openMCP() {
	a.mcpSelected, a.mode = 0, mcpMode
}

func (a *App) handleMCPKey(key uv.KeyPressEvent) {
	setups := mcpSetups(a.noctBinary())
	rows := len(a.approvals) + len(setups)
	a.mcpSelected = min(a.mcpSelected, rows-1)
	if a.moveMCPSelection(key.String(), rows) {
		return
	}
	if a.mcpSelected < len(a.approvals) {
		a.decideSelectedApproval(key.String())
		return
	}
	if key.String() == "enter" || key.String() == "c" {
		a.copyMCPSetup(setups[a.mcpSelected-len(a.approvals)])
	}
}

// moveMCPSelection closes the popup or moves its selection, telling whether the key did either.
func (a *App) moveMCPSelection(key string, rows int) bool {
	switch key {
	case "esc", "q":
		a.mode = normalMode
	case "up", "k":
		a.mcpSelected = max(a.mcpSelected-1, 0)
	case "down", "j":
		a.mcpSelected = min(a.mcpSelected+1, rows-1)
	case "pgup", "pgdown", "home", "end":
		a.mcpSelected, _ = listJump(key, a.mcpSelected, rows, rows)
	default:
		return false
	}
	return true
}

func (a *App) decideSelectedApproval(key string) {
	selected := a.approvals[a.mcpSelected]
	switch key {
	case "y", "enter":
		a.decideApproval(selected, true)
	case "n", "delete":
		a.decideApproval(selected, false)
	}
}

func (a *App) copyMCPSetup(s mcpSetup) {
	a.copyToClipboard(s.text)
	a.notice = "copied: add noct to " + s.client
	if s.file != "" {
		a.notice = "copied: " + s.client + "'s config"
	}
	a.mode = normalMode
}

// noctBinary is the path of this noct, for commands that start it.
func (a *App) noctBinary() string {
	return orDefault(a.binary.path, "noct")
}

func (a *App) drawMCP(screen uv.Screen) {
	pal := &a.palette
	width := min(mcpPanelWidth, a.size.X)
	textWidth := max(width-6, 10)
	setups := mcpSetups(a.noctBinary())
	lines := a.mcpApprovalLines(textWidth)
	lines = append(lines, a.mcpConnectedLines()...)
	lines = append(lines, a.mcpSetupLines(setups)...)
	hints := a.hint("mcp", approvalHints)
	if i := a.mcpSelected - len(a.approvals); i >= 0 && i < len(setups) {
		hints = a.hint("mcp", setupHints)
		lines = append(lines, a.mcpSetupDetailLines(setups[i], textWidth)...)
	}
	area := image.Rect(0, 0, a.size.X, a.size.Y)
	inner := a.drawPanel(screen, centeredRect(area, width, panelHeight(len(lines)+2)), "MCP")
	for i, line := range lines {
		if inner.Min.Y+i >= inner.Max.Y-2 {
			break
		}
		drawText(screen, inner.Min.X+1, inner.Min.Y+i, inner.Max.X, line.text, line.style)
	}
	drawText(screen, inner.Min.X+1, inner.Max.Y-1, inner.Max.X, hints, pal.panelMuted)
}

func (a *App) mcpRowStyle(row int) uv.Style {
	if row == a.mcpSelected {
		return a.palette.selectedRow
	}
	return a.palette.panelText
}

// mcpApprovalLines are the approvals waiting, each with its summary; none when nothing waits.
func (a *App) mcpApprovalLines(textWidth int) []styledLine {
	if len(a.approvals) == 0 {
		return nil
	}
	pal := &a.palette
	lines := []styledLine{{fmt.Sprintf("Waiting for approval (%d)", len(a.approvals)), pal.panelHeading}}
	for i, ap := range a.approvals {
		lines = append(lines, styledLine{"AI asks to " + ap.tool, a.mcpRowStyle(i)})
		for _, line := range wrapText(ap.summary, textWidth-2) {
			lines = append(lines, styledLine{"  " + line, pal.panelMuted})
		}
	}
	return append(lines, styledLine{"", pal.panelText})
}

func (a *App) mcpConnectedLines() []styledLine {
	pal := &a.palette
	lines := []styledLine{{"Connected", pal.panelHeading}}
	if len(a.aiClients) == 0 {
		lines = append(lines, styledLine{"no AI client yet: add noct to one below", pal.panelMuted})
	}
	for _, c := range a.aiClients {
		lines = append(lines, styledLine{strings.TrimSpace("● " + c.Name + " " + c.Version), pal.panelText})
	}
	return lines
}

func (a *App) mcpSetupLines(setups []mcpSetup) []styledLine {
	pal := &a.palette
	lines := []styledLine{{"", pal.panelText}, {"Add noct to", pal.panelHeading}}
	for i, s := range setups {
		lines = append(lines, styledLine{s.client, a.mcpRowStyle(len(a.approvals) + i)})
	}
	return lines
}

// mcpSetupDetailLines show the selected setup in full: where it goes and what to paste.
func (a *App) mcpSetupDetailLines(s mcpSetup, textWidth int) []styledLine {
	pal := &a.palette
	lines := []styledLine{{"", pal.panelText}}
	if s.file != "" {
		lines = append(lines, styledLine{"in " + s.file + ":", pal.panelMuted})
	}
	for _, line := range wrapText(s.text, textWidth) {
		lines = append(lines, styledLine{line, pal.panelText})
	}
	return lines
}

// wrapText cuts text into lines at most width runes long, at spaces when it can, keeping its own
// line breaks.
func wrapText(text string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		runes := []rune(paragraph)
		for len(runes) > width {
			cut := wrapCut(runes, width)
			lines = append(lines, string(runes[:cut]))
			runes = []rune(strings.TrimLeft(string(runes[cut:]), " "))
		}
		lines = append(lines, string(runes))
	}
	return lines
}

// wrapCut is where a line longer than width breaks: the last space in its second half, or width.
func wrapCut(runes []rune, width int) int {
	for i := width; i > width/2; i-- {
		if runes[i] == ' ' {
			return i
		}
	}
	return width
}
