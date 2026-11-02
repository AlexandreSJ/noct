// Package ipc is the protocol between the noct daemon and the terminals attached to it:
// length-prefixed messages over a Unix socket.
package ipc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// Version changes whenever messages change; a client and daemon of different versions refuse
// to talk rather than misread each other.
const Version = 3

// sendPiece is how much of a message is written under one deadline, for connections with a send limit.
const sendPiece = 64 << 10

// maxPayload bounds one message, so a corrupted length cannot make a reader allocate gigabytes.
const maxPayload = 16 << 20

type Kind byte

const (
	// KindHello opens a connection from a client (Hello, JSON).
	KindHello Kind = iota + 1
	// KindInput carries raw bytes typed in the client's terminal.
	KindInput
	// KindResize reports a new client terminal size (Size, JSON).
	KindResize
	// KindOutput carries bytes for the client to write to its terminal.
	KindOutput
	// KindExit ends the connection (Exit, JSON), sent by the daemon.
	KindExit
)

// Size is a terminal size in cells and, when the terminal reports them, pixels.
type Size struct {
	Cols    int `json:"cols"`
	Rows    int `json:"rows"`
	XPixels int `json:"xpixels,omitempty"`
	YPixels int `json:"ypixels,omitempty"`
}

// Purpose says what a client connects for.
type Purpose string

const (
	Attach Purpose = "attach"
	Quit   Purpose = "quit"
	// OpenPane asks for a pane that runs a flow's step (Hello.Pane).
	OpenPane Purpose = "pane"
	// Ask asks the workspace to do one thing for the MCP server (Hello.Request): list or read
	// panes, run something where the user sees it, or ask the user to approve an action.
	Ask Purpose = "request"
)

// Request is one thing asked of the workspace, with its arguments as JSON. The answer is an Exit
// whose Reason is RequestDone, with Data as JSON, or says why it failed.
type Request struct {
	Action string          `json:"action"`
	Args   json.RawMessage `json:"args,omitempty"`
}

// Restart is the Reason of the Exit an attached client gets when noct stops to run the version
// installed over it: the client starts the binary on disk, which attaches again.
const Restart = "restart"

// RequestDone is the Reason of an Exit that answers a request that went well.
const RequestDone = "done"

// PaneRequest is a pane a flow's step runs in, beside the pane of the run that asks (Near).
// With Wait the answer comes when the step's program exits, with its code; without it, once
// the pane is open.
type PaneRequest struct {
	Command string `json:"command"`
	Dir     string `json:"dir,omitempty"`
	Title   string `json:"title,omitempty"`
	Near    int    `json:"near,omitempty"`
	Wait    bool   `json:"wait,omitempty"`
}

// Reasons of the Exit that answers a pane request; any other reason says why it failed.
const (
	PaneOpened = "opened"
	PaneExited = "exited"
)

type Hello struct {
	Version int      `json:"version"`
	Purpose Purpose  `json:"purpose"`
	Env     []string `json:"env,omitempty"`
	// Dir is the client's working directory: where new terminals start and project scripts are found.
	Dir  string `json:"dir,omitempty"`
	Size Size   `json:"size"`
	// ColorProfile is the client terminal's colorprofile.Profile, detected on the client side
	// because only it can see its terminal.
	ColorProfile byte `json:"color_profile"`
	// Pane is the pane asked for, with Purpose OpenPane.
	Pane *PaneRequest `json:"pane,omitempty"`
	// Request is what is asked, with Purpose Ask.
	Request *Request `json:"request,omitempty"`
	// Session is the session to attach to, with Purpose Attach: the one used last when empty.
	Session string `json:"session,omitempty"`
	// NewSession creates the session to attach to, named Session or numbered when that is empty.
	NewSession bool `json:"new_session,omitempty"`
	// Setup opens noct's setup once attached, as noct setup asks.
	Setup bool `json:"setup,omitempty"`
}

type Exit struct {
	Reason string `json:"reason"`
	// Code is the exit code of a pane's program, with Reason PaneExited.
	Code int `json:"code,omitempty"`
	// Data is the answer to a request, with Reason RequestDone.
	Data json.RawMessage `json:"data,omitempty"`
	// Refused says the connection was turned down, and Reason why, as for a session that does
	// not exist.
	Refused bool `json:"refused,omitempty"`
}

type Message struct {
	Kind    Kind
	Payload []byte
}

// Conn sends and receives messages. Sends may come from several goroutines.
type Conn struct {
	conn        net.Conn
	writeLock   sync.Mutex
	sendTimeout time.Duration
}

