package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/layout"
)

// A run that ends out of sight is told: as the system's notification, the terminal's when the
// system has none or the terminal is over ssh, or with the terminal's bell, as the config says.

// runEnded tells that the run in pane id ended, unless it shows on screen.
func (a *App) runEnded(id layout.PaneID, name string, code int) {
	if a.inSight(id) {
		return
	}
	a.notify(runEndedText(name, code))
}

func runEndedText(name string, code int) string {
	if code == 0 {
		return name + " finished"
	}
	return fmt.Sprintf("%s failed with exit %d", name, code)
}

// inSight tells whether pane id shows on the attached terminal's screen.
func (a *App) inSight(id layout.PaneID) bool {
	if a.client == nil {
		return false
	}
	t, _ := a.tabOfPane(id)
	_, shown := a.takeShot().frames[id]
	return t != nil && t == a.currentTab() && shown
}

func (a *App) notify(text string) {
	switch a.config.Notify {
	case config.NotifyOff:
	case config.NotifyBell:
		a.toTerminal("\a")
	default:
		if a.client != nil && envValue(a.client.hello.Env, "SSH_CONNECTION") != "" || a.desktopNotify == nil || !a.desktopNotify("noct", text) {
			a.toTerminal("\x1b]9;noct: " + strings.ReplaceAll(text, "\x1b", "") + "\x07")
		}
	}
}

// toTerminal sends s to the attached terminal with the next frame.
func (a *App) toTerminal(s string) {
	if a.screen != nil {
		_, _ = a.screen.WriteString(s)
		a.notifyScreenChanged()
	}
}

// desktopNotification shows a notification with the desktop's own tool, telling whether there is one.
func desktopNotification(title, body string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript", "-e", "on run argv\ndisplay notification (item 2 of argv) with title (item 1 of argv)\nend run", title, body)
	default:
		path, err := exec.LookPath("notify-send")
		if err != nil {
			return false
		}
		cmd = exec.Command(path, "--app-name=noct", title, body)
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { _ = cmd.Wait() }() // only reaps it: a notification that fails tells nothing
	return true
}
