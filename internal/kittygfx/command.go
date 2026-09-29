// Package kittygfx lets programs in panes show images with kitty's graphics protocol: their image
// commands become images in the outer terminal, displayed through Unicode placeholder cells that
// live in the pane's text, so images scroll, clip and clear along with it.
package kittygfx

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
)

const (
	commandStart = "\x1b_G"
	commandEnd   = "\x1b\\"
)

// Command is one graphics escape sequence: control keys like a=T,f=100 and an optional payload.
type Command struct {
	keys    map[string]string
	Payload []byte
}

// ParseCommand reads the body of a command, the part between ESC _G and ESC \.
func ParseCommand(body []byte) Command {
	control, payload, _ := bytes.Cut(body, []byte(";"))
	cmd := Command{keys: make(map[string]string), Payload: payload}
	for _, pair := range strings.Split(string(control), ",") {
		key, value, ok := strings.Cut(pair, "=")
		if ok && key != "" {
			cmd.keys[key] = value
		}
	}
	return cmd
}

func (c Command) Get(key string) string {
	return c.keys[key]
}

func (c Command) Has(key string) bool {
	_, ok := c.keys[key]
	return ok
}

// Int reads a numeric key; missing or malformed values count as 0, as they do in kitty.
func (c Command) Int(key string) int {
	n, _ := strconv.Atoi(c.keys[key]) // 0 on error is the protocol's default
	return n
}

// Action is the a key, which defaults to transmitting the image data only.
func (c Command) Action() string {
	if action := c.Get("a"); action != "" {
		return action
	}
	return "t"
}

// With returns a copy of the command with key set to value.
func (c Command) With(key, value string) Command {
	keys := make(map[string]string, len(c.keys)+1)
	for k, v := range c.keys {
		keys[k] = v
	}
	keys[key] = value
	return Command{keys: keys, Payload: c.Payload}
}

// Without returns a copy of the command without the given keys.
func (c Command) Without(removed ...string) Command {
	keys := make(map[string]string, len(c.keys))
	for k, v := range c.keys {
		if !slices.Contains(removed, k) {
			keys[k] = v
		}
	}
	return Command{keys: keys, Payload: c.Payload}
}

// Encode writes the complete escape sequence, keys sorted so output is deterministic.
func (c Command) Encode() []byte {
	names := make([]string, 0, len(c.keys))
	for k := range c.keys {
		names = append(names, k)
	}
	slices.Sort(names)
	pairs := make([]string, len(names))
	for i, k := range names {
		pairs[i] = k + "=" + c.keys[k]
	}
	var out bytes.Buffer
	out.WriteString(commandStart)
	out.WriteString(strings.Join(pairs, ","))
	if len(c.Payload) > 0 {
		out.WriteByte(';')
		out.Write(c.Payload)
	}
	out.WriteString(commandEnd)
	return out.Bytes()
}
