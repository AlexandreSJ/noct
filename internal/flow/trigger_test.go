package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestTriggersReadAndWriteTheirYAML(t *testing.T) {
	f := parse(t, `
triggers:
  - manual
  - cron: "0 9 * * 1-5"
  - watch: {paths: ["./src/**/*.go"], debounce: 2s}
  - after: {flow: deploy, status: ok}
steps:
  - run: 'true'
`)
	if len(f.Triggers) != 4 || !f.Triggers[0].Manual || f.Triggers[1].Cron != "0 9 * * 1-5" ||
		f.Triggers[2].Watch.Paths[0] != "./src/**/*.go" || f.Triggers[3].After.Flow != "deploy" {
		t.Fatalf("triggers = %+v", f.Triggers)
	}
	if err := f.Validate(nil); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(f)
	if err != nil || !strings.Contains(string(out), "- manual\n") {
		t.Errorf("marshaled:\n%s %v", out, err)
	}
}

func TestValidateRejectsBadTriggers(t *testing.T) {
	cases := map[string]string{
		"bad cron":       `[{cron: "61 * * * *"}]`,
		"short cron":     `[{cron: "* *"}]`,
		"two kinds":      `[{cron: "* * * * *", after: {flow: x}}]`,
		"no watch paths": `[{watch: {paths: []}}]`,
		"bad debounce":   `[{watch: {paths: [a], debounce: soon}}]`,
		"after itself":   `[{after: {flow: test}}]`,
		"after status":   `[{after: {flow: x, status: maybe}}]`,
		"unknown word":   `[sometimes]`,
	}
	for name, triggers := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "test"+Ext)
			writeFile(t, file, "triggers: "+triggers+"\nsteps: [{run: 'true'}]\n")
			f, err := Load(file)
			if err == nil {
				err = f.Validate(nil)
			}
			if err == nil {
				t.Error("accepted")
			}
		})
	}
}

func TestCronMatches(t *testing.T) {
	at := func(s string) time.Time {
		t.Helper()
		v, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	cases := []struct {
		cron, at string
		want     bool
	}{
		{"0 9 * * 1-5", "2026-09-28 09:00", true}, // a Monday
		{"0 9 * * 1-5", "2026-09-27 09:00", false},
		{"0 9 * * 1-5", "2026-09-28 09:01", false},
		{"*/15 * * * *", "2026-09-28 10:45", true},
		{"*/15 * * * *", "2026-09-28 10:46", false},
		{"0 0 1 jan *", "2027-01-01 00:00", true},
		{"30 8 * * sun,7", "2026-09-27 08:30", true},
		{"0 12 15 * mon", "2026-09-15 12:00", true}, // day of month or of week
		{"0 12 15 * mon", "2026-09-28 12:00", true},
		{"0 12 15 * mon", "2026-09-16 12:00", false},
		{"5-10/5 * * * *", "2026-09-28 10:10", true},
	}
	for _, c := range cases {
		cron, err := ParseCron(c.cron)
		if err != nil {
			t.Fatalf("%s: %v", c.cron, err)
		}
		if got := cron.Matches(at(c.at)); got != c.want {
			t.Errorf("%s at %s: %v", c.cron, c.at, got)
		}
	}
}

func TestMatchPath(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/a/b/c.go", true},
		{"src/**/*.go", "src/a/b/c.txt", false},
		{"src/*.go", "src/a/c.go", false},
		{"**", "any/thing", true},
		{"docs/*.md", "docs/a.md", true},
	}
	for _, c := range cases {
		if got := MatchPath(c.pattern, c.path); got != c.want {
			t.Errorf("%s ~ %s: %v", c.pattern, c.path, got)
		}
	}
}

// Flows that start each other after they end would run forever: each gets a problem.
func TestAfterTriggersInALoopAreProblems(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a"+Ext), "triggers: [{after: {flow: b}}]\nsteps: [{run: 'true'}]\n")
	writeFile(t, filepath.Join(dir, "b"+Ext), "triggers: [{after: {flow: a}}]\nsteps: [{run: 'true'}]\n")
	writeFile(t, filepath.Join(dir, "c"+Ext), "triggers: [{after: {flow: a}}]\nsteps: [{run: 'true'}]\n")
	flows, err := LoadAll(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range flows {
		looped := f.Problem != nil && strings.Contains(f.Problem.Error(), "loop")
		if looped != (f.Name != "c") {
			t.Errorf("%s: problem %v", f.Name, f.Problem)
		}
	}
}

func TestEnablingTriggersIsKeptApartFromTheFlow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "triggers.json")
	if Enabled(path)["x"] {
		t.Fatal("a flow starts enabled")
	}
	if err := SetEnabled(path, "x", true); err != nil {
		t.Fatal(err)
	}
	if err := SetEnabled(path, "y", true); err != nil {
		t.Fatal(err)
	}
	if err := SetEnabled(path, "x", false); err != nil {
		t.Fatal(err)
	}
	if on := Enabled(path); on["x"] || !on["y"] {
		t.Errorf("enabled = %v", on)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
