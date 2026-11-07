package pty

import (
	"io"
	"testing"
	"time"
)

func TestForegroundCommandIsTheJobTheShellRuns(t *testing.T) {
	// An interactive shell gives the job its own process group and the keyboard.
	p, err := Start(StartOptions{Argv: []string{"sh", "-i", "-c", "sh -c 'sleep 5; :' 'a b'; :"}, Env: []string{"PATH=/usr/bin:/bin"}, Size: Size{Cols: 20, Rows: 5}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	go func() { _, _ = io.Copy(io.Discard, p) }()

	deadline := time.Now().Add(5 * time.Second)
	want := "sh -c 'sleep 5; :' 'a b'"
	for p.ForegroundCommand() != want {
		if time.Now().After(deadline) {
			t.Fatalf("foreground command %q, want %q", p.ForegroundCommand(), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Stop ends a program that ignores SIGTERM too, once the grace is over.
func TestStopKillsAProgramThatIgnoresTerm(t *testing.T) {
	p, err := Start(StartOptions{Argv: []string{"sh", "-c", "trap '' TERM; echo ready; while :; do sleep 1; done"}, Env: []string{"PATH=/usr/bin:/bin"}, Size: Size{Cols: 20, Rows: 5}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	ready := make(chan struct{})
	go func() {
		buf := make([]byte, 64)
		n, _ := p.Read(buf) // the trap is set once it prints
		if n > 0 {
			close(ready)
		}
		_, _ = io.Copy(io.Discard, p)
	}()
	<-ready
	exited := make(chan int, 1)
	go func() { exited <- p.Wait() }()
	p.Stop(100 * time.Millisecond)
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("the program still runs after Stop")
	}
}

func TestShellQuote(t *testing.T) {
	for arg, want := range map[string]string{
		"claude":        "claude",
		"--model=opus":  "--model=opus",
		"two words":     "'two words'",
		"it's":          `'it'\''s'`,
		"":              "''",
		"/usr/bin/node": "/usr/bin/node",
	} {
		if got := shellQuote(arg); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", arg, got, want)
		}
	}
}
