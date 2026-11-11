package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/script"
)

var (
	red  = color.RGBA{0xff, 0, 0, 0xff}
	blue = color.RGBA{0, 0, 0xff, 0xff}
)

// writePNG writes a w by h picture, red over blue, with its right column transparent.
func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := red
			if y >= h/2 {
				c = blue
			}
			if x == w-1 {
				c = color.RGBA{}
			}
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestDrawHalfBlocksKeepsTheShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flag.png")
	writePNG(t, path, 4, 4)
	img, err := (noteImages{}).load(path)
	if err != nil {
		t.Fatal(err)
	}
	screen := uv.NewScreenBuffer(20, 10)
	rows := drawHalfBlocks(screen, image.Rect(0, 0, 8, 10), img, 4)
	if rows != 4 {
		t.Fatalf("a square picture 8 cells wide took %d rows, want 4: two pixels a cell", rows)
	}
	top, bottom, clear := screen.CellAt(0, 0), screen.CellAt(0, 3), screen.CellAt(7, 0)
	if top.Content != "▀" || !colorsEqual(top.Style.Fg, red) || !colorsEqual(top.Style.Bg, red) {
		t.Errorf("top-left %+v, want red over red", top)
	}
	if !colorsEqual(bottom.Style.Fg, blue) || !colorsEqual(bottom.Style.Bg, blue) {
		t.Errorf("bottom-left %+v, want blue", bottom)
	}
	if clear.Content != " " {
		t.Errorf("a transparent column drew %+v, want nothing", clear)
	}
}

func TestANotesImageShowsInTheDetails(t *testing.T) {
	a := testApp(t)
	saveScript(t, "deploy", "run: make deploy\n")
	dir := filepath.Join(script.PersonalDir(), "deploy")
	writePNG(t, filepath.Join(dir, "diagram.png"), 8, 8)
	notes := "Before the image.\n\n![the diagram](diagram.png)\n![missing](nope.png)\nAfter it.\n"
	if err := os.WriteFile(filepath.Join(dir, script.NotesName), []byte(notes), 0o644); err != nil {
		t.Fatal(err)
	}
	a.size = image.Pt(140, 40)
	openScriptsPanel(t, a)
	screen := drawnApp(a)
	text := screenText(a)
	for _, want := range []string{"Before the image.", "▣ missing", "After it."} {
		if !strings.Contains(text, want) {
			t.Errorf("the details do not show %q:\n%s", want, text)
		}
	}
	blocks := 0
	for y := range a.size.Y {
		for x := range a.size.X {
			if c := screen.CellAt(x, y); c != nil && c.Content == "▀" && colorsEqual(c.Style.Fg, red) {
				blocks++
			}
		}
	}
	if blocks == 0 || strings.Contains(text, "![the diagram]") {
		t.Errorf("%d red half blocks, and the line shows as text: %v", blocks, strings.Contains(text, "![the diagram]"))
	}
}

func TestATooBigImageIsNotDecoded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.png")
	writePNG(t, path, 5000, 4000)
	if _, err := (noteImages{}).load(path); err == nil || !strings.Contains(err.Error(), "too big") {
		t.Errorf("err = %v", err)
	}
}
