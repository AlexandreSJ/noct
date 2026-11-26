// Package pane runs one program in a pseudo-terminal and keeps its screen in a VT emulator.
package pane

import (
	"image"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/AlexandreSJ/noct/third_party/vt"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/kittygfx"
	"github.com/AlexandreSJ/noct/internal/pty"
	"github.com/AlexandreSJ/noct/internal/sections"
)

// inputQueueSize bounds how many input chunks wait for the program to read them: past it input
// is dropped, so it holds a long paste (up to 4 KiB per chunk) or a lot of typing ahead.
const inputQueueSize = 4096

// syncTimeout is how long a synchronized frame may keep the screen waiting, as in Alacritty.
const syncTimeout = 150 * time.Millisecond

type Options struct {
	Argv []string
	Env  []string
	Dir  string
	Size pty.Size
	// OnScreenChange is called from a background goroutine after new output reaches the screen.
	OnScreenChange func()
	// OnExit is called once, from a background goroutine, after the program exits, with its exit
	// code (-1 when a signal ended it).
	OnExit func(exitCode int)
	// Images, when set, shows the program's kitty graphics as images; otherwise they are dropped.
	Images *kittygfx.Session
	// OnTerminalGraphics receives graphics commands for the outer terminal before the placeholders
	// that display them reach the screen. It is called from a background goroutine.
	OnTerminalGraphics func(commands []byte)
	// Log, when set, receives everything the program prints and is closed after it exits.
	Log io.WriteCloser
	// History is what an earlier program printed in this place, shown above what this one prints.
	History []byte
	// KeepScrollback ignores the program erasing the scrollback (ESC[3J, which clear sends), so a
	// log replayed here keeps what was printed before a clear.
	KeepScrollback bool
	// Views shows the output marked by package sections in two views, the clean one first, which
	// ToggleView switches.
	Views bool
	// HistoryUnderFullScreen keeps the history scrollable while a full-screen program shows,
	// with the main screen above that program, so a log replayed here that ends inside one
	// still scrolls.
	HistoryUnderFullScreen bool
}

// Pane is a terminal: a program, its pseudo-terminal and the emulated screen it draws on.
//
// The emulator encodes keys and replies to terminal queries by writing to a blocking pipe,
// which queueScreenInput drains. Every write to that pipe happens under screenLock and only
// while the pane is open, so after Close the reader can stop without leaving a writer stuck.
type Pane struct {
	process *pty.Process

	screenLock sync.RWMutex
	screen     *vt.Emulator
	size       pty.Size
	closed     atomic.Bool
	// historyUnderFullScreen is Options.HistoryUnderFullScreen.
	historyUnderFullScreen bool

	title         atomic.Value
	cursorVisible atomic.Bool
	// mouseModes holds the mouse reporting modes the program turned on, one bit each.
	mouseModes atomic.Uint32

	images             *kittygfx.Session
	graphicsSplitter   kittygfx.Splitter
	onTerminalGraphics func(commands []byte)
	log                io.WriteCloser

	// clean is the clean view's screen, fed what fullFilter and cleanFilter leave of the output
	// to each, when the pane has views; showFull shows the other.
	clean                   *vt.Emulator
	fullFilter, cleanFilter sections.Filter
	showFull                atomic.Bool

	// outputLock keeps output in order between the reader and the synchronized frame timeout.
	outputLock  sync.Mutex
	syncGate    syncGate
	syncTimeout *time.Timer
}

func Start(opts Options) (*Pane, error) {
	process, err := pty.Start(pty.StartOptions{Argv: opts.Argv, Env: opts.Env, Dir: opts.Dir, Size: opts.Size})
	if err != nil {
		return nil, err
	}
	p := &Pane{
		process:            process,
		screen:             vt.NewEmulator(opts.Size.Cols, opts.Size.Rows),
		size:               opts.Size,
		images:             opts.Images,
		onTerminalGraphics: opts.OnTerminalGraphics,
		log:                opts.Log,

		historyUnderFullScreen: opts.HistoryUnderFullScreen,
	}
	p.title.Store("")
	p.cursorVisible.Store(true)
	if opts.Views {
		p.clean = vt.NewEmulator(opts.Size.Cols, opts.Size.Rows)
		p.fullFilter.View, p.cleanFilter.View = sections.FullView, sections.CleanView
		go p.drainClean()
	}
	if len(opts.History) > 0 {
		p.writeScreens(opts.History, func(data []byte) []byte { return renderHistory(data, opts.Size.Cols) }) // text and colors ask nothing back
	}
	// Callbacks run while screenLock is held, so they only touch atomics.
	p.screen.SetCallbacks(vt.Callbacks{
		// Invalid UTF-8 would reach the real terminal as is and throw its cursor off.
		Title:            func(title string) { p.title.Store(strings.ToValidUTF8(title, "�")) },
		CursorVisibility: func(visible bool) { p.cursorVisible.Store(visible) },
		EnableMode:       func(mode ansi.Mode) { p.setMouseMode(mode, true) },
		DisableMode:      func(mode ansi.Mode) { p.setMouseMode(mode, false) },
	})

	if opts.KeepScrollback {
		// handlers registered last run first; the screen itself was erased by ESC[2J already
		p.screen.RegisterCsiHandler('J', func(params ansi.Params) bool {
			n, _, _ := params.Param(0, 0)
			return n == 3
		})
	}

	inputQueue := make(chan []byte, inputQueueSize)
	go p.pumpOutputToScreen(opts.OnScreenChange, opts.OnExit)
	go p.queueScreenInput(inputQueue)
	go p.writeInputToProcess(inputQueue)
	return p, nil
}

