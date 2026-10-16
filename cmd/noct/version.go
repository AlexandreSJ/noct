package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/AlexandreSJ/noct/internal/update"
)

// version is the release this binary was built as, set by the release build with
// -ldflags "-X main.version=v0.1.0"; empty in any other build.
var version string

// updateTimeout bounds noct update: listing the releases and downloading one.
const updateTimeout = 5 * time.Minute

// noctVersion is the release the binary was built as: the one the release build set, or the
// module version go install recorded, or dev.
func noctVersion() string {
	if version != "" {
		return version
	}
	info, _ := debug.ReadBuildInfo()
	return moduleVersion(info)
}

// moduleVersion is the version go install recorded when it fetched noct as a module, which has
// a checksum. A build in a checkout records a pseudo-version of its commit too (v0.0.0-…+dirty),
// but it is a dev build: updating would replace it with a release.
func moduleVersion(info *debug.BuildInfo) string {
	if info == nil || info.Main.Sum == "" || !update.IsRelease(info.Main.Version) {
		return update.Dev
	}
	return info.Main.Version
}

func updateCommand() error {
	target, err := update.Executable()
	if err != nil {
		return err
	}
	return updateNoct(update.GitHub(), noctVersion(), target, os.Stdin, os.Stdout)
}

// updateNoct shows the changelog of the releases newer than current and, once the user agrees,
// installs the newest over target. Without write access to target's folder, it prints the
// command that installs it with sudo instead.
func updateNoct(source update.Source, current, target string, stdin io.Reader, stdout io.Writer) error {
	if !update.IsRelease(current) {
		return fmt.Errorf("this noct is a development build (%s); releases are at https://github.com/%s/releases", current, update.Repository)
	}
	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()
	listing, err := source.Releases(ctx, "")
	if err != nil {
		return err
	}
	newer := update.Newer(listing.Releases, current)
	if len(newer) == 0 {
		fmt.Fprintf(stdout, "noct %s is the newest release\n", current)
		return nil
	}
	for _, r := range newer {
		fmt.Fprintf(stdout, "%s\n%s\n\n", r.Version, strings.TrimSpace(r.Notes))
	}
	latest := newer[0].Version
	if !update.CanReplace(target) {
		fmt.Fprintf(stdout, "%s is not writable by you; to update, run:\n\n  %s\n\nthen restart noct (noct quit, then noct).\n",
			filepath.Dir(target), source.ManualCommand(latest, target))
		return nil
	}
	fmt.Fprintf(stdout, "update noct %s to %s? [y/N] ", current, latest)
	answer, _ := bufio.NewReader(stdin).ReadString('\n') // no answer is a no
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		fmt.Fprintln(stdout, "not updated")
		return nil
	}
	if err := source.Install(ctx, latest, target); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "noct %s installed; a running noct keeps %s until it restarts (noct quit, then noct)\n", latest, current)
	return nil
}
