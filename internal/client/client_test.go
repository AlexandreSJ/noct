package client

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTraceOutputCopiesWhatTheTerminalGets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace")
	var terminal bytes.Buffer
	writer, closeTrace, err := traceOutput(&terminal, path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(writer, "\x1b[Hframe")
	closeTrace()
	traced, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.String() != "\x1b[Hframe" || string(traced) != "\x1b[Hframe" {
		t.Fatalf("terminal got %q and the trace %q, want both %q", terminal.String(), traced, "\x1b[Hframe")
	}
}

func TestTraceOutputWithoutAPathWritesOnlyToTheTerminal(t *testing.T) {
	var terminal bytes.Buffer
	writer, closeTrace, err := traceOutput(&terminal, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeTrace()
	if writer != io.Writer(&terminal) {
		t.Fatal("without NOCT_TRACE the output should go straight to the terminal")
	}
}