func (p *Pane) pumpOutputToScreen(onScreenChange func(), onExit func(exitCode int)) {
	buf := make([]byte, 32*1024)
	for {
		n, err := p.process.Read(buf)
		if n > 0 {
			p.writeLog(buf[:n])
			p.showOutput(buf[:n], onScreenChange)
		}
		if err != nil {
			break
		}
	}
	p.releaseFrame(onScreenChange)
	exitCode := p.process.Wait()
	p.Close()
	if p.log != nil {
		_ = p.log.Close() // nothing is left to write, so a failing close loses nothing
	}
	if onExit != nil {
		onExit(exitCode)
	}
}

// writeLog keeps the output; a log that fails is dropped rather than stopping the program.
func (p *Pane) writeLog(output []byte) {
	if p.log == nil {
		return
	}
	if _, err := p.log.Write(output); err != nil {
		_ = p.log.Close()
		p.log = nil
	}
}

// showOutput puts output on the screen, except a synchronized frame still being drawn: that one
// waits for its end, or for syncTimeout.
func (p *Pane) showOutput(data []byte, onScreenChange func()) {
	p.outputLock.Lock()
	defer p.outputLock.Unlock()
	ready := p.syncGate.feed(data)
	switch {
	case !p.syncGate.pending():
		if p.syncTimeout != nil {
			p.syncTimeout.Stop()
			p.syncTimeout = nil
		}
	case p.syncTimeout == nil:
		p.syncTimeout = time.AfterFunc(syncTimeout, func() { p.releaseFrame(onScreenChange) })
	case len(ready) > 0:
		p.syncTimeout.Reset(syncTimeout) // a frame ended; the one held now gets its own time
	}
	p.screenOutput(ready, onScreenChange)
}

// releaseFrame shows the output held back, when the frame took too long or the program ended.
func (p *Pane) releaseFrame(onScreenChange func()) {
	p.outputLock.Lock()
	defer p.outputLock.Unlock()
	if p.syncTimeout != nil {
		p.syncTimeout.Stop()
		p.syncTimeout = nil
	}
	p.screenOutput(p.syncGate.release(), onScreenChange)
}

func (p *Pane) screenOutput(data []byte, onScreenChange func()) {
	if len(data) == 0 {
		return
	}
	p.handleOutput(data)
	if onScreenChange != nil {
		onScreenChange()
	}
}

// handleOutput feeds text to the emulator and graphics commands to the image session in order,
// so a program asking for the cursor position right after an image sees the cursor moved past it.
func (p *Pane) handleOutput(data []byte) {
	if p.images == nil {
		p.withOpenScreen(func(*vt.Emulator) { p.writeScreens(data, nil) })
		return
	}
	for _, segment := range p.graphicsSplitter.Split(data) {
		if segment.Command != nil {
			p.handleGraphics(*segment.Command)
			continue
		}
		p.withOpenScreen(func(*vt.Emulator) { p.writeScreens(segment.Text, nil) })
	}
}

func (p *Pane) handleGraphics(cmd kittygfx.Command) {
	result := p.images.Handle(cmd)
	if len(result.ToTerminal) > 0 && p.onTerminalGraphics != nil {
		p.onTerminalGraphics(result.ToTerminal)
	}
	p.withOpenScreen(func(screen *vt.Emulator) {
		if result.Place != nil {
			placeImage(screen, *result.Place)
		}
		if len(result.ToProgram) > 0 {
			screen.SendText(string(result.ToProgram))
		}
	})
}

