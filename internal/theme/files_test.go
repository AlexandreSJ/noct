package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTheme(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSaveWritesOnlyWhatDiffersFromTheBase(t *testing.T) {
	dir := t.TempDir()
	noct := Builtins()[0]
	mine := noct
	mine.Name, mine.Base, mine.Builtin = "mine", "noct", false
	mine.Colors[FocusedBorder] = "#ff0000"
	if err := Save(dir, mine, noct); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "mine.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, `base = "noct"`) || !strings.Contains(text, "[panels]") || !strings.Contains(text, `focused_border = "#ff0000"`) {
		t.Errorf("saved:\n%s", text)
	}
	if strings.Count(text, "=") != 2 {
		t.Errorf("saved more than the base and one color:\n%s", text)
	}
}

func TestLoadDirReadsWhatSaveWrote(t *testing.T) {
	dir := t.TempDir()
	frost := Builtins()[5]
	mine := frost
	mine.Name, mine.Base, mine.Builtin = "mine", "frost", false
	mine.Colors[ActiveTabText] = "60"
	mine.Colors[PopupBackground] = "#101010"
	if err := Save(dir, mine, frost); err != nil {
		t.Fatal(err)
	}
	themes, errs := LoadDir(dir, Builtins())
	if len(errs) != 0 || len(themes) != 1 {
		t.Fatalf("loaded %v, errors %v", themes, errs)
	}
	got := themes[0]
	if got.Name != "mine" || got.Base != "frost" || got.Builtin || got.Colors != mine.Colors {
		t.Errorf("loaded %+v, want %+v", got, mine)
	}
}

func TestLoadDirBuildsOnAnotherUserTheme(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "a", "base = \"noct\"\n[panels]\nfocused_border = \"#111111\"\n")
	writeTheme(t, dir, "b", "base = \"a\"\n[tabs]\nactive_text = \"#222222\"\n")
	themes, errs := LoadDir(dir, Builtins())
	if len(errs) != 0 || len(themes) != 2 {
		t.Fatalf("loaded %v, errors %v", themes, errs)
	}
	b, _ := Find(themes, "b")
	if b.Colors[FocusedBorder] != "#111111" || b.Colors[ActiveTabText] != "#222222" || b.Colors[Background] != "#1a1b26" {
		t.Errorf("b has %v", b.Colors)
	}
}

func TestLoadDirSkipsABrokenThemeAndKeepsTheRest(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "bad", "[panels]\nfocused_border = \"#12345\"\n")
	writeTheme(t, dir, "unknown", "[panels]\nsparkles = \"#123456\"\n")
	writeTheme(t, dir, "cycle1", "base = \"cycle2\"\n")
	writeTheme(t, dir, "cycle2", "base = \"cycle1\"\n")
	writeTheme(t, dir, "orphan", "base = \"nope\"\n")
	writeTheme(t, dir, "noct", "")
	writeTheme(t, dir, "good", "[state]\nerror = \"#ff0000\"\n")
	themes, errs := LoadDir(dir, Builtins())
	if len(themes) != 1 || themes[0].Name != "good" || themes[0].Base != "noct" {
		t.Fatalf("loaded %v", themes)
	}
	if len(errs) != 6 {
		t.Fatalf("errors %v, want one per broken file", errs)
	}
	for _, name := range []string{"bad.toml", "unknown.toml", "cycle1.toml", "cycle2.toml", "orphan.toml", "noct.toml"} {
		found := false
		for _, err := range errs {
			found = found || strings.Contains(err.Error(), name)
		}
		if !found {
			t.Errorf("no error names %s: %v", name, errs)
		}
	}
}

func TestLoadDirOfAMissingFolderIsEmpty(t *testing.T) {
	themes, errs := LoadDir(filepath.Join(t.TempDir(), "none"), Builtins())
	if themes != nil || errs != nil {
		t.Errorf("loaded %v, %v", themes, errs)
	}
}

func TestCheckNameRefusesBuiltinsAndOddNames(t *testing.T) {
	if err := CheckName("mine"); err != nil {
		t.Errorf("mine: %v", err)
	}
	for _, name := range []string{"", "-x", "a b", "noct", "frost"} {
		if CheckName(name) == nil {
			t.Errorf("%q was taken as a name", name)
		}
	}
}

func TestDirIsInTheConfigFolder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/cfg")
	dir, err := Dir()
	if err != nil || dir != "/tmp/cfg/noct/themes" {
		t.Errorf("Dir() = %q, %v", dir, err)
	}
}
