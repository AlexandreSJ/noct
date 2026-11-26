package app

import (
	"image"
	"os"
	"slices"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/flow"
)

func openInspectOn(t *testing.T, a *App) *inspectPanel {
	t.Helper()
	pressDrawn(a, prefixPress, letter('i'))
	return a.currentTab().focusedView().(*inspectPanel)
}

func answerPrompt(a *App, text string) {
	pressDrawn(a, clearLine)
	typeDrawn(a, text)
	pressDrawn(a, enterPress)
}

func flowNames(a *App) []string {
	var names []string
	for _, f := range a.flows {
		names = append(names, f.Name)
	}
	return names
}

// N makes a folder, empty until a flow goes in; m moves a flow into it and out again; c copies one.
func TestTheInspectMakesFoldersAndMovesAndCopiesFlows(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "deploy", "steps: [{run: 'true'}]\n")
	p := openInspectOn(t, a)
	pressDrawn(a, letter('N'))
	answerPrompt(a, "ops/db")
	if p.rowKey != "flows:ops/db" || !slices.Contains(a.flowFolders, "ops/db") {
		t.Fatalf("the new folder: selected %q, folders %v", p.rowKey, a.flowFolders)
	}
	if !strings.Contains(screenText(a), "- ops") {
		t.Errorf("the new empty folder does not show open in its folder:\n%s", screenText(a))
	}
	if err := a.memory.RememberFlow("deploy", map[string]string{"x": "1"}); err != nil {
		t.Fatal(err)
	}
	p.flowName, p.rowKey = "deploy", ""
	pressDrawn(a, letter('m'))
	answerPrompt(a, "ops/db/deploy")
	if got := flowNames(a); !slices.Equal(got, []string{"ops/db/deploy"}) {
		t.Fatalf("after the move the flows are %v", got)
	}
	if a.memory.FlowValues("ops/db/deploy")["x"] != "1" || p.flowName != "ops/db/deploy" {
		t.Errorf("the move lost the remembered values or the selection: %q", p.flowName)
	}
	pressDrawn(a, letter('c'))
	answerPrompt(a, "deploy")
	if got := flowNames(a); !slices.Equal(got, []string{"deploy", "ops/db/deploy"}) {
		t.Errorf("after the copy the flows are %v", got)
	}
}

// m on a folder moves it with its flows; delete removes it after asking.
func TestTheInspectMovesAndDeletesFolders(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "ops/deploy", "steps: [{run: 'true'}]\n")
	saveFlow(t, "hello", "steps: [{run: 'true'}]\n")
	p := openInspectOn(t, a)
	p.rowKey = "flows:ops"
	pressDrawn(a, letter('m'))
	answerPrompt(a, "team/ops")
	if got := flowNames(a); !slices.Contains(got, "team/ops/deploy") {
		t.Fatalf("after moving the folder the flows are %v", got)
	}
	p.rowKey = "flows:team"
	pressDrawn(a, uv.KeyPressEvent{Code: uv.KeyDelete}, letter('y'))
	if got := flowNames(a); !slices.Equal(got, []string{"hello"}) {
		t.Errorf("after deleting the folder the flows are %v", got)
	}
	if _, err := os.Stat(flow.FilePath(flow.Dir(), "team/ops/deploy")); err == nil {
		t.Error("the folder's flow is still on disk")
	}
}

// A function moved to a folder is called by its new name.
func TestMovingAFunctionRenamesItsCalls(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFunction(t, "greet", "steps: [{run: 'true'}]\n")
	saveFlow(t, "morning", "steps:\n  - {call: greet}\n")
	p := openInspectOn(t, a)
	p.flowName, p.rowKey = "ƒ greet", ""
	pressDrawn(a, letter('m'))
	answerPrompt(a, "utils/greet")
	if steps := loadFlowSteps(t, "morning"); len(steps) != 1 || steps[0].Call != "utils/greet" {
		t.Errorf("morning's call is %+v, want utils/greet", steps)
	}
}

// Dragging a flow onto a folder's row moves it there.
func TestDraggingAFlowOntoAFolderMovesIt(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(140, 40)
	saveFlow(t, "ops/deploy", "steps: [{run: 'true'}]\n")
	saveFlow(t, "hello", "steps: [{run: 'true'}]\n")
	p := openInspectOn(t, a)
	frame, _ := a.currentTab().frameOf(a.currentTab().focused, a.workspaceArea())
	origin := contentRect(frame).Min
	rowAt := func(key string) image.Point {
		for i, r := range p.rows() {
			if r.key() == key {
				return origin.Add(image.Pt(p.list.Min.X+2, p.list.Min.Y+i-p.first))
			}
		}
		t.Fatalf("no row %s in %v", key, inspectRowKeys(p))
		return image.Point{}
	}
	from, to := rowAt("hello"), rowAt("flows:ops")
	a.handleEvent(uv.MouseClickEvent{X: from.X, Y: from.Y, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseMotionEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft})
	a.handleEvent(uv.MouseReleaseEvent{X: to.X, Y: to.Y, Button: uv.MouseLeft})
	if got := flowNames(a); !slices.Equal(got, []string{"ops/deploy", "ops/hello"}) {
		t.Errorf("after the drop the flows are %v", got)
	}
}

// f filters the keys popup and the wheel scrolls it; a click inside keeps it open.
func TestTheKeysPopupFiltersAndScrolls(t *testing.T) {
	a := testApp(t)
	a.size = image.Pt(100, 20)
	saveFlow(t, "hello", "steps: [{run: 'true'}]\n")
	openInspectOn(t, a)
	pressDrawn(a, letter('?'))
	if a.mode != infoMode {
		t.Fatalf("? opened mode %v", a.mode)
	}
	a.handleEvent(uv.MouseWheelEvent{X: 50, Y: 10, Button: uv.MouseWheelDown})
	if a.info.first != infoWheelLines {
		t.Errorf("the wheel scrolled to %d", a.info.first)
	}
	click(a, a.info.rect.Min.X+3, a.info.rect.Min.Y+3)
	if a.mode != infoMode {
		t.Fatal("a click inside the popup closed it")
	}
	pressDrawn(a, letter('f'))
	typeDrawn(a, "zoom")
	screen := screenText(a)
	if !strings.Contains(screen, "zoom in") || strings.Contains(screen, "run with params") || a.info.first != 0 {
		t.Errorf("the filter did not keep the zoom keys alone:\n%s", screen)
	}
	pressDrawn(a, enterPress, escPress)
	if a.mode != infoMode || a.info.filter.on() {
		t.Errorf("esc did not clear the filter first: mode %v", a.mode)
	}
	pressDrawn(a, escPress)
	if a.mode == infoMode {
		t.Error("esc did not close the popup")
	}
}
