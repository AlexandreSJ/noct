package app

import (
	"fmt"
	"image"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"

	"github.com/AlexandreSJ/noct/internal/pty"
)

// runStarter starts a script's program at the given size; onExit runs on the event loop.
type runStarter func(size pty.Size, onExit func(exitCode int)) (view, error)

// runView is a script running in a terminal, with a status line under it. When the script ends
// its output stays until the user closes it, so a failure can be read, and r runs it again.
type runView struct {
	name    string
	palette *palette
	start   runStarter
	// close takes the view out of its tab.
	close func()
	// dir is the folder the run started in, where a terminal opened beside it starts.
	dir string

	terminal view
	size     pty.Size
	running  bool
	exitCode int
	// run counts starts, so the exit of a run that was replaced is not taken for the new one's.
	run int
}

func newRunView(name string, p *palette, start runStarter, close func(), size pty.Size, dir string) (*runView, error) {
	v := &runView{name: name, palette: p, start: start, close: close, dir: dir}
	return v, v.begin(size)
}

func (v *runView) begin(size pty.Size) error {
	v.size = size
	v.run++
	run := v.run
	terminal, err := v.start(v.terminalSize(), func(code int) { v.finished(run, code) })
	if err != nil {
		return err
	}
	v.terminal, v.running = terminal, true
	return nil
}

// Dir is the folder the run started in, not the one its program may have moved to.
func (v *runView) Dir() string { return v.dir }

func (v *runView) finished(run, exitCode int) {
	if run == v.run {
		v.running, v.exitCode = false, exitCode
	}
}

// terminalSize leaves the last row for the status line.
func (v *runView) terminalSize() pty.Size {
	return pty.Size{Cols: v.size.Cols, Rows: max(v.size.Rows-1, 1)}
}

func (v *runView) restart() {
	v.terminal.Close()
	if err := v.begin(v.size); err != nil {
		v.running, v.exitCode = false, -1
	}
}

func (v *runView) SendKey(key uv.KeyPressEvent) {
	if v.running {
		v.terminal.SendKey(key)
		return
	}
	switch key.String() {
	case "r":
		v.restart()
	case "enter", "esc", "q", "ctrl+d":
		v.close()
		v.Close()
	}
}

func (v *runView) Paste(text string) {
	if v.running {
		v.terminal.Paste(text)
	}
}

func (v *runView) Resize(size pty.Size) {
	v.size = size
	v.terminal.Resize(v.terminalSize())
}

func (v *runView) Draw(screen uv.Screen, area uv.Rectangle) {
	v.DrawScrolled(screen, area, 0)
}

func (v *runView) ScrollbackLen() int {
	if s, ok := v.terminal.(scrollable); ok {
		return s.ScrollbackLen()
	}
	return 0
}

func (v *runView) DrawScrolled(screen uv.Screen, area uv.Rectangle, offset int) {
	output := image.Rect(area.Min.X, area.Min.Y, area.Max.X, max(area.Max.Y-1, area.Min.Y))
	if s, ok := v.terminal.(scrollable); ok {
		s.DrawScrolled(screen, output, offset)
	} else {
		v.terminal.Draw(screen, output)
	}
	if area.Dy() > 1 {
		text, style := v.status()
		drawText(screen, area.Min.X, area.Max.Y-1, area.Max.X, text, style)
	}
}

// status is the line under the output, in the color of the hints; the title tells how it ended.
func (v *runView) status() (string, uv.Style) {
	switch {
	case v.running:
		return "running  ctrl+c stops", v.palette.panelMuted
	case v.exitCode == 0:
		return "✓ done  r run again  enter close", v.palette.success
	}
	return fmt.Sprintf("✗ exit %d  r run again  enter close", v.exitCode), v.palette.failure
}

// FooterRows is the status line: it is not part of the script's output, so it is not selected.
func (v *runView) FooterRows() int {
	return 1
}

// Terminal is where the script's output is, for reading it as text.
func (v *runView) Terminal() view {
	return v.terminal
}

// WantsMouseAt passes the mouse to the script while it runs, if it asked for it.
func (v *runView) WantsMouseAt(point image.Point) bool {
	target, ok := v.terminal.(mouseAware)
	return v.running && ok && target.WantsMouseAt(point)
}

func (v *runView) SendMouse(event uv.MouseEvent) {
	if target, ok := v.terminal.(mouseAware); ok {
		target.SendMouse(event)
	}
}

func (v *runView) Cursor() (uv.Position, bool) {
	if !v.running {
		return image.Point{}, false
	}
	return v.terminal.Cursor()
}

func (v *runView) Title() string {
	switch {
	case v.running:
		return v.name + " ●"
	case v.exitCode == 0:
		return v.name + " ✓"
	}
	return fmt.Sprintf("%s ✗ %d", v.name, v.exitCode)
}

func (v *runView) Close() {
	v.terminal.Close()
}