func (p *Pane) queueScreenInput(inputQueue chan<- []byte) {
	defer close(inputQueue)
	buf := make([]byte, 4096)
	for {
		n, err := p.screen.Read(buf)
		if n > 0 {
			select {
			case inputQueue <- append([]byte(nil), buf[:n]...):
			default:
				// The program stopped reading its input. Dropping it keeps the emulator, which
				// writes replies while the screen is locked, from freezing every pane's drawing.
			}
		}
		// The only empty write after closing is the wake-up sent by Close.
		if err != nil || (n == 0 && p.closed.Load()) {
			return
		}
	}
}

func (p *Pane) writeInputToProcess(inputQueue <-chan []byte) {
	for chunk := range inputQueue {
		_, _ = p.process.Write(chunk) // fails only after the program exits; the input is dropped
	}
}

// withOpenScreen runs update with exclusive access to the emulator, unless the pane is closed.
func (p *Pane) withOpenScreen(update func(screen *vt.Emulator)) {
	p.screenLock.Lock()
	defer p.screenLock.Unlock()
	if p.closed.Load() {
		return
	}
	update(p.screen)
}

func (p *Pane) SendKey(key uv.KeyPressEvent) {
	key.Mod &^= lockModifiers
	p.withOpenScreen(func(screen *vt.Emulator) {
		if sequence, ok := modifiedSpecialKeySequence(key); ok {
			screen.SendText(sequence)
			return
		}
		if sequence, ok := ctrlAltSequence(key); ok {
			screen.SendText(sequence)
			return
		}
		if isPlainText(key) {
			screen.SendText(key.Text)
			return
		}
		screen.SendKey(withoutUnencodedShift(key))
	})
}

func (p *Pane) Paste(text string) {
	p.withOpenScreen(func(screen *vt.Emulator) { screen.Paste(text) })
}

func (p *Pane) Resize(size pty.Size) {
	if size.Cols < 1 || size.Rows < 1 {
		return
	}
	p.withOpenScreen(func(screen *vt.Emulator) {
		if size == p.size {
			return
		}
		p.size = size
		screen.Resize(size.Cols, size.Rows)
		if p.clean != nil {
			p.clean.Resize(size.Cols, size.Rows)
		}
		_ = p.process.Resize(size) // fails only if the program already exited
	})
}

// Draw copies the whole pane screen into area, clipped to the area's size.
func (p *Pane) Draw(target uv.Screen, area uv.Rectangle) {
	p.DrawScrolled(target, area, 0)
}

// ScrollbackLen is how many lines that scrolled off the top are kept. Full-screen programs
// (the alternate screen) have none, unless HistoryUnderFullScreen puts the main screen there.
func (p *Pane) ScrollbackLen() int {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	return p.historyLen()
}

func (p *Pane) historyLen() int {
	if !p.shown().IsAltScreen() {
		return p.shown().ScrollbackLen()
	}
	if p.historyUnderFullScreen {
		return p.shown().ScrollbackLen() + p.shown().Height()
	}
	return 0
}

// FullScreen tells whether a full-screen program shows, on the alternate screen.
func (p *Pane) FullScreen() bool {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	return p.shown().IsAltScreen()
}

// DrawScrolled draws the pane as it looked offset lines up its history; 0 is the live screen.
// History and screen form one sequence of lines, oldest first.
func (p *Pane) DrawScrolled(target uv.Screen, area uv.Rectangle, offset int) {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	history := p.historyLen()
	first := history - min(max(offset, 0), history)
	width := min(area.Dx(), p.shown().Width())
	height := min(area.Dy(), p.shown().Height())
	for y := range height {
		p.drawLine(target, area.Min.Add(image.Pt(0, y)), first+y, history, width)
	}
}

// LineCount is how many lines history and screen have, as DrawScrolled orders them.
func (p *Pane) LineCount() int {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	return p.historyLen() + p.shown().Height()
}

// LineText is the text of a line of history and screen, oldest first, without the spaces after it.
func (p *Pane) LineText(line int) string {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	return p.lineText(line, p.historyLen())
}

// LastLinesMatching are up to n of the last lines of history and screen whose text match takes,
// oldest first, for a view of only them.
func (p *Pane) LastLinesMatching(match func(text string) bool, n int) []int {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	history := p.historyLen()
	var lines []int
	for line := history + p.shown().Height() - 1; line >= 0 && len(lines) < n; line-- {
		if match(p.lineText(line, history)) {
			lines = append(lines, line)
		}
	}
	slices.Reverse(lines)
	return lines
}

