package uv_test

import (
	"image/color"
	"math/rand"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/AlexandreSJ/noct/third_party/vt"
)

// noct: the renderer only sends what changed since the last frame, so any frame it gets wrong
// stays on the terminal as leftover cells. These tests replay its output into an emulator.

const renderWidth, renderHeight = 40, 8

var renderGlyphs = []string{"a", "│", "▄", "◂", " ", " ", "漢"}

// randomFrame draws a few random blocks of text, some with wide characters and other styles, on
// the given background, as noct does with a theme's background.
func randomFrame(r *rand.Rand, background color.Color) uv.DrawableFunc {
	seed, blocks := r.Int63(), 1+r.Intn(4)
	return func(screen uv.Screen, area uv.Rectangle) {
		fill := rand.New(rand.NewSource(seed))
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				screen.SetCell(x, y, &uv.Cell{Content: " ", Width: 1, Style: uv.Style{Bg: background}})
			}
		}
		for range blocks {
			left, top := fill.Intn(renderWidth), fill.Intn(renderHeight)
			right, bottom := min(left+1+fill.Intn(renderWidth/2), renderWidth), min(top+1+fill.Intn(4), renderHeight)
			for y := top; y < bottom; y++ {
				for x := left; x < right; x++ {
					cell := uv.Cell{Content: renderGlyphs[fill.Intn(len(renderGlyphs))], Width: 1}
					cell.Style.Bg = background
					switch fill.Intn(6) {
					case 0:
						cell.Style.Fg = color.RGBA{R: 200, A: 255}
					case 1:
						cell.Style.Bg = color.RGBA{G: 90, A: 255}
					case 2:
						cell.Style.Attrs = uv.AttrBold
					case 3:
						cell.Style.Attrs = uv.AttrReverse
					}
					if cell.Content == "漢" {
						if x+1 >= right {
							continue
						}
						cell.Width = 2
						screen.SetCell(x, y, &cell)
						x++
						continue
					}
					screen.SetCell(x, y, &cell)
				}
			}
		}
	}
}

// screenText shows each cell's text followed by a letter for its background: t for the
// terminal's own, and the red component otherwise, so leftover colors show up too.
func screenText(cellAt func(x, y int) *uv.Cell) string {
	var text strings.Builder
	for y := range renderHeight {
		for x := range renderWidth {
			cell := cellAt(x, y)
			switch {
			case cell == nil:
				text.WriteString(" t")
			case cell.Width > 0:
				text.WriteString(cell.Content + backgroundLetter(cell.Style))
			}
		}
		text.WriteString("\n")
	}
	return text.String()
}

func backgroundLetter(style uv.Style) string {
	background := style.Bg
	if style.Attrs&uv.AttrReverse != 0 {
		background = style.Fg
	}
	if background == nil {
		return "t"
	}
	red, _, _, _ := background.RGBA()
	return string(rune('a' + red>>8%20))
}

func TestRendererOutputMatchesTheFrame(t *testing.T) {
	themeBackground := color.RGBA{R: 20, G: 10, B: 10, A: 255}
	for seed := range int64(1000) {
		r := rand.New(rand.NewSource(seed))
		var background color.Color // the terminal's own background on even seeds
		if seed%2 == 1 {
			background = themeBackground
		}
		terminal := vt.NewEmulator(renderWidth, renderHeight)
		screen := uv.NewTerminalScreen(terminal, []string{"TERM=xterm-kitty"})
		screen.SetColorProfile(colorprofile.TrueColor) // the emulator is no terminal to detect it from
		_ = screen.Resize(renderWidth, renderHeight)
		_ = screen.EnterAltScreen()
		for frame := range 8 {
			draw := randomFrame(r, background)
			if err := screen.Display(draw); err != nil {
				t.Fatal(err)
			}
			want := uv.NewScreenBuffer(renderWidth, renderHeight)
			draw(want, want.Bounds())
			if got, want := screenText(terminal.CellAt), screenText(want.CellAt); got != want {
				t.Fatalf("seed %d, frame %d (background %v): the terminal shows\n%s\nbut the frame is\n%s", seed, frame, background, got, want)
			}
		}
	}
}

func bufferText(buffer *uv.Buffer, y int) string {
	var text strings.Builder
	for x := range buffer.Width() {
		cell := buffer.CellAt(x, y)
		if cell.Width == 0 {
			text.WriteString("_") // the second half of a wide cell
			continue
		}
		text.WriteString(cell.Content)
	}
	return text.String()
}

func TestDeletingCellsBeforeAWideCellMovesIt(t *testing.T) {
	buffer := uv.NewBuffer(5, 1)
	buffer.SetCell(0, 0, &uv.Cell{Content: "a", Width: 1})
	buffer.SetCell(1, 0, &uv.Cell{Content: "b", Width: 1})
	buffer.SetCell(2, 0, &uv.Cell{Content: "漢", Width: 2})
	buffer.SetCell(4, 0, &uv.Cell{Content: "c", Width: 1})
	buffer.DeleteCell(1, 0, 1, nil)
	if got := bufferText(buffer, 0); got != "a漢_c " {
		t.Fatalf("after deleting b the line is %q, want %q", got, "a漢_c ")
	}
}

func TestAWideCellOverTheFirstHalfOfAnotherBlanksItsSecondHalf(t *testing.T) {
	buffer := uv.NewBuffer(4, 1)
	buffer.SetCell(1, 0, &uv.Cell{Content: "漢", Width: 2})
	buffer.SetCell(0, 0, &uv.Cell{Content: "字", Width: 2})
	if got := bufferText(buffer, 0); got != "字_  " {
		t.Fatalf("the line is %q, want %q", got, "字_  ")
	}
}

func TestClearingTheBottomKeepsTheLastUnchangedLine(t *testing.T) {
	terminal := vt.NewEmulator(renderWidth, renderHeight)
	screen := uv.NewTerminalScreen(terminal, []string{"TERM=xterm-kitty"})
	_ = screen.Resize(renderWidth, renderHeight)
	_ = screen.EnterAltScreen()
	lines := []string{"top", "kept", "cleared"}
	draw := func(screen uv.Screen, _ uv.Rectangle) {
		for y, line := range lines {
			for x, r := range line {
				screen.SetCell(x, y, &uv.Cell{Content: string(r), Width: 1})
			}
		}
	}
	_ = screen.Display(uv.DrawableFunc(draw))
	lines = lines[:2]
	_ = screen.Display(uv.DrawableFunc(draw))
	if cell := terminal.CellAt(0, 1); cell == nil || cell.Content != "k" {
		t.Fatalf("the line above the cleared ones starts with %v, want k", cell)
	}
}
