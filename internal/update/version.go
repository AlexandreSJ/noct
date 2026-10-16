// Package update finds noct's releases on GitHub, with the changelog of each, and replaces the
// noct binary with a newer release.
package update

import (
	"strconv"
	"strings"
)

// Dev is the version of a binary built without one, as by go build in a checkout.
const Dev = "dev"

// semver is a version as vMAJOR.MINOR.PATCH, with an optional -prerelease; +build is ignored.
type semver struct {
	numbers    [3]int
	prerelease []string
}

func parseSemver(version string) (semver, bool) {
	rest, ok := strings.CutPrefix(version, "v")
	if !ok {
		return semver{}, false
	}
	rest, _, _ = strings.Cut(rest, "+")
	core, prerelease, hasPrerelease := strings.Cut(rest, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 || hasPrerelease && prerelease == "" {
		return semver{}, false
	}
	var v semver
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return semver{}, false
		}
		v.numbers[i] = n
	}
	if hasPrerelease {
		v.prerelease = strings.Split(prerelease, ".")
	}
	return v, true
}

// IsRelease tells whether version can be compared with the releases: a dev build cannot.
func IsRelease(version string) bool {
	_, ok := parseSemver(version)
	return ok
}

// Compare orders two versions as semantic versioning does: negative when a is older than b,
// positive when newer, zero when equal. A version that cannot be read is older than any other.
func Compare(a, b string) int {
	va, okA := parseSemver(a)
	vb, okB := parseSemver(b)
	if !okA || !okB {
		return boolOrder(okA) - boolOrder(okB)
	}
	for i := range va.numbers {
		if va.numbers[i] != vb.numbers[i] {
			return va.numbers[i] - vb.numbers[i]
		}
	}
	return comparePrerelease(va.prerelease, vb.prerelease)
}

func boolOrder(ok bool) int {
	if ok {
		return 1
	}
	return 0
}

// comparePrerelease orders prereleases: none comes after any, numbers before words, and a
// longer list after a shorter one it starts with.
func comparePrerelease(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return len(b) - len(a)
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := compareIdentifier(a[i], b[i]); c != 0 {
			return c
		}
	}
	return len(a) - len(b)
}

func compareIdentifier(a, b string) int {
	na, errA := strconv.Atoi(a)
	nb, errB := strconv.Atoi(b)
	switch {
	case errA == nil && errB == nil:
		return na - nb
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}
	return strings.Compare(a, b)
}
