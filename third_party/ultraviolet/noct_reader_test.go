package uv

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"
)

// An alt+letter key followed by mouse motion reports arrives while the mouse keeps moving, not
// once it stops.
func TestAltKeyArrivesWhileTheMouseMoves(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	events := make(chan Event, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go NewTerminalReader(r, "xterm-256color").StreamEvents(ctx, events) //nolint:errcheck

	go func() {
		_, _ = w.Write([]byte("\x1bt"))
		for x := 1; ctx.Err() == nil; x++ {
			_, _ = w.Write([]byte(fmt.Sprintf("\x1b[<35;%d;5M", x%80+1))) // motion, no button
			time.Sleep(5 * time.Millisecond)
		}
	}()
	deadline := time.After(DefaultEscTimeout * 4) // the motion never stops: only decoding it early passes
	for {
		select {
		case event := <-events:
			if key, ok := event.(KeyPressEvent); ok {
				if !key.MatchString("alt+t") {
					t.Fatalf("got %v, want alt+t", key)
				}
				return
			}
		case <-deadline:
			t.Fatal("alt+t waited for the mouse to stop")
		}
	}
}
