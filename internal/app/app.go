// Package app is noct's workspace, run by the daemon: the home screen, tabs of panes, noct mode,
// the render loop and the terminal attached to it.
package app

import (
	"errors"
	"fmt"
	"image"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/layout"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/script"
)

// frameInterval caps rendering at about 60 frames per second; output only marks the screen dirty.
const frameInterval = time.Second / 60

// pruneInterval is how often logs past the configured limits are deleted while noct runs.
const pruneInterval = time.Hour

type App struct {
	// client is the attached terminal and screen draws on it; both are nil while detached.
	client *client
	screen *uv.TerminalScreen
	size   image.Point

	tabs      []*tab
	activeTab int
	nextPane  layout.PaneID

	config     config.Config
	configPath string
	prefix     prefixKey
	shortcut   directShortcut
	palette    palette

	mode        mode
	drag        *mouseDrag
	selection   *textSelection
	configPanel configPanel
	prompt      textPrompt
	confirm     confirmation
	choice      choice
	form        paramsForm
	scriptForm  scriptForm
	// formsOpened counts script forms, so an editor exiting late finds the form it belongs to.
	formsOpened   int
	finder        finder
	info          infoPopup
	scrollOffset  int
	statusMessage string
	// notice is news that is not a problem, such as a copy done; it shows in the theme's accent.
	notice   string
	quitting bool

	intro *intro
	// flickers holds when each clicked character of the home's art started flickering.
	flickers map[artCell]time.Time
	// hovered is the cell the pointer was last seen over on the home.
	hovered  image.Point
	homeTick <-chan time.Time

	homeHintShown   bool
	homeHintExpired <-chan time.Time
	homeQuitArmed   bool

	shellArgv []string
	shellEnv  []string
	binary    binaryStamp
	// workDir is the attached terminal's directory: new terminals start there, and it decides
	// which project's scripts are shown.
	workDir string
	// launch starts a program in a new pane; tests replace it to run without processes.
	launch func(p program, size pty.Size) (view, error)

	scripts script.Library
	// scriptsLoaded counts library loads, so panels know when what they cached is stale.
	scriptsLoaded int
	// expanded are the folders open in the scripts panels, by path; folders start closed.
	expanded map[string]bool
	versions *script.Versions
	memory   *script.Memory
	flows    []flow.Flow

	imagesEnabled    bool
	nextImageID      atomic.Uint32
	cellPixels       atomic.Value
	terminalGraphics chan []byte

	connections   chan connection
	screenChanged chan struct{}
	// loopTasks carries work from background goroutines, such as a program exiting, to the loop.
	loopTasks chan func()
	stopped   chan struct{}
	// triggered are the flows a trigger started that are still running.
	triggered map[string]bool
	// approvals are what AIs asked to do that wait for the user, the oldest first.
	approvals []*approval
	// aiClients are the AI clients running noct mcp right now; mcpSelected is the MCP popup's row.
	aiClients   []*aiClient
	mcpSelected int
}

func New() *App {
	a := &App{
		shellArgv:        userShell(),
		shellEnv:         paneEnvironment(os.Environ()),
		terminalGraphics: make(chan []byte),
		connections:      make(chan connection),
		screenChanged:    make(chan struct{}, 1),
		loopTasks:        make(chan func()),
		stopped:          make(chan struct{}),
	}
	a.workDir, _ = os.Getwd() // until a terminal attaches and reports its own
	a.launch = a.startProgram
	a.binary = currentBinary()
	a.versions = script.NewVersions(a.notifyScreenChanged)
	a.loadConfig()
	a.loadMemory()
	a.reloadScripts()
	return a
}

// loadMemory reads the remembered parameter values; without them, parameters start from defaults.
func (a *App) loadMemory() {
	memory, err := script.LoadMemory(filepath.Join(logstore.Dir(), script.MemoryFile))
	if err != nil {
		a.statusMessage = err.Error()
	}
	a.memory = memory
}

// loadConfig reads the config file. Problems are shown in the tab bar and defaults take over.
func (a *App) loadConfig() {
	cfg := config.Default()
	path, err := config.Path()
	if err == nil {
		cfg, err = config.Load(path)
	}
	prefix, prefixErr := prefixFromConfig(cfg.Prefix)
	if prefixErr != nil {
		cfg.Prefix = config.Default().Prefix
		prefix, _ = prefixFromConfig(cfg.Prefix) // the default prefix is always valid
		err = errors.Join(err, prefixErr)
	}
	a.config, a.configPath, a.prefix = cfg, path, prefix
	a.shortcut = shortcutFromConfig(cfg.Shortcut)
	a.palette = newPalette(themeNamed(cfg.Appearance.Theme), cfg.Appearance.Background == config.BackgroundTheme)
	if err != nil {
		a.statusMessage = err.Error()
	}
}

// Serve runs the workspace for the terminals that connect through listener, until the user quits.
// Programs keep running while no terminal is attached.
func (a *App) Serve(listener net.Listener) (err error) {
	defer func() { err = errors.Join(err, a.shutdown()) }()
	go a.acceptClients(listener)
	go a.watchTriggers()
	defer listener.Close()
	return a.loop()
}

