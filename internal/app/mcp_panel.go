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
	approvalHints = "y approve  n deny  ↑↓ choose  esc close"
	setupHints    = "enter copy  ↑↓ choose  esc close"
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
type aiClient struct {
	Name    string `json:"name"`
	Version string `json:"version"`
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
// it keeps open for as long as it runs.
func (a *App) connectAIClient(c connection, client *aiClient) {
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
			a.aiClients = slices.DeleteFunc(a.aiClients, func(other *aiClient) bool { return other == client })
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
	switch key.String() {
	case "esc", "q":
		a.mode = normalMode
		return
	case "up", "k":
		a.mcpSelected = max(a.mcpSelected-1, 0)
		return
	case "down", "j":
		a.mcpSelected = min(a.mcpSelected+1, rows-1)
		return
	}
	if a.mcpSelected < len(a.approvals) {
		selected := a.approvals[a.mcpSelected]
		switch key.String() {
		case "y", "enter":
			a.decideApproval(selected, true)
		case "n", "delete":
			a.decideApproval(selected, false)
		}
		return
	}
	if s := setups[a.mcpSelected-len(a.approvals)]; key.String() == "enter" || key.String() == "c" {
		a.copyToClipboard(s.text)
		a.notice = "copied: add noct to " + s.client
		if s.file != "" {
			a.notice = "copied: " + s.client + "'s config"
		}
		a.mode = normalMode
	}
}

// noctBinary is the path of this noct, for commands that start it.
func (a *App) noctBinary() string {
	return orDefault(a.binary.path, "noct")
}

// drawMCP draws the MCP popup: approvals, connected clients, setups, and the selected setup's
// command in full.
func (a *App) drawMCP(screen uv.Screen) {
	pal := &a.palette
	width := min(mcpPanelWidth, a.size.X)
	textWidth := max(width-6, 10)
	setups := mcpSetups(a.noctBinary())
	rowStyle := func(row int) uv.Style {
		if row == a.mcpSelected {
			return pal.selectedRow
		}
		return pal.panelText
	}
	var lines []styledLine
	if len(a.approvals) > 0 {
		lines = append(lines, styledLine{fmt.Sprintf("Waiting for approval (%d)", len(a.approvals)), pal.panelHeading})
		for i, ap := range a.approvals {
			lines = append(lines, styledLine{"AI asks to " + ap.tool, rowStyle(i)})
			for _, line := range wrapText(ap.summary, textWidth-2) {
				lines = append(lines, styledLine{"  " + line, pal.panelMuted})
			}
		}
		lines = append(lines, styledLine{"", pal.panelText})
	}
	lines = append(lines, styledLine{"Connected", pal.panelHeading})
	if len(a.aiClients) == 0 {
		lines = append(lines, styledLine{"no AI client yet: add noct to one below", pal.panelMuted})
	}
	for _, c := range a.aiClients {
		lines = append(lines, styledLine{strings.TrimSpace("● " + c.Name + " " + c.Version), pal.panelText})
	}
	lines = append(lines, styledLine{"", pal.panelText}, styledLine{"Add noct to", pal.panelHeading})
	for i, s := range setups {
		lines = append(lines, styledLine{s.client, rowStyle(len(a.approvals) + i)})
	}
	hints := approvalHints
	if i := a.mcpSelected - len(a.approvals); i >= 0 && i < len(setups) {
		hints = setupHints
		lines = append(lines, styledLine{"", pal.panelText})
		if setups[i].file != "" {
			lines = append(lines, styledLine{"in " + setups[i].file + ":", pal.panelMuted})
		}
		for _, line := range wrapText(setups[i].text, textWidth) {
			lines = append(lines, styledLine{line, pal.panelText})
		}
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

// wrapText cuts text into lines at most width runes long, at spaces when it can, keeping its own
// line breaks.
func wrapText(text string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		runes := []rune(paragraph)
		for len(runes) > width {
			cut := width
			for i := width; i > width/2; i-- {
				if runes[i] == ' ' {
					cut = i
					break
				}
			}
			lines = append(lines, string(runes[:cut]))
			runes = []rune(strings.TrimLeft(string(runes[cut:]), " "))
		}
		lines = append(lines, string(runes))
	}
	return lines
}
