package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexandreSJ/noct/internal/ipc"
)

// projectTemplates puts devTemplate in a project's .noct/templates as its, and returns the app
// working in the project.
func projectTemplates(t *testing.T) *App {
	t.Helper()
	templatesFolder(t)
	a := testApp(t)
	writeTemplate(t, filepath.Join(a.workDir, ".noct", "templates"), "dev", devTemplate)
	return a
}

func TestAProjectsTemplateAsksEveryCommand(t *testing.T) {
	a := projectTemplates(t)
	l := recordLaunches(a)
	a.openTemplates()
	if len(a.templates.list) != 1 || !a.templates.list[0].Project || !strings.Contains(screenText(a), "project") {
		t.Fatalf("listed %+v:\n%s", a.templates.list, screenText(a))
	}
	press(a, enterPress)
	if hasTyped(l, "fastfetch\n") {
		t.Error("a project's template ran its command without asking")
	}
	asked := 0
	for _, s := range a.shells {
		if s.rerun != "" {
			asked++
		}
	}
	if asked != 2 {
		t.Errorf("%d commands offered, want fastfetch and claude, each asked", asked)
	}
}

func TestANewSessionOpensWithATemplate(t *testing.T) {
	a := projectTemplates(t)
	tpl, err := a.templateToOpen(ipc.Hello{NewSession: true, Template: "dev", Dir: a.workDir})
	if err != nil || tpl == nil || tpl.Tab != "dev" {
		t.Fatalf("template %+v, %v", tpl, err)
	}
	for _, hello := range []ipc.Hello{{Template: "dev"}, {NewSession: true, Template: "nope", Dir: a.workDir}} {
		if _, err := a.templateToOpen(hello); err == nil {
			t.Errorf("%+v opened a template", hello)
		}
	}
}
