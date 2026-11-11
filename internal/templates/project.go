package templates

import (
	"os"
	"path/filepath"
)

// A project can keep templates in its repository, in .noct/templates. They come with the
// repository, which anyone with access to it can change, so their commands are always asked.

// ProjectDir finds the templates of the project that contains dir: the nearest .noct/templates in
// dir or one of its parents, the way git finds its repository. It is "" outside a project.
func ProjectDir(dir string) string {
	if dir == "" {
		return ""
	}
	for {
		candidate := filepath.Join(dir, ".noct", "templates")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ListProject reads the project's templates as List does, each marked as the project's and with
// every command asked.
func ListProject(dir string) []Entry {
	entries := List(dir)
	for i := range entries {
		entries[i].Project = true
		entries[i].Template = entries[i].Template.AskEverything()
	}
	return entries
}

// LoadProject reads one of the project's templates, with every command asked.
func LoadProject(dir, name string) (Template, error) {
	t, err := Load(dir, name)
	return t.AskEverything(), err
}

// AskEverything is t with each command that would run asked instead.
func (t Template) AskEverything() Template {
	t.Root = t.Root.askEverything()
	return t
}

func (n Node) askEverything() Node {
	if n.Pane != nil {
		p := *n.Pane
		if p.Start == Run {
			p.Start = Ask
		}
		n.Pane = &p
	}
	for _, list := range []*[]Node{&n.Columns, &n.Rows} {
		children := make([]Node, len(*list))
		for i, child := range *list {
			children[i] = child.askEverything()
		}
		if len(children) > 0 {
			*list = children
		}
	}
	return n
}
