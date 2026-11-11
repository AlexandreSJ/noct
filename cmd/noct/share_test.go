package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/config"
	"github.com/AlexandreSJ/noct/internal/theme"
)

// shareDirs is a machine of its own: a config and a themes folder, answering yes or no.
func shareDirs(t *testing.T, answer string) (shareIO, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	out := &bytes.Buffer{}
	return shareIO{in: strings.NewReader(answer + "\n"), out: out, interactive: answer != "", configPath: filepath.Join(dir, "config.toml"), themesDir: filepath.Join(dir, "themes")}, out
}

// exported is the code a command printed, the last word of its line.
func exported(t *testing.T, out *bytes.Buffer) string {
	t.Helper()
	fields := strings.Fields(out.String())
	if len(fields) < 3 || fields[len(fields)-3] != "noct" || fields[len(fields)-2] != "import" {
		t.Fatalf("printed %q, want a noct import line last", out.String())
	}
	return fields[len(fields)-1]
}

func TestAThemeGoesToAnotherMachine(t *testing.T) {
	from, out := shareDirs(t, "")
	frost, _ := theme.Find(theme.Builtins(), "frost")
	mine := frost
	mine.Name, mine.Base, mine.Builtin = "dusk", "frost", false
	mine.Colors[theme.FocusedBorder] = "#ff00ff"
	mine.ANSI[1] = "#aa0000"
	if err := theme.Save(from.themesDir, mine, frost); err != nil {
		t.Fatal(err)
	}
	if err := exportCommand([]string{"theme", "dusk"}, from); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b[30;107m") {
		t.Error("drew a QR code with no terminal to show it")
	}
	code := exported(t, out)

	to, said := shareDirs(t, "")
	if err := importCommand([]string{code}, to); err != nil {
		t.Fatalf("%v; said %q", err, said)
	}
	got, errs := theme.LoadDir(to.themesDir, theme.Builtins())
	if len(errs) != 0 || len(got) != 1 || got[0].Colors != mine.Colors || got[0].ANSI != mine.ANSI {
		t.Fatalf("imported %+v (%v), want %+v", got, errs, mine)
	}
	if err := importCommand([]string{code}, to); err == nil {
		t.Error("replaced the theme without asking")
	}
}

func TestATerminalGetsTheQRCode(t *testing.T) {
	from, out := shareDirs(t, "y")
	if err := exportCommand([]string{"config"}, from); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\x1b[30;107m") || !strings.Contains(out.String(), "Scan the code") {
		t.Errorf("printed %q, want a QR code over the line", out.String())
	}
	exported(t, out)
	out.Reset()
	if err := exportCommand([]string{"config", "--no-qr"}, from); err != nil || strings.Contains(out.String(), "\x1b[30;107m") {
		t.Errorf("--no-qr printed %q, %v", out.String(), err)
	}
}

func TestSettingsGoOnlyAsChangedAndAfterYes(t *testing.T) {
	from, out := shareDirs(t, "")
	cfg := config.Default()
	cfg.Mouse, cfg.Logs.KeepDays, cfg.Editor = false, 30, "nvim"
	cfg.CustomBinds = []config.CustomBind{{Key: "prefix m", Name: "make", Command: "make"}}
	if err := config.Save(from.configPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := exportCommand([]string{"config"}, from); err != nil {
		t.Fatal(err)
	}
	code := exported(t, out)

	to, said := shareDirs(t, "n")
	if err := importCommand([]string{code}, to); err == nil || !strings.Contains(said.String(), "keep_days: 90 → 30") || !strings.Contains(said.String(), "check them") {
		t.Fatalf("no: err %v, said %q; want the changes shown and nothing saved", err, said)
	}
	if _, err := os.Stat(to.configPath); err == nil {
		t.Fatal("saved after no")
	}
	to.in = strings.NewReader("y\n")
	if err := importCommand([]string{code}, to); err != nil {
		t.Fatal(err)
	}
	got, _ := config.Load(to.configPath)
	if got.Mouse || got.Logs.KeepDays != 30 || got.Editor != "nvim" || len(got.CustomBinds) != 0 {
		t.Errorf("imported %+v; want the changed settings and no binds", got)
	}
}

func TestKeysGoWithTheCommandsTheirBindsRun(t *testing.T) {
	from, out := shareDirs(t, "")
	cfg := config.Default()
	cfg.Keys = map[string]map[string][]string{"scripts": {"edit": {"x"}}}
	cfg.CustomBinds = []config.CustomBind{{Key: "prefix m", Name: "make", Command: "make test"}}
	if err := config.Save(from.configPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := exportCommand([]string{"shortcuts"}, from); err != nil {
		t.Fatal(err)
	}
	code := exported(t, out)

	to, said := shareDirs(t, "")
	if err := importCommand([]string{code}, to); err == nil || !strings.Contains(said.String(), "prefix m  command make test") || !strings.Contains(said.String(), "--yes") {
		t.Fatalf("without a terminal: err %v, said %q; want the bind's command shown, nothing changed", err, said)
	}
	if err := importCommand([]string{code, "--yes"}, to); err != nil {
		t.Fatal(err)
	}
	got, _ := config.Load(to.configPath)
	if got.Keys["scripts"]["edit"][0] != "x" || len(got.CustomBinds) != 1 || got.CustomBinds[0].Command != "make test" {
		t.Errorf("imported keys %v, binds %v", got.Keys, got.CustomBinds)
	}
}

func TestAKittyThemeFileImports(t *testing.T) {
	to, said := shareDirs(t, "")
	path := filepath.Join(t.TempDir(), "Tokyo Night.conf")
	content := "background #1a1b26\nforeground #c0caf5\n"
	for n := range 16 {
		content += fmt.Sprintf("color%d #1122%02d\n", n, n)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := importCommand([]string{"theme", path}, to); err != nil {
		t.Fatalf("%v; said %q", err, said)
	}
	if _, err := os.Stat(filepath.Join(to.themesDir, "tokyo-night.toml")); err != nil {
		t.Errorf("no tokyo-night theme: %v; said %q", err, said)
	}
	if err := importCommand([]string{"theme", path, "--name", "noct"}, to); err == nil || !strings.Contains(err.Error(), "comes with") {
		t.Errorf("named after a built-in theme: %v", err)
	}
}
