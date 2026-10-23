package app

import (
	"strings"
	"testing"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

// The status sits at the right of a frame's line, and a title too long for what is left is cut
// with "…".
func TestTitleLineKeepsTheStatusAtTheRight(t *testing.T) {
	cases := []struct{ title, status, want string }{
		{"shell", "", "─ shell ─────────────────────"},
		{"shell", "↑ 0/18", "─ shell ──────────── ↑ 0/18 ─"},
		{"a very long panel name", "⛶ fullscreen", "─ a very lo… ─ ⛶ fullscreen ─"},
	}
	for _, c := range cases {
		buffer := uv.NewScreenBuffer(29, 1)
		drawTitleLine(buffer, 0, 29, 0, c.title, c.status, uv.Style{}, uv.Style{})
		if got := strings.TrimRight(buffer.Render(), " "); got != strings.TrimRight(c.want, " ") {
			t.Errorf("title %q status %q drew\n%q, want\n%q", c.title, c.status, got, c.want)
		}
	}
}
