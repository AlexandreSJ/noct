package app

import (
	"bytes"
	"image"
	"path/filepath"
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/kittygfx"
)

func TestAKittyCommandSplitsItsPayload(t *testing.T) {
	if got := kittyCommand("a=p,i=3", ""); got != "\x1b_Ga=p,i=3,q=2;\x1b\\" {
		t.Errorf("no payload: %q", got)
	}
	got := kittyCommand("a=t,i=3", strings.Repeat("A", kittyChunk+10))
	parts := strings.Split(strings.TrimSuffix(got, "\x1b\\"), "\x1b\\")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "\x1b_Ga=t,i=3,q=2,m=1;") || !strings.HasPrefix(parts[1], "\x1b_Gq=2,m=0;") {
		t.Errorf("chunks %q", parts)
	}
}

func TestPlacementCellsKeepTheShape(t *testing.T) {
	cell := image.Pt(10, 20)
	for _, c := range []struct {
		pixels      image.Point
		width, rows int
		want        image.Point
	}{
		{image.Pt(400, 200), 80, 16, image.Pt(40, 10)},
		{image.Pt(4000, 2000), 80, 16, image.Pt(64, 16)},
		{image.Pt(100, 1000), 80, 16, image.Pt(3, 16)},
	} {
		if got := placementCells(c.pixels, c.width, c.rows, cell); got != c.want {
			t.Errorf("%v in %d×%d = %v, want %v", c.pixels, c.width, c.rows, got, c.want)
		}
	}
}

func TestAKittyTerminalGetsTheImageOnceAndItsPlaceholders(t *testing.T) {
	a := testApp(t)
	var out bytes.Buffer
	a.screen = uv.NewTerminalScreen(&out, uv.Environ{"TERM=xterm-kitty"})
	a.client = &client{}
	a.imagesEnabled = true
	path := filepath.Join(t.TempDir(), "logo.png")
	writePNG(t, path, 40, 40)
	images := noteImages{}
	draw := func() (uv.ScreenBuffer, string) {
		buf := uv.NewScreenBuffer(30, 20)
		if _, err := a.drawKittyImage(buf, image.Rect(0, 0, 30, 20), images, path, 8); err != nil {
			t.Fatal(err)
		}
		a.screen.Flush()
		sent := out.String()
		out.Reset()
		return buf, sent
	}
	buf, sent := draw()
	if strings.Count(sent, "a=t,f=100") != 1 || strings.Count(sent, "a=p,U=1") != 1 {
		t.Fatalf("sent %q, want the image and its placement", sent)
	}
	id := images[path].kitty.id
	if c := buf.CellAt(1, 2); c == nil || c.Content != kittygfx.PlaceholderText(id, 2, 1) || !colorsEqual(c.Style.Fg, kittygfx.PlaceholderColor(id)) {
		t.Errorf("cell 1,2 is %+v, want the image's placeholder", c)
	}
	if _, again := draw(); strings.Contains(again, "\x1b_G") {
		t.Errorf("drawn again, sent %q, want nothing new", again)
	}
	a.client = &client{} // another terminal attached
	if _, sent := draw(); strings.Count(sent, "a=t,f=100") != 1 {
		t.Errorf("a new terminal got %q, want the image sent to it", sent)
	}
}
