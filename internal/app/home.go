package app

import (
	_ "embed"
	"image"
	"math/rand/v2"
	"strings"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// homeHintDuration is how long a key press on the home screen shows its hint.
const homeHintDuration = 3 * time.Second

// homeTitleGap is the number of blank lines between the title and the raven.
const homeTitleGap = 2

//go:embed raven2.ascii
var ravenArt string

// titleLines spell noct with the same block characters as the raven.
var titleLines = []string{
	"█▀▄ █▀█ █▀▀ ▀█▀",
	"█ █ █▄█ █▄▄  █",
}

// logoLines is the raven without trailing spaces or blank lines around it.
var logoLines = trimArt(ravenArt)

func trimArt(art string) []string {
	var lines []string
	for _, line := range strings.Split(art, "\n") {
		lines = append(lines, strings.TrimRight(line, " "))
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func artWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, ansi.StringWidth(line))
	}
	return width
}

// drawHome shows the screen of noct without panes: the title, the raven below it and, under both,
// a hint. The hint's row is always kept, so showing it never moves the logo. A screen too small for
// the art falls back to the plain title. A menu on top clears the screen, so it reads alone.
func (a *App) drawHome(screen uv.Screen) {
	if a.mode != normalMode {
		return
	}
	area := homeArea(a.size)
	arts, hintY := homeLayout(area)
	now := time.Now()
	for i, art := range arts {
		lines := art.lines
		if i == 0 && a.intro != nil {
			lines = a.intro.frame(lines, now.Sub(a.intro.start))
		}
		lines = flickerFrame(lines, i, a.flickers, now)
		style := a.palette.focusedTitle
		if i > 0 {
			style = a.palette.logo
		}
		for row, line := range lines {
			drawText(screen, art.at.X, art.at.Y+row, area.Max.X, line, style)
		}
	}
	if a.homeHintShown {
		drawCentered(screen, area, hintY, a.homeHint(), a.palette.panelMuted)
	}
}

func homeArea(size image.Point) image.Rectangle {
	return image.Rect(0, 1, size.X, size.Y)
}

// homeArt is a block of the home's art as drawn: its lines and the cell of its top left corner.
type homeArt struct {
	lines []string
	at    image.Point
}

// homeLayout places the title and, when it fits, the raven below it, as one block centered in
// area, and returns them with the row of the hint. Drawing and clicks share it, so a click lands
// on the character it shows.
func homeLayout(area image.Rectangle) ([]homeArt, int) {
	title, logo := titleLines, logoLines
	blockHeight := len(title) + homeTitleGap + len(logo) + 2
	if blockHeight > area.Dy() || artWidth(logo) > area.Dx() {
		title, logo, blockHeight = []string{"noct"}, nil, 3
	}
	y := area.Min.Y + (area.Dy()-blockHeight)/2
	arts := []homeArt{{title, image.Pt(artLeft(area, title), y)}}
	y += len(title)
	if logo != nil {
		y += homeTitleGap
		arts = append(arts, homeArt{logo, image.Pt(artLeft(area, logo), y)})
		y += len(logo)
	}
	return arts, y + 1
}

// artLeft is the column that centers lines in area, keeping their left edges aligned.
func artLeft(area image.Rectangle, lines []string) int {
	return area.Min.X + max(area.Dx()-artWidth(lines), 0)/2
}

// handleHomeKey shows the prefix key that opens the commands. Ctrl+D twice in a row quits,
// the way ctrl+d leaves a shell; the second press must come while the hint is still showing.
func (a *App) handleHomeKey(key uv.KeyPressEvent) {
	quitting := key.MatchString("ctrl+d")
	if quitting && a.homeQuitArmed {
		a.quitting = true
		return
	}
	a.homeQuitArmed = quitting
	a.homeHintShown = true
	a.homeHintExpired = time.After(homeHintDuration)
}

func (a *App) homeHint() string {
	if a.homeQuitArmed {
		return "ctrl+d again to quit"
	}
	return a.prefix.String()
}

