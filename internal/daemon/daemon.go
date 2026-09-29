// Package daemon runs the background process that owns noct's workspace, and starts it on demand.
package daemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/AlexandreSJ/noct/internal/app"
	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/logstore"
)

// startTimeout bounds how long a new daemon may take to open its socket.
const startTimeout = 5 * time.Second

// Run serves the workspace on the socket until the user quits.
func Run(socketPath string) error {
	listener, err := ipc.Listen(socketPath)
	if err != nil {
		return err
	}
	return app.New().Serve(listener)
}

// EnsureRunning starts a daemon in the background unless one already answers on the socket.
// The daemon runs in its own session, so closing the terminal does not stop it.
func EnsureRunning(socketPath string) error {
	if reachable(socketPath) {
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding the noct executable: %w", err)
	}
	logFile, err := openDaemonLog()
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(executable, "daemon")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting the noct daemon: %w", err)
	}
	_ = cmd.Process.Release() // the daemon outlives this process; nobody waits for it here

	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) {
		if reachable(socketPath) {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("the noct daemon did not start; see %s", logFile.Name())
}

func reachable(socketPath string) bool {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func openDaemonLog() (*os.File, error) {
	dir := logstore.Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}
	file, err := os.OpenFile(filepath.Join(dir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening the daemon log: %w", err)
	}
	return file, nil
}
