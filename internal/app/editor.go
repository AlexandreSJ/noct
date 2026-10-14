package app

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/layout"
)

// customChoice is the choice that lets the user type any command.
const customChoice = "custom"

// otherEditors are offered after vim and nano when installed. hx is Helix.
var otherEditors = []string{"nvim", "hx", "micro", "emacs", "kak", "vi", "joe", "mcedit"}

// Terminal file managers run in a pane; graphical ones open their own window.
var (
	terminalFileManagers  = []string{"yazi", "ranger", "lf", "nnn", "vifm", "mc"}
	graphicalFileManagers = []string{"thunar", "nautilus", "dolphin", "nemo", "pcmanfm", "pcmanfm-qt", "caja"}
)

// editorChoices lists vim and nano, the other known editors that are installed, and custom.
func editorChoices(installed func(program string) bool) []string {
	return append(append([]string{"vim", "nano"}, found(otherEditors, installed)...), customChoice)
}

// fileManagerChoices lists a terminal pane, the file managers that are installed, and custom.
func fileManagerChoices(installed func(program string) bool) []string {
	choices := []string{config.FileManagerTerminal}
	choices = append(choices, found(terminalFileManagers, installed)...)
	choices = append(choices, found(graphicalFileManagers, installed)...)
	return append(choices, customChoice)
}

func found(programs []string, installed func(program string) bool) []string {
	var present []string
	for _, program := range programs {
		if installed(program) {
			present = append(present, program)
		}
	}
	return present
}

func isInstalled(program string) bool {
	_, err := exec.LookPath(program)
	return err == nil
}

// programSetting is a config row that names a program: one of the choices found on the system,
// or, under custom, a command typed in the row below it.
type programSetting struct {
	// choices end with customChoice.
	choices []string
	custom  bool
	draft   textField
}

func newProgramSetting(choices []string, current string) programSetting {
	s := programSetting{choices: choices}
	if !slices.Contains(choices[:len(choices)-1], current) {
		s.custom, s.draft = true, newTextField(current)
	}
	return s
}

// shown is what the choice row shows for the setting's value.
func (s *programSetting) shown(current string) string {
	if s.custom {
		return customChoice
	}
	return current
}

// step moves to another choice and returns the setting's new value. Reaching custom keeps the
// current value until a command is typed.
func (s *programSetting) step(current string, step int) string {
	index := slices.Index(s.choices, current)
	if s.custom || index < 0 {
		index = len(s.choices) - 1
	}
	next := s.choices[(index+step+len(s.choices))%len(s.choices)]
	s.custom = next == customChoice
	if s.custom {
		return current
	}
	return next
}

// command is the setting's value with the typed command saved: the command, or current while
// custom is not chosen or nothing is typed.
func (s *programSetting) command(current string) string {
	typed := strings.TrimSpace(s.draft.String())
	if !s.custom || typed == "" {
		return current
	}
	return typed
}

// dropEmptyCustom leaves custom when the panel closes without a command typed for it.
func (s *programSetting) dropEmptyCustom() {
	if s.custom && strings.TrimSpace(s.draft.String()) == "" {
		s.custom = false
	}
}

// commandArgv runs a configured command on files through the shell, so it may carry arguments.
// It fails when the command's program is not installed.
func commandArgv(command string, files ...string) ([]string, bool) {
	fields := strings.Fields(command)
	if len(fields) == 0 || !isInstalled(fields[0]) {
		return nil, false
	}
	return append([]string{"/bin/sh", "-c", command + ` "$@"`, fields[0]}, files...), true
}

// openFolder shows a folder with the configured file manager: a terminal pane in it, a terminal
// file manager in a pane, or a graphical one in its own window.
func (a *App) openFolder(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.statusMessage = err.Error()
		return
	}
	manager := a.config.FileManager
	if manager == config.FileManagerTerminal {
		a.openPanel(func(id layout.PaneID, frame image.Rectangle) (view, error) {
			return a.startInPane(id, frame, a.shellArgv, dir, fmt.Sprintf("pane%d", id))
		})
		return
	}
	argv, ok := commandArgv(manager, dir)
	if !ok {
		a.statusMessage = "file manager " + manager + " not found; choose another in the config"
		return
	}
	if slices.Contains(graphicalFileManagers, manager) {
		a.startDetached(argv, dir)
		return
	}
	a.openPanel(func(id layout.PaneID, frame image.Rectangle) (view, error) {
		return a.startInPane(id, frame, argv, dir, "files")
	})
}

// startDetached starts a program with its own window, in its own session so it outlives noct.
func (a *App) startDetached(argv []string, dir string) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, a.shellEnv
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		a.statusMessage = err.Error()
		return
	}
	go func() { _ = cmd.Wait() }() // reaps the process; its outcome is the window's business
	a.notice = "opened " + dir
}
