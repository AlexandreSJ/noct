package app

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// linkedView shows rows of text, a link the program set on the cells of linked.
type linkedView struct {
	fakeView
	rows   []string
	linked string
	dir    string
}

func (v *linkedView) Draw(screen uv.Screen, area uv.Rectangle) {
	for y, row := range v.rows {
		for x, r := range []rune(row) {
			if area.Min.X+x >= area.Max.X {
				break
			}
			cell := &uv.Cell{Content: string(r), Width: 1}
			if v.linked != "" && y == 0 {
				cell.Link = uv.Link{URL: v.linked}
			}
			screen.SetCell(area.Min.X+x, area.Min.Y+y, cell)
		}
	}
}

func (v *linkedView) Dir() string { return v.dir }

// linksApp shows v in a pane and records what is opened; at is where a column and row of the pane
// are on screen.
func linksApp(t *testing.T, v *linkedView) (a *App, opened *[]string, at func(x, y int) image.Point) {
	t.Helper()
	a = testApp(t, newTab(1, v))
	opened = &[]string{}
	a.openURL = func(target string) bool { *opened = append(*opened, target); return true }
	frame, _ := a.currentTab().frameOf(1, a.workspaceArea())
	content := contentRect(frame)
	return a, opened, func(x, y int) image.Point { return content.Min.Add(image.Pt(x, y)) }
}

func ctrlClick(a *App, p image.Point) {
	a.handleMouse(uv.MouseClickEvent{X: p.X, Y: p.Y, Button: uv.MouseLeft, Mod: uv.ModCtrl})
}

func TestCtrlClickOpensAURLInTheText(t *testing.T) {
	a, opened, at := linksApp(t, &linkedView{rows: []string{"see https://example.com/docs?a=1, then go"}})
	ctrlClick(a, at(10, 0))
	if len(*opened) != 1 || (*opened)[0] != "https://example.com/docs?a=1" {
		t.Errorf("opened %q, want the URL without the comma after it", *opened)
	}
	a.handleMouse(uv.MouseClickEvent{X: at(10, 0).X, Y: at(10, 0).Y, Button: uv.MouseLeft})
	ctrlClick(a, at(1, 0))
	if len(*opened) != 1 {
		t.Errorf("opened %q, want nothing from a plain click nor off the URL", *opened)
	}
}

func TestCtrlClickOpensAWrappedURL(t *testing.T) {
	a, opened, at := linksApp(t, &linkedView{})
	width := contentRect(mustPaneFrame(t, a, 1)).Dx()
	url := "https://example.com/" + strings.Repeat("a", width)
	line := "> " + url
	v := a.currentTab().views[1].(*linkedView)
	v.rows = []string{line[:width], line[width:]}
	ctrlClick(a, at(3, 1))
	if len(*opened) != 1 || (*opened)[0] != url {
		t.Errorf("opened %q, want the URL over both rows", *opened)
	}
}

func TestCtrlClickOpensTheLinkTheProgramSet(t *testing.T) {
	a, opened, at := linksApp(t, &linkedView{rows: []string{"the docs"}, linked: "https://noct.dev"})
	ctrlClick(a, at(5, 0))
	if len(*opened) != 1 || (*opened)[0] != "https://noct.dev" {
		t.Errorf("opened %q, want the OSC 8 link", *opened)
	}
}

func TestCtrlClickOpensAPathThatExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	a, opened, at := linksApp(t, &linkedView{rows: []string{"main.go:12:4: undefined: x", "missing.go:1: no"}, dir: dir})
	ctrlClick(a, at(2, 0))
	ctrlClick(a, at(2, 1))
	if len(*opened) != 1 || (*opened)[0] != filepath.Join(dir, "main.go") {
		t.Errorf("opened %q, want only main.go, from the pane's folder", *opened)
	}
}

func TestOverSSHALinkIsCopied(t *testing.T) {
	a, opened, at := linksApp(t, &linkedView{rows: []string{"https://example.com"}})
	a.client = &client{hello: helloWith("SSH_CONNECTION=1 2 3 4")}
	ctrlClick(a, at(3, 0))
	if len(*opened) != 0 || !strings.Contains(a.notice, "copied https://example.com") {
		t.Errorf("opened %q, notice %q; want it copied", *opened, a.notice)
	}
}

func TestTrimLink(t *testing.T) {
	for in, want := range map[string]string{
		"https://a.b/c.":          "https://a.b/c",
		"https://a.b/(x)":         "https://a.b/(x)",
		"https://a.b/c)":          "https://a.b/c",
		"https://en.wiki/A_(b)),": "https://en.wiki/A_(b)",
	} {
		if got := string(trimLink([]rune(in))); got != want {
			t.Errorf("trimLink(%q) = %q, want %q", in, got, want)
		}
	}
}

func mustPaneFrame(t *testing.T, a *App, id int) image.Rectangle {
	t.Helper()
	frame, ok := a.currentTab().frameOf(1, a.workspaceArea())
	if !ok {
		t.Fatal("pane not on screen")
	}
	return frame
}
