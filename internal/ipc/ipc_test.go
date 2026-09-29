package ipc

import (
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"testing"
)

func pipePair() (*Conn, *Conn) {
	a, b := net.Pipe()
	return NewConn(a), NewConn(b)
}

func TestMessagesRoundTrip(t *testing.T) {
	client, daemon := pipePair()
	hello := Hello{Version: Version, Purpose: Attach, Env: []string{"TERM=xterm-kitty"}, Size: Size{Cols: 80, Rows: 24}, ColorProfile: 5}
	go func() {
		_ = client.SendJSON(KindHello, hello)
		_, _ = client.Writer().Write([]byte("\x1b[31mhi")) // arrives as output
		client.Close()
	}()

	first, err := daemon.Receive()
	if err != nil || first.Kind != KindHello {
		t.Fatalf("first message %v, %v", first.Kind, err)
	}
	var got Hello
	if err := json.Unmarshal(first.Payload, &got); err != nil || got.Size != hello.Size || got.Env[0] != "TERM=xterm-kitty" {
		t.Errorf("hello %+v, %v", got, err)
	}
	second, err := daemon.Receive()
	if err != nil || second.Kind != KindOutput || string(second.Payload) != "\x1b[31mhi" {
		t.Errorf("second message %v %q, %v", second.Kind, second.Payload, err)
	}
	if _, err := daemon.Receive(); err != io.EOF {
		t.Errorf("after close: %v, want EOF", err)
	}
}

func TestOversizedMessageIsRejected(t *testing.T) {
	a, b := net.Pipe()
	go func() {
		_, _ = a.Write([]byte{byte(KindInput), 0xff, 0xff, 0xff, 0xff})
		a.Close()
	}()
	if _, err := NewConn(b).Receive(); err == nil {
		t.Error("a 4 GiB message was accepted")
	}
}

func TestListenRefusesASecondDaemonAndReplacesStaleSockets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "noct", "noct.sock")
	first, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Listen(path); err == nil {
		t.Error("a second daemon could listen")
	}
	first.Close()
	// Closing a Unix listener removes the socket file; recreate a stale one to be sure.
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	stale.(*net.UnixListener).SetUnlinkOnClose(false)
	stale.Close()

	again, err := Listen(path)
	if err != nil {
		t.Fatalf("a stale socket blocked a new daemon: %v", err)
	}
	again.Close()
}
