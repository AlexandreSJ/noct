package script

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func writeScript(t *testing.T, root, name, content string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadRendersTheCommandWithValues(t *testing.T) {
	dir := writeScript(t, t.TempDir(), "login", `
description: Log in
run: ./login.sh {{.env}} {{.user}}
params:
  - name: env
    choices: [dev, prod]
  - name: user
`)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "login" || s.Description != "Log in" {
		t.Fatalf("loaded %+v", s)
	}
	command, err := s.Command(map[string]string{"env": "dev", "user": "ana"})
	if err != nil || command != "./login.sh dev ana" {
		t.Fatalf("command %q, %v", command, err)
	}
	if _, err := s.Command(map[string]string{"env": "dev"}); err == nil {
		t.Fatal("a missing value rendered without error")
	}
}

func TestValidateRejectsBrokenScripts(t *testing.T) {
	cases := map[string]string{
		"empty run":              `run: ""`,
		"bad template":           `run: echo {{.x`,
		"bad param name":         "run: echo\nparams: [{name: my-param}]",
		"repeated param":         "run: echo\nparams: [{name: a}, {name: a}]",
		"default not in choices": "run: echo\nparams: [{name: a, choices: [x], default: y}]",
		"unknown source":         "run: echo\nparams: [{name: a, from: 'vault:x'}]",
		"bad output name":        "run: echo\noutputs: [my-output]",
		"repeated output":        "run: echo\noutputs: [a, a]",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeScript(t, t.TempDir(), "s", content)); err == nil {
				t.Fatal("loaded without error")
			}
		})
	}
}

func TestWorkingDir(t *testing.T) {
	s := Script{Dir: "/scripts/s"}
	if got := s.WorkingDir("/work"); got != "/work" {
		t.Errorf("no workdir: %s", got)
	}
	s.Workdir = "tools"
	if got := s.WorkingDir("/work"); got != "/scripts/s/tools" {
		t.Errorf("relative workdir: %s", got)
	}
	s.Workdir = "/abs"
	if got := s.WorkingDir("/work"); got != "/abs" {
		t.Errorf("absolute workdir: %s", got)
	}
}

