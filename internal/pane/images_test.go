package pane

import (
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/kittygfx"
	"github.com/AlexandreSJ/noct/internal/pty"
)

func TestImageCommandBecomesPlaceholdersAndMovesCursor(t *testing.T) {
	logo := filepath.Join(t.TempDir(), "logo.png")
	file, err := os.Create(logo)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 40, 80))); err != nil {
		t.Fatal(err)
	}
	file.Close()

	var forwarded []string
	exited := make(chan struct{})
	path := base64.StdEncoding.EncodeToString([]byte(logo))
	p, err := Start(Options{
		// 4 columns of a 10x20px cell are 40px; the 40x80 image is then 80px = 4 rows tall.
		Argv:               []string{"sh", "-c", `printf '\033_Ga=T,f=100,t=f,c=4;` + path + `\033\\X'`},
		Env:                []string{"PATH=/usr/bin:/bin"},
		Size:               pty.Size{Cols: 20, Rows: 10},
		Images:             kittygfx.NewSession(func() uint32 { return 7 }, func() image.Point { return image.Pt(10, 20) }),
		OnTerminalGraphics: func(commands []byte) { forwarded = append(forwarded, string(commands)) },
		OnExit:             func(int) { close(exited) },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	waitFor(t, exited)

	if len(forwarded) != 1 || !strings.Contains(forwarded[0], "U=1") || !strings.Contains(forwarded[0], "i=7") {
		t.Fatalf("forwarded %q", forwarded)
	}
	p.screenLock.RLock()
	defer p.screenLock.RUnlock()
	for _, at := range []image.Point{{0, 0}, {3, 0}, {0, 3}, {3, 3}} {
		if cell := p.screen.CellAt(at.X, at.Y); cell == nil || !strings.HasPrefix(cell.Content, "\U0010EEEE") {
			t.Errorf("no placeholder at %v", at)
		}
	}
	if cell := p.screen.CellAt(4, 4); cell == nil || cell.Content != "X" {
		t.Errorf("text after the image is not right of and below it: %+v", cell)
	}
}
