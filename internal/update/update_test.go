package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int // sign
	}{
		{"v0.1.0", "v0.1.0", 0},
		{"v0.2.0", "v0.1.9", 1},
		{"v0.10.0", "v0.9.0", 1},
		{"v1.0.0", "v0.99.99", 1},
		{"v0.1.0-beta", "v0.1.0", -1},
		{"v0.1.0-beta.2", "v0.1.0-beta.10", -1},
		{"v0.1.0-beta", "v0.1.0-beta.1", -1},
		{"v0.1.0-1", "v0.1.0-alpha", -1},
		{"v0.1.0+build", "v0.1.0", 0},
		{"v0.1.1-0.20261016120000-abcdef123456", "v0.1.0", 1}, // a pseudo-version after v0.1.0
		{"v0.1.1-0.20261016120000-abcdef123456", "v0.1.1", -1},
		{"dev", "v0.1.0", -1},
		{"v0.1.0", "dev", 1},
		{"dev", "dev", 0},
	}
	for _, test := range tests {
		got := Compare(test.a, test.b)
		if sign(got) != test.want {
			t.Errorf("Compare(%q, %q) = %d, want sign %d", test.a, test.b, got, test.want)
		}
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

func TestIsRelease(t *testing.T) {
	for version, want := range map[string]bool{
		"v0.1.0": true, "v1.2.3-rc.1": true, "dev": false, "0.1.0": false, "v1.2": false, "v1.2.3-": false, "": false,
	} {
		if got := IsRelease(version); got != want {
			t.Errorf("IsRelease(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestReleasesNewestFirstWithoutDrafts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`[
			{"tag_name": "v0.1.0", "body": "first beta", "published_at": "2026-10-16T12:00:00Z"},
			{"tag_name": "v0.3.0", "body": "draft", "draft": true},
			{"tag_name": "nightly", "body": "not a version"},
			{"tag_name": "v0.2.0", "body": "- faster", "published_at": "2026-11-01T12:00:00Z"}
		]`))
	}))
	defer server.Close()
	source := Source{API: server.URL, Client: server.Client()}

	listing, err := source.Releases(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	releases := listing.Releases
	if len(releases) != 2 || releases[0].Version != "v0.2.0" || releases[1].Version != "v0.1.0" {
		t.Fatalf("releases = %+v, want v0.2.0 then v0.1.0", releases)
	}
	if releases[0].Notes != "- faster" || releases[0].Published.Month() != 11 {
		t.Errorf("v0.2.0 = %+v, want its notes and date", releases[0])
	}
	if newer := Newer(releases, "v0.1.0"); len(newer) != 1 || newer[0].Version != "v0.2.0" {
		t.Errorf("Newer(v0.1.0) = %+v, want v0.2.0", newer)
	}
	if newer := Newer(releases, Dev); newer != nil {
		t.Errorf("Newer(dev) = %+v, want none", newer)
	}
}

func TestReleasesReportsAFailedAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer server.Close()
	_, err := Source{API: server.URL, Client: server.Client()}.Releases(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v, want the status", err)
	}
}

// releaseServer serves a release's archive for this system with the given checksum list.
func releaseServer(t *testing.T, version string, binary []byte, sums func(archive []byte) string) Source {
	t.Helper()
	archive := tarGz(t, map[string][]byte{"README.md": []byte("readme"), "noct": binary})
	name := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + version + "/" + name:
			_, _ = w.Write(archive)
		case "/" + version + "/checksums.txt":
			_, _ = w.Write([]byte(sums(archive)))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return Source{Downloads: server.URL, Client: server.Client()}
}

func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zipped := gzip.NewWriter(&out)
	archive := tar.NewWriter(zipped)
	for name, content := range files {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func checksumLine(archive []byte, name string) string {
	sum := sha256.Sum256(archive)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

func TestInstallReplacesTheBinary(t *testing.T) {
	name := ArchiveName("v0.2.0", runtime.GOOS, runtime.GOARCH)
	source := releaseServer(t, "v0.2.0", []byte("new noct"), func(archive []byte) string {
		return checksumLine([]byte("other"), "noct_v0.2.0_plan9_386.tar.gz") + checksumLine(archive, name)
	})
	target := filepath.Join(t.TempDir(), "noct")
	if err := os.WriteFile(target, []byte("old noct"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := source.Install(context.Background(), "v0.2.0", target); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	if string(content) != "new noct" {
		t.Errorf("target holds %q, want the new binary", content)
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("target mode = %v, want executable", info.Mode())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".noct-*")); len(leftovers) > 0 {
		t.Errorf("left %v behind", leftovers)
	}
}

func TestInstallRefusesAWrongChecksum(t *testing.T) {
	name := ArchiveName("v0.2.0", runtime.GOOS, runtime.GOARCH)
	source := releaseServer(t, "v0.2.0", []byte("tampered"), func([]byte) string {
		return checksumLine([]byte("what was published"), name)
	})
	target := filepath.Join(t.TempDir(), "noct")
	if err := os.WriteFile(target, []byte("old noct"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := source.Install(context.Background(), "v0.2.0", target)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v, want a checksum mismatch", err)
	}
	if content, _ := os.ReadFile(target); string(content) != "old noct" {
		t.Errorf("target holds %q, want it untouched", content)
	}
}

func TestInstallWithoutABuildForThisSystem(t *testing.T) {
	source := releaseServer(t, "v0.2.0", []byte("new noct"), func(archive []byte) string {
		return checksumLine(archive, "noct_v0.2.0_plan9_386.tar.gz")
	})
	err := source.Install(context.Background(), "v0.2.0", filepath.Join(t.TempDir(), "noct"))
	if err == nil || !strings.Contains(err.Error(), "no build for") {
		t.Errorf("err = %v, want no build for this system", err)
	}
}

func TestCanReplace(t *testing.T) {
	dir := t.TempDir()
	if !CanReplace(filepath.Join(dir, "noct")) {
		t.Error("CanReplace in a writable folder = false")
	}
	if CanReplace(filepath.Join(dir, "missing", "noct")) {
		t.Error("CanReplace in a missing folder = true")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("left %v behind", entries)
	}
}

func TestManualCommand(t *testing.T) {
	source := Source{Downloads: "https://example.com/releases/download"}
	got := source.ManualCommand("v0.2.0", "/opt/my tools/noct")
	want := "curl -fsSL https://example.com/releases/download/v0.2.0/" + ArchiveName("v0.2.0", runtime.GOOS, runtime.GOARCH) +
		" | tar -xzf - -C /tmp noct && sudo install -m 755 /tmp/noct '/opt/my tools/noct'"
	if got != want {
		t.Errorf("ManualCommand =\n%s\nwant\n%s", got, want)
	}
}

func TestReleasesTellsWhenGitHubLimitsTheChecks(t *testing.T) {
	reset := time.Now().Add(20 * time.Minute).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
		http.Error(w, "API rate limit exceeded", http.StatusForbidden)
	}))
	defer server.Close()
	_, err := Source{API: server.URL, Client: server.Client()}.Releases(context.Background(), "")
	var limited *RateLimited
	if !errors.As(err, &limited) || !limited.Reset.Equal(reset) {
		t.Errorf("err = %v, want rate limited until %v", err, reset)
	}
}

func TestReleasesAsksOnlyWhetherTheyChanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(`[{"tag_name": "v0.1.0", "body": "first beta"}]`))
	}))
	defer server.Close()
	source := Source{API: server.URL, Client: server.Client()}

	first, err := source.Releases(context.Background(), "")
	if err != nil || first.Unchanged || first.ETag != `"v1"` || len(first.Releases) != 1 {
		t.Fatalf("first = %+v, %v; want the release and its ETag", first, err)
	}
	again, err := source.Releases(context.Background(), first.ETag)
	if err != nil || !again.Unchanged || again.ETag != `"v1"` || again.Releases != nil {
		t.Errorf("again = %+v, %v; want unchanged", again, err)
	}
}

func TestCacheKeepsTheLastCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", CacheFile)
	if cache, err := LoadCache(path); err != nil || !cache.Checked.IsZero() {
		t.Fatalf("missing cache = %+v, %v; want empty", cache, err)
	}
	want := Cache{
		Releases: []Release{{Version: "v0.1.0", Notes: "- first beta", Published: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)}},
		Checked:  time.Date(2026, 10, 16, 13, 0, 0, 0, time.UTC),
		ETag:     `"v1"`,
	}
	if err := SaveCache(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadCache(path)
	if err != nil || len(got.Releases) != 1 || got.Releases[0] != want.Releases[0] || !got.Checked.Equal(want.Checked) || got.ETag != want.ETag || !got.WaitUntil.IsZero() {
		t.Errorf("loaded %+v, %v; want %+v", got, err, want)
	}
}
