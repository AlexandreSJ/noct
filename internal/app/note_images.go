package app

import (
	"image"
	"image/color"
	_ "image/gif" // decoders for image.Decode
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// A line of a script's notes that is only an image, ![what](file.png), shows the image in the
// details, drawn with half blocks: each cell is two pixels, its upper half in the text's color and
// its lower half in the background's, so it shows on any terminal with colors. PNG, JPEG and GIF
// files beside the script, or anywhere by their path, are read; nothing is fetched.

var noteImage = regexp.MustCompile(`^\s*!\[([^\]]*)\]\(([^)\s]+)\)\s*$`)

const (
	// maxNoteImageRows bounds how tall an image in the notes is drawn.
	maxNoteImageRows = 16
	// maxNoteImageBytes and maxNoteImagePixels bound what is decoded, since notes come with the
	// scripts, a cloned project's too.
	maxNoteImageBytes  = 8 << 20
	maxNoteImagePixels = 4096 * 4096
)

// noteImages keeps the images the notes show, read again when their file changes.
type noteImages map[string]noteImageFile

type noteImageFile struct {
	modTime time.Time
	img     image.Image
	err     error
	drawn   map[image.Point][][]uv.Cell
	kitty   kittyImage
}

// imageIn is the file a notes line shows as an image, from the script's folder when relative.
func imageIn(line, dir string) (path, alt string, ok bool) {
	m := noteImage.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	path = m[2]
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return path, m[1], true
}

// load reads the image at path, or why it cannot show.
func (c noteImages) load(path string) (image.Image, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if cached, ok := c[path]; ok && cached.modTime.Equal(info.ModTime()) {
		return cached.img, cached.err
	}
	img, err := decodeNoteImage(path, info.Size())
	c[path] = noteImageFile{modTime: info.ModTime(), img: img, err: err}
	return img, err
}

func decodeNoteImage(path string, size int64) (image.Image, error) {
	if size > maxNoteImageBytes {
		return nil, errImageTooBig
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return nil, err
	}
	if config.Width*config.Height > maxNoteImagePixels {
		return nil, errImageTooBig
	}
	if _, err := file.Seek(0, 0); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(file)
	return img, err
}

type imageError string

func (e imageError) Error() string { return string(e) }

const errImageTooBig = imageError("the image is too big to show")

// drawHalfBlocks draws img as large as fits area's width and up to maxRows rows, keeping its
// shape, and returns how many rows it took.
func drawHalfBlocks(screen uv.Screen, area image.Rectangle, img image.Image, maxRows int) int {
	return drawBlocks(screen, area, halfBlocks(img, area.Dx(), maxRows))
}

func drawBlocks(screen uv.Screen, area image.Rectangle, rows [][]uv.Cell) int {
	for y, row := range rows {
		for x := range row {
			screen.SetCell(area.Min.X+x, area.Min.Y+y, &row[x])
		}
	}
	return len(rows)
}

// halfBlocks are the cells that draw img in up to width columns and maxRows rows.
func halfBlocks(img image.Image, width, maxRows int) [][]uv.Cell {
	bounds := img.Bounds()
	if bounds.Empty() || width <= 0 || maxRows <= 0 {
		return nil
	}
	// A cell is about twice as tall as wide, so its two halves are about square pixels.
	scale := min(float64(width)/float64(bounds.Dx()), float64(2*maxRows)/float64(bounds.Dy()))
	columns := max(int(float64(bounds.Dx())*scale), 1)
	height := max(int(float64(bounds.Dy())*scale), 1)
	pixel := func(x, y int) color.Color {
		if y >= height {
			return nil
		}
		x0, y0 := bounds.Min.X+x*bounds.Dx()/columns, bounds.Min.Y+y*bounds.Dy()/height
		x1, y1 := bounds.Min.X+(x+1)*bounds.Dx()/columns, bounds.Min.Y+(y+1)*bounds.Dy()/height
		return averageColor(img, image.Rect(x0, y0, max(x1, x0+1), max(y1, y0+1))) // a pixel at least, when enlarging
	}
	rows := make([][]uv.Cell, (height+1)/2)
	for r := range rows {
		rows[r] = make([]uv.Cell, columns)
		for x := range columns {
			top, bottom := pixel(x, 2*r), pixel(x, 2*r+1)
			cell := uv.Cell{Content: " ", Width: 1}
			switch {
			case top != nil:
				cell.Content, cell.Style = "▀", uv.Style{Fg: top, Bg: bottom}
			case bottom != nil:
				cell.Content, cell.Style = "▄", uv.Style{Fg: bottom}
			}
			rows[r][x] = cell
		}
	}
	return rows
}

// blocks are the cells of the image at path for a size, worked out once per size.
func (c noteImages) blocks(path string, width, maxRows int) ([][]uv.Cell, error) {
	img, err := c.load(path)
	if err != nil {
		return nil, err
	}
	f := c[path]
	size := image.Pt(width, maxRows)
	if f.drawn == nil {
		f.drawn = map[image.Point][][]uv.Cell{}
	}
	if _, ok := f.drawn[size]; !ok {
		f.drawn = map[image.Point][][]uv.Cell{size: halfBlocks(img, width, maxRows)} // only the last size: a resize makes the others stale
		c[path] = f
	}
	return f.drawn[size], nil
}

// averageColor is the mean of the pixels of img in r, or nil where they are mostly transparent.
func averageColor(img image.Image, r image.Rectangle) color.Color {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return nil
	}
	var red, green, blue, alpha, n uint64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			red, green, blue, alpha, n = red+uint64(cr), green+uint64(cg), blue+uint64(cb), alpha+uint64(ca), n+1
		}
	}
	if alpha/n < 0x8000 {
		return nil
	}
	// RGBA is alpha-premultiplied: dividing by the alpha gives the color itself.
	return color.RGBA{R: uint8(red * 0xff / alpha), G: uint8(green * 0xff / alpha), B: uint8(blue * 0xff / alpha), A: 0xff}
}
