package app

import (
	"strings"
	"testing"
	"time"

	uv "github.com/AlexandreSJ/noct/third_party/ultraviolet"
)

func TestIntroTurnsBinaryIntoTheTitle(t *testing.T) {
	in := newIntro(artWidth(titleLines), time.Now())

	for i, line := range in.frame(titleLines, 0) {
		for column, r := range []rune(line) {
			original := []rune(titleLines[i])[column]
			if original == ' ' && r != ' ' || original != ' ' && r != '0' && r != '1' {
				t.Fatalf("line %d column %d is %q at the start, want a digit in the letters' shape", i, column, r)
			}
		}
	}
	if got := in.frame(titleLines, introDuration); strings.Join(got, "\n") != strings.Join(titleLines, "\n") {
		t.Errorf("after the intro:\n%s", strings.Join(got, "\n"))
	}
	for column := 1; column < len(in.reveal); column++ {
		if in.reveal[column] >= introDuration {
			t.Errorf("column %d reveals at %v, after the intro ends", column, in.reveal[column])
		}
	}
	first, last := in.reveal[0], in.reveal[len(in.reveal)-1]
	if first >= introDuration-introSweep || last < introSweep {
		t.Errorf("reveals from %v to %v, want a sweep from left to right", first, last)
	}
}

func TestIntroPlaysWhenAttachingToTheHomeScreen(t *testing.T) {
	a := testApp(t)
	a.startIntro()
	if a.intro == nil || a.homeTick == nil {
		t.Fatal("no intro")
	}
	a.intro.start = time.Now().Add(-introDuration)
	a.advanceHomeArt()
	if a.intro != nil || a.homeTick != nil {
		t.Error("the intro did not end after its duration")
	}
}

func TestHoveringACharacterOfTheHomeFlickersOnlyIt(t *testing.T) {
	a := testApp(t)
	a.config.Mouse = true
	arts, _ := homeLayout(homeArea(a.size))
	title := arts[0]
	hover(a, title.at.X, title.at.Y) // the first cell of the title, a block
	want := artCell{art: 0, line: 0, column: 0}
	if _, ok := a.flickers[want]; !ok || len(a.flickers) != 1 || a.homeTick == nil {
		t.Fatalf("flickers %v after hovering the title's first character", a.flickers)
	}
	if a.intro != nil {
		t.Error("hovering replayed the whole intro")
	}

	shown := flickerFrame(title.lines, 0, a.flickers, time.Now())
	for line, text := range shown {
		for column, r := range []rune(text) {
			original := []rune(title.lines[line])[column]
			if line == 0 && column == 0 {
				if r != '0' && r != '1' {
					t.Errorf("the clicked cell shows %q, want a digit", r)
				}
			} else if r != original {
				t.Errorf("line %d column %d shows %q, want only the clicked cell to flicker", line, column, r)
			}
		}
	}

	hover(a, title.at.X+3, title.at.Y) // a blank between letters
	hover(a, 0, a.size.Y-1)            // an empty corner
	click(a, title.at.X+1, title.at.Y) // a click on the art is not a hover
	if len(a.flickers) != 1 {
		t.Errorf("flickers %v after hovering blanks and clicking the art", a.flickers)
	}

	started := a.flickers[want]
	hover(a, title.at.X, title.at.Y) // back in the same cell
	a.flickers[want] = started.Add(-time.Millisecond)
	hover(a, title.at.X, title.at.Y) // moving within it
	if !a.flickers[want].Equal(started.Add(-time.Millisecond)) {
		t.Error("motion within the hovered cell restarted its flicker")
	}

	a.flickers[want] = time.Now().Add(-flickerDuration)
	a.advanceHomeArt()
	if len(a.flickers) != 0 || a.homeTick != nil {
		t.Error("the flicker did not end after its duration")
	}
}

func TestHoveringTheRavenFlickersItsCharacter(t *testing.T) {
	a := testApp(t)
	a.config.Mouse = true
	arts, _ := homeLayout(homeArea(a.size))
	raven := arts[1]
	column := strings.IndexFunc(raven.lines[0], func(r rune) bool { return r != ' ' })
	hover(a, raven.at.X+column, raven.at.Y)
	if _, ok := a.flickers[artCell{art: 1, line: 0, column: column}]; !ok || len(a.flickers) != 1 {
		t.Errorf("flickers %v after hovering the raven", a.flickers)
	}
}

func hover(a *App, x, y int) {
	a.handleEvent(uv.MouseMotionEvent{X: x, Y: y, Button: uv.MouseNone})
}