// DrawLines draws the lines of history and screen given, a row each from the top of area.
func (p *Pane) DrawLines(target uv.Screen, area uv.Rectangle, lines []int) {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	history := p.historyLen()
	width := min(area.Dx(), p.shown().Width())
	for y, line := range lines[:min(len(lines), area.Dy())] {
		p.drawLine(target, area.Min.Add(image.Pt(0, y)), line, history, width)
	}
}

func (p *Pane) lineText(line, history int) string {
	var text strings.Builder
	for x := 0; x < p.shown().Width(); {
		cell := p.lineCell(x, line, history)
		if cell == nil || cell.Content == "" {
			text.WriteByte(' ')
			x++
			continue
		}
		text.WriteString(cell.Content)
		x += max(cell.Width, 1)
	}
	return strings.TrimRight(text.String(), " ")
}

// drawLine copies the first width cells of a history-and-screen line to target at origin.
func (p *Pane) drawLine(target uv.Screen, origin image.Point, line, history, width int) {
	for x := 0; x < width; {
		cell := p.lineCell(x, line, history)
		if cell == nil {
			x++
			continue
		}
		if x+cell.Width > width {
			return // a wide character cut by the edge would spill into the frame
		}
		target.SetCell(origin.X+x, origin.Y, cell)
		x += max(cell.Width, 1)
	}
}

// lineCell reads a cell of the history-and-screen sequence. Under a full-screen program the
// history ends with the main screen, when HistoryUnderFullScreen keeps it.
func (p *Pane) lineCell(x, line, history int) *uv.Cell {
	if line >= history {
		return p.shown().CellAt(x, line-history)
	}
	if scrollback := p.shown().ScrollbackLen(); line >= scrollback {
		return p.shown().MainScreenCellAt(x, line-scrollback)
	}
	return p.shown().ScrollbackCellAt(x, line)
}

// Cursor returns the cursor position relative to the pane and whether the program shows it.
func (p *Pane) Cursor() (uv.Position, bool) {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	return p.shown().CursorPosition(), p.cursorVisible.Load()
}

// ForegroundProgram is the program the user is running in the pane's shell, if any.
func (p *Pane) ForegroundProgram() (string, bool) {
	return p.process.ForegroundProgram()
}

// ForegroundCommand is the command line the user is running in the pane's shell, or "".
func (p *Pane) ForegroundCommand() string {
	return p.process.ForegroundCommand()
}

// Dir is the directory the pane's program is in now, or "" when it cannot be told.
func (p *Pane) Dir() string {
	return p.process.Dir()
}

// Title is the title the program set with an escape sequence, or "" if none.
func (p *Pane) Title() string {
	return p.title.Load().(string)
}

// Stop ends the program, as a flow does to a step past its timeout, leaving its output on screen.
func (p *Pane) Stop(grace time.Duration) {
	p.process.Stop(grace)
}

// Close hangs up the program and stops the pane's goroutines. It is safe to call more than once.
func (p *Pane) Close() {
	p.screenLock.Lock()
	defer p.screenLock.Unlock()
	if p.closed.Swap(true) {
		return
	}
	_ = p.process.Close() // hanging up an exited program is not an error worth reporting
	// vt.Emulator.Close races with a pending Read, so wake the reader with an empty write instead.
	p.screen.SendText("")
	if p.clean != nil {
		p.clean.SendText("")
	}
}

func (p *Pane) shown() *vt.Emulator {
	if p.clean != nil && !p.showFull.Load() {
		return p.clean
	}
	return p.screen
}

// writeScreens gives each view's screen what its view shows of data, through shape when set.
func (p *Pane) writeScreens(data []byte, shape func([]byte) []byte) {
	if shape == nil {
		shape = func(data []byte) []byte { return data }
	}
	if p.clean == nil {
		_, _ = p.screen.Write(shape(data))
		return
	}
	_, _ = p.screen.Write(shape(p.fullFilter.Write(data)))
	_, _ = p.clean.Write(shape(p.cleanFilter.Write(data)))
}

// drainClean drops what the clean screen answers: only the full one talks to the program.
func (p *Pane) drainClean() {
	buf := make([]byte, 4096)
	for {
		n, err := p.clean.Read(buf)
		if err != nil || n == 0 && p.closed.Load() {
			return
		}
	}
}

// ToggleView switches a pane with views between the clean and the full one, and tells whether
// it has them.
func (p *Pane) ToggleView() bool {
	if p.clean == nil {
		return false
	}
	p.showFull.Store(!p.showFull.Load())
	return true
}

// Views tells whether the pane has views, and whether the full one shows.
func (p *Pane) Views() (has, full bool) {
	return p.clean != nil, p.showFull.Load()
}
