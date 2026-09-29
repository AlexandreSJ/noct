package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/AlexandreSJ/noct/internal/client"
	"github.com/AlexandreSJ/noct/internal/daemon"
	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/mcp"
)

// maxAuditLog bounds the MCP audit log; past it, it rotates like a pane's log.
const maxAuditLog = 5 << 20

// serveMCP runs noct's MCP server on this process's input and output, for an AI client such as
// Claude Code. Every call goes to the audit log, mcp.log in noct's logs.
func serveMCP() error {
	audit, err := logstore.Create(filepath.Join(logstore.LogsDir(), "mcp.log"), maxAuditLog)
	if err != nil {
		return err
	}
	defer audit.Close()
	server := mcp.NewServer(noctVersion(), mcp.Deps{
		Library:    library,
		ScriptsDir: scriptsDir,
		Config:     loadConfig,
		Workspace:  daemonWorkspace{socket: ipc.SocketPath()},
		Audit:      audit,
		Now:        time.Now,
	})
	server.Connected = daemonWorkspace{socket: ipc.SocketPath()}.stayConnected
	return server.Serve(os.Stdin, os.Stdout)
}

// daemonWorkspace asks the noct daemon, starting it when it is not running, so an AI's runs and
// panes are there when the user attaches.
type daemonWorkspace struct {
	socket string
}

func (w daemonWorkspace) Do(action string, args any, into any) error {
	if err := daemon.EnsureRunning(w.socket); err != nil {
		return err
	}
	return client.Request(w.socket, action, args, into)
}

// reconnectEvery is how long a client waits to show itself again to a daemon it cannot reach.
const reconnectEvery = 3 * time.Second

// stayConnected shows the AI client in noct's MCP popup for as long as the session lasts, keeping
// a request open that the daemon never answers. It does not start the daemon, so that quitting
// noct stays quit; a daemon started later sees the client within reconnectEvery.
func (w daemonWorkspace) stayConnected(name, version string, gone <-chan struct{}) {
	for {
		ended := make(chan struct{})
		go func() {
			_ = client.Request(w.socket, "client", map[string]string{"name": name, "version": version}, nil)
			close(ended)
		}()
		select {
		case <-gone:
			return // the process exits next, which closes the request
		case <-ended:
		}
		select {
		case <-gone:
			return
		case <-time.After(reconnectEvery):
		}
	}
}

// noctVersion is the module version the binary was built from, or dev.
func noctVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
