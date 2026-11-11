package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogTimesPrintsTheLinesAsked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.log")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".times", []byte("1 1762862400\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := printLogTimes([]string{"--from", "2", "--to", "2", path}, &out); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(out.String()), "\n"); len(lines) != 1 || !strings.HasSuffix(lines[0], " b") {
		t.Errorf("printed %q, want line b alone", out.String())
	}
	if err := printLogTimes(nil, &out); err == nil {
		t.Error("ran without a file")
	}
}
