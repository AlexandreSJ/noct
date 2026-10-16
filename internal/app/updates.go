package app

import (
	"context"
	"errors"
	"time"

	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/update"
)

// The daemon looks for new releases once a day, unless the config turns that off, and keeps what
// it found in updates.json, so a restart or the popup does not ask GitHub again before then; the
// updates popup (prefix+c) checks on demand, installs a release once the user asks, and restarts
// noct into it.

const (
	// updateCheckInterval is how old the last check gets before the daemon checks again; it
	// looks every updateDueTick, so a computer that slept checks soon after waking.
	updateCheckInterval  = 24 * time.Hour
	updateDueTick        = time.Hour
	updateCheckTimeout   = 30 * time.Second
	updateInstallTimeout = 5 * time.Minute
	// updateCheckCooldown keeps checks apart, so holding c cannot spend GitHub's limit of 60
	// requests an hour.
	updateCheckCooldown = time.Minute
)

// updates is what noct knows of its releases.
type updates struct {
	// releases are the published releases, the newest first, as the last check found them.
	releases []update.Release
	checked  bool
	checking bool
	// lastChecked is when GitHub last answered, and etag what it answered, to ask next time
	// only whether the releases changed.
	lastChecked time.Time
	etag        string
	// nextCheck is when a check may start again: after the cooldown, or when GitHub's limit
	// resets.
	nextCheck time.Time
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

// checkIfDue checks on its own once the last check is a day old.
func (a *App) checkIfDue() {
	if a.checksOnItsOwn() && time.Since(a.updates.lastChecked) >= updateCheckInterval {
		a.checkForUpdates()
	}
}

// loadUpdateCache takes what the last check found, so the releases show without asking again.
func (a *App) loadUpdateCache() {
	cache, err := update.LoadCache(a.updateCachePath)
	if err != nil {
		a.statusMessage = err.Error()
		return
	}
	a.updates.releases, a.updates.lastChecked, a.updates.etag = cache.Releases, cache.Checked, cache.ETag
	a.updates.checked, a.updates.nextCheck = !cache.Checked.IsZero(), cache.WaitUntil
}

// saveUpdateCache keeps what the last check found for the next run.
func (a *App) saveUpdateCache() {
	if a.updateCachePath == "" {
		return
	}
	cache := update.Cache{Releases: a.updates.releases, Checked: a.updates.lastChecked, ETag: a.updates.etag}
	if a.updates.nextCheck.After(time.Now().Add(updateCheckCooldown)) {
		cache.WaitUntil = a.updates.nextCheck // GitHub's limit, which outlasts a restart
	}
	if err := update.SaveCache(a.updateCachePath, cache); err != nil {
		a.updates.problem = err.Error()
	}
}

// newestRelease is the newest release after this one, if there is one.
func (a *App) newestRelease() (update.Release, bool) {
	newer := update.Newer(a.updates.releases, a.version)
	if len(newer) == 0 {
		return update.Release{}, false
	}
	return newer[0], true
}

// canCheck tells whether a check may start now.
func (a *App) canCheck() bool {
	return !a.updates.checking && !time.Now().Before(a.updates.nextCheck)
}

// checkForUpdates lists the releases in the background, unless a check ran too recently.
func (a *App) checkForUpdates() {
	if !a.canCheck() {
		return
	}
	a.updates.checking, a.updates.problem = true, ""
	a.updates.nextCheck = time.Now().Add(updateCheckCooldown)
	source, etag := a.updateSource, a.updates.etag
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
		defer cancel()
		listing, err := source.Releases(ctx, etag)
		a.onLoop(func() { a.checked(listing, err) })
	}()
}

// checked keeps what a check found, and saves it.
func (a *App) checked(listing update.Listing, err error) {
	a.updates.checking = false
	if limited := (*update.RateLimited)(nil); errors.As(err, &limited) && limited.Reset.After(a.updates.nextCheck) {
		a.updates.nextCheck = limited.Reset
		a.saveUpdateCache()
	}
	if err != nil {
		a.updates.problem = err.Error()
		return
	}
	if !listing.Unchanged {
		a.updates.releases = listing.Releases
	}
	a.updates.checked, a.updates.lastChecked, a.updates.etag = true, time.Now(), listing.ETag
	a.saveUpdateCache()
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
