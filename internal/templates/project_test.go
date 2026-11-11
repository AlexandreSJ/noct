package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAProjectsTemplatesAreFoundAboveAndAskEveryCommand(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".noct", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "layout: columns\npanes:\n  - terminal: { run: make dev, start: run }\n  - rows:\n      - terminal: { run: tail -f log, start: run }\n      - logs: {}\n"
	if err := os.WriteFile(filepath.Join(dir, "dev.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "cmd", "app")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := ProjectDir(inside); got != dir {
		t.Fatalf("ProjectDir = %q, want %q", got, dir)
	}
	if got := ProjectDir(t.TempDir()); got != "" {
		t.Errorf("outside a project: %q", got)
	}
	entries := ListProject(dir)
	if len(entries) != 1 || !entries[0].Project || entries[0].Err != nil {
		t.Fatalf("entries %+v", entries)
	}
	var starts []Start
	var walk func(Node)
	walk = func(n Node) {
		if n.Pane != nil && n.Pane.Command != "" {
			starts = append(starts, n.Pane.Start)
		}
		for _, c := range n.children() {
			walk(c)
		}
	}
	walk(entries[0].Template.Root)
	if len(starts) != 2 || starts[0] != Ask || starts[1] != Ask {
		t.Errorf("commands start %v, want every one asked", starts)
	}
	again, _ := Load(dir, "dev")
	walk = func(n Node) {
		if n.Pane != nil && n.Pane.Command == "make dev" && n.Pane.Start != Run {
			t.Error("asking changed the template as the file has it")
		}
		for _, c := range n.children() {
			walk(c)
		}
	}
	walk(again.Root)
}
