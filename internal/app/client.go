package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// Kitty users can let keys through to noct with `map --when-focus-on var:in_noct <keys>`.
const (
	setInNoctVariable   = "\x1b]1337;SetUserVar=in_noct=MQ==\x07"
	clearInNoctVariable = "\x1b]1337;SetUserVar=in_noct\x07"
)

// helloTimeout bounds how long a new connection may take to say who it is.
const helloTimeout = 5 * time.Second

// clientGone reports that a client disconnected without detaching, e.g. its window closed.
type clientGone struct {
	client *client
}

// connection is a new client that introduced itself, waiting for the main loop to take it.
type connection struct {
	conn  *ipc.Conn
	hello ipc.Hello
}

// client is the terminal attached to the workspace. Its keystrokes arrive as input messages and
// are decoded here into events; frames are rendered into output messages by its screen.
type client struct {
	conn   *ipc.Conn
	hello  ipc.Hello
	screen *uv.TerminalScreen
	events chan uv.Event
	keys   *io.PipeWriter
	ctx    context.Context
	cancel context.CancelFunc
}

func newClient(c connection) *client {
	screen := uv.NewTerminalScreen(c.conn.Writer(), c.hello.Env)
	screen.SetColorProfile(colorprofile.Profile(c.hello.ColorProfile))
	keysReader, keysWriter := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	cl := &client{
		conn:   c.conn,
		hello:  c.hello,
		screen: screen,
		events: make(chan uv.Event, 64),
		keys:   keysWriter,
		ctx:    ctx,
		cancel: cancel,
	}
	go cl.decodeKeys(keysReader, envValue(c.hello.Env, "TERM"))
	go cl.readMessages()
	return cl
}

// readMessages feeds typed bytes to the key decoder and turns resizes into events.
func (c *client) readMessages() {
	for {
		message, err := c.conn.Receive()
		if err != nil {
			c.emit(clientGone{client: c})
			return
		}
		switch message.Kind {
		case ipc.KindInput:
			_, _ = c.keys.Write(message.Payload) // fails only once the client is closed
		case ipc.KindResize:
			var size ipc.Size
			if json.Unmarshal(message.Payload, &size) == nil {
				c.emitSize(size)
			}
		}
	}
}

// decodeKeys runs the key decoder until the client closes. Events for a client the workspace
// dropped are discarded, so the decoder never blocks forever.
func (c *client) decodeKeys(keys io.Reader, termType string) {
	decoded := make(chan uv.Event)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_ = uv.NewTerminalReader(keys, termType).StreamEvents(c.ctx, decoded) // ends when keys close
	}()
	for {
		select {
		case event := <-decoded:
			c.emit(event)
		case <-finished:
			return
		}
	}
}

func (c *client) emit(event uv.Event) {
	select {
	case c.events <- event:
	case <-c.ctx.Done():
	}
}

func (c *client) emitSize(size ipc.Size) {
	if size.Cols <= 0 || size.Rows <= 0 {
		return
	}
	c.emit(uv.WindowSizeEvent{Width: size.Cols, Height: size.Rows})
	if size.XPixels > 0 && size.YPixels > 0 {
		c.emit(uv.CellSizeEvent{Width: size.XPixels / size.Cols, Height: size.YPixels / size.Rows})
	}
}

// start prepares the client's terminal for the workspace.
func (c *client) start() error {
	if err := c.screen.EnterAltScreen(); err != nil {
		return fmt.Errorf("entering alternate screen: %w", err)
	}
	if err := c.screen.EnableBracketedPaste(); err != nil {
		return fmt.Errorf("enabling bracketed paste: %w", err)
	}
	_, _ = c.screen.WriteString(setInNoctVariable) // queued; the first frame sends it
	_, _ = c.screen.WriteString(ansi.RequestModeSynchronizedOutput)
	c.emitSize(c.hello.Size)
	return nil
}

// useSynchronizedUpdates sends each frame as one synchronized update (mode 2026) when the
// terminal says it has the mode. Until then, and for terminals without it, the cursor is hidden
// while a frame is drawn: drawing moves it to every cell that changes, and a terminal that shows
// that makes it jump around the screen on every frame, such as an animation's. Hiding it on
// terminals that have the mode would only make it blink.
func (a *App) useSynchronizedUpdates(report uv.ModeReportEvent) {
	if report.Mode != ansi.ModeSynchronizedOutput || a.screen == nil {
		return
	}
	has := report.Value == ansi.ModeSet || report.Value == ansi.ModeReset || report.Value == ansi.ModePermanentlySet
	_ = a.screen.SetSynchronizedUpdates(has) // only records it
}

// end gives the client's terminal back as it was, tells the client why and disconnects it.
func (c *client) end(reason string) {
	_ = c.screen.Reset() // a client that cannot be written to is gone anyway
	_, _ = c.screen.WriteString(clearInNoctVariable)
	_ = c.screen.Flush()
	_ = c.conn.SendJSON(ipc.KindExit, ipc.Exit{Reason: reason})
	c.close()
}

func (c *client) close() {
	c.cancel()
	_ = c.keys.CloseWithError(io.EOF)
	_ = c.conn.Close()
}

// acceptClients hands every connection that introduces itself to the main loop.
func (a *App) acceptClients(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return // the listener closed: the daemon is stopping
		}
		go a.greet(ipc.NewConn(conn))
	}
}

func (a *App) greet(conn *ipc.Conn) {
	hello, err := receiveHello(conn)
	if err != nil {
		_ = conn.SendJSON(ipc.KindExit, ipc.Exit{Reason: err.Error()})
		_ = conn.Close()
		return
	}
	select {
	case a.connections <- connection{conn: conn, hello: hello}:
	case <-a.stopped:
		_ = conn.Close()
	}
}

func receiveHello(conn *ipc.Conn) (ipc.Hello, error) {
	type result struct {
		message ipc.Message
		err     error
	}
	received := make(chan result, 1)
	go func() {
		message, err := conn.Receive()
		received <- result{message, err}
	}()
	var r result
	select {
	case r = <-received:
	case <-time.After(helloTimeout):
		return ipc.Hello{}, fmt.Errorf("no hello within %s", helloTimeout)
	}
	if r.err != nil || r.message.Kind != ipc.KindHello {
		return ipc.Hello{}, fmt.Errorf("expected a hello: %v", r.err)
	}
	var hello ipc.Hello
	if err := json.Unmarshal(r.message.Payload, &hello); err != nil {
		return ipc.Hello{}, fmt.Errorf("reading hello: %w", err)
	}
	if hello.Version != ipc.Version {
		return ipc.Hello{}, fmt.Errorf("the running noct daemon speaks protocol %d, this noct %d: run `noct quit` and start noct again", ipc.Version, hello.Version)
	}
	return hello, nil
}

func envValue(env []string, name string) string {
	for _, entry := range env {
		if len(entry) > len(name) && entry[:len(name)] == name && entry[len(name)] == '=' {
			return entry[len(name)+1:]
		}
	}
	return ""
}
