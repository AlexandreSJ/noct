package kittygfx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/png" // f=100 means PNG, the only compressed format in the protocol
	"io"
	"os"
)

// fallbackCellSize assumes the usual 1:2 cell when the terminal does not report pixel sizes.
var fallbackCellSize = image.Pt(10, 20)

// Placement asks the pane to write an image's placeholder cells at its cursor.
type Placement struct {
	ImageID uint32
	Cols    int
	Rows    int
	// KeepCursor leaves the cursor where it was (C=1) instead of moving it past the image.
	KeepCursor bool
}

// Result is what one command asks of noct. Any field may be empty.
type Result struct {
	// ToTerminal holds commands for the terminal noct runs in.
	ToTerminal []byte
	// ToProgram is the reply kitty would have sent to the program.
	ToProgram []byte
	Place     *Placement
}

// Session follows the images one pane's program sends. Programs choose their own image ids,
// which could collide across panes, so each image gets an id unique in the outer terminal.
type Session struct {
	newImageID func() uint32
	cellSize   func() image.Point
	outerIDs   map[int]uint32
	sizes      map[uint32]image.Point
	upload     []Command
}

// NewSession needs a source of outer-terminal image ids and the outer terminal's cell size in pixels.
func NewSession(newImageID func() uint32, cellSize func() image.Point) *Session {
	return &Session{
		newImageID: newImageID,
		cellSize:   cellSize,
		outerIDs:   make(map[int]uint32),
		sizes:      make(map[uint32]image.Point),
	}
}

func (s *Session) Handle(cmd Command) Result {
	// Chunks after the first carry only m (and q); a new action means the upload was abandoned.
	if s.upload != nil && !cmd.Has("a") {
		return s.continueUpload(cmd)
	}
	s.upload = nil
	switch cmd.Action() {
	case "q":
		return Result{ToProgram: reply(cmd, "OK")}
	case "t", "T":
		if cmd.Get("m") == "1" {
			s.upload = []Command{cmd}
			return Result{}
		}
		return s.transmit([]Command{cmd})
	case "p":
		return s.put(cmd)
	case "d":
		return s.delete(cmd)
	}
	return Result{} // animation and other actions are not supported
}

func (s *Session) continueUpload(chunk Command) Result {
	s.upload = append(s.upload, chunk)
	if chunk.Get("m") == "1" {
		return Result{}
	}
	chunks := s.upload
	s.upload = nil
	return s.transmit(chunks)
}

// transmit forwards a complete upload as one unit, so uploads from different panes never
// interleave, and turns a=T into a virtual placement shown through placeholders.
func (s *Session) transmit(chunks []Command) Result {
	first := chunks[0]
	if first.Get("U") == "1" {
		return Result{ToTerminal: encodeAll(chunks)} // the program draws its own placeholders
	}
	outerID := s.newImageID()
	if programID := first.Int("i"); programID != 0 {
		s.outerIDs[programID] = outerID
	}
	result := Result{ToProgram: reply(first, "OK")}
	if pixels, ok := imageSize(first, joinPayloads(chunks)); ok {
		s.sizes[outerID] = pixels
	}
	forwarded := first.Without("i", "I", "p", "C").With("i", fmt.Sprint(outerID)).With("q", "2")
	if first.Action() == "T" {
		cols, rows, ok := s.placementCells(first, outerID)
		if !ok {
			return Result{ToProgram: reply(first, "EINVAL:image size unknown")}
		}
		forwarded = forwarded.With("U", "1").With("c", fmt.Sprint(cols)).With("r", fmt.Sprint(rows))
		result.Place = &Placement{ImageID: outerID, Cols: cols, Rows: rows, KeepCursor: first.Get("C") == "1"}
	}
	chunks = append([]Command{forwarded}, chunks[1:]...)
	for i := 1; i < len(chunks); i++ {
		chunks[i] = chunks[i].With("q", "2")
	}
	result.ToTerminal = encodeAll(chunks)
	return result
}

