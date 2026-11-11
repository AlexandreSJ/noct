package share

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestACodeReadsBackWhatItCarries(t *testing.T) {
	p := Payload{Kind: KindTheme, Name: "dusk", Body: []byte("base = \"noct\"\n\n[ansi]\ncolor1 = \"#ff0000\"\n")}
	code := Encode(p)
	if !strings.HasPrefix(code, "noct1.") || strings.ContainsAny(code, " \n+/=") {
		t.Fatalf("code %q, want one word after noct1.", code)
	}
	got, err := Decode(code[:20] + "\n  " + code[20:]) // as a copy from a wrapped terminal can be
	if err != nil || got.Kind != p.Kind || got.Name != p.Name || string(got.Body) != string(p.Body) {
		t.Errorf("decoded %+v, %v; want %+v", got, err, p)
	}
}

func TestABadCodeTellsWhy(t *testing.T) {
	cases := map[string]string{
		"hello":                              "starts with noct1.",
		"noct1.!!!":                          "copy all of it",
		Encode(Payload{Kind: KindKeys})[:12]: "copy all of it",
		Encode(Payload{Kind: "plugin"}):      `"plugin"`,
	}
	for code, want := range cases {
		if _, err := Decode(code); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Decode(%q) = %v, want an error with %q", code, err, want)
		}
	}
}

// modulesOf reads the half blocks back into the modules they draw.
func modulesOf(drawn string) [][]bool {
	var rows [][]bool
	for _, line := range strings.Split(strings.TrimSuffix(drawn, "\n"), "\n") {
		line = strings.TrimSuffix(strings.TrimPrefix(line, "\x1b[30;107m"), "\x1b[0m")
		var top, bottom []bool
		for _, r := range line {
			top = append(top, r == '█' || r == '▀')
			bottom = append(bottom, r == '█' || r == '▄')
		}
		rows = append(rows, top, bottom)
	}
	return rows
}

func TestTheQRCodeScans(t *testing.T) {
	text := Command(Encode(Payload{Kind: KindConfig, Body: []byte(strings.Repeat("mouse = true\n", 40))}))
	drawn, err := QR(text)
	if err != nil {
		t.Fatal(err)
	}
	modules := modulesOf(drawn)
	if len(modules) < 21 || !modules[quietZone][quietZone] || modules[0][0] {
		t.Fatalf("drawn %d rows, corner dark %v, border dark %v", len(modules), modules[quietZone][quietZone], modules[0][0])
	}
	zbar, err := exec.LookPath("zbarimg")
	if err != nil {
		t.Skip("zbarimg is not installed to scan it")
	}
	const scale = 4
	img := image.NewGray(image.Rect(0, 0, len(modules[0])*scale, len(modules)*scale))
	for y, row := range modules {
		for x, isDark := range row {
			c := color.Gray{Y: 255}
			if isDark {
				c = color.Gray{}
			}
			for i := range scale * scale {
				img.SetGray(x*scale+i%scale, y*scale+i/scale, c)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "qr.png")
	f, _ := os.Create(path)
	png.Encode(f, img)
	f.Close()
	out, err := exec.Command(zbar, "--quiet", "--raw", path).Output()
	if err != nil || strings.TrimSpace(string(out)) != text {
		t.Errorf("scanned %q (%v), want %q", out, err, text)
	}
}

func TestTooMuchForAQRCode(t *testing.T) {
	if _, err := QR(strings.Repeat("x", 5000)); err == nil || !strings.Contains(err.Error(), "copy the line") {
		t.Errorf("err = %v", err)
	}
}
