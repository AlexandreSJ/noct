package flow

import (
	"slices"
	"testing"
)

// A flow renamed into a folder keeps its last result and its triggers turned on.
func TestARenamedFlowKeepsItsResultAndTriggers(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	f, err := Create(dir, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveResult(ResultsDir(), Result{Flow: "deploy", OK: true}); err != nil {
		t.Fatal(err)
	}
	if err := SetEnabled(TriggersFile(), "deploy", true); err != nil {
		t.Fatal(err)
	}
	moved, err := f.Rename(dir, "ops/deploy")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := LastResult(ResultsDir(), "ops/deploy"); !ok {
		t.Error("the last result stayed under the old name")
	}
	if on := Enabled(TriggersFile()); !on["ops/deploy"] || on["deploy"] {
		t.Errorf("triggers on: %v", on)
	}
	if _, err := moved.Copy(dir, "ops/deploy"); err == nil {
		t.Error("a copy over an existing flow was made")
	}
	if folders := Folders(dir); !slices.Equal(folders, []string{"ops"}) {
		t.Errorf("folders %v", folders)
	}
	if _, err := MoveFolder(dir, "ops", "ops/inner", nil); err == nil {
		t.Error("a folder went inside itself")
	}
}
