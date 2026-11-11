package share

import (
	"errors"
	"strings"

	"rsc.io/qr"
)

// quietZone is the light border around a QR code that scanners look for, in modules.
const quietZone = 2

// QR draws text as a QR code with half blocks, two modules a line, dark on light whatever the
// terminal's colors.
func QR(text string) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		code, err = qr.Encode(text, qr.L)
	}
	if err != nil {
		return "", errors.New("too long for a QR code; copy the line instead")
	}
	dark := func(x, y int) bool {
		x, y = x-quietZone, y-quietZone
		return x >= 0 && y >= 0 && x < code.Size && y < code.Size && code.Black(x, y)
	}
	size := code.Size + 2*quietZone
	var out strings.Builder
	for y := 0; y < size; y += 2 {
		out.WriteString("\x1b[30;107m")
		for x := range size {
			switch top, bottom := dark(x, y), dark(x, y+1) && y+1 < size; {
			case top && bottom:
				out.WriteString("█")
			case top:
				out.WriteString("▀")
			case bottom:
				out.WriteString("▄")
			default:
				out.WriteString(" ")
			}
		}
		out.WriteString("\x1b[0m\n")
	}
	return out.String(), nil
}
