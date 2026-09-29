package client

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// OpenPane asks the running workspace for a pane that runs a flow's step, and returns the
// step's exit code, or 0 once the pane opens when the request does not wait.
func OpenPane(socketPath string, req ipc.PaneRequest) (int, error) {
	conn, err := ipc.Dial(socketPath)
	if err != nil {
		return 0, fmt.Errorf("reaching the noct workspace: %w", err)
	}
	defer conn.Close()
	if err := conn.SendJSON(ipc.KindHello, ipc.Hello{Version: ipc.Version, Purpose: ipc.OpenPane, Pane: &req}); err != nil {
		return 0, err
	}
	m, err := conn.Receive()
	if err != nil {
		return 0, fmt.Errorf("waiting for the step's pane: %w", err)
	}
	var exit ipc.Exit
	if m.Kind != ipc.KindExit || json.Unmarshal(m.Payload, &exit) != nil {
		return 0, errors.New("the noct workspace gave no answer about the pane")
	}
	switch exit.Reason {
	case ipc.PaneExited:
		return exit.Code, nil
	case ipc.PaneOpened:
		return 0, nil
	}
	return 0, errors.New(exit.Reason)
}
