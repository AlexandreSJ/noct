package app

import (
	"cmp"
	"image"
	"maps"
	"slices"
	"strings"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// first is the first of list, or "".
func first(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// The config's default agent is the one an agent step asks with use: default, or with no use. Its
// row opens the agents popup: auto, the agents this machine has, each with the model it runs, and
// a custom agent. enter makes the agent selected the default; m picks a model for one noct knows,
// from the ones its program lists or typed, and noct writes the command around it, so the
// program's flags stay noct's to keep current; e changes an agent's command, and del removes one
// of the config's own or gives one noct knows back its command. The agents noct knows that this
// machine lacks are left out.

// agentsPopup is the agents popup: its selected row, and the model picker over it while open.
type agentsPopup struct {
	// selected is the row: 0 for auto, then the agents shown, then the custom agent.
	selected int
	models   *modelPicker
}

// modelPicker lists the models an agent's program offers, read in the background: listing them
// may take seconds, as opencode looks them up.
type modelPicker struct {
	agent string
	// current is the model the config picks for the agent, "" for its default.
	current string
	// models are the ones its program lists; loading is on until they come.
	models   []string
	loading  bool
	selected int
	first    int
	filter   listFilter
}

const (
	agentsPanelWidth = 72
	agentsTitle      = "default agent"
	// otherModel, last among the models, asks for one typed.
	otherModel = "other model…"
)

// readAgents and listAgentModels are variables so tests can tell what this machine has.
var (
	readAgents      = flow.Agents
	listAgentModels = flow.AgentModels
)

func agentConfigOf(cfg config.Config) flow.AgentConfig {
	return flow.AgentConfig{Own: cfg.Agents, Models: cfg.AgentModels}
}

// agentStatuses are what readAgents tells of each agent, read once while the config is open:
// finding an agent may run a program, as ollama list.
func (a *App) agentStatuses() []flow.AgentStatus {
	if a.configPanel.agents == nil {
		a.configPanel.agents = readAgents(agentConfigOf(a.config))
	}
	return a.configPanel.agents
}

// shownAgents are the agents the popup offers: the config's own, the ones this machine has, and
// the default, with why it cannot be asked when this machine lacks it.
func (a *App) shownAgents() []flow.AgentStatus {
	return slices.DeleteFunc(slices.Clone(a.agentStatuses()), func(s flow.AgentStatus) bool {
		return !s.Own && !s.Installed && s.Name != a.config.Agent
	})
}

// defaultAgent is the agent the config's default stands for now, auto's first one ready, with
// why it cannot be asked when it cannot.
func (a *App) defaultAgent() (flow.AgentStatus, string) {
	statuses := a.agentStatuses()
	if a.config.Agent == flow.AutoAgent {
		for _, s := range statuses {
			if s.Problem == nil {
				return s, ""
			}
		}
		return flow.AgentStatus{}, "no agent ready: see noct agents"
	}
	for _, s := range statuses {
		if s.Name == a.config.Agent {
			if s.Problem != nil {
				return s, s.Problem.Error()
			}
			return s, ""
		}
	}
	return flow.AgentStatus{}, "there is no agent " + a.config.Agent + ": pick another"
}

// agentLabel is an agent with the model it runs, as ollama · qwen3.5:4b.
func agentLabel(s flow.AgentStatus) string {
	if s.Model == "" {
		return s.Name
	}
	return s.Name + " · " + s.Model
}

// agentValue is what the config's row shows: the default agent and its model.
func (a *App) agentValue() string {
	s, problem := a.defaultAgent()
	switch {
	case a.config.Agent == flow.AutoAgent && problem == "":
		return "auto: " + agentLabel(s)
	case problem != "":
		return a.config.Agent
	}
	return agentLabel(s)
}

// agentRowLines are the default agent's row and, when it cannot be asked, why.
func (a *App) agentRowLines() []configLine {
	lines := []configLine{{label: "default agent", value: a.agentValue(), row: agentRow}}
	if _, problem := a.defaultAgent(); problem != "" {
		lines = append(lines, configLine{label: "✗ " + problem, note: true, warning: true})
	}
	return lines
}

// openAgents shows the agents popup on the default agent.
func (a *App) openAgents() {
	a.configPanel.agents = nil // read them anew: one may have been installed since
	a.agentsPopup = agentsPopup{}
	for i, s := range a.shownAgents() {
		if s.Name == a.config.Agent {
			a.agentsPopup.selected = i + 1
		}
	}
	a.mode = agentsMode
}

// backToAgents shows the popup again after a prompt, reading the agents anew.
func (a *App) backToAgents() {
	a.configPanel.agents = nil
	a.mode = agentsMode
	a.agentsPopup.selected = min(a.agentsPopup.selected, len(a.shownAgents())+1)
}

// selectedAgent is the agent on the row selected; false on auto and on the custom agent.
func (a *App) selectedAgent() (flow.AgentStatus, bool) {
	agents := a.shownAgents()
	if i := a.agentsPopup.selected - 1; i >= 0 && i < len(agents) {
		return agents[i], true
	}
	return flow.AgentStatus{}, false
}

func (a *App) handleAgentsKey(key uv.KeyPressEvent) {
	if a.agentsPopup.models != nil {
		a.handleModelKey(key)
		return
	}
	p := &a.agentsPopup
	rows := len(a.shownAgents()) + 2
	name := key.String()
	if selected, moved := listMove(name, p.selected, rows, rows); moved {
		p.selected = selected
		return
	}
	agent, onAgent := a.selectedAgent()
	switch {
	case name == "esc" || name == "q":
		a.mode = configMode
	case name == "enter" || name == "space":
		a.pickAgentRow()
	case name == "m" && onAgent && flow.IsPreset(agent.Name):
		a.openModelPicker(agent.Name)
	case name == "e" && onAgent:
		a.editAgentCommand(agent)
	case (name == "delete" || name == "backspace") && onAgent && agent.Own:
		a.removeAgent(agent.Name)
	}
}

// pickAgentRow makes the agent selected the default and goes back to the config; on the custom
// agent, it asks for one first.
func (a *App) pickAgentRow() {
	if a.agentsPopup.selected == len(a.shownAgents())+1 {
		a.askCustomAgent(func(name string) {
			a.setDefaultAgent(name)
		}, a.backToAgents)
		return
	}
	name := flow.AutoAgent
	if agent, ok := a.selectedAgent(); ok {
		name = agent.Name
	}
	a.setDefaultAgent(name)
}

func (a *App) setDefaultAgent(name string) {
	cfg := a.config
	cfg.Agent = name
	a.applyConfig(cfg)
	a.configPanel.agents = nil
	a.mode = configMode
}

// editAgentCommand asks for an agent's command, starting from the one it runs now.
func (a *App) editAgentCommand(agent flow.AgentStatus) {
	a.askAgentCommand(agent.Name, agent.Command, func(string) { a.backToAgents() }, a.backToAgents)
}

// removeAgent removes an agent of the config's own after asking; the default, when it was this
// one, goes back to auto. One that has a name noct knows gets noct's command back.
func (a *App) removeAgent(name string) {
	question := "remove the agent " + name + "?"
	if flow.IsPreset(name) {
		question = "reset " + name + "'s command?"
	}
	a.askToConfirm(question, func() {
		cfg := a.config
		cfg.Agents = maps.Clone(cfg.Agents)
		delete(cfg.Agents, name)
		if cfg.Agent == name && !flow.IsPreset(name) {
			cfg.Agent = flow.AutoAgent
		}
		a.applyConfig(cfg)
		a.backToAgents()
	})
	a.confirm.cancel = a.backToAgents
}

// openModelPicker lists the models of the agent called name, read in the background.
func (a *App) openModelPicker(name string) {
	m := &modelPicker{agent: name, current: a.config.AgentModels[name], loading: true}
	a.agentsPopup.models = m
	list := listAgentModels // read here: a test may put it back while the lister runs
	go func() {
		models := list(name)
		a.onLoop(func() {
			m.models, m.loading = models, false
			m.selected = max(slices.Index(m.shown(), m.current), 0)
		})
	}()
}

// shown are the picker's rows: the agent's default model, the models the filter keeps, the one
// the config picks when its program does not list it, and otherModel.
func (m *modelPicker) shown() []string {
	rows := []string{""}
	models := m.models
	if m.current != "" && !slices.Contains(models, m.current) {
		models = append([]string{m.current}, models...)
	}
	for _, model := range models {
		if m.filter.keeps(model) {
			rows = append(rows, model)
		}
	}
	return append(rows, otherModel)
}

func (a *App) handleModelKey(key uv.KeyPressEvent) {
	m := a.agentsPopup.models
	if m.filter.typing {
		if used, changed := m.filter.handleKey(key); used {
			if changed {
				m.selected, m.first = min(1, len(m.shown())-1), 0
			}
			return
		}
	}
	name := key.String()
	if m.loading {
		if name == "esc" || name == "q" {
			a.agentsPopup.models = nil
		}
		return // nothing to pick yet: enter would pick the default model
	}
	rows := m.shown()
	if selected, moved := listMove(name, m.selected, len(rows), a.modelRowsHeight()); moved {
		m.selected = selected
		return
	}
	switch {
	case name == "esc" && m.filter.on():
		m.filter.clear()
	case name == "esc" || name == "q":
		a.agentsPopup.models = nil
	case isFilterKey(name):
		m.filter.start()
	case name == "enter" || name == "space":
		a.pickModelRow(m, rows[min(m.selected, len(rows)-1)])
	}
}

// pickModelRow saves the model picked for the agent, or asks for one typed.
func (a *App) pickModelRow(m *modelPicker, model string) {
	if model != otherModel {
		a.setAgentModel(m.agent, model)
		return
	}
	hints := "{enter} save  esc cancel"
	if example := cmp.Or(flow.PresetModel(m.agent), first(m.models)); example != "" {
		hints += "  such as " + example
	}
	a.openPrompt("model of "+m.agent, m.current, hints, func(typed string) {
		if err := flow.ValidModel(typed); typed != "" && err != nil {
			a.statusMessage = err.Error()
		} else {
			a.setAgentModel(m.agent, typed)
		}
		a.backToAgents()
	})
	a.prompt.cancel = a.backToAgents
}

// setAgentModel saves the model the agent runs, "" for its default, and gives it noct's command
// back if the config had its own: the model is all noct's command needs.
func (a *App) setAgentModel(name, model string) {
	cfg := a.config
	cfg.AgentModels = maps.Clone(cfg.AgentModels)
	if cfg.AgentModels == nil {
		cfg.AgentModels = map[string]string{}
	}
	if model == "" {
		delete(cfg.AgentModels, name)
	} else {
		cfg.AgentModels[name] = model
	}
	if _, own := cfg.Agents[name]; own {
		cfg.Agents = maps.Clone(cfg.Agents)
		delete(cfg.Agents, name)
	}
	a.applyConfig(cfg)
	a.agentsPopup.models = nil
	a.backToAgents()
}

// agentsLine is one line of the popup; row is the row it selects, or -1.
type agentsLine struct {
	label, value string
	style        uv.Style
	row          int
}

// agentsLayout is the popup's frame and lines, and its hints.
func (a *App) agentsLayout() (image.Rectangle, []agentsLine, string) {
	width := min(agentsPanelWidth, a.size.X)
	textWidth := max(width-6, 10)
	var lines []agentsLine
	var hints string
	if m := a.agentsPopup.models; m != nil {
		lines, hints = a.modelLines(m)
	} else {
		lines, hints = a.agentLines(textWidth)
	}
	more := moreHintRows(hints, width)
	height := min(panelHeight(len(lines)+2+more), a.size.Y)
	return centeredRect(image.Rect(0, 0, a.size.X, a.size.Y), width, height), lines, hints
}

// agentLines are auto, the agents and the custom agent, then the command the one selected runs,
// or why it cannot run.
func (a *App) agentLines(textWidth int) ([]agentsLine, string) {
	pal := &a.palette
	style := func(row int) uv.Style {
		if row == a.agentsPopup.selected {
			return pal.selectedRow
		}
		return pal.panelText
	}
	mark := func(name string) string {
		if name == a.config.Agent {
			return "● " + name
		}
		return "  " + name
	}
	var auto flow.AgentStatus
	for _, s := range a.agentStatuses() {
		if s.Problem == nil {
			auto = s
			break
		}
	}
	autoValue := "✗ none ready"
	if auto.Name != "" {
		autoValue = agentLabel(auto)
	}
	lines := []agentsLine{{label: mark(flow.AutoAgent), value: autoValue, style: style(0), row: 0}}
	agents := a.shownAgents()
	for i, s := range agents {
		value := s.Model
		if s.Problem != nil {
			value = strings.TrimSpace("✗ " + s.Model)
		}
		lines = append(lines, agentsLine{label: mark(s.Name), value: value, style: style(i + 1), row: i + 1})
	}
	custom := len(agents) + 1
	lines = append(lines, agentsLine{label: "  + custom agent", style: style(custom), row: custom}, agentsLine{row: -1})

	hints := "{enter} pick  esc back"
	about, aboutStyle := "", pal.panelMuted
	agent, onAgent := a.selectedAgent()
	switch {
	case a.agentsPopup.selected == 0:
		about = auto.Command
	case a.agentsPopup.selected == custom:
		hints = "{enter} add  esc back"
	case agent.Problem != nil:
		about, aboutStyle = "✗ "+agent.Problem.Error(), pal.status
	default:
		about = agent.Command
	}
	switch {
	case onAgent && agent.Own && flow.IsPreset(agent.Name):
		hints = "{enter} pick  m model  e command  del reset  esc back"
	case onAgent && agent.Own:
		hints = "{enter} pick  e command  del remove  esc back"
	case onAgent:
		hints = "{enter} pick  m model  e command  esc back"
	}
	if about != "" {
		for _, line := range wrapText(about, textWidth) {
			lines = append(lines, agentsLine{label: line, style: aboutStyle, row: -1})
		}
	}
	return lines, a.hint("agents", hints)
}

// modelRowsHeight is how many models the picker shows at once: what the screen holds below the
// frame, the filter and the hints.
func (a *App) modelRowsHeight() int {
	rows := a.size.Y - 8
	if m := a.agentsPopup.models; m != nil && m.filter.on() {
		rows -= 2
	}
	return max(rows, 3)
}

// modelLines are the filter and the models the picker shows, scrolled to the one selected.
func (a *App) modelLines(m *modelPicker) ([]agentsLine, string) {
	pal := &a.palette
	hints := "{enter} pick  " + filterHint + "  esc back"
	var lines []agentsLine
	if m.filter.on() {
		lines = append(lines, agentsLine{label: "filter", row: -2}, agentsLine{row: -1})
	}
	if m.loading {
		return append(lines, agentsLine{label: "loading…", style: pal.panelMuted, row: -1}), a.hint("agents", "esc back")
	}
	rows := m.shown()
	height := min(len(rows), a.modelRowsHeight())
	m.selected = min(m.selected, len(rows)-1)
	m.first = listFirst(m.first, m.selected, len(rows), height)
	current := m.current
	fallback := flow.PresetModel(m.agent)
	if m.agent == "ollama" && len(m.models) > 0 && !slices.ContainsFunc(m.models, func(model string) bool {
		return model == fallback || model == fallback+":latest"
	}) {
		fallback = m.models[0] // ollama runs the first one it has rather than download one
	}
	for i := m.first; i < m.first+height; i++ {
		label, value := rows[i], ""
		if label == "" {
			label, value = "default", fallback
		}
		if rows[i] == current && rows[i] != otherModel {
			label = "● " + label
		} else {
			label = "  " + label
		}
		style := pal.panelText
		if i == m.selected {
			style = pal.selectedRow
		}
		lines = append(lines, agentsLine{label: label, value: value, style: style, row: i})
	}
	if len(m.models) == 0 {
		lines = append(lines, agentsLine{row: -1}, agentsLine{label: "no models listed", style: pal.panelMuted, row: -1})
	}
	return lines, a.hint("agents", hints)
}

func (a *App) drawAgents(screen uv.Screen) {
	pal := &a.palette
	rect, lines, hints := a.agentsLayout()
	title := agentsTitle
	if m := a.agentsPopup.models; m != nil {
		title = "model of " + m.agent
	}
	inner := a.drawPanel(screen, rect, title)
	more := moreHintRows(hints, rect.Dx())
	for i, line := range lines {
		y := inner.Min.Y + i
		if y >= inner.Max.Y-2-more {
			break
		}
		if line.row == -2 {
			m := a.agentsPopup.models
			kept := filterCount(len(m.shown())-2, len(m.models))
			m.filter.draw(screen, inner.Min.X+1, y, inner.Max.X, kept, pal)
			continue
		}
		if line.row >= 0 {
			room := max(inner.Dx()-ansi.StringWidth(line.label)-4, 1)
			drawRow(screen, inner, y, line.label, ansi.Truncate(line.value, room, "…"), line.style)
			continue
		}
		drawText(screen, inner.Min.X+1, y, inner.Max.X, line.label, line.style)
	}
	drawHints(screen, inner, hints, pal.panelMuted)
}

// clickAgents selects the row clicked, and picks it when it was selected already; a click outside
// the popup goes back.
func (a *App) clickAgents(point image.Point) {
	rect, lines, _ := a.agentsLayout()
	if !point.In(rect) {
		if a.agentsPopup.models != nil {
			a.agentsPopup.models = nil
		} else {
			a.mode = configMode
		}
		return
	}
	i := point.Y - rect.Inset(2).Min.Y
	if i < 0 || i >= len(lines) || lines[i].row < 0 {
		return
	}
	row := lines[i].row
	if m := a.agentsPopup.models; m != nil {
		if m.selected == row {
			a.pickModelRow(m, m.shown()[row])
		}
		m.selected = row
		return
	}
	if a.agentsPopup.selected == row {
		a.pickAgentRow()
		return
	}
	a.agentsPopup.selected = row
}