func (s *Session) put(cmd Command) Result {
	outerID, ok := s.outerIDs[cmd.Int("i")]
	if !ok {
		return Result{ToProgram: reply(cmd, "ENOENT:no such image")}
	}
	cols, rows, ok := s.placementCells(cmd, outerID)
	if !ok {
		return Result{ToProgram: reply(cmd, "EINVAL:image size unknown")}
	}
	placement := Command{keys: map[string]string{
		"a": "p", "U": "1", "q": "2",
		"i": fmt.Sprint(outerID), "c": fmt.Sprint(cols), "r": fmt.Sprint(rows),
	}}
	return Result{
		ToTerminal: placement.Encode(),
		ToProgram:  reply(cmd, "OK"),
		Place:      &Placement{ImageID: outerID, Cols: cols, Rows: rows, KeepCursor: cmd.Get("C") == "1"},
	}
}

// delete frees images by id, or all of this pane's images for d=a (the default).
// Deleting by position or placement is not supported; placeholders stay until overwritten.
func (s *Session) delete(cmd Command) Result {
	var ids []uint32
	switch cmd.Get("d") {
	case "i", "I":
		if outerID, ok := s.outerIDs[cmd.Int("i")]; ok {
			ids = append(ids, outerID)
		}
	case "", "a", "A":
		for _, outerID := range s.outerIDs {
			ids = append(ids, outerID)
		}
	}
	var out []byte
	for _, id := range ids {
		out = append(out, Command{keys: map[string]string{"a": "d", "d": "I", "i": fmt.Sprint(id), "q": "2"}}.Encode()...)
	}
	return Result{ToTerminal: out}
}

// placementCells sizes a placement the way kitty does: c and r as given, the missing one from
// the image's aspect ratio, or the image's natural size in cells.
func (s *Session) placementCells(cmd Command, outerID uint32) (cols, rows int, ok bool) {
	cols, rows = cmd.Int("c"), cmd.Int("r")
	if cols <= 0 || rows <= 0 {
		pixels, known := s.sizes[outerID]
		if !known || pixels.X <= 0 || pixels.Y <= 0 {
			return 0, 0, false
		}
		cell := s.cellSize()
		if cell.X <= 0 || cell.Y <= 0 {
			cell = fallbackCellSize
		}
		switch {
		case cols > 0:
			rows = ceilDiv(pixels.Y*cols*cell.X, pixels.X*cell.Y)
		case rows > 0:
			cols = ceilDiv(pixels.X*rows*cell.Y, pixels.Y*cell.X)
		default:
			cols, rows = ceilDiv(pixels.X, cell.X), ceilDiv(pixels.Y, cell.Y)
		}
	}
	return min(cols, MaxPlacementCells), min(rows, MaxPlacementCells), true
}

// imageSize reads the pixel size from s and v, or from the PNG header in the data or the file
// it names. Shared memory is not readable from here.
func imageSize(cmd Command, payload []byte) (image.Point, bool) {
	if width, height := cmd.Int("s"), cmd.Int("v"); width > 0 && height > 0 {
		return image.Pt(width, height), true
	}
	if cmd.Get("f") != "100" {
		return image.Point{}, false
	}
	data, err := base64.StdEncoding.DecodeString(string(payload))
	if err != nil {
		return image.Point{}, false
	}
	switch cmd.Get("t") {
	case "", "d":
		return decodeSize(bytes.NewReader(data))
	case "f", "t":
		file, err := os.Open(string(data))
		if err != nil {
			return image.Point{}, false
		}
		defer file.Close()
		return decodeSize(file)
	}
	return image.Point{}, false
}

func decodeSize(r io.Reader) (image.Point, bool) {
	config, _, err := image.DecodeConfig(r)
	if err != nil {
		return image.Point{}, false
	}
	return image.Pt(config.Width, config.Height), true
}

// reply builds kitty's response. Kitty answers only commands that name an image (i or I),
// and q=1 silences success while q=2 silences everything.
func reply(cmd Command, message string) []byte {
	quiet := cmd.Int("q")
	if (cmd.Int("i") == 0 && cmd.Int("I") == 0) || quiet >= 2 || (quiet == 1 && message == "OK") {
		return nil
	}
	keys := map[string]string{}
	for _, key := range []string{"i", "I", "p"} {
		if cmd.Has(key) {
			keys[key] = cmd.Get(key)
		}
	}
	return Command{keys: keys, Payload: []byte(message)}.Encode()
}

func joinPayloads(chunks []Command) []byte {
	var payload []byte
	for _, chunk := range chunks {
		payload = append(payload, chunk.Payload...)
	}
	return payload
}

func encodeAll(commands []Command) []byte {
	var out []byte
	for _, cmd := range commands {
		out = append(out, cmd.Encode()...)
	}
	return out
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}
