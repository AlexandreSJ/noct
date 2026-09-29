package pane

import "bytes"

var (
	beginSync = []byte("\x1b[?2026h")
	endSync   = []byte("\x1b[?2026l")
)

// maxHeldFrame caps what a frame may hold back, so a program that never ends one still shows.
const maxHeldFrame = 1 << 20

// syncGate holds back a frame the program draws in synchronized output (mode 2026), from
// ESC[?2026h to ESC[?2026l, so the screen never shows it half drawn: a spinner redrawn inside
// one would hide the cursor and move it on every frame noct happened to catch in the middle.
type syncGate struct {
	held    []byte
	holding bool
	// searched is how far into held the end of the frame is known not to start.
	searched int
}

// feed takes program output and returns what may reach the screen now, in order. The markers
// stay in the output, so the emulator sees the program's modes as sent.
func (g *syncGate) feed(data []byte) []byte {
	g.held = append(g.held, data...)
	var ready []byte
	for {
		if !g.holding {
			i := bytes.Index(g.held, beginSync)
			if i < 0 {
				// Only a marker cut between two reads waits for the next one.
				return g.pass(ready, len(g.held)-partialSuffix(g.held, beginSync))
			}
			ready = g.pass(ready, i)
			g.holding, g.searched = true, len(beginSync)
		}
		i := bytes.Index(g.held[g.searched:], endSync)
		if i < 0 {
			if len(g.held) > maxHeldFrame {
				return g.pass(ready, len(g.held)) // gives up on this frame
			}
			g.searched = max(g.searched, len(g.held)-len(endSync)+1)
			return ready
		}
		ready = g.pass(ready, g.searched+i+len(endSync))
	}
}

// pass moves the first n held bytes to ready, ending the frame being held.
func (g *syncGate) pass(ready []byte, n int) []byte {
	ready = append(ready, g.held[:n]...)
	g.held = append(g.held[:0], g.held[n:]...)
	g.holding = false
	return ready
}

// pending tells whether output waits in the gate.
func (g *syncGate) pending() bool {
	return len(g.held) > 0
}

// release gives up on the frame being held and returns everything held.
func (g *syncGate) release() []byte {
	held := g.held
	g.held, g.holding = nil, false
	return held
}

// partialSuffix is how long the end of data that could begin marker is.
func partialSuffix(data, marker []byte) int {
	for n := min(len(marker)-1, len(data)); n > 0; n-- {
		if bytes.HasSuffix(data, marker[:n]) {
			return n
		}
	}
	return 0
}
