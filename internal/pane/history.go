package pane

import (
	"bytes"
	"strings"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/AlexandreSJ/noct/third_party/vt"
)

// historyRows is the height of the screen the history is drawn on, before it scrolls away.
const historyRows = 100

// historyEnd marks the end of what drawing the history sent back as if to the program.
const historyEnd = "\x00noct-history-end\x00"

// historyMark separates the history from what the new program prints.
const historyMark = "\x1b[2m── restored from the log ──\x1b[0m\r\n"

// renderHistory draws an earlier program's raw output on a screen of its own and returns the
// lines it left, as text and colors only. Replayed as is, the output's queries (cursor position,
// colors) would be answered to the new program, and a full-screen program would leave the
// alternate screen on.
func renderHistory(history []byte, cols int) []byte {
	screen := vt.NewEmulator(max(cols, 1), historyRows)
	drained := make(chan struct{})
	go discardUntil(screen, historyEnd, drained)
	_, _ = screen.Write(history)
	_, _ = screen.WriteString("\x1b[?1049l") // back to the main screen, where the shell was
	screen.SendText(historyEnd)
	<-drained

	var out bytes.Buffer
	lines := screen.Scrollback().Lines()
	for y := range screen.Height() {
		line := make(uv.Line, screen.Width())
		for x := range line {
			if cell := screen.CellAt(x, y); cell != nil {
				line[x] = *cell
			}
		}
		lines = append(lines, line)
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1].String()) == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		out.WriteString(line.Render())
		out.WriteString("\x1b[0m\r\n")
	}
	if out.Len() > 0 {
		out.WriteString(historyMark)
	}
	return out.Bytes()
}

// discardUntil reads what a screen sends to its program until end, then closes done.
func discardUntil(screen *vt.Emulator, end string, done chan<- struct{}) {
	defer close(done)
	var tail []byte
	buf := make([]byte, 4096)
	for {
		n, err := screen.Read(buf)
		tail = append(tail, buf[:n]...)
		if bytes.HasSuffix(tail, []byte(end)) || err != nil {
			return
		}
		tail = tail[max(len(tail)-len(end), 0):]
	}
}
