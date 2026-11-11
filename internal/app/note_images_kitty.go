package app

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/kittygfx"
)

// In terminals that draw kitty's Unicode placeholders (kitty, Ghostty), an image in the notes
// shows in full resolution: it is sent once to each attached terminal as a PNG, placed virtually
// over a number of cells, and the notes draw placeholder cells where it goes, so it scrolls and is
// cut by the panel like text. Other terminals get the half blocks.

// kittyChunk is how much of an image's base64 one graphics command carries.
const kittyChunk = 4096

// assumedCell is a cell's size in pixels when the terminal does not tell it.
var assumedCell = image.Pt(8, 16)

// kittyImage is an image sent to the attached terminal: its id there, and the cells it was last
// placed over.
type kittyImage struct {
	id     uint32
	client *client
	placed image.Point
}

// drawKittyImage draws path's image over up to maxRows rows of area with placeholders, sending it
// first when the attached terminal does not have it yet, and returns the rows it took.
func (a *App) drawKittyImage(screen uv.Screen, area image.Rectangle, images noteImages, path string, maxRows int) (int, error) {
	img, err := images.load(path)
	if err != nil {
		return 0, err
	}
	f := images[path]
	if f.kitty.client != a.client || f.kitty.id == 0 {
		upload, err := kittyUpload(img)
		if err != nil {
			return 0, err
		}
		f.kitty = kittyImage{id: a.newImageID(), client: a.client}
		a.toTerminal(kittyCommand(fmt.Sprintf("a=t,f=100,i=%d", f.kitty.id), upload))
	}
	size := placementCells(img.Bounds().Size(), area.Dx(), maxRows, orPoint(a.cellSize(), assumedCell))
	if size != f.kitty.placed {
		a.toTerminal(kittyCommand(fmt.Sprintf("a=p,U=1,i=%d,p=1,c=%d,r=%d", f.kitty.id, size.X, size.Y), ""))
		f.kitty.placed = size
	}
	images[path] = f
	fg := kittygfx.PlaceholderColor(f.kitty.id)
	for row := range size.Y {
		for col := range size.X {
			screen.SetCell(area.Min.X+col, area.Min.Y+row, &uv.Cell{Content: kittygfx.PlaceholderText(f.kitty.id, row, col), Width: 1, Style: uv.Style{Fg: fg}})
		}
	}
	return size.Y, nil
}

// placementCells are the columns and rows an image of pixels takes, as wide as fits width and at
// most maxRows tall, keeping its shape on cells of cell pixels.
func placementCells(pixels image.Point, width, maxRows int, cell image.Point) image.Point {
	limit := min(kittygfx.MaxPlacementCells, 1<<16)
	cols := min(width, ceilDiv(pixels.X, cell.X), limit)
	rows := ceilDiv(cols*cell.X*pixels.Y, max(pixels.X*cell.Y, 1))
	if rows > maxRows {
		rows = maxRows
		cols = min(cols, max(rows*cell.Y*pixels.X/max(pixels.Y*cell.X, 1), 1))
	}
	return image.Pt(max(cols, 1), max(rows, 1))
}

func ceilDiv(a, b int) int {
	return (a + max(b, 1) - 1) / max(b, 1)
}

func orPoint(p, fallback image.Point) image.Point {
	if p.X <= 0 || p.Y <= 0 {
		return fallback
	}
	return p
}

// kittyUpload is the image as kitty takes it: PNG, in base64.
func kittyUpload(img image.Image) (string, error) {
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out.Bytes()), nil
}

// kittyCommand is a graphics command with control keys and a base64 payload, split into chunks
// as the protocol asks, every one quiet (q=2) so no reply reaches noct's input.
func kittyCommand(control, payload string) string {
	var out strings.Builder
	first := true
	for {
		chunk, rest := payload, ""
		if len(chunk) > kittyChunk {
			chunk, rest = payload[:kittyChunk], payload[kittyChunk:]
		}
		keys := "q=2"
		if first {
			keys = control + ",q=2"
		}
		if rest != "" || !first && chunk != "" {
			keys += map[bool]string{true: ",m=1", false: ",m=0"}[rest != ""]
		}
		fmt.Fprintf(&out, "\x1b_G%s;%s\x1b\\", keys, chunk)
		if rest == "" {
			return out.String()
		}
		payload, first = rest, false
	}
}
