package app

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
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/ipc"
	"github.com/AlexandreSJ/noct/internal/update"
)

const releasesJSON = `[
	{"tag_name": "v0.1.0", "body": "- first beta", "published_at": "2026-10-16T12:00:00Z"},
	{"tag_name": "v0.2.0", "body": "- faster panes\r\n- a fix", "published_at": "2026-11-01T12:00:00Z"}
]`

// updatesApp is a test app at version that finds releasesJSON, can install v0.2.0 over a file
// of its own; runLoopTask runs what its background work sends to the loop.
func updatesApp(t *testing.T, version string) *App {
	t.Helper()
	a := testApp(t, sideBySideTab(1, 2))
	a.version = version
	a.loopTasks, a.stopped = make(chan func()), make(chan struct{})
	t.Cleanup(func() { close(a.stopped) })

	archive := testArchive(t, []byte("noct v0.2.0"))
	name := update.ArchiveName("v0.2.0", runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/releases":
			_, _ = w.Write([]byte(releasesJSON))
		case "/download/v0.2.0/" + name:
			_, _ = w.Write(archive)
		case "/download/v0.2.0/checksums.txt":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + name + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	a.updateSource = update.Source{API: server.URL + "/api", Downloads: server.URL + "/download", Client: server.Client()}
	a.updateTarget = filepath.Join(t.TempDir(), "noct")
	if err := os.WriteFile(a.updateTarget, []byte("noct "+version), 0o755); err != nil {
		t.Fatal(err)
	}
	return a
}

func testArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zipped := gzip.NewWriter(&out)
	archive := tar.NewWriter(zipped)
	if err := archive.WriteHeader(&tar.Header{Name: "noct", Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	_, _ = archive.Write(binary)
	_ = archive.Close()
	_ = zipped.Close()
	return out.Bytes()
}

func TestPrefixCChecksAndShowsTheChangelog(t *testing.T) {
	a := updatesApp(t, "v0.1.0")
	press(a, prefixPress, letter('c'))
	if a.mode != updatesMode || !a.updates.checking {
		t.Fatalf("mode=%v checking=%v after prefix+c, want the updates popup checking", a.mode, a.updates.checking)
	}
	runLoopTask(t, a)

	text := screenText(a)
	for _, want := range []string{"updates · noct v0.1.0", "v0.2.0 is out: u installs it", "v0.2.0 · Nov 1, 2026 · new", "- faster panes", "v0.1.0 · Oct 16, 2026 · this version", "- first beta", "c check  u update"} {
		if !strings.Contains(text, want) {
			t.Errorf("popup lacks %q:\n%s", want, text)
		}
	}
	if !strings.Contains(firstLine(text), "noct v0.1.0 ↑ v0.2.0") {
		t.Errorf("tab bar = %q, want the version and the newer one", firstLine(text))
	}
	press(a, letter('q'))
	if a.mode != normalMode {
		t.Errorf("q left mode %v", a.mode)
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

func TestUpdateInstallsThenRestarts(t *testing.T) {
	a := updatesApp(t, "v0.1.0")
	a.client = &client{} // restarting needs a terminal to start the new binary
	press(a, prefixPress, letter('c'))
	runLoopTask(t, a)
	press(a, letter('u'))
	if !a.updates.installing {
		t.Fatal("u did not start installing")
	}
	runLoopTask(t, a)
	if content, _ := os.ReadFile(a.updateTarget); string(content) != "noct v0.2.0" {
		t.Fatalf("binary holds %q after u, want v0.2.0; problem: %s", content, a.updates.problem)
	}
	if text := screenText(a); !strings.Contains(firstLine(text), "v0.2.0 ready: restart") || !strings.Contains(text, "v0.2.0 is installed: r restarts noct") {
		t.Errorf("screen after installing:\n%s", text)
	}

	press(a, letter('r'), letter('n'))
	if a.mode != updatesMode || a.quitting {
		t.Errorf("mode=%v quitting=%v after declining the restart, want the popup back", a.mode, a.quitting)
	}
	press(a, letter('r'), uv.KeyPressEvent{Code: uv.KeyEnter})
	if !a.quitting || a.exitReason != ipc.Restart {
		t.Errorf("quitting=%v reason=%q after accepting the restart", a.quitting, a.exitReason)
	}
}

func TestUpdateShowsTheCommandWhenTheFolderIsReadOnly(t *testing.T) {
	a := updatesApp(t, "v0.1.0")
	a.updateTarget = filepath.Join(t.TempDir(), "missing", "noct") // a folder noct cannot write
	press(a, prefixPress, letter('c'))
	runLoopTask(t, a)
	press(a, letter('u'))
	if a.updates.installing || !strings.Contains(a.updates.manual, "sudo install -m 755") {
		t.Fatalf("installing=%v manual=%q, want the command", a.updates.installing, a.updates.manual)
	}
	if text := screenText(a); !strings.Contains(text, "run this in a terminal") || !strings.Contains(text, "y copies it") {
		t.Errorf("popup:\n%s", text)
	}
	press(a, letter('y'))
	if !a.updates.copied {
		t.Error("y did not copy the command")
	}
}

func TestNewestReleaseSaysSo(t *testing.T) {
	a := updatesApp(t, "v0.2.0")
	press(a, prefixPress, letter('c'))
	runLoopTask(t, a)
	text := screenText(a)
	if !strings.Contains(text, "noct v0.2.0 is the newest release") || strings.Contains(text, "· new") || strings.Contains(text, "u update") {
		t.Errorf("popup:\n%s", text)
	}
	if strings.Contains(firstLine(text), "↑") {
		t.Errorf("tab bar = %q, want no newer version", firstLine(text))
	}
}

func TestChecksOnItsOwnOnlyForReleasesWithChecksOn(t *testing.T) {
	a := updatesApp(t, "v0.1.0")
	if !a.checksOnItsOwn() {
		t.Error("a release with the default config does not check")
	}
	a.config.Update.Check = false
	if a.checksOnItsOwn() {
		t.Error("checks with the config off")
	}
	a.config.Update.Check, a.version = true, update.Dev
	if a.checksOnItsOwn() {
		t.Error("a dev build checks")
	}
}

func TestAltCommaOpensTheConfig(t *testing.T) {
	a := updatesApp(t, "v0.1.0")
	press(a, uv.KeyPressEvent{Code: ',', Mod: uv.ModAlt})
	if a.mode != configMode {
		t.Errorf("alt+, opened mode %v, want the config", a.mode)
	}
}
