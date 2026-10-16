package app

import (
	"context"
	"time"

	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/update"
)

// The daemon looks for new releases when it starts and once a day after, unless the config
// turns that off; the updates popup (prefix+c) checks on demand, installs a release once the
// user asks, and restarts noct into it.

const (
	updateCheckInterval  = 24 * time.Hour
	updateCheckTimeout   = 30 * time.Second
	updateInstallTimeout = 5 * time.Minute
)

// updates is what noct knows of its releases.
type updates struct {
	// releases are the published releases, the newest first, as the last check found them.
	releases []update.Release
	checked  bool
	checking bool
	// problem says why the last check or install failed.
	problem    string
	installing bool
	// installed is the release written over the binary, which runs once noct restarts.
	installed string
	// manual is the command that installs the newest release by hand, when this user cannot
	// write the binary's folder.
	manual string
	// first is the popup's first line on screen; visible and lines are how many it showed and
	// had, as drawn.
	first, visible, lines int
	copied                bool
}

// checksOnItsOwn tells whether noct looks for releases without being asked: a dev build has no
// version to compare.
func (a *App) checksOnItsOwn() bool {
	return a.config.Update.Check && update.IsRelease(a.version)
}

// newestRelease is the newest release after this one, if there is one.
func (a *App) newestRelease() (update.Release, bool) {
	newer := update.Newer(a.updates.releases, a.version)
	if len(newer) == 0 {
		return update.Release{}, false
	}
	return newer[0], true
}

// checkForUpdates lists the releases in the background.
func (a *App) checkForUpdates() {
	if a.updates.checking {
		return
	}
	a.updates.checking, a.updates.problem = true, ""
	source := a.updateSource
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		releases, err := source.Releases(ctx)
		a.onLoop(func() { a.checked(releases, err) })
	}()
}

// checked keeps what a check found.
func (a *App) checked(releases []update.Release, err error) {
	a.updates.checking = false
	if err != nil {
		a.updates.problem = err.Error()
		return
	}
	a.updates.releases, a.updates.checked = releases, true
}

// installUpdate writes the newest release over the binary in the background or, when this user
// cannot, shows the command that does it by hand.
func (a *App) installUpdate() {
	newest, ok := a.newestRelease()
	if !ok || a.updates.installing || a.updates.installed == newest.Version {
		return
	}
	if a.updateTarget == "" {
		a.updates.problem = "cannot find the noct executable to replace"
		return
	}
	if !update.CanReplace(a.updateTarget) {
		a.updates.manual = a.updateSource.ManualCommand(newest.Version, a.updateTarget)
		return
	}
	a.updates.installing, a.updates.problem = true, ""
	source, target := a.updateSource, a.updateTarget
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), updateInstallTimeout)
		defer cancel()
		err := source.Install(ctx, newest.Version, target)
		a.onLoop(func() { a.installed(newest.Version, err) })
	}()
}

// installed keeps the outcome of an install.
func (a *App) installed(version string, err error) {
	a.updates.installing = false
	if err != nil {
		a.updates.problem = err.Error()
		return
	}
	a.updates.installed = version
	a.binary = currentBinary() // the update says to restart; the rebuilt notice would repeat it
}

// askToRestart restarts noct into the release installed, once the user agrees: the sessions are
// saved and the attached terminal starts the new binary, which opens them again.
func (a *App) askToRestart() {
	if a.updates.installed == "" || a.client == nil {
		return
	}
	question := "restart noct into " + a.updates.installed + "? programs in panes stop; the sessions open again as they were"
	a.askToConfirm(question, func() {
		a.exitReason = ipc.Restart
		a.quitting = true
	})
}
