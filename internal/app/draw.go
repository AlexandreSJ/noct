package app

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/layout"
)

func (a *App) render() error {
	a.applyMouseMode() // the home follows every motion; leaving it or coming back switches
	a.placeCursor()
	if err := a.screen.Display(uv.DrawableFunc(a.draw)); err != nil {
		return fmt.Errorf("drawing screen: %w", err)
	}
	return nil
}

func (a *App) draw(screen uv.Screen, area uv.Rectangle) {
	at, shot := a.now(), a.takeShot()
	if shot.tab != nil {
		a.seen(shot.focused) // a pane in focus on screen has no activity to tell
	}
	a.startAnimations(a.lastShot, shot, at)
	a.lastShot = shot
	if a.palette.background != nil {
		fillRect(screen, area, uv.Style{Bg: a.palette.background})
	}
	a.drawTabBar(screen)
	if a.showsHome() {
		a.drawHome(screen)
	} else {
		a.drawPanes(screen, a.currentTab())
	}
	a.drawAnimations(screen, at)
	a.drawOverlay(screen, at)
	a.recordFrame(screen)
}

// drawModeOverlay shows the panel of the current mode on top of everything else.
func (a *App) drawModeOverlay(screen uv.Screen) {
	switch a.mode {
	case commandMode:
		a.drawShortcutsPopup(screen, "commands", "commands", commandShortcuts, a.shortcutNote())
	case moveMode:
		a.drawShortcutsPopup(screen, "move", "move", moveShortcuts, "")
	case resizeMode:
		a.drawShortcutsPopup(screen, "resize", "resize", resizeShortcuts, "")
	case choiceMode:
		a.drawChoice(screen)
	case promptMode:
		a.drawPrompt(screen)
	case configMode:
		a.drawConfigPanel(screen)
	case confirmMode:
		a.drawShortcutsPopup(screen, a.confirm.question, "confirm", a.confirmShortcuts(), "")
	case formMode:
		a.drawForm(screen)
	case scriptFormMode:
		a.drawScriptForm(screen)
	case finderMode:
		a.drawFinder(screen)
	case infoMode:
		a.drawInfo(screen)
	case mcpMode:
		a.drawMCP(screen)
	case sessionsMode:
		a.drawSessions(screen)
	case updatesMode:
		a.drawUpdates(screen)
	case colorsMode:
		a.drawColors(screen)
	case templateSaveMode:
		a.drawTemplateSave(screen)
	case templatesMode:
		a.drawTemplates(screen)
	case setupMode:
		a.drawSetup(screen)
	}
}

func (a *App) drawPanes(screen uv.Screen, t *tab) {
	for _, placed := range t.visible(a.workspaceArea()) {
		id, frame := placed.id, placed.frame
		pal := a.palette.forPane(t.paneColors[id])
		viewScreen := screen
		if pal.background != nil || pal.text != nil {
			viewScreen = themedScreen{Screen: screen, background: pal.background, text: pal.text}
		}
		fillRect(screen, frame, uv.Style{Bg: pal.background}) // floating panes cover what is below
		lineStyle, titleStyle := pal.unfocusedFrame, pal.unfocusedTitle
		if id == t.focused {
			lineStyle, titleStyle = pal.focusedFrame, pal.focusedTitle
		}
		title := a.viewTitle(t, id)
		if !t.fullscreen && t.tree.Folded(id) {
			drawFoldedPane(screen, frame, title, lineStyle, titleStyle)
			continue
		}
		var status []string
		if t.fullscreen {
			status = append(status, "⛶ fullscreen")
		}
		scrolled, isScrollable := t.views[id].(scrollable)
		if id == t.focused && a.mode == scrollMode && isScrollable {
			status = append(status, a.scrollPosition())
			drawFrameStatus(screen, frame, title, strings.Join(status, "  "), lineStyle, titleStyle)
			scrolled.DrawScrolled(viewScreen, contentRect(frame), a.scrollOffset)
			a.highlightSelection(screen, t, id)
			continue
		}
		drawFrameStatus(screen, frame, title, strings.Join(status, "  "), lineStyle, titleStyle)
		t.views[id].Draw(viewScreen, contentRect(frame))
		if command := a.pendingRerun(id); command != "" {
			a.drawRerun(screen, contentRect(frame), command)
		}
		a.highlightSelection(screen, t, id)
	}
}

