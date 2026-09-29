package kittygfx

import (
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePNG(t *testing.T, width, height int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logo.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return path
}

func newTestSession() *Session {
	next := uint32(0)
	return NewSession(func() uint32 { next++; return next }, func() image.Point { return image.Pt(10, 20) })
}

func command(body string) Command {
	return ParseCommand([]byte(body))
}

func TestFastfetchImageBecomesPlaceholders(t *testing.T) {
	path := writePNG(t, 760, 688)
	cmd := command("a=T,f=100,t=f,c=44;" + base64.StdEncoding.EncodeToString([]byte(path)))

	result := newTestSession().Handle(cmd)

	// 44 cols of 10px = 440px wide, so 688*440/760 = 398.3px tall = 20 rows of 20px.
	if result.Place == nil || *result.Place != (Placement{ImageID: 1, Cols: 44, Rows: 20}) {
		t.Fatalf("placement %+v", result.Place)
	}
	forwarded := string(result.ToTerminal)
	for _, want := range []string{"a=T", "U=1", "c=44", "r=20", "i=1", "q=2", "t=f", "f=100"} {
		if !strings.Contains(forwarded, want) {
			t.Errorf("forwarded command %q lacks %s", forwarded, want)
		}
	}
	if result.ToProgram != nil {
		t.Errorf("kitty does not answer commands without an id, got %q", result.ToProgram)
	}
}

func TestImageIDsAreTranslatedAndAnswered(t *testing.T) {
	session := newTestSession()
	session.Handle(command("a=T,f=24,s=20,v=40,i=7,c=2,r=2;AAAA"))     // outer id 1
	transmit := session.Handle(command("a=t,f=24,s=20,v=40,i=9;AAAA")) // outer id 2

	if got := string(transmit.ToProgram); got != "\x1b_Gi=9;OK\x1b\\" {
		t.Errorf("reply %q", got)
	}
	put := session.Handle(command("a=p,i=9,c=3"))
	if put.Place == nil || put.Place.ImageID != 2 || put.Place.Cols != 3 || put.Place.Rows != 3 {
		t.Fatalf("placement %+v", put.Place)
	}
	missing := session.Handle(command("a=p,i=5"))
	if !strings.Contains(string(missing.ToProgram), "ENOENT") {
		t.Errorf("reply for an unknown image %q", missing.ToProgram)
	}
}

func TestQuietRepliesAreSuppressed(t *testing.T) {
	session := newTestSession()
	if r := session.Handle(command("a=q,i=1,q=1")); r.ToProgram != nil {
		t.Errorf("q=1 still answered OK: %q", r.ToProgram)
	}
	if r := session.Handle(command("a=q,i=1")); string(r.ToProgram) != "\x1b_Gi=1;OK\x1b\\" {
		t.Errorf("query answer %q", r.ToProgram)
	}
}

func TestChunkedUploadIsForwardedWhole(t *testing.T) {
	session := newTestSession()
	if r := session.Handle(command("a=T,f=24,s=10,v=10,c=1,r=1,m=1;AAAA")); r.ToTerminal != nil {
		t.Fatal("forwarded before the upload finished")
	}
	session.Handle(command("m=1;BBBB"))
	result := session.Handle(command("m=0;CCCC"))

	if got := strings.Count(string(result.ToTerminal), commandStart); got != 3 {
		t.Errorf("forwarded %d chunks, want 3", got)
	}
	if result.Place == nil {
		t.Error("finished upload was not placed")
	}
}

func TestDeleteForwardsOuterIDs(t *testing.T) {
	session := newTestSession()
	session.Handle(command("a=T,f=24,s=10,v=10,i=4,c=1,r=1,q=2;AAAA"))
	result := session.Handle(command("a=d,d=i,i=4"))
	if got := string(result.ToTerminal); !strings.Contains(got, "i=1") || !strings.Contains(got, "d=I") {
		t.Errorf("delete %q", got)
	}
}

func TestSplitterSeparatesCommandsFromText(t *testing.T) {
	var s Splitter
	first := s.Split([]byte("hello\x1b_Ga=T;AAAA\x1b\\world\x1b"))
	second := s.Split([]byte("_Gi=1;OK\x1b"))
	third := s.Split([]byte("\\!"))

	var texts []string
	var commands int
	for _, segment := range append(append(first, second...), third...) {
		if segment.Command != nil {
			commands++
		} else {
			texts = append(texts, string(segment.Text))
		}
	}
	if commands != 2 || strings.Join(texts, "|") != "hello|world|!" {
		t.Errorf("commands=%d texts=%q", commands, texts)
	}
}

func TestSplitterPassesOtherEscapesThrough(t *testing.T) {
	var s Splitter
	segments := s.Split([]byte("\x1b[31mred\x1b[m"))
	if len(segments) != 1 || string(segments[0].Text) != "\x1b[31mred\x1b[m" {
		t.Errorf("segments %+v", segments)
	}
}

func TestPlaceholderEncodesRowColumnAndID(t *testing.T) {
	if got := []rune(PlaceholderText(42, 1, 0)); len(got) != 3 || got[1] != 0x030D || got[2] != 0x0305 {
		t.Errorf("placeholder %U", got)
	}
	if got := []rune(PlaceholderText(42+2<<24, 0, 0)); len(got) != 4 || got[3] != 0x030E {
		t.Errorf("high byte diacritic missing: %U", got)
	}
}
