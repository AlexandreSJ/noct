// Package share turns a theme, the config or the keys into one line to copy, or a QR code, that
// noct import reads on another machine.
package share

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// What a code carries.
const (
	KindTheme  = "theme"
	KindConfig = "config"
	KindKeys   = "keys"
)

var Kinds = []string{KindTheme, KindConfig, KindKeys}

// prefix starts every code and says its format, so a later one can change it.
const prefix = "noct1."

// maxCode bounds what a code decodes to, so a pasted code cannot fill the memory.
const maxCode = 1 << 20

// Payload is what a code carries: its kind, a name (the theme's) and the TOML.
type Payload struct {
	Kind, Name string
	Body       []byte
}

// Encode is the payload as a code: compressed, in URL-safe base64 after the prefix.
func Encode(p Payload) string {
	var packed bytes.Buffer
	w, _ := flate.NewWriter(&packed, flate.BestCompression) // the level is valid
	fmt.Fprintf(w, "%s\n%s\n", p.Kind, p.Name)
	w.Write(p.Body)
	w.Close()
	return prefix + base64.RawURLEncoding.EncodeToString(packed.Bytes())
}

// Decode reads a code back. Spaces and line breaks a copy left in it are ignored.
func Decode(code string) (Payload, error) {
	code = strings.Join(strings.Fields(code), "")
	data, ok := strings.CutPrefix(code, prefix)
	if !ok {
		return Payload{}, errors.New("not a noct code: it starts with " + prefix)
	}
	packed, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return Payload{}, errors.New("the code is cut or changed: copy all of it")
	}
	text, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(packed)), maxCode+1))
	if err != nil || len(text) > maxCode {
		return Payload{}, errors.New("the code is cut or changed: copy all of it")
	}
	kind, rest, _ := strings.Cut(string(text), "\n")
	name, body, ok := strings.Cut(rest, "\n")
	if !ok || !isKind(kind) {
		return Payload{}, fmt.Errorf("the code carries %q, which this noct does not know", kind)
	}
	return Payload{Kind: kind, Name: name, Body: []byte(body)}, nil
}

func isKind(kind string) bool {
	for _, k := range Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Command is the line that imports the code.
func Command(code string) string {
	return "noct import " + code
}