// themedScreen gives cells that use the terminal's default colors the theme's colors instead.
type themedScreen struct {
	uv.Screen
	background color.Color
	text       color.Color
}

func (s themedScreen) SetCell(x, y int, cell *uv.Cell) {
	if cell != nil && (cell.Style.Bg == nil || cell.Style.Fg == nil) {
		themed := *cell // the cell belongs to the pane's screen and must not change
		if themed.Style.Bg == nil {
			themed.Style.Bg = s.background
		}
		if themed.Style.Fg == nil {
			themed.Style.Fg = s.text
		}
		cell = &themed
	}
	s.Screen.SetCell(x, y, cell)
}

// placeCursor puts the real cursor where the focused program has it, or hides it. Showing or
// hiding is sent only when it changes: a program redrawing a spinner would blink it otherwise.
func (a *App) placeCursor() {
	t := a.currentTab()
	if t == nil || t.isEmpty() || a.mode != normalMode && a.mode != commandMode && a.mode != moveMode {
		a.hideCursor()
		return
	}
	position, visible := t.focusedView().Cursor()
	frame, _ := t.frameOf(t.focused, a.workspaceArea())
	content := contentRect(frame)
	cursor := content.Min.Add(position)
	if !visible || !cursor.In(content) {
		a.hideCursor()
		return
	}
	_ = a.screen.SetCursorPosition(cursor.X, cursor.Y)
	if !a.screen.CursorVisible() {
		_ = a.screen.ShowCursor()
	}
}

func (a *App) hideCursor() {
	if a.screen.CursorVisible() {
		_ = a.screen.HideCursor() // cursor visibility is cosmetic
	}
}

// drawTabBarStatus shows, right-aligned, the latest message: a problem in the theme's attention
// color, a notice in its accent. Modes are not named there: their popups and titles say it.
// After it come noct's version, with a newer one in the accent, and the session's name.
func (a *App) drawTabBarStatus(screen uv.Screen, minX int) {
	text, style := a.statusParts()
	version, news := a.versionTabBarLabel()
	label := a.sessionTabBarLabel()
	x := max(a.size.X-ansi.StringWidth(text+version+news+label), minX)
	x = drawText(screen, x, 0, a.size.X, text, style)
	x = drawText(screen, x, 0, a.size.X, version, a.palette.tab)
	x = drawText(screen, x, 0, a.size.X, news, a.palette.focusedTitle)
	drawText(screen, x, 0, a.size.X, label, a.palette.tab)
}

// versionTabBarLabel is noct's version and, once there is one, the newer release: out, or
// installed and waiting for a restart.
func (a *App) versionTabBarLabel() (version, news string) {
	version = " noct " + a.version + " "
	if a.updates.installed != "" {
		return version, a.updates.installed + " ready: restart "
	}
	if newest, ok := a.newestRelease(); ok {
		return version, "↑ " + newest.Version + " "
	}
	return version, ""
}

// sessionTabBarLabel names the session on screen, once there is more than one to tell apart.
func (a *App) sessionTabBarLabel() string {
	if len(a.sessions) < 2 {
		return ""
	}
	return " ◆ " + a.session.name + " "
}

// statusParts are the message the tab bar shows and its style.
func (a *App) statusParts() (text string, style uv.Style) {
	style = a.palette.status
	switch {
	case a.statusMessage != "":
		text = " " + a.statusMessage + " "
	case a.lockHint:
		text, style = " "+a.unlockKeys()+" unlock ", a.palette.activeTab
	case a.notice != "":
		text, style = " "+a.notice+" ", a.palette.focusedTitle
	case len(a.approvals) > 0:
		text = fmt.Sprintf(" ⚑ %d waiting for approval: %s ", len(a.approvals), a.prefix.String()+" "+a.hint("commands", "{y}"))
	}
	return text, style
}

// tabBarStatus is what drawTabBarStatus shows, for the tab bar to leave room for it.
func (a *App) tabBarStatus() string {
	text, _ := a.statusParts()
	version, news := a.versionTabBarLabel()
	return text + version + news + a.sessionTabBarLabel()
}