func TestCreateMakesARunnableScriptAndDeleteRemovesIt(t *testing.T) {
	root := t.TempDir()
	s, err := Create(root, Fields{Name: "smoke-test", Run: "echo 'it: works'", Description: "Smoke: test"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if s.Run != "echo 'it: works'" || s.Description != "Smoke: test" {
		t.Errorf("created %+v", s)
	}
	if !strings.Contains(s.Notes(), "smoke-test") {
		t.Errorf("notes: %q", s.Notes())
	}
	if content, _ := os.ReadFile(s.File()); !strings.HasPrefix(string(content), "# description:") {
		t.Errorf("no hints on top:\n%s", content)
	}
	if _, err := Create(root, Fields{Name: "smoke-test", Run: "true"}, true); err == nil {
		t.Error("created the same script twice")
	}
	if _, err := Create(root, Fields{Name: "../escape", Run: "true"}, true); err == nil {
		t.Error("created a script outside the library")
	}
	if _, err := Create(root, Fields{Name: "empty"}, true); err == nil {
		t.Error("created a script without a command")
	}
	if err := Delete(s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Dir); !os.IsNotExist(err) {
		t.Errorf("script still there: %v", err)
	}
}

func TestLibraryFolders(t *testing.T) {
	data, project, elsewhere := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	personal := PersonalDir()
	writeScript(t, personal, "deploy", "run: echo personal")
	writeScript(t, personal, "broken", "run: ''")
	writeScript(t, filepath.Join(personal, "tests"), "smoke", "run: echo smoke")
	writeScript(t, filepath.Join(personal, "tests"), "deploy", "run: echo tests")
	if err := os.MkdirAll(filepath.Join(personal, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeScript(t, elsewhere, "remote", "run: echo remote")
	writeScript(t, filepath.Join(project, ProjectDirName, "scripts"), "build", "run: make")
	nested := filepath.Join(project, "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	lib := LoadLibrary(personal, []string{elsewhere, "/no/such/folder"}, nested)
	var folders []string
	for _, f := range lib.Folders {
		folders = append(folders, f.Name)
	}
	if want := []string{"empty", filepath.Base(elsewhere), "project", "tests"}; strings.Join(folders, ",") != strings.Join(sortedCopy(want), ",") {
		t.Fatalf("folders %v", folders)
	}
	if len(lib.Scripts) != 2 || lib.Scripts[0].Name != "broken" || lib.Scripts[0].Problem == nil {
		t.Errorf("scripts outside folders: %+v", lib.Scripts)
	}
	if len(lib.Problems) != 1 || !strings.Contains(lib.Problems[0].Error(), "/no/such/folder") {
		t.Errorf("problems %v", lib.Problems)
	}

	if s, ok := lib.Find("tests/deploy"); !ok || s.Run != "echo tests" || s.ID() != "tests/deploy" {
		t.Errorf("tests/deploy: %+v", s)
	}
	if s, ok := lib.Find("deploy"); !ok || s.Run != "echo personal" {
		t.Errorf("deploy is the one outside folders: %+v", s)
	}
	if s, ok := lib.Find("smoke"); !ok || s.Folder != "tests" {
		t.Errorf("a name found in a single folder: %+v", s)
	}
	if s, ok := lib.Find("build"); !ok || s.Folder != "project" {
		t.Errorf("project script: %+v", s)
	}
}

func sortedCopy(names []string) []string {
	sorted := append([]string(nil), names...)
	slices.Sort(sorted)
	return sorted
}
func TestStartingValues(t *testing.T) {
	s := Script{Params: []Param{
		{Name: "env", Choices: []string{"dev", "prod"}},
		{Name: "user", Default: "admin", Remember: true},
		{Name: "region", Default: "us"},
		{Name: "password", From: "env:PASSWORD"},
		{Name: "token", Secret: true, Remember: true},
	}}
	remembered := map[string]string{"user": "ana", "region": "eu", "token": "leaked"}
	getenv := func(name string) string { return map[string]string{"PASSWORD": "hunter2"}[name] }

	got := s.StartingValues(remembered, getenv)
	want := map[string]string{"env": "dev", "user": "ana", "region": "us", "password": "hunter2", "token": ""}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s = %q, want %q", name, got[name], value)
		}
	}
}

func TestCheckValues(t *testing.T) {
	s := Script{Params: []Param{{Name: "env", Choices: []string{"dev"}}, {Name: "pw", From: "env:PW"}}}
	if err := s.CheckValues(map[string]string{"env": "dev", "pw": "x"}); err != nil {
		t.Errorf("valid values: %v", err)
	}
	if err := s.CheckValues(map[string]string{"env": "prod", "pw": "x"}); err == nil {
		t.Error("accepted a value outside the choices")
	}
	if err := s.CheckValues(map[string]string{"env": "dev"}); err == nil || !strings.Contains(err.Error(), "PW") {
		t.Errorf("unset env: %v", err)
	}
}

func TestMemoryKeepsOnlyRememberedNonSecretValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "params.json")
	s := Script{Name: "login", Params: []Param{
		{Name: "user", Remember: true},
		{Name: "token", Remember: true, Secret: true},
		{Name: "env"},
	}}
	m, err := LoadMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Remember(s, map[string]string{"user": "ana", "token": "secret", "env": "dev"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadMemory(path)
	if err != nil {
		t.Fatal(err)
	}
	values := reloaded.Values(s)
	if values["user"] != "ana" || len(values) != 1 {
		t.Fatalf("remembered %v", values)
	}
	if content, _ := os.ReadFile(path); strings.Contains(string(content), "secret") {
		t.Fatal("a secret reached the disk")
	}
}

func TestDetectFindsTheVersion(t *testing.T) {
	if got := Detect("", "echo 'tool version 1.24.2 (linux)'"); got != "1.24.2" {
		t.Errorf("version_cmd: %q", got)
	}
	if got := Detect("", "echo 'no numbers here'"); got != "no numbers here" {
		t.Errorf("no version: %q", got)
	}
	if got := Detect("/bin/sh", ""); got == "" {
		t.Error("empty version for sh")
	}
}

func TestVersionsReportsMissingPrograms(t *testing.T) {
	v := NewVersions(nil)
	if version, known := v.Lookup(Script{Program: "noct-no-such-program"}); !known || version != NotFound {
		t.Errorf("missing program: %q %v", version, known)
	}
	if version, known := v.Lookup(Script{}); !known || version != "" {
		t.Errorf("no program: %q %v", version, known)
	}
}

func TestUpdateKeepsTheRestOfTheFile(t *testing.T) {
	root := t.TempDir()
	dir := writeScript(t, root, "login", `# how to log in
description: old
run: login {{.user}}
params:
  - name: user # the test account
    default: ana
`)
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	fields := FieldsOf(s)
	fields.Name, fields.Run, fields.Description = "sign-in", "sign-in --user {{.user}}", "Signs in"
	updated, err := Update(s, fields)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "sign-in" || updated.Run != "sign-in --user {{.user}}" || updated.Description != "Signs in" {
		t.Fatalf("updated %+v", updated)
	}
	content, _ := os.ReadFile(updated.File())
	for _, kept := range []string{"# how to log in", "# the test account", "default: ana"} {
		if !strings.Contains(string(content), kept) {
			t.Errorf("lost %q:\n%s", kept, content)
		}
	}
	if _, err := Update(updated, Fields{Name: "sign-in", Run: "echo {{.missing"}); err == nil {
		t.Error("saved a command that does not parse")
	}
}

func TestResolveDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := ResolveDir("~/tests/scripts"); got != filepath.Join(home, "tests", "scripts") {
		t.Errorf("~ path: %s", got)
	}
	if got := ResolveDir(""); got != PersonalDir() {
		t.Errorf("empty: %s", got)
	}
	if got := ResolveDir("/srv/scripts"); got != "/srv/scripts" {
		t.Errorf("absolute: %s", got)
	}
}

func TestBackupKeepsTheLastWorkingVersion(t *testing.T) {
	dir := writeScript(t, t.TempDir(), "s", "run: echo good\n")
	s, _ := Load(dir)
	if err := Backup(s); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.File(), []byte("run: echo {{.broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := Script{Name: "s", Dir: dir, Problem: errors.New("broken")}
	if err := Backup(broken); err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(s.BackupFile()); string(content) != "run: echo good\n" {
		t.Errorf("backup %q, want the working version", content)
	}
}

func TestCreateWithoutHintsAndWithParams(t *testing.T) {
	params, err := ParseParams(FormatParams(nil) + "- name: env\n  choices: [dev, prod]\n")
	if err != nil || len(params) != 1 {
		t.Fatalf("params %v, %v", params, err)
	}
	s, err := Create(t.TempDir(), Fields{Name: "deploy", Run: "deploy {{.env}}", Params: params, Notes: "Only on Fridays."}, false)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(s.File())
	if strings.Contains(string(content), "#") || len(s.Params) != 1 || s.Notes() != "Only on Fridays." {
		t.Errorf("created %+v:\n%s", s, content)
	}

	fields := FieldsOf(s)
	fields.Params, fields.Run, fields.Notes = nil, "deploy prod", "Any day."
	updated, err := Update(s, fields)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Params) != 0 || updated.Notes() != "Any day." {
		t.Errorf("updated %+v", updated)
	}
	if _, err := Update(updated, Fields{Name: "deploy", Run: "deploy {{.env}}"}); err == nil {
		t.Error("saved a command that uses a parameter that is gone")
	}
}

func TestValidateCatchesUnknownParameters(t *testing.T) {
	cases := map[string]bool{
		"echo {{.who}}":  true,
		"echo {{.whom}}": false,
		"{{if .who}}hi {{.who}}{{else}}{{.x}}{{end}}": false,
		"echo {{.who | printf \"%q\"}}":               true,
	}
	for run, valid := range cases {
		s := Script{Run: run, Params: []Param{{Name: "who"}}}
		if err := s.Validate(); (err == nil) != valid {
			t.Errorf("%q: error %v, want valid=%v", run, err, valid)
		}
	}
}

func TestRunsKeepTheLatestThousand(t *testing.T) {
	path := filepath.Join(t.TempDir(), RunsFile)
	start := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	for i := range runsKept + 5 {
		name := "a"
		if i == runsKept+4 {
			name = "b"
		}
		if err := RecordRun(path, name, start.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := Runs(path)
	if err != nil || len(runs) != runsKept || runs[0].At != start.Add(5*time.Second) {
		t.Fatalf("%d runs from %v, %v", len(runs), runs[0].At, err)
	}

	scripts := []Script{{Name: "c"}, {Name: "a"}, {Name: "b"}}
	SortByLastRun(scripts, LastRuns(runs))
	if scripts[0].Name != "b" || scripts[1].Name != "a" || scripts[2].Name != "c" {
		t.Errorf("by last run: %v %v %v", scripts[0].Name, scripts[1].Name, scripts[2].Name)
	}
	scripts[0].Edited, scripts[2].Edited = start, start.Add(time.Hour)
	SortByEdited(scripts)
	if scripts[0].Name != "c" {
		t.Errorf("by edit: %v first", scripts[0].Name)
	}
}
