package app

import (
	"bytes"
	"image"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/AlexandreSJ/noct/third_party/vt"
	"github.com/charmbracelet/colorprofile"

	"github.com/AlexandreSJ/noct/internal/config"
)

// terminalReplay draws an app's frames to a real TerminalScreen and plays the bytes it writes on
// an emulator, to check that what a terminal shows is what noct drew.
type terminalReplay struct {
	t      *testing.T
	a      *App
	out    bytes.Buffer
	screen *uv.TerminalScreen
	term   *vt.Emulator
}

func newTerminalReplay(t *testing.T, a *App) *terminalReplay {
	t.Helper()
	r := &terminalReplay{t: t, a: a, term: vt.NewEmulator(a.size.X, a.size.Y)}
	r.screen = uv.NewTerminalScreen(&r.out, uv.Environ{"TERM=xterm-256color", "COLORTERM=truecolor"})
	r.screen.SetColorProfile(colorprofile.TrueColor)
	_ = r.screen.Resize(a.size.X, a.size.Y)
	_ = r.screen.EnterAltScreen()
	return r
}

// frame draws one frame and fails where the emulator shows something else than what was drawn.
func (r *terminalReplay) frame(step string) {
	r.t.Helper()
	drawn := uv.NewScreenBuffer(r.a.size.X, r.a.size.Y)
	err := r.screen.Display(uv.DrawableFunc(func(s uv.Screen, area uv.Rectangle) {
		r.a.draw(s, area)
		for y := range r.a.size.Y {
			for x := range r.a.size.X {
				if c := s.CellAt(x, y); c != nil {
					cell := *c
					drawn.SetCell(x, y, &cell)
				}
			}
		}
	}))
	if err != nil {
		r.t.Fatal(err)
	}
	_, _ = r.term.Write(r.out.Bytes())
	r.out.Reset()
	for y := range r.a.size.Y {
		for x := range r.a.size.X {
			want, got := drawn.CellAt(x, y), r.term.CellAt(x, y)
			if want == nil || got == nil || want.Width == 0 {
				continue
			}
			fgShows := want.Content != " " // a blank shows its background only
			if want.Content != got.Content || !colorsEqual(orBlack(want.Style.Bg), orBlack(got.Style.Bg)) || fgShows && !colorsEqual(orBlack(want.Style.Fg), orBlack(got.Style.Fg)) {
				r.t.Errorf("%s: cell (%d, %d) shows %q fg %v bg %v, drawn %q fg %v bg %v", step, x, y, got.Content, got.Style.Fg, got.Style.Bg, want.Content, want.Style.Fg, want.Style.Bg)
				return
			}
		}
	}
}

// orBlack stands for the terminal's default color, which both sides leave nil.
func orBlack(c interface{ RGBA() (r, g, b, a uint32) }) interface{ RGBA() (r, g, b, a uint32) } {
	if c == nil {
		return image.Black
	}
	return c
}

func TestATabNameThatShrinksWhilePulsingLeavesNoOldColor(t *testing.T) {
	a, tb := twoPanesAnimated(t)
	a.size = image.Pt(80, 20)
	other := newTab(5, &cellView{})
	a.session.tabs = append(a.session.tabs, other)
	other.name = "building the project"
	r := newTerminalReplay(t, a)
	r.frame("start")
	a.markActivity(5) // the tab out of sight pulses
	frozen := a.now()
	for i, name := range []string{"building the proj", "build", "b", "building again"} {
		a.clock = func() time.Time { return frozen.Add(time.Duration(i+1) * 60 * time.Millisecond) }
		other.name = name
		r.frame("renamed to " + name)
	}
	_ = tb
}

func TestTabNamesThatChangeWhileTabsMoveLeaveNoOldColor(t *testing.T) {
	for _, background := range []string{config.BackgroundTerminal, config.BackgroundTheme} {
		t.Run(background, func(t *testing.T) {
			a, first := twoPanesAnimated(t)
			a.config.Appearance.Background = background
			a.applyPalette()
			a.size = image.Pt(80, 20)
			second := newTab(5, &cellView{})
			a.session.tabs = append(a.session.tabs, second)
			first.name, second.name = "claude code working", "zsh"
			r := newTerminalReplay(t, a)
			r.frame("start")
			frozen := a.now()
			step := 0
			next := func(what string, change func()) {
				step++
				a.clock = func() time.Time { return frozen.Add(time.Duration(step) * 40 * time.Millisecond) }
				change()
				r.frame(what)
			}
			next("switch to the second", func() { a.session.activeTab = 1 })
			next("first shrinks", func() { first.name = "claude" })
			next("second grows", func() { second.name = "~/workspace/noct" })
			next("switch back", func() { a.session.activeTab = 0 })
			next("active shrinks", func() { first.name = "c" })
			next("a tab opens", func() { a.openNewTab() })
			next("its name changes at once", func() { a.currentTab().name = "a much longer name" })
			next("and shrinks", func() { a.currentTab().name = "x" })
			a.markActivity(5)
			next("the second pulses and shrinks", func() { second.name = "~" })
			for range 20 {
				next("time passes", func() {})
			}
		})
	}
}
