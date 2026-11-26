package app

import (
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// Dragging over a popup's text selects it, shown reversed, and letting go copies it.
func TestDraggingOverAPopupCopiesItsText(t *testing.T) {
	a := testApp(t)
	a.config.Mouse = true
	a.openPrompt("rename", "some text here", "enter saves", func(string) {})
	drawnApp(a)
	area := a.popupArea.Inset(1)
	y := -1
	for row := area.Min.Y; row < area.Max.Y && y < 0; row++ {
		if strings.Contains(screenRow(a, row), "some text here") {
			y = row
		}
	}
	if y < 0 {
		t.Fatalf("no prompt text in the popup:\n%s", screenText(a))
	}
	x := strings.Index(screenRow(a, y), "some")
	from, to := uv.Mouse{X: area.Min.X, Y: y, Button: uv.MouseLeft}, uv.Mouse{X: area.Max.X - 1, Y: y, Button: uv.MouseLeft}
	a.handleMouse(uv.MouseClickEvent(from))
	a.handleMouse(uv.MouseMotionEvent(to))
	drawnApp(a)
	if c := drawnApp(a).CellAt(x, y); c == nil || c.Style.Attrs&uv.AttrReverse == 0 {
		t.Error("the text dragged over is not shown selected")
	}
	a.handleMouse(uv.MouseReleaseEvent(to))
	if text := a.popupSel.text(); !strings.Contains(text, "some text here") || a.notice != "copied to the clipboard" {
		t.Errorf("copied %q, notice %q", text, a.notice)
	}
}

func screenRow(a *App, y int) string {
	lines := strings.Split(screenText(a), "\n")
	if y < 0 || y >= len(lines) {
		return ""
	}
	return lines[y]
}
