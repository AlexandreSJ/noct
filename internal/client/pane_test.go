package client

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// OpenPane sends the request in its hello and gives back the exit code the workspace answers.
func TestOpenPaneReturnsTheExitCode(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "noct.sock")
	listener, err := ipc.Listen(socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	asked := make(chan ipc.Hello, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		c := ipc.NewConn(conn)
		defer c.Close()
		m, _ := c.Receive()
		var hello ipc.Hello
		_ = json.Unmarshal(m.Payload, &hello)
		asked <- hello
		_ = c.SendJSON(ipc.KindExit, ipc.Exit{Reason: ipc.PaneExited, Code: 7})
	}()
	code, err := OpenPane(socket, ipc.PaneRequest{Command: "make", Wait: true})
	if err != nil || code != 7 {
		t.Fatalf("code %d, %v", code, err)
	}
	if hello := <-asked; hello.Purpose != ipc.OpenPane || hello.Pane == nil || hello.Pane.Command != "make" {
		t.Errorf("hello = %+v", hello)
	}
}

func TestOpenPaneWithoutAWorkspaceFails(t *testing.T) {
	if _, err := OpenPane(filepath.Join(t.TempDir(), "none.sock"), ipc.PaneRequest{}); err == nil {
		t.Error("no error without a workspace")
	}
}
