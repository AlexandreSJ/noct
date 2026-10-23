package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompletePath(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"README.md", "REVIEW.md", "src/main.go"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct{ typed, want string }{
		{"REA", "README.md"},
		{"R", "RE"},
		{"RE", "RE"},
		{"s", "src/"},
		{"src/m", "src/main.go"},
		{"nope/x", "nope/x"},
		{"zzz", "zzz"},
		{"", ""},
		{dir + "/REV", dir + "/REVIEW.md"},
	}
	for _, c := range cases {
		if got := completePath(c.typed, dir); got != c.want {
			t.Errorf("completePath(%q) = %q, want %q", c.typed, got, c.want)
		}
	}
}
