package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"text/tabwriter"
	"time"

	"github.com/AlexandreSJ/noct/internal/client"
	saved "github.com/AlexandreSJ/noct/internal/session"
)

// attachCommand attaches to the session named after the command, noct attach dev, or without a
// name lists the sessions there are to attach to.
func attachCommand(socket string, args []string, out io.Writer) error {
	switch len(args) {
	case 0:
		return listSessions(socket, out)
	case 1:
		return attach(socket, client.Target{Session: args[0]})
	}
	return errors.New("usage: noct attach [session]")
}

// newSessionCommand creates a session, named or numbered, and attaches to it.
func newSessionCommand(socket string, args []string) error {
	if len(args) > 1 {
		return errors.New("usage: noct new [session]")
	}
	target := client.Target{New: true}
	if len(args) == 1 {
		if err := saved.CheckName(args[0]); err != nil {
			return err
		}
		target.Session = args[0]
	}
	return attach(socket, target)
}

// listSessions prints the sessions, asking the running workspace or, without one, reading the file.
func listSessions(socket string, out io.Writer) error {
	var infos []saved.Info
	if running(socket) {
		if err := client.Request(socket, "sessions", nil, &infos); err != nil {
			return err
		}
	} else {
		f, err := saved.Load(saved.Path())
		if err != nil {
			return err
		}
		infos = f.Infos()
	}
	if len(infos) == 0 {
		fmt.Fprintln(out, "no sessions yet; noct starts one")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	now := time.Now()
	for _, info := range infos {
		fmt.Fprintf(w, "%s\t%s\t%d tabs\t%s\n", info.Name, sessionState(info), info.Tabs, lastAccess(info.LastAccess, now))
	}
	return w.Flush()
}

func sessionState(info saved.Info) string {
	switch {
	case info.Attached:
		return "attached"
	case info.Open:
		return "open"
	}
	return "closed"
}

func lastAccess(when, now time.Time) string {
	if when.IsZero() {
		return "never used"
	}
	if when.Year() == now.Year() && when.YearDay() == now.YearDay() {
		return "used today " + when.Format("15:04")
	}
	return "used " + when.Format("2006-01-02 15:04")
}

// killSession deletes a session: the running workspace stops its programs, and the file forgets it.
func killSession(socket string, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: noct kill <session>")
	}
	name := args[0]
	if running(socket) {
		return client.Request(socket, "delete_session", map[string]string{"name": name}, nil)
	}
	path := saved.Path()
	f, err := saved.Load(path)
	if err != nil {
		return err
	}
	index := f.Find(name)
	if index < 0 {
		return fmt.Errorf("there is no session named %s", name)
	}
	f.Sessions = append(f.Sessions[:index], f.Sessions[index+1:]...)
	return saved.Save(path, f)
}

// running tells whether a workspace answers on the socket, without starting one.
func running(socket string) bool {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
