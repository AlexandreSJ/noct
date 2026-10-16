package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/update"
)

// releasesServer publishes v0.1.0 and v0.2.0, with a build of v0.2.0 for this system.
func releasesServer(t *testing.T) update.Source {
	t.Helper()
	var archive bytes.Buffer
	zipped := gzip.NewWriter(&archive)
	files := tar.NewWriter(zipped)
	binary := []byte("noct v0.2.0")
	_ = files.WriteHeader(&tar.Header{Name: "noct", Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg})
	_, _ = files.Write(binary)
	_ = files.Close()
	_ = zipped.Close()
	name := update.ArchiveName("v0.2.0", runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(archive.Bytes())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/releases":
			_, _ = w.Write([]byte(`[{"tag_name": "v0.2.0", "body": "- faster"}, {"tag_name": "v0.1.0", "body": "- first beta"}]`))
		case "/download/v0.2.0/" + name:
			_, _ = w.Write(archive.Bytes())
		case "/download/v0.2.0/checksums.txt":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return update.Source{API: server.URL + "/api", Downloads: server.URL + "/download", Client: server.Client()}
}

func installedBinary(t *testing.T) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "noct")
	if err := os.WriteFile(target, []byte("noct v0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	return target
}

func TestUpdateInstallsOnceTheUserAgrees(t *testing.T) {
	target := installedBinary(t)
	var out strings.Builder
	if err := updateNoct(releasesServer(t), "v0.1.0", target, strings.NewReader("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "v0.2.0\n- faster") || strings.Contains(out.String(), "first beta") {
		t.Errorf("output lacks only the newer changelog:\n%s", out.String())
	}
	if content, _ := os.ReadFile(target); string(content) != "noct v0.2.0" {
		t.Errorf("binary holds %q, want v0.2.0", content)
	}
}

func TestUpdateWithoutAYesChangesNothing(t *testing.T) {
	target := installedBinary(t)
	var out strings.Builder
	if err := updateNoct(releasesServer(t), "v0.1.0", target, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(target); string(content) != "noct v0.1.0" || !strings.Contains(out.String(), "not updated") {
		t.Errorf("binary holds %q, output:\n%s", content, out.String())
	}
}

func TestUpdateOnTheNewestRelease(t *testing.T) {
	var out strings.Builder
	if err := updateNoct(releasesServer(t), "v0.2.0", installedBinary(t), strings.NewReader("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "noct v0.2.0 is the newest release\n" {
		t.Errorf("output = %q", out.String())
	}
}

func TestUpdatePrintsTheCommandForAFolderItCannotWrite(t *testing.T) {
	target := filepath.Join(t.TempDir(), "missing", "noct")
	var out strings.Builder
	if err := updateNoct(releasesServer(t), "v0.1.0", target, strings.NewReader("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "sudo install -m 755 /tmp/noct "+target) {
		t.Errorf("output lacks the command:\n%s", out.String())
	}
}

func TestUpdateRefusesADevBuild(t *testing.T) {
	err := updateNoct(releasesServer(t), update.Dev, installedBinary(t), strings.NewReader("y\n"), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "development build") {
		t.Errorf("err = %v", err)
	}
}

func TestModuleVersionIsDevForACheckoutBuild(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"go install of a tag", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0", Sum: "h1:abc="}}, "v0.1.0"},
		{"go install of a commit", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1-0.20261016160344-08af8c875876", Sum: "h1:abc="}}, "v0.1.1-0.20261016160344-08af8c875876"},
		{"go build in a checkout", &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261016160344-08af8c875876+dirty"}}, update.Dev},
		{"go build without VCS", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, update.Dev},
		{"no build info", nil, update.Dev},
	}
	for _, test := range tests {
		if got := moduleVersion(test.info); got != test.want {
			t.Errorf("%s: moduleVersion = %q, want %q", test.name, got, test.want)
		}
	}
}
