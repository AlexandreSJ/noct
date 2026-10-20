package templates

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// docsExample is the template of docs/templates.md.
const docsExample = `name: dev
colors:
  tabs.active_background: "#9ece6a"
layout: columns
panes:
  - terminal: { name: info, run: fastfetch, start: run }
  - rows:
      - terminal: { run: claude, start: ask }
      - scripts: {}
        colors: { panels.focused_border: "#f7768e" }
`

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAShapeBecomesATemplateAndBack(t *testing.T) {
	shape := layout.Shape{Direction: layout.SideBySide, Share: 1, Children: []layout.Shape{
		{Pane: 1, Share: 0.6},
		{Direction: layout.Stacked, Stacked: true, Share: 0.4, Children: []layout.Shape{{Pane: 2, Share: 0.5}, {Pane: 3, Share: 0.5}}},
	}}
	panes := map[layout.PaneID]Pane{
		1: {Kind: "terminal", Name: "server", Command: "npm run dev", Start: Run},
		2: {Kind: "terminal", Command: "claude", Start: Ask},
		3: {Kind: "logs"},
	}
	node := FromShape(shape, panes)
	next := layout.PaneID(10)
	back, backPanes := node.Shape(func() layout.PaneID { next++; return next })
	if back.Direction != layout.SideBySide || len(back.Children) != 2 || math.Abs(back.Children[0].Share-0.6) > 0.001 {
		t.Fatalf("back as %+v", back)
	}
	stack := back.Children[1]
	if !stack.Stacked || stack.Direction != layout.Stacked || len(stack.Children) != 2 {
		t.Fatalf("the stack came back as %+v", stack)
	}
	if p := backPanes[back.Children[0].Pane]; !reflect.DeepEqual(p, panes[1]) {
		t.Errorf("the first pane came back as %+v", p)
	}
	if p := backPanes[stack.Children[1].Pane]; p.Kind != "logs" {
		t.Errorf("the last pane came back as %+v", p)
	}
	if _, err := layout.FromShape(back, func(id layout.PaneID) layout.PaneID { return id }); err != nil {
		t.Errorf("the shape does not build: %v", err)
	}
}

func TestPanesLeftOutCloseTheLayout(t *testing.T) {
	shape := layout.Shape{Direction: layout.SideBySide, Children: []layout.Shape{{Pane: 1}, {Pane: 2}}}
	node := FromShape(shape, map[layout.PaneID]Pane{1: {Kind: "scripts"}})
	if node.Pane == nil || node.Pane.Kind != "scripts" || node.Columns != nil {
		t.Errorf("a layout with one pane left is %+v", node)
	}
}

func TestTheFileReadsLikeTheDocs(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "dev", docsExample)
	tpl, err := Load(dir, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Name != "dev" || tpl.Tab != "dev" || tpl.Colors["tabs.active_background"] != "#9ece6a" || len(tpl.Root.Columns) != 2 {
		t.Fatalf("loaded %+v", tpl)
	}
	info, rows := tpl.Root.Columns[0], tpl.Root.Columns[1]
	if !reflect.DeepEqual(*info.Pane, Pane{Kind: "terminal", Name: "info", Command: "fastfetch", Start: Run}) {
		t.Errorf("info is %+v", *info.Pane)
	}
	if len(rows.Rows) != 2 || rows.Rows[0].Pane.Start != Ask || rows.Rows[1].Pane.Kind != "scripts" || rows.Rows[1].Pane.Colors["panels.focused_border"] != "#f7768e" {
		t.Errorf("rows are %+v", rows)
	}
}

func TestSaveWritesWhatLoadReads(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "dev", docsExample)
	tpl, _ := Load(dir, "dev")
	tpl.Name = "copy"
	if err := Save(dir, tpl); err != nil {
		t.Fatal(err)
	}
	again, err := Load(dir, "copy")
	if err != nil {
		t.Fatal(err)
	}
	if again.Root.Columns[1].Rows[1].Pane.Colors["panels.focused_border"] != "#f7768e" || again.Root.Columns[0].Pane.Command != "fastfetch" {
		t.Errorf("read back %+v", again)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "copy.yaml"))
	if strings.Contains(string(content), "share") {
		t.Errorf("even shares were written:\n%s", content)
	}
}

