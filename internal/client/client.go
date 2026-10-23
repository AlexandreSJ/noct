// Package client is the side of noct that runs in the user's terminal: it attaches that terminal
// to the daemon, forwarding keys and size changes and writing what the daemon draws.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
	"github.com/muesli/cancelreader"
	"golang.org/x/sys/unix"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// emergencyReset gives the terminal back when the daemon vanished without doing it: main screen,
// no bracketed paste or mouse reports, visible cursor, default colors, and kitty's in_noct
// variable cleared.
const emergencyReset = "\x1b[?1049l\x1b[?2004l\x1b[?1002l\x1b[?1006l\x1b[?25h\x1b[0m\x1b]1337;SetUserVar=in_noct\x07"

// ErrDaemonStopped is the end of an attach when the daemon went away without saying so.
var ErrDaemonStopped = errors.New("the noct daemon stopped unexpectedly")

// Target is the session to attach to: the one used last when Session is empty, or a new one.
type Target struct {
	Session string
	New     bool
	// Setup opens noct's setup once attached.
	Setup bool
}

// Attach connects this terminal to a session of the daemon until it detaches or quits, and
// returns why.
func Attach(socketPath string, target Target) (reason string, err error) {
	conn, err := ipc.Dial(socketPath)
	if err != nil {
		return "", fmt.Errorf("connecting to the noct daemon: %w", err)
	}
	defer conn.Close()

	stdin, stdout := os.Stdin, os.Stdout
	dir, _ := os.Getwd() // without it, the daemon keeps its own directory
	hello := ipc.Hello{
		Version:      ipc.Version,
		Purpose:      ipc.Attach,
		Env:          os.Environ(),
		Dir:          dir,
		Size:         terminalSize(stdout),
		ColorProfile: byte(colorprofile.Detect(stdout, os.Environ())),
		Session:      target.Session,
		NewSession:   target.New,
		Setup:        target.Setup,
	}
	state, err := term.MakeRaw(stdin.Fd())
	if err != nil {
		return "", fmt.Errorf("switching the terminal to raw mode: %w", err)
	}
	defer term.Restore(stdin.Fd(), state) //nolint:errcheck // nothing better to do if restoring fails

	if err := conn.SendJSON(ipc.KindHello, hello); err != nil {
		return "", err
	}
	input, err := cancelreader.NewReader(stdin)
	if err != nil {
		return "", fmt.Errorf("reading the terminal: %w", err)
	}
	defer input.Cancel()
	go forwardInput(conn, input)

	done := make(chan struct{})
	defer close(done)
	go forwardResizes(conn, stdout, done)

	terminal, closeTrace, err := traceOutput(stdout, os.Getenv("NOCT_TRACE"))
	if err != nil {
		return "", err
	}
	defer closeTrace()
	reason, err = writeOutput(conn, terminal)
	if err != nil {
		_, _ = io.WriteString(stdout, emergencyReset)
	}
	return reason, err
}

// Quit asks a running daemon to close everything and stop. Without a daemon, there is nothing to do.
func Quit(socketPath string) error {
	conn, err := ipc.Dial(socketPath)
	if err != nil {
		return nil
	}
	defer conn.Close()
	if err := conn.SendJSON(ipc.KindHello, ipc.Hello{Version: ipc.Version, Purpose: ipc.Quit}); err != nil {
		return err
	}
	_, err = writeOutput(conn, io.Discard)
	return err
}

func forwardInput(conn *ipc.Conn, input io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := input.Read(buf)
		if n > 0 && conn.Send(ipc.KindInput, buf[:n]) != nil {
			return
		}
		if err != nil {
			return
		}
	}
}

func forwardResizes(conn *ipc.Conn, terminal *os.File, done <-chan struct{}) {
	changes := make(chan os.Signal, 1)
	signal.Notify(changes, syscall.SIGWINCH)
	defer signal.Stop(changes)
	for {
		select {
		case <-done:
			return
		case <-changes:
			if conn.SendJSON(ipc.KindResize, terminalSize(terminal)) != nil {
				return
			}
		}
	}
}

// traceOutput also copies everything written to the terminal into the file at path, when there is
// one, so leftover cells on the screen can be replayed and traced back to the bytes that drew them.
func traceOutput(terminal io.Writer, path string) (io.Writer, func(), error) {
	if path == "" {
		return terminal, func() {}, nil
	}
	trace, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("opening the NOCT_TRACE file: %w", err)
	}
	return io.MultiWriter(terminal, trace), func() { _ = trace.Close() }, nil // a short trace is still useful
}

// writeOutput copies what the daemon draws to the terminal until it says the session ended.
// An error means the daemon went away without saying so.
func writeOutput(conn *ipc.Conn, terminal io.Writer) (string, error) {
	for {
		message, err := conn.Receive()
		if errors.Is(err, io.EOF) {
			return "", ErrDaemonStopped
		}
		if err != nil {
			return "", err
		}
		switch message.Kind {
		case ipc.KindOutput:
			if _, err := terminal.Write(message.Payload); err != nil {
				return "", fmt.Errorf("writing to the terminal: %w", err)
			}
		case ipc.KindExit:
			var exit ipc.Exit
			_ = json.Unmarshal(message.Payload, &exit) // an unreadable reason still ends the session
			if exit.Refused {
				return "", errors.New(exit.Reason)
			}
			return exit.Reason, nil
		}
	}
}

func terminalSize(terminal *os.File) ipc.Size {
	size, err := unix.IoctlGetWinsize(int(terminal.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return ipc.Size{Cols: 80, Rows: 24}
	}
	return ipc.Size{Cols: int(size.Col), Rows: int(size.Row), XPixels: int(size.Xpixel), YPixels: int(size.Ypixel)}
}
