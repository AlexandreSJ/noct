package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/logstore"
	"github.com/AlexandreSJ/noct/internal/script"
)

// exitCode ends noct with a script's exit code, without a message: the script printed its own.
type exitCode int

func (c exitCode) Error() string {
	return fmt.Sprintf("exit %d", int(c))
}

func loadConfig() config.Config {
	cfg := config.Default()
	if path, err := config.Path(); err == nil {
		cfg, _ = config.Load(path) // an unreadable config still gives the defaults
	}
	return cfg
}

// scriptsDir is the folder of the user's own scripts, from the config.
func scriptsDir() string {
	return script.ResolveDir(loadConfig().ScriptsDir)
}

func library() script.Library {
	dir, _ := os.Getwd() // without it, only personal scripts are found
	return script.LoadLibrary(scriptsDir(), loadConfig().ScriptFolders, dir)
}

func runsPath() string {
	return filepath.Join(logstore.Dir(), script.RunsFile)
}

// notFound names every folder a script was looked for in.
func notFound(name string) error {
	places := []string{scriptsDir()}
	for _, f := range library().Folders {
		if f.Kind != script.GroupFolder {
			places = append(places, f.Dir)
		}
	}
	return fmt.Errorf("Script not found at %s: %s", strings.Join(places, " or "), name) //nolint:staticcheck // shown to the user as a sentence
}

// listScripts prints each script, folder/name inside a folder, with its program's version and
// what it does.
func listScripts(out io.Writer) error {
	lib := library()
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	for _, s := range lib.All() {
		if s.Problem != nil {
			fmt.Fprintf(table, "✗ %s\tbroken\t%s\n", s.ID(), s.Problem)
			continue
		}
		fmt.Fprintf(table, "%s\t%s\t%s\n", s.ID(), programVersion(s), s.Description)
	}
	if err := table.Flush(); err != nil {
		return err
	}
	return problemsError(lib)
}

func programVersion(s script.Script) string {
	if s.Program == "" {
		return "-"
	}
	path, err := exec.LookPath(s.Program)
	if err != nil {
		return script.NotFound
	}
	return script.Detect(path, s.VersionCmd)
}

// checkScripts reports every script and flow that cannot be loaded.
func checkScripts(out io.Writer) error {
	lib := library()
	flowProblems, flows := checkFlows(out, lib)
	problems := append(append(lib.Broken(), lib.Problems...), flowProblems...)
	if len(problems) > 0 {
		return fmt.Errorf("%d problems:\n  %w", len(problems), errors.Join(problems...))
	}
	fmt.Fprintf(out, "%d scripts and %d flows ok\n", len(lib.All()), flows)
	return nil
}

// problemsError reports the scripts that cannot be loaded and the folders that cannot be read.
func problemsError(lib script.Library) error {
	problems := append(lib.Broken(), lib.Problems...)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%d scripts have problems:\n  %w", len(problems), errors.Join(problems...))
}

// runScript runs a script in this terminal. Values come from name=value arguments; the other
// parameters are asked when this is a terminal, and otherwise keep their starting values.
func runScript(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: noct -s <script> [name=value ...]")
	}
	s, ok := library().Find(args[0])
	if !ok {
		return notFound(args[0])
	}
	if s.Problem != nil {
		return fmt.Errorf("script %s is broken: %w", s.Name, s.Problem)
	}
	if s.Draft {
		return fmt.Errorf("script %s is a draft: review it and run noct promote %s", s.ID(), s.ID())
	}
	memory, err := script.LoadMemory(filepath.Join(logstore.Dir(), script.MemoryFile))
	if err != nil {
		fmt.Fprintln(os.Stderr, "noct:", err) // run with defaults instead of remembered values
	}
	values := s.StartingValues(memory.Values(s), os.Getenv)
	given, err := parseValues(s, args[1:], values)
	if err != nil {
		return err
	}
	if term.IsTerminal(os.Stdin.Fd()) {
		if err := askValues(s, values, given); err != nil {
			return err
		}
	}
	if err := s.CheckValues(values); err != nil {
		return err
	}
	command, err := s.Command(values)
	if err != nil {
		return err
	}
	if err := memory.Remember(s, values); err != nil {
		fmt.Fprintln(os.Stderr, "noct:", err)
	}
	if err := script.RecordRun(runsPath(), s.ID(), time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "noct:", err)
	}
	return runCommand(command, s.WorkingDir(mustGetwd()))
}

func parseValues(s script.Script, args []string, values map[string]string) (map[string]bool, error) {
	given := map[string]bool{}
	for _, arg := range args {
		name, value, ok := strings.Cut(arg, "=")
		if !ok || !hasParam(s, name) {
			return nil, fmt.Errorf("%q is not name=value for a parameter of %s", arg, s.Name)
		}
		values[name], given[name] = value, true
	}
	return given, nil
}

func hasParam(s script.Script, name string) bool {
	for _, p := range s.Params {
		if p.Name == name {
			return true
		}
	}
	return false
}

// askValues asks for each parameter not given; an empty answer keeps the value shown.
func askValues(s script.Script, values map[string]string, given map[string]bool) error {
	input := bufio.NewReader(os.Stdin)
	for _, p := range s.Params {
		if given[p.Name] || !p.Asked() {
			continue
		}
		answer, err := ask(input, p, values[p.Name])
		if err != nil {
			return err
		}
		if answer != "" {
			values[p.Name] = answer
		}
	}
	return nil
}

func ask(input *bufio.Reader, p script.Param, current string) (string, error) {
	shown := current
	if len(p.Choices) > 0 {
		shown = strings.Join(p.Choices, "/") + ", " + current
	}
	if p.Secret {
		fmt.Fprintf(os.Stderr, "%s: ", p.Label())
		secret, err := term.ReadPassword(os.Stdin.Fd())
		fmt.Fprintln(os.Stderr)
		return string(secret), err
	}
	fmt.Fprintf(os.Stderr, "%s [%s]: ", p.Label(), shown)
	line, err := input.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// runCommand runs the command line in the user's shell, attached to this terminal.
func runCommand(command, dir string) error {
	cmd := exec.Command(shell(), "-c", command)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exitCode(exit.ExitCode())
	}
	return err
}

// shell is the user's shell, which runs command lines as the TUI's panes would.
func shell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
