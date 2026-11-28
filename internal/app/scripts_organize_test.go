package app

import (
	"image"
	"os"
	"path/filepath"
	"slices"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/script"
)

func scriptIDs(a *App) []string {
	var ids []string
	for _, s := range a.scripts.All() {
		ids = append(ids, s.ID())
	}
	slices.Sort(ids)
	return ids
}

// N makes a directory in the selected one, and a / in its path makes the directories around it,
// as the inspect's N does for flows.
func TestNMakesADirectoryInsideTheSelectedOne(t *testing.T) {
	a := testApp(t)
	saveScriptIn(t, filepath.Join(script.PersonalDir(), "ops"), "backup", "run: true\n")
	panel := openScriptsPanel(t, a)
	panel.selectKey("folder:" + filepath.Join(script.PersonalDir(), "ops"))
	pressDrawn(a, letter('N'))
	if got := a.prompt.input.String(); got != "ops/" {
		t.Fatalf("the prompt starts with %q, want ops/", got)
	}
	typeDrawn(a, "db/daily")
	pressDrawn(a, enterPress)
	if info, err := os.Stat(filepath.Join(script.PersonalDir(), "ops", "db", "daily")); err != nil || !info.IsDir() {
		t.Fatalf("no directory ops/db/daily: %v; status %q", err, a.statusMessage)
	}
	if row, ok := panel.current(); !ok || row.folder == nil || row.folder.Name != "ops/db/daily" {
		t.Errorf("selected %+v, want the new directory", row)
	}
}

// m moves a script into directories, its remembered values and the flows that run it with it.
func TestMMovesAScriptWithWhatGoesByItsID(t *testing.T) {
	a := testApp(t)
	saveScript(t, "greet", greetScript)
	saveFlow(t, "morning", "steps: [{script: greet}]\n")
	a.reloadFlows()
	if err := a.memory.Remember(script.Script{Name: "greet", Params: []script.Param{{Name: "who", Remember: true}}}, map[string]string{"who": "ana"}); err != nil {
		t.Fatal(err)
	}
	panel := openScriptsPanel(t, a)
	panel.selectKey(filepath.Join(script.PersonalDir(), "greet"))
	pressDrawn(a, letter('m'))
	answerPrompt(a, "team/hello/greet")
	if got := scriptIDs(a); !slices.Equal(got, []string{"team/hello/greet"}) {
		t.Fatalf("scripts %v; status %q", got, a.statusMessage)
	}
	if steps := loadFlowSteps(t, "morning"); len(steps) != 1 || steps[0].Script != "team/hello/greet" {
		t.Errorf("morning runs %+v, want team/hello/greet", steps)
	}
	if got := a.memory.Values(script.Script{Name: "greet", Folder: "team/hello"})["who"]; got != "ana" {
		t.Errorf("the remembered who is %q, want ana", got)
	}
	if row, ok := panel.current(); !ok || row.script == nil || row.script.ID() != "team/hello/greet" {
		t.Errorf("selected %+v, want the moved script", row)
	}
}

// m on a directory moves it with its scripts; c copies a script.
func TestMMovesADirectoryAndCCopiesAScript(t *testing.T) {
	a := testApp(t)
	saveScriptIn(t, filepath.Join(script.PersonalDir(), "ops"), "backup", "run: true\n")
	saveFlow(t, "nightly", "steps: [{script: ops/backup}]\n")
	a.reloadFlows()
	panel := openScriptsPanel(t, a)
	panel.selectKey("folder:" + filepath.Join(script.PersonalDir(), "ops"))
	pressDrawn(a, letter('m'))
	answerPrompt(a, "infra/ops")
	if got := scriptIDs(a); !slices.Equal(got, []string{"infra/ops/backup"}) {
		t.Fatalf("scripts %v; status %q", got, a.statusMessage)
	}
	if steps := loadFlowSteps(t, "nightly"); steps[0].Script != "infra/ops/backup" {
		t.Errorf("nightly runs %q", steps[0].Script)
	}
	panel.selectKey(filepath.Join(script.PersonalDir(), "infra", "ops", "backup"))
	pressDrawn(a, letter('c'))
	if got := a.prompt.input.String(); got != "infra/ops/backup-copy" {
		t.Fatalf("the copy prompt starts with %q", got)
	}
	answerPrompt(a, "backup")
	if got := scriptIDs(a); !slices.Equal(got, []string{"backup", "infra/ops/backup"}) {
		t.Errorf("scripts %v; status %q", got, a.statusMessage)
	}
}

// Dragging a script onto a directory's row moves it there, as a flow is in the inspect.
func TestDraggingAScriptOntoADirectoryMovesIt(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveScriptIn(t, filepath.Join(script.PersonalDir(), "ops"), "backup", "run: true\n")
	saveScript(t, "hello", "run: true\n")
	panel := openScriptsPanel(t, a)
	drawnApp(a)
	frame, _ := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	origin := contentRect(frame).Min
	rowAt := func(key string) image.Point {
		for i, r := range panel.rows() {
			if r.key() == key {
				return origin.Add(image.Pt(panel.list.Min.X+2, panel.list.Min.Y+i-panel.first))
			}
		}
		t.Fatalf("no row %s in %v", key, rowLabels(panel))
		return image.Point{}
	}
	from, to := rowAt(filepath.Join(script.PersonalDir(), "hello")), rowAt("folder:"+filepath.Join(script.PersonalDir(), "ops"))
	a.handleEvent(uv.MouseClickEvent{X: from.X, Y: from.Y, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseMotionEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseReleaseEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft})
	if got := scriptIDs(a); !slices.Equal(got, []string{"ops/backup", "ops/hello"}) {
		t.Errorf("after the drop the scripts are %v; status %q", got, a.statusMessage)
	}
}

// A new script's name starts in the selected directory, and a / in it makes directories.
func TestANewScriptsPathMakesItsDirectories(t *testing.T) {
	a := testApp(t)
	a.config.Editor = "cat"
	l := recordLaunches(a)
	openScriptsPanel(t, a)
	press(a, letter('a'))
	typeText(a, "ops/db/load")
	press(a, tabPress, tabPress, enterPress)
	editPart(t, l, "true")
	press(a, uv.KeyPressEvent{Code: 's', Mod: uv.ModCtrl})
	if _, ok := a.scripts.Find("ops/db/load"); !ok {
		t.Fatalf("scripts %v; status %q", scriptIDs(a), a.statusMessage)
	}
}