func (a *App) tabLabel(t *tab) string {
	if t.name != "" {
		return t.name
	}
	return a.viewTitle(t, t.focused)
}

func (a *App) viewTitle(t *tab, id layout.PaneID) string {
	if t.isEmpty() {
		return "noct"
	}
	if name := t.paneNames[id]; name != "" {
		return name
	}
	if title := t.views[id].Title(); title != "" {
		return title
	}
	return filepath.Base(a.shellArgv[0])
}

// drawFoldedPane draws a pane folded in a stack as its title on a line.
func drawFoldedPane(screen uv.Screen, frame image.Rectangle, title string, lineStyle, titleStyle uv.Style) {
	if frame.Empty() {
		return
	}
	for x := frame.Min.X; x < frame.Max.X; x++ {
		setRune(screen, x, frame.Min.Y, "─", lineStyle)
	}
	drawText(screen, frame.Min.X+2, frame.Min.Y, frame.Max.X-1, " ▸ "+title+" ", titleStyle)
}

func drawFrame(screen uv.Screen, frame image.Rectangle, title string, lineStyle, titleStyle uv.Style) {
	drawFrameStatus(screen, frame, title, "", lineStyle, titleStyle)
}

// drawFrameStatus draws a frame with a status, such as the scroll position, at the right of its
// top line.
func drawFrameStatus(screen uv.Screen, frame image.Rectangle, title, status string, lineStyle, titleStyle uv.Style) {
	if frame.Dx() < 2 || frame.Dy() < 2 {
		return
	}
	right, bottom := frame.Max.X-1, frame.Max.Y-1
	drawTitleLine(screen, frame.Min.X+1, right, frame.Min.Y, title, status, lineStyle, titleStyle)
	for x := frame.Min.X + 1; x < right; x++ {
		setRune(screen, x, bottom, "─", lineStyle)
	}
	for y := frame.Min.Y + 1; y < bottom; y++ {
		setRune(screen, frame.Min.X, y, "│", lineStyle)
		setRune(screen, right, y, "│", lineStyle)
	}
	setRune(screen, frame.Min.X, frame.Min.Y, "┌", lineStyle)
	setRune(screen, right, frame.Min.Y, "┐", lineStyle)
	setRune(screen, frame.Min.X, bottom, "└", lineStyle)
	setRune(screen, right, bottom, "┘", lineStyle)
}

// drawTitleLine draws a frame's line from x up to maxX, with the title near its start and the
// status near its end. The title gives way to the status, cut with "…".
func drawTitleLine(screen uv.Screen, x, maxX, y int, title, status string, lineStyle, titleStyle uv.Style) {
	for column := x; column < maxX; column++ {
		setRune(screen, column, y, "─", lineStyle)
	}
	titleEnd := maxX - 1
	if status != "" {
		status = " " + status + " "
		start := max(maxX-1-ansi.StringWidth(status), x+1)
		drawText(screen, start, y, maxX-1, status, titleStyle)
		titleEnd = start - 1
	}
	if room := titleEnd - x - 3; title != "" && room > 0 {
		drawText(screen, x+1, y, titleEnd, " "+ansi.Truncate(title, room, "…")+" ", titleStyle)
	}
}

func fillRect(screen uv.Screen, area image.Rectangle, style uv.Style) {
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			setRune(screen, x, y, " ", style)
		}
	}
}

func setRune(screen uv.Screen, x, y int, content string, style uv.Style) {
	screen.SetCell(x, y, &uv.Cell{Content: content, Width: 1, Style: style})
}

// drawText writes text from column x up to maxX, cutting it at a grapheme boundary.
// It returns the column right after the last cell written.
func drawText(screen uv.Screen, x, y, maxX int, text string, style uv.Style) int {
	for text != "" {
		grapheme, width := ansi.FirstGraphemeCluster(text, ansi.GraphemeWidth)
		text = text[len(grapheme):]
		if width == 0 {
			continue
		}
		if x+width > maxX {
			break
		}
		screen.SetCell(x, y, &uv.Cell{Content: grapheme, Width: width, Style: style})
		x += width
	}
	return x
}