func TestUnevenSharesAreKept(t *testing.T) {
	shape := layout.Shape{Direction: layout.SideBySide, Children: []layout.Shape{{Pane: 1, Share: 0.7}, {Pane: 2, Share: 0.3}}}
	node := FromShape(shape, map[layout.PaneID]Pane{1: {Kind: "logs"}, 2: {Kind: "logs"}})
	if node.Columns[0].Share != 0.7 || node.Columns[1].Share != 0.3 {
		t.Errorf("shares %v and %v", node.Columns[0].Share, node.Columns[1].Share)
	}
	even := FromShape(layout.Shape{Direction: layout.SideBySide, Children: []layout.Shape{{Pane: 1, Share: 0.5}, {Pane: 2, Share: 0.5}}}, map[layout.PaneID]Pane{1: {Kind: "logs"}, 2: {Kind: "logs"}})
	if even.Columns[0].Share != 0 {
		t.Errorf("even shares kept as %v", even.Columns[0].Share)
	}
}

func TestABrokenTemplateIsReportedAndSkipped(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "bogus", "panes:\n  - bogus: {}\n")
	write(t, dir, "both", "panes:\n  - terminal: {}\n    columns: [{logs: {}}, {logs: {}}]\n")
	write(t, dir, "soon", "panes:\n  - terminal: { run: ls, start: soon }\n")
	write(t, dir, "none", "panes: []\n")
	write(t, dir, "good", "panes:\n  - logs: {}\n")
	tpls, errs := LoadAll(dir)
	if len(tpls) != 1 || tpls[0].Name != "good" {
		t.Fatalf("loaded %v", tpls)
	}
	if len(errs) != 4 {
		t.Fatalf("errors %v, want one per broken file", errs)
	}
	for _, name := range []string{"bogus.yaml", "both.yaml", "soon.yaml", "none.yaml"} {
		found := false
		for _, err := range errs {
			found = found || strings.Contains(err.Error(), name)
		}
		if !found {
			t.Errorf("no error names %s: %v", name, errs)
		}
	}
}

func TestARunWithoutAStartAsks(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x", "panes:\n  - terminal: { run: claude }\n")
	tpl, err := Load(dir, "x")
	if err != nil || tpl.Root.Pane.Start != Ask {
		t.Errorf("loaded %+v, %v; want ask", tpl.Root, err)
	}
}

func TestSaveRenameDelete(t *testing.T) {
	dir := t.TempDir()
	tpl := Template{Name: "a", Root: Node{Pane: &Pane{Kind: "logs"}}}
	if err := Save(dir, tpl); err != nil {
		t.Fatal(err)
	}
	if err := Rename(dir, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.yaml")); err == nil {
		t.Error("a.yaml is still there")
	}
	tpl.Name = "c"
	_ = Save(dir, tpl)
	if err := Rename(dir, "b", "c"); err == nil {
		t.Error("renaming over c was allowed")
	}
	if err := Delete(dir, "b"); err != nil {
		t.Fatal(err)
	}
	if tpls, _ := LoadAll(dir); len(tpls) != 1 || tpls[0].Name != "c" {
		t.Errorf("left %v", tpls)
	}
}

func TestCheckNameUsesTheScriptRule(t *testing.T) {
	if err := CheckName("dev"); err != nil {
		t.Error(err)
	}
	for _, name := range []string{"", "-x", "a b"} {
		if CheckName(name) == nil {
			t.Errorf("%q was taken as a name", name)
		}
	}
}

func TestDirIsInTheConfigFolder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/cfg")
	if dir, err := Dir(); err != nil || dir != "/tmp/cfg/noct/templates" {
		t.Errorf("Dir() = %q, %v", dir, err)
	}
}