// The title's intro when noct opens: each cell of the letters flickers between 0 and 1 until a
// sweep from left to right, jittered per column, reveals the title within introDuration.
const (
	introDuration = 1000 * time.Millisecond
	introSweep    = 700 * time.Millisecond // the rest of introDuration is each column's jitter
	introFrame    = 30 * time.Millisecond
)

type intro struct {
	start time.Time
	// reveal is when each column shows its real character.
	reveal []time.Duration
}

func newIntro(width int, start time.Time) *intro {
	in := &intro{start: start, reveal: make([]time.Duration, width)}
	for column := range width {
		sweep := introSweep * time.Duration(column) / time.Duration(max(width-1, 1))
		in.reveal[column] = sweep + time.Duration(rand.Int64N(int64(introDuration-introSweep)))
	}
	return in
}

// frame is the title as it shows elapsed into the intro: cells of columns not yet revealed are
// 0 or 1, and blank cells stay blank so the digits already have the letters' shape.
func (in *intro) frame(lines []string, elapsed time.Duration) []string {
	shown := make([]string, len(lines))
	for i, line := range lines {
		runes := []rune(line)
		for column, r := range runes {
			if r != ' ' && column < len(in.reveal) && elapsed < in.reveal[column] {
				runes[column] = rune('0' + rand.IntN(2))
			}
		}
		shown[i] = string(runes)
	}
	return shown
}

// startIntro plays the intro on the home screen.
func (a *App) startIntro() {
	a.intro = newIntro(artWidth(titleLines), time.Now())
	a.homeTick = time.After(introFrame)
}

// The pointer entering a character of the home's art makes only that cell flicker between 0 and 1
// for flickerDuration; entering it again while it flickers starts it over.
const flickerDuration = 400 * time.Millisecond

// artCell is a character of the home's art: which block of homeLayout, and its line and column.
type artCell struct{ art, line, column int }

// hoverHome flickers the character of the art under the pointer at point when the pointer has just
// entered it; motion within a cell, over blanks or off the art does nothing.
func (a *App) hoverHome(point image.Point) {
	if point == a.hovered {
		return
	}
	a.hovered = point
	a.flickerCellAt(point)
}

// flickerCellAt starts the flicker of the character of the art at point; blank cells and cells
// off the art do nothing.
func (a *App) flickerCellAt(point image.Point) {
	arts, _ := homeLayout(homeArea(a.size))
	for i, art := range arts {
		line, column := point.Y-art.at.Y, point.X-art.at.X
		if line < 0 || line >= len(art.lines) || column < 0 {
			continue
		}
		runes := []rune(art.lines[line])
		if column >= len(runes) || runes[column] == ' ' {
			continue
		}
		if a.flickers == nil {
			a.flickers = map[artCell]time.Time{}
		}
		a.flickers[artCell{i, line, column}] = time.Now()
		if a.homeTick == nil {
			a.homeTick = time.After(introFrame)
		}
		return
	}
}

// flickerFrame is lines of the art at index art with each of its cells still flickering at now
// shown as 0 or 1.
func flickerFrame(lines []string, art int, flickers map[artCell]time.Time, now time.Time) []string {
	var shown []string
	for cell, start := range flickers {
		if cell.art != art || cell.line >= len(lines) || now.Sub(start) >= flickerDuration {
			continue
		}
		if shown == nil {
			shown = append([]string(nil), lines...)
		}
		runes := []rune(shown[cell.line])
		if cell.column < len(runes) && runes[cell.column] != ' ' {
			runes[cell.column] = rune('0' + rand.IntN(2))
			shown[cell.line] = string(runes)
		}
	}
	if shown == nil {
		return lines
	}
	return shown
}

// advanceHomeArt asks for the next frame of the intro and of the flickering cells, and ends each
// that is over; with none left, the frames stop.
func (a *App) advanceHomeArt() {
	now := time.Now()
	if a.intro != nil && now.Sub(a.intro.start) >= introDuration {
		a.intro = nil
	}
	for cell, start := range a.flickers {
		if now.Sub(start) >= flickerDuration {
			delete(a.flickers, cell)
		}
	}
	if a.intro == nil && len(a.flickers) == 0 {
		a.homeTick = nil
		return
	}
	a.homeTick = time.After(introFrame)
}