func NewConn(conn net.Conn) *Conn {
	return &Conn{conn: conn}
}

// LimitSends makes a send fail, and close the connection, when the other side reads nothing for
// timeout, so a peer that stops reading, such as a suspended terminal, cannot block the sender.
func (c *Conn) LimitSends(timeout time.Duration) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	c.sendTimeout = timeout
}

func (c *Conn) Send(kind Kind, payload []byte) error {
	header := make([]byte, 5, 5+len(payload))
	header[0] = byte(kind)
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	if err := c.write(append(header, payload...)); err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	return nil
}

// write sends message whole. With a send limit it goes in pieces, each with its own deadline, so
// a slow link that keeps reading is not cut; a message cut short would break every later one, so
// the connection closes.
func (c *Conn) write(message []byte) error {
	if c.sendTimeout == 0 {
		_, err := c.conn.Write(message)
		return err
	}
	for len(message) > 0 {
		piece := message[:min(len(message), sendPiece)]
		_ = c.conn.SetWriteDeadline(time.Now().Add(c.sendTimeout)) // fails only on a closed conn, which Write reports
		if _, err := c.conn.Write(piece); err != nil {
			c.conn.Close()
			return err
		}
		message = message[len(piece):]
	}
	return nil
}

func (c *Conn) SendJSON(kind Kind, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding message: %w", err)
	}
	return c.Send(kind, payload)
}

// Receive blocks for the next message; io.EOF means the other side closed the connection.
func (c *Conn) Receive() (Message, error) {
	var header [5]byte
	if _, err := io.ReadFull(c.conn, header[:]); err != nil {
		return Message{}, err
	}
	length := binary.BigEndian.Uint32(header[1:])
	if length > maxPayload {
		return Message{}, fmt.Errorf("message of %d bytes is too large", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return Message{}, fmt.Errorf("reading message: %w", err)
	}
	return Message{Kind: Kind(header[0]), Payload: payload}, nil
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

// Writer sends everything written to it as output messages, for a renderer to draw through.
func (c *Conn) Writer() io.Writer {
	return outputWriter{c}
}

type outputWriter struct{ conn *Conn }

func (w outputWriter) Write(p []byte) (int, error) {
	if err := w.conn.Send(KindOutput, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// SocketPath is where the daemon listens: under XDG_RUNTIME_DIR, or a per-user temporary
// directory when it is not set.
func SocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "noct-"+strconv.Itoa(os.Getuid()))
	} else {
		dir = filepath.Join(dir, "noct")
	}
	return filepath.Join(dir, "noct.sock")
}

// Listen creates the socket, readable only by its owner, replacing a stale one left by a
// daemon that died. It fails if another daemon is already listening or starting.
func Listen(path string) (net.Listener, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating the socket directory: %w", err)
	}
	if err := checkPrivateDir(dir); err != nil {
		return nil, err
	}
	// The lock spans the check for a live daemon and the stale socket's removal, so two
	// daemons starting at once cannot both pass the check and delete each other's socket.
	lock, err := lockDaemon(filepath.Join(dir, "noct.lock"))
	if err != nil {
		return nil, err
	}
	listener, err := listenReplacingStale(path)
	if err != nil {
		lock.Close()
		return nil, err
	}
	return lockedListener{Listener: listener, lock: lock}, nil
}

// checkPrivateDir refuses a socket directory another user could replace the socket in, such
// as a /tmp/noct-<uid> someone else created before this user's first daemon.
func checkPrivateDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("checking the socket directory: %w", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || !ok || int(stat.Uid) != os.Getuid() || info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("the socket directory %s must be a folder only this user can access", dir)
	}
	return nil
}

func lockDaemon(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening the daemon lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("a noct daemon is already running")
	}
	return file, nil
}

func listenReplacingStale(path string) (net.Listener, error) {
	if conn, err := net.Dial("unix", path); err == nil {
		conn.Close()
		return nil, errors.New("a noct daemon is already running")
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("removing a stale socket: %w", err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, fmt.Errorf("restricting the socket: %w", err)
	}
	return listener, nil
}

// lockedListener holds the daemon lock for as long as it listens.
type lockedListener struct {
	net.Listener
	lock *os.File
}

func (l lockedListener) Close() error {
	return errors.Join(l.Listener.Close(), l.lock.Close())
}

func Dial(path string) (*Conn, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}
	return NewConn(conn), nil
}
