package client

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// Request asks the running workspace to do one thing and reads its answer into into. It waits
// as long as the workspace takes: an approval waits for the user.
func Request(socketPath, action string, args any, into any) error {
	raw, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("encoding the arguments of %s: %w", action, err)
	}
	conn, err := ipc.Dial(socketPath)
	if err != nil {
		return fmt.Errorf("reaching the noct workspace: %w", err)
	}
	defer conn.Close()
	hello := ipc.Hello{Version: ipc.Version, Purpose: ipc.Ask, Request: &ipc.Request{Action: action, Args: raw}}
	if err := conn.SendJSON(ipc.KindHello, hello); err != nil {
		return err
	}
	m, err := conn.Receive()
	if err != nil {
		return fmt.Errorf("waiting for the workspace to %s: %w", action, err)
	}
	var exit ipc.Exit
	if m.Kind != ipc.KindExit || json.Unmarshal(m.Payload, &exit) != nil {
		return errors.New("the noct workspace gave no answer")
	}
	if exit.Reason != ipc.RequestDone {
		return errors.New(exit.Reason)
	}
	if into == nil || len(exit.Data) == 0 {
		return nil
	}
	return json.Unmarshal(exit.Data, into)
}
