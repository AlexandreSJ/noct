package app

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/script"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// rowLabels are the lines of the panel's list, as drawn.
func rowLabels(p *scriptsPanel) []string {
	var labels []string
	for _, row := range p.rows() {
		label, _ := p.rowText(row)
		labels = append(labels, label)
	}
	return labels
}

func TestFoldersComeFirstWithTheirScriptsBelow(t *testing.T) {
	a := testApp(t)
	saveScript(t, "zeta", "run: true\n")
	saveScript(t, "alpha", "run: true\n")
	saveScriptIn(t, filepath.Join(script.PersonalDir(), "tests"), "smoke", "run: true\n")
	saveScriptIn(t, filepath.Join(script.PersonalDir(), "tests"), "load", "run: true\n")
	panel := openScriptsPanel(t, a)

	if got := strings.Join(rowLabels(panel), ","); got != "+ tests,alpha,zeta" {
		t.Fatalf("rows %q, want the folder closed first", got)
	}
	press(a, enterPress) // on the folder: open it
	want := "- tests,load,smoke,alpha,zeta"
	if got := strings.Join(rowLabels(panel), ","); got != want {
		t.Fatalf("rows %q, want %q", got, want)
	}
	press(a, downPress, downPress, leftPress)
	if row, _ := panel.current(); row.script != nil {
		t.Errorf("left on a script in a folder selected %v, want the folder", row.script.Name)
	}
}

func TestNewScriptGoesIntoTheSelectedFolder(t *testing.T) {
	a := testApp(t)
	a.config.Editor = "cat"
	l := recordLaunches(a)
	saveScriptIn(t, filepath.Join(script.PersonalDir(), "tests"), "smoke", "run: true\n")
	openScriptsPanel(t, a)

	press(a, enterPress, downPress, letter('n')) // open tests, then on tests/smoke
	if a.mode != choiceMode {
		t.Fatalf("n: mode %v, want the choice of what to create", a.mode)
	}
	press(a, letter('s'))
	if a.scriptForm.folder == nil || a.scriptForm.folder.Name != "tests" {
		t.Fatalf("new script folder %v", a.scriptForm.folder)
	}
	typeText(a, "load")
	press(a, tabPress, tabPress, enterPress)
	editPart(t, l, "k6 run load.js")
	press(a, uv.KeyPressEvent{Code: 's', Mod: uv.ModCtrl})
	if s, ok := a.scripts.Find("tests/load"); !ok || s.Run != "k6 run load.js" {
		t.Fatalf("tests/load: %+v; status %q", s, a.statusMessage)
	}
}

func TestMakingAndLinkingFolders(t *testing.T) {
	a := testApp(t)
	panel := openScriptsPanel(t, a)

	press(a, letter('n'), downPress, upPress, enterPress) // directory is the first choice
	typeText(a, "suites")
	press(a, enterPress)
	if info, err := os.Stat(filepath.Join(script.PersonalDir(), "suites")); err != nil || !info.IsDir() {
		t.Fatalf("no folder: %v", err)
	}

	elsewhere := t.TempDir()
	saveScriptIn(t, elsewhere, "shared", "run: true\n")
	press(a, letter('a'))
	typeText(a, elsewhere)
	press(a, enterPress)
	if len(a.config.ScriptFolders) != 1 {
		t.Fatalf("linked folders %v", a.config.ScriptFolders)
	}
	if _, ok := a.scripts.Find(filepath.Base(elsewhere) + "/shared"); !ok {
		t.Fatalf("the linked folder's script is missing: %v", rowLabels(panel))
	}

	panel.selectKey("folder:" + elsewhere)
	press(a, uv.KeyPressEvent{Code: uv.KeyDelete}, enterPress)
	if len(a.config.ScriptFolders) != 0 {
		t.Errorf("still linked: %v", a.config.ScriptFolders)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "shared", script.FileName)); err != nil {
		t.Errorf("unlinking touched the files: %v", err)
	}
}

func TestTheBrowsedFolderAndItsBarAreInTheAccent(t *testing.T) {
	a := testApp(t)
	a.palette = newPalette(theme.Builtins()[0], false)
	saveScriptIn(t, filepath.Join(script.PersonalDir(), "tests"), "smoke", "run: true\n")
	saveScriptIn(t, filepath.Join(script.PersonalDir(), "other"), "idle", "run: true\n")
	panel := openScriptsPanel(t, a)
	press(a, enterPress, downPress) // open other, then on other/idle
	row, _ := panel.current()
	if row.script == nil {
		t.Fatalf("selected %q, want a script", panel.selectedKey)
	}

	buffer := uv.NewScreenBuffer(60, 10)
	panel.drawList(buffer, image.Rect(0, 0, 60, 10))
	for y, r := range panel.rows() {
		inBrowsed := r.folder == row.folder
		if r.script != nil {
			bar := buffer.CellAt(1, y)
			if bar.Content != "│" || (bar.Style.Fg == a.palette.focusedFrame.Fg) != inBrowsed {
				t.Errorf("row %d: bar %q, accent %v, want accent %v", y, bar.Content, bar.Style.Fg == a.palette.focusedFrame.Fg, inBrowsed)
			}
			continue
		}
		name := buffer.CellAt(2, y) // after "+ " or "- "
		if (name.Style.Fg == a.palette.focusedTitle.Fg) != inBrowsed {
			t.Errorf("folder row %d accent %v, want %v", y, name.Style.Fg == a.palette.focusedTitle.Fg, inBrowsed)
		}
	}
}
