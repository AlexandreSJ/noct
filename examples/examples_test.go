package examples

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexandreSJ/noct/internal/flow"
	"github.com/AlexandreSJ/noct/internal/script"
)

// TestInstalledDemoLoads installs the demo and checks every script, flow and function loads, with no
// scope warnings, the way `noct check` does.
func TestInstalledDemoLoads(t *testing.T) {
	scriptsDir := filepath.Join(t.TempDir(), "scripts")
	flowsDir := filepath.Join(t.TempDir(), "flows")
	functionsDir := filepath.Join(t.TempDir(), "functions")

	written, err := InstallDemo(scriptsDir, flowsDir, functionsDir)
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

	flows, functions, err := flow.LoadWithFunctions(flowsDir, functionsDir, lib.Find)
	if err != nil {
		t.Fatalf("LoadWithFunctions: %v", err)
	}
	if len(flows) == 0 || len(functions) == 0 {
		t.Fatalf("loaded %d flows and %d functions", len(flows), len(functions))
	}
	for _, f := range append(flows, functions...) {
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
	functionsDir := filepath.Join(t.TempDir(), "functions")

	written, err := InstallDemo(scriptsDir, flowsDir, functionsDir)
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

	again, err := InstallDemo(scriptsDir, flowsDir, functionsDir)
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

// A demo flow an older noct installed outside the demo folder moves into it, edits and all.
func TestADemoInstalledOutsideItsFolderMovesIntoIt(t *testing.T) {
	scriptsDir := filepath.Join(t.TempDir(), "scripts")
	flowsDir := filepath.Join(t.TempDir(), "flows")
	functionsDir := filepath.Join(t.TempDir(), "functions")
	if err := os.MkdirAll(flowsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(flowsDir, "demo-deploy.yaml")
	if err := os.WriteFile(old, []byte("# edited\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallDemo(scriptsDir, flowsDir, functionsDir); err != nil {
		t.Fatal(err)
	}
	moved, err := os.ReadFile(filepath.Join(flowsDir, "demo", "demo-deploy.yaml"))
	if err != nil || string(moved) != "# edited\nsteps: []\n" {
		t.Errorf("the old demo flow did not move with its edits: %q, %v", moved, err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("the old demo flow is still outside the folder")
	}
}