// loop reacts to input, pane output, pane exits, connections and timers, and redraws at most once
// per frame so heavy output cannot starve keyboard input.
func (a *App) loop() error {
	var nextFrame <-chan time.Time
	var lastFrame time.Time
	a.pruneLogs()
	prune := time.NewTicker(pruneInterval)
	defer prune.Stop()
	for !a.quitting {
		select {
		case event := <-a.clientEvents():
			if err := a.handleEvent(event); err != nil {
				return err
			}
		case c := <-a.connections:
			a.handleConnection(c)
		case commands := <-a.terminalGraphics:
			if a.screen != nil {
				_, _ = a.screen.WriteString(string(commands)) // only queues; the next frame's flush reports failures
			}
		case <-a.screenChanged:
		case task := <-a.loopTasks:
			task()
		case <-prune.C:
			a.pruneLogs()
		case <-a.homeTick:
			a.advanceHomeArt()
		case <-a.homeHintExpired:
			a.homeHintShown, a.homeHintExpired, a.homeQuitArmed = false, nil, false
		case <-nextFrame:
			nextFrame = nil
		}
		if a.quitting || nextFrame != nil || a.client == nil {
			continue
		}
		if wait := frameInterval - time.Since(lastFrame); wait > 0 {
			nextFrame = time.After(wait)
			continue
		}
		if err := a.render(); err != nil {
			a.dropClient() // a client that cannot be drawn on is gone
			continue
		}
		lastFrame = time.Now()
	}
	return nil
}

// clientEvents is nil while detached, which makes the loop ignore it.
func (a *App) clientEvents() <-chan uv.Event {
	if a.client == nil {
		return nil
	}
	return a.client.events
}

func (a *App) handleConnection(c connection) {
	switch c.hello.Purpose {
	case ipc.Quit:
		a.quitting = true
		_ = c.conn.SendJSON(ipc.KindExit, ipc.Exit{Reason: "quit"}) // best effort: the daemon stops either way
		_ = c.conn.Close()
	case ipc.OpenPane:
		a.openRequestedPane(c)
	case ipc.Ask:
		a.answerRequest(c)
	default:
		a.attach(c)
	}
}

// attach makes c the workspace's terminal. One terminal is attached at a time: a new one takes
// over from the previous.
func (a *App) attach(c connection) {
	if a.client != nil {
		a.detachClient("attached from another terminal")
	}
	a.client = newClient(c)
	a.screen = a.client.screen
	a.imagesEnabled = outerShowsPlaceholders(c.hello.Env)
	if c.hello.Dir != "" && c.hello.Dir != a.workDir {
		a.workDir = c.hello.Dir
		a.reloadScripts()
	}
	if err := a.client.start(); err != nil {
		a.dropClient() // the client failed, not the workspace
		return
	}
	a.applyMouseMode()
	if a.currentTab() == nil {
		a.startIntro()
	}
	if a.binary.rebuilt() {
		a.notice = rebuiltMessage
	}
}

// detachClient hands the terminal back to its shell; the workspace keeps running.
func (a *App) detachClient(reason string) {
	a.client.end(reason)
	a.client, a.screen = nil, nil
	a.mode = normalMode
}

// dropClient forgets a client that disconnected on its own.
func (a *App) dropClient() {
	a.client.close()
	a.client, a.screen = nil, nil
}

func (a *App) handleEvent(event uv.Event) error {
	switch event := event.(type) {
	case uv.CellSizeEvent:
		a.cellPixels.Store(image.Pt(event.Width, event.Height))
	case uv.WindowSizeEvent:
		return a.resize(image.Pt(event.Width, event.Height))
	case uv.KeyPressEvent:
		a.handleKey(event)
	case clientGone:
		if event.client == a.client {
			a.dropClient()
		}
	case uv.MouseEvent:
		a.handleMouse(event)
	case uv.PasteEvent:
		if t := a.currentTab(); t != nil && !t.isEmpty() && a.mode == normalMode {
			t.focusedView().Paste(event.Content)
		}
	}
	return nil
}

func (a *App) resize(size image.Point) error {
	a.size = size
	if a.screen == nil {
		return nil
	}
	if err := a.screen.Resize(size.X, size.Y); err != nil {
		return fmt.Errorf("resizing screen: %w", err)
	}
	for _, t := range a.tabs {
		t.resizeViews(a.workspaceArea())
	}
	return nil
}

// workspaceArea is the screen below the tab bar.
func (a *App) workspaceArea() image.Rectangle {
	return image.Rect(0, 1, a.size.X, a.size.Y)
}

func (a *App) currentTab() *tab {
	if len(a.tabs) == 0 {
		return nil
	}
	return a.tabs[a.activeTab]
}

// notifyScreenChanged is called by panes for every chunk of output; a pending notice is enough.
func (a *App) notifyScreenChanged() {
	select {
	case a.screenChanged <- struct{}{}:
	default:
	}
}

// onLoop runs task on the event loop, where the workspace may be changed. Tasks sent after the
// workspace stopped are dropped.
func (a *App) onLoop(task func()) {
	select {
	case a.loopTasks <- task:
	case <-a.stopped:
	}
}

func (a *App) shutdown() error {
	close(a.stopped)
	for _, t := range a.tabs {
		for _, v := range t.views {
			v.Close()
		}
	}
	if a.client != nil {
		a.detachClient("quit")
	}
	return nil
}
