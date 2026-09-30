// Package pane runs one program in a pseudo-terminal and keeps its screen in a VT emulator.
package pane

import (
	"image"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/AlexandreSJ/noct/third_party/vt"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/kittygfx"
	"github.com/AlexandreSJ/noct/internal/pty"
)

// inputQueueSize bounds how many input chunks wait for the program to read them.
const inputQueueSize = 256

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

	title         atomic.Value
	cursorVisible atomic.Bool
	// mouseModes holds the mouse reporting modes the program turned on, one bit each.
	mouseModes atomic.Uint32

	images             *kittygfx.Session
	graphicsSplitter   kittygfx.Splitter
	onTerminalGraphics func(commands []byte)
	log                io.WriteCloser

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
	}
	p.title.Store("")
	p.cursorVisible.Store(true)
	// Callbacks run while screenLock is held, so they only touch atomics.
	p.screen.SetCallbacks(vt.Callbacks{
		// Invalid UTF-8 would reach the real terminal as is and throw its cursor off.
		Title:            func(title string) { p.title.Store(strings.ToValidUTF8(title, "�")) },
		CursorVisibility: func(visible bool) { p.cursorVisible.Store(visible) },
		EnableMode:       func(mode ansi.Mode) { p.setMouseMode(mode, true) },
		DisableMode:      func(mode ansi.Mode) { p.setMouseMode(mode, false) },
	})

	inputQueue := make(chan []byte, inputQueueSize)
	go p.pumpOutputToScreen(opts.OnScreenChange, opts.OnExit)
	go p.queueScreenInput(inputQueue)
	go p.writeInputToProcess(inputQueue)
	return p, nil
}

// pumpOutputToScreen feeds program output into the emulator until the program exits.
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
		p.withOpenScreen(func(screen *vt.Emulator) { _, _ = screen.Write(data) })
		return
	}
	for _, segment := range p.graphicsSplitter.Split(data) {
		if segment.Command != nil {
			p.handleGraphics(*segment.Command)
			continue
		}
		p.withOpenScreen(func(screen *vt.Emulator) { _, _ = screen.Write(segment.Text) })
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
			inputQueue <- append([]byte(nil), buf[:n]...)
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
		_ = p.process.Resize(size) // fails only if the program already exited
	})
}

// Draw copies the whole pane screen into area, clipped to the area's size.
func (p *Pane) Draw(target uv.Screen, area uv.Rectangle) {
	p.DrawScrolled(target, area, 0)
}

// ScrollbackLen is how many lines that scrolled off the top are kept. Full-screen programs
// (the alternate screen) have none.
func (p *Pane) ScrollbackLen() int {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	return p.historyLen()
}

func (p *Pane) historyLen() int {
	if p.screen.IsAltScreen() {
		return 0
	}
	return p.screen.ScrollbackLen()
}

// DrawScrolled draws the pane as it looked offset lines up its history; 0 is the live screen.
// History and screen form one sequence of lines, oldest first.
func (p *Pane) DrawScrolled(target uv.Screen, area uv.Rectangle, offset int) {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	history := p.historyLen()
	first := history - min(max(offset, 0), history)
	width := min(area.Dx(), p.screen.Width())
	height := min(area.Dy(), p.screen.Height())
	for y := range height {
		for x := 0; x < width; {
			cell := p.lineCell(x, first+y, history)
			if cell == nil {
				x++
				continue
			}
			if x+cell.Width > width {
				break // a wide character cut by the edge would spill into the frame
			}
			target.SetCell(area.Min.X+x, area.Min.Y+y, cell)
			x += max(cell.Width, 1)
		}
	}
}

// lineCell reads a cell of the history-and-screen sequence.
func (p *Pane) lineCell(x, line, history int) *uv.Cell {
	if line < history {
		return p.screen.ScrollbackCellAt(x, line)
	}
	return p.screen.CellAt(x, line-history)
}

// Cursor returns the cursor position relative to the pane and whether the program shows it.
func (p *Pane) Cursor() (uv.Position, bool) {
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	return p.screen.CursorPosition(), p.cursorVisible.Load()
}

// mouseModeBits are the mouse reporting modes a program can ask for, from clicks only to every
// motion.
var mouseModeBits = map[ansi.Mode]uint32{
	ansi.ModeMouseX10:         1 << 0,
	ansi.ModeMouseNormal:      1 << 1,
	ansi.ModeMouseButtonEvent: 1 << 2,
	ansi.ModeMouseAnyEvent:    1 << 3,
}

// Modes that also report releases and dragging.
const (
	reportsRelease = 1<<1 | 1<<2 | 1<<3
	reportsMotion  = 1<<2 | 1<<3
)

func (p *Pane) setMouseMode(mode ansi.Mode, on bool) {
	bit, ok := mouseModeBits[mode]
	if !ok {
		return
	}
	if on {
		p.mouseModes.Or(bit)
	} else {
		p.mouseModes.And(^bit)
	}
}

// WantsMouse tells whether the program asked for mouse events, as vim and htop do.
func (p *Pane) WantsMouse() bool {
	return p.mouseModes.Load() != 0
}

// WantsMouseAt is WantsMouse: a program takes the mouse over its whole screen.
func (p *Pane) WantsMouseAt(image.Point) bool {
	return p.WantsMouse()
}

// SendMouse passes a mouse event, positioned relative to the pane, to the program, if the modes
// it turned on report that kind of event.
func (p *Pane) SendMouse(event uv.MouseEvent) {
	modes := p.mouseModes.Load()
	switch event.(type) {
	case uv.MouseReleaseEvent:
		if modes&reportsRelease == 0 {
			return
		}
	case uv.MouseMotionEvent:
		if modes&reportsMotion == 0 {
			return
		}
	}
	if modes != 0 {
		p.withOpenScreen(func(screen *vt.Emulator) { screen.SendMouse(event) })
	}
}

// ForegroundProgram is the program the user is running in the pane's shell, if any.
func (p *Pane) ForegroundProgram() (string, bool) {
	return p.process.ForegroundProgram()
}

// Dir is the directory the pane's program is in now, or "" when it cannot be told.
func (p *Pane) Dir() string {
	return p.process.Dir()
}

// Title is the title the program set with an escape sequence, or "" if none.
func (p *Pane) Title() string {
	return p.title.Load().(string)
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
}
