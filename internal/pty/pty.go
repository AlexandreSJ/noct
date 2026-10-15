// Package pty starts programs attached to pseudo-terminals.
package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unicode"

	creackpty "github.com/creack/pty"
	"golang.org/x/sys/unix"
)

type Size struct {
	Cols int
	Rows int
}

// Process is a running program whose stdin, stdout and stderr are a pseudo-terminal.
type Process struct {
	terminal *os.File
	cmd      *exec.Cmd
	exited   atomic.Bool
}

type StartOptions struct {
	Argv []string
	Env  []string
	Dir  string
	Size Size
}

func Start(opts StartOptions) (*Process, error) {
	if len(opts.Argv) == 0 {
		return nil, fmt.Errorf("starting process: empty command")
	}
	cmd := exec.Command(opts.Argv[0], opts.Argv[1:]...)
	cmd.Env = opts.Env
	cmd.Dir = opts.Dir

	terminal, err := creackpty.StartWithSize(cmd, winsize(opts.Size))
	if err != nil {
		return nil, fmt.Errorf("starting %q in a pty: %w", opts.Argv[0], err)
	}
	return &Process{terminal: terminal, cmd: cmd}, nil
}

func (p *Process) Read(b []byte) (int, error) {
	return p.terminal.Read(b)
}

func (p *Process) Write(b []byte) (int, error) {
	return p.terminal.Write(b)
}

func (p *Process) Resize(size Size) error {
	if err := creackpty.Setsize(p.terminal, winsize(size)); err != nil {
		return fmt.Errorf("resizing pty: %w", err)
	}
	return nil
}

// Wait blocks until the program exits and returns its exit code, or -1 when a signal ended it.
func (p *Process) Wait() int {
	_ = p.cmd.Wait() // the exit status below says everything the caller needs
	p.exited.Store(true)
	return p.cmd.ProcessState.ExitCode()
}

// ForegroundProgram is the name of the program the terminal gives the keyboard to, and whether
// it is some program other than the one started (a shell running htop reports htop).
func (p *Process) ForegroundProgram() (string, bool) {
	group, ok := p.foregroundGroup()
	if !ok {
		return "", false
	}
	name := processName(group)
	return name, name != ""
}

// ForegroundCommand is the command line of the program the terminal gives the keyboard to, quoted
// for a shell, or "" when only the started program runs. A session keeps it to offer typing it
// again, so it is "" where the arguments cannot be told apart (without /proc, ps joins them with
// spaces and loses their quoting) and when one holds a control character, which a shell's line
// editor would take as a key.
func (p *Process) ForegroundCommand() string {
	group, ok := p.foregroundGroup()
	if !ok {
		return ""
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", group))
	if err != nil {
		return ""
	}
	args := strings.Split(strings.TrimRight(string(cmdline), "\x00"), "\x00")
	for i, arg := range args {
		if strings.ContainsFunc(arg, unicode.IsControl) {
			return ""
		}
		args[i] = shellQuote(arg)
	}
	return strings.Join(args, " ")
}

// foregroundGroup is the process group the terminal gives the keyboard to, unless it is the
// started program's own.
func (p *Process) foregroundGroup() (int, bool) {
	raw, err := p.terminal.SyscallConn()
	if err != nil {
		return 0, false
	}
	group, groupErr := 0, error(nil)
	// SyscallConn keeps the file non-blocking, unlike Fd, so a pending Read still ends on Close.
	err = raw.Control(func(fd uintptr) { group, groupErr = unix.IoctlGetInt(int(fd), unix.TIOCGPGRP) })
	if err != nil || groupErr != nil || group <= 0 || group == p.cmd.Process.Pid {
		return 0, false
	}
	return group, true
}

// shellQuote leaves plain words as they are and puts anything else in single quotes.
func shellQuote(arg string) string {
	plain := arg != "" && !strings.ContainsFunc(arg, func(r rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./=:,+@%", r)
	})
	if plain {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

// Dir is the directory the program is in now, such as a shell after cd, or "" where the system
// does not tell it (only Linux's /proc does).
func (p *Process) Dir() string {
	if p.exited.Load() {
		return ""
	}
	dir, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", p.cmd.Process.Pid))
	if err != nil {
		return ""
	}
	return dir
}

// processName is the base name of the program a process runs, or "" if it cannot be read.
func processName(pid int) string {
	if cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
		argv0, _, _ := strings.Cut(string(cmdline), "\x00")
		return filepath.Base(argv0)
	}
	output, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return filepath.Base(strings.TrimSpace(string(output)))
}

// Close hangs up the program the way closing a terminal window does: SIGHUP to its whole
// process group. Closing the pty alone is not enough while a Read is still pending on it.
func (p *Process) Close() error {
	var hangupErr error
	// After Wait the pid may belong to someone else, so only signal a live program.
	if !p.exited.Load() {
		hangupErr = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGHUP)
	}
	if errors.Is(hangupErr, syscall.ESRCH) {
		hangupErr = nil
	}
	return errors.Join(hangupErr, p.terminal.Close())
}

func winsize(size Size) *creackpty.Winsize {
	return &creackpty.Winsize{Cols: uint16(max(size.Cols, 1)), Rows: uint16(max(size.Rows, 1))}
}
