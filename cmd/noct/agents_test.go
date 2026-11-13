package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/config"
)

// noct agents default <name> writes agent = <name> into the config, keeping the rest; it refuses a
// name no agent has, and default itself.
func TestSetDefaultAgentWritesTheConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("agent = \"claude\"\nmouse = false\n[agents]\nmine = \"cat\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	if err := setDefaultAgent(out, path, "mine"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil || cfg.Agent != "mine" || cfg.Mouse || cfg.Agents["mine"] != "cat" {
		t.Fatalf("config %+v, %v", cfg, err)
	}
	if !strings.Contains(out.String(), `Saved agent = "mine"`) {
		t.Errorf("output: %s", out)
	}
	for _, name := range []string{"nobody", "default"} {
		if err := setDefaultAgent(out, path, name); err == nil {
			t.Errorf("%s was saved", name)
		}
	}
	if cfg, _ := config.Load(path); cfg.Agent != "mine" {
		t.Errorf("a refused name changed the config: %s", cfg.Agent)
	}
}
