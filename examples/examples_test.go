package examples

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/script"
)

// TestInstalledDemoLoads installs the demo and checks every script and flow loads, with no
// scope warnings, the way `noct check` does.
func TestInstalledDemoLoads(t *testing.T) {
	scriptsDir := filepath.Join(t.TempDir(), "scripts")
	flowsDir := filepath.Join(t.TempDir(), "flows")

	written, err := InstallDemo(scriptsDir, flowsDir)
	if err != nil {
		t.Fatalf("InstallDemo: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("InstallDemo wrote nothing")
	}

	lib := script.LoadLibrary(scriptsDir, nil, "")
	if len(lib.Problems) > 0 {
		t.Fatalf("library problems: %v", lib.Problems)
	}
	if broken := lib.Broken(); len(broken) > 0 {
		t.Fatalf("broken scripts: %v", broken)
	}
	if len(lib.All()) == 0 {
		t.Fatal("no scripts loaded")
	}

	flows, err := flow.LoadAll(flowsDir, lib.Find)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(flows) == 0 {
		t.Fatal("no flows loaded")
	}
	for _, f := range flows {
		if f.Problem != nil {
			t.Errorf("flow %s: %v", f.Name, f.Problem)
		}
		for _, w := range f.Warnings(lib.Find) {
			t.Errorf("flow %s, step %s: %s", f.Name, w.Path, w.Text)
		}
	}
}

// TestSecondInstallKeepsEdits checks that installing again writes nothing and leaves a file the
// user changed between installs alone.
func TestSecondInstallKeepsEdits(t *testing.T) {
	scriptsDir := filepath.Join(t.TempDir(), "scripts")
	flowsDir := filepath.Join(t.TempDir(), "flows")

	written, err := InstallDemo(scriptsDir, flowsDir)
	if err != nil {
		t.Fatalf("first InstallDemo: %v", err)
	}

	edited := written[0]
	const mark = "\n# edited by the user\n"
	original, err := os.ReadFile(edited)
	if err != nil {
		t.Fatalf("reading %s: %v", edited, err)
	}
	if err := os.WriteFile(edited, append(original, mark...), 0o644); err != nil {
		t.Fatalf("editing %s: %v", edited, err)
	}

	again, err := InstallDemo(scriptsDir, flowsDir)
	if err != nil {
		t.Fatalf("second InstallDemo: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("second install wrote %v, want nothing", again)
	}

	after, err := os.ReadFile(edited)
	if err != nil {
		t.Fatalf("re-reading %s: %v", edited, err)
	}
	if string(after) != string(original)+mark {
		t.Errorf("second install overwrote the user's edit to %s", edited)
	}
}
