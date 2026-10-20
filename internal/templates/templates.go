// Package templates keeps tab templates: a tab's layout, its panes and their colors as a YAML
// file, to open the same tab again in a new place.
package templates

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/AlexandreSJ/noct/internal/layout"
)

// Start is how a terminal's command starts when the template opens.
type Start string

const (
	// Run types the command into the shell at once.
	Run Start = "run"
	// Ask offers it in the run? box, to run with enter.
	Ask Start = "ask"
	// None opens the shell alone.
	None Start = "none"
)

// kinds are the panes a template can hold: the ones sessions open again.
var kinds = []string{"terminal", "scripts", "inspect", "logs"}

type Pane struct {
	Kind, Name, Command string
	Start               Start
	// Colors are the pane's own, by the theme file's table and key.
	Colors map[string]string
}

// Template is one tab: Name is the file's, Tab the tab's.
type Template struct {
	Name, Tab string
	// Colors are the tab's own, by the theme file's table and key.
	Colors map[string]string
	Root   Node
}

// Node is a pane, or a group of nodes side by side (Columns) or one under the other (Rows).
// Share is the part of its group it takes, 0 for an even split.
type Node struct {
	Columns, Rows []Node
	Stacked       bool
	Share         float64
	Pane          *Pane
}

func (n Node) children() []Node { return append(n.Columns, n.Rows...) }

// FromShape is the template node of a tab's layout, with the panes given. A pane left out of
// panes is left out of the layout, which closes over it; nothing left gives an empty node.
func FromShape(s layout.Shape, panes map[layout.PaneID]Pane) Node {
	if len(s.Children) == 0 {
		p, ok := panes[s.Pane]
		if !ok {
			return Node{}
		}
		return Node{Pane: &p, Share: s.Share}
	}
	var kept []Node
	for _, child := range s.Children {
		if n := FromShape(child, panes); n.Pane != nil || len(n.children()) > 0 {
			kept = append(kept, n)
		}
	}
	switch len(kept) {
	case 0:
		return Node{}
	case 1:
		kept[0].Share = s.Share
		return kept[0]
	}
	if len(kept) < len(s.Children) || even(kept) {
		for i := range kept {
			kept[i].Share = 0 // the layout closed over a pane, or splits evenly: split evenly
		}
	}
	n := Node{Stacked: s.Stacked, Share: s.Share}
	if s.Direction == layout.SideBySide {
		n.Columns = kept
	} else {
		n.Rows = kept
	}
	return n
}

func even(nodes []Node) bool {
	for _, n := range nodes {
		if math.Abs(n.Share-1/float64(len(nodes))) > 0.001 {
			return false
		}
	}
	return true
}

// Shape is the layout the node describes, with a new pane ID from newID for each pane, and the
// panes by those IDs.
func (n Node) Shape(newID func() layout.PaneID) (layout.Shape, map[layout.PaneID]Pane) {
	panes := map[layout.PaneID]Pane{}
	var build func(n Node) layout.Shape
	build = func(n Node) layout.Shape {
		if n.Pane != nil {
			id := newID()
			panes[id] = *n.Pane
			return layout.Shape{Pane: id, Share: n.Share}
		}
		s := layout.Shape{Direction: layout.SideBySide, Stacked: n.Stacked, Share: n.Share}
		if n.Rows != nil {
			s.Direction = layout.Stacked
		}
		for _, child := range n.children() {
			s.Children = append(s.Children, build(child))
		}
		return s
	}
	return build(n), panes
}

// UnmarshalYAML reads a node: {terminal: {name, run, start}} and the like, or {columns: [...]}
// or {rows: [...]}, with colors, share and stacked beside.
func (n *Node) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a pane or a group is a map, such as {terminal: {}}", value.Line)
	}
	var colors map[string]string
	for i := 0; i+1 < len(value.Content); i += 2 {
		key, v := value.Content[i].Value, value.Content[i+1]
		var err error
		switch {
		case slices.Contains(kinds, key):
			if n.Pane != nil {
				return fmt.Errorf("line %d: one pane per item", v.Line)
			}
			var fields struct{ Name, Run, Start string }
			err = v.Decode(&fields)
			n.Pane = &Pane{Kind: key, Name: fields.Name, Command: fields.Run, Start: Start(fields.Start)}
		case key == "columns":
			err = v.Decode(&n.Columns)
		case key == "rows":
			err = v.Decode(&n.Rows)
		case key == "colors":
			err = v.Decode(&colors)
		case key == "share":
			err = v.Decode(&n.Share)
		case key == "stacked":
			err = v.Decode(&n.Stacked)
		default:
			// a pane noct does not know, as from a newer noct: the app leaves it out and says so
			if n.Pane != nil {
				return fmt.Errorf("line %d: one pane per item", v.Line)
			}
			var fields struct{ Name, Run, Start string }
			err = v.Decode(&fields)
			n.Pane = &Pane{Kind: key, Name: fields.Name, Command: fields.Run, Start: Start(fields.Start)}
		}
		if err != nil {
			return err
		}
	}
	return n.check(colors, value.Line)
}

// check makes the node one thing, a pane or a group, and gives a pane its start.
func (n *Node) check(colors map[string]string, line int) error {
	groups := 0
	for _, g := range [][]Node{n.Columns, n.Rows} {
		if g != nil {
			groups++
		}
	}
	switch {
	case n.Pane != nil && groups > 0, groups > 1:
		return fmt.Errorf("line %d: an item is one pane or one group", line)
	case n.Pane == nil && groups == 0:
		return fmt.Errorf("line %d: an item needs a pane or a group", line)
	case n.Pane == nil && colors != nil:
		return fmt.Errorf("line %d: colors belong to a pane", line)
	}
	if n.Pane == nil {
		switch children := n.children(); len(children) {
		case 0:
			return fmt.Errorf("line %d: an empty group", line)
		case 1:
			share := n.Share
			*n = children[0] // a group of one is that one
			n.Share = share
		}
		return nil
	}
	p := n.Pane
	p.Colors = colors
	switch {
	case p.Start == "" && p.Command != "":
		p.Start = Ask
	case p.Start != "" && p.Start != Run && p.Start != Ask && p.Start != None:
		return fmt.Errorf("line %d: start is run, ask or none, not %s", line, p.Start)
	}
	if p.Command == "" {
		p.Start = ""
	}
	return nil
}

// MarshalYAML writes the node as UnmarshalYAML reads it, panes on one line each.
func (n Node) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key string, value *yaml.Node) {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, value)
	}
	scalar := func(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: v} }
	if p := n.Pane; p != nil {
		fields := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
		for _, f := range [][2]string{{"name", p.Name}, {"run", p.Command}, {"start", string(p.Start)}} {
			if f[1] != "" {
				fields.Content = append(fields.Content, scalar(f[0]), scalar(f[1]))
			}
		}
		add(p.Kind, fields)
		if len(p.Colors) > 0 {
			add("colors", colorsNode(p.Colors))
		}
	} else {
		group := &yaml.Node{Kind: yaml.SequenceNode}
		for _, child := range n.children() {
			var c yaml.Node
			if err := c.Encode(child); err != nil {
				return nil, err
			}
			group.Content = append(group.Content, &c)
		}
		key := "columns"
		if n.Rows != nil {
			key = "rows"
		}
		add(key, group)
	}
	if n.Share != 0 {
		add("share", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: fmt.Sprintf("%.3g", n.Share)})
	}
	if n.Stacked {
		add("stacked", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	}
	return node, nil
}

func colorsNode(colors map[string]string) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
	keys := make([]string, 0, len(colors))
	for k := range colors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &yaml.Node{Kind: yaml.ScalarNode, Value: colors[k], Style: yaml.DoubleQuotedStyle})
	}
	return node
}

// file is a template as it is written: the top group is spelled out with its layout.
type file struct {
	Name    string            `yaml:"name,omitempty"`
	Colors  map[string]string `yaml:"colors,omitempty"`
	Layout  string            `yaml:"layout,omitempty"`
	Stacked bool              `yaml:"stacked,omitempty"`
	Panes   []Node            `yaml:"panes"`
}

func (f file) template(name string) (Template, error) {
	t := Template{Name: name, Tab: f.Name, Colors: f.Colors}
	switch {
	case len(f.Panes) == 0:
		return t, errors.New("a template needs at least one pane")
	case len(f.Panes) == 1:
		t.Root = f.Panes[0]
		t.Root.Share = 0
	case f.Layout == "" || f.Layout == "columns":
		t.Root = Node{Columns: f.Panes, Stacked: f.Stacked}
	case f.Layout == "rows":
		t.Root = Node{Rows: f.Panes, Stacked: f.Stacked}
	default:
		return t, fmt.Errorf("layout is columns or rows, not %s", f.Layout)
	}
	return t, nil
}

func fileOf(t Template) file {
	f := file{Name: t.Tab, Colors: t.Colors}
	switch {
	case t.Root.Pane != nil:
		f.Panes = []Node{t.Root}
	case t.Root.Rows != nil:
		f.Layout, f.Stacked, f.Panes = "rows", t.Root.Stacked, t.Root.Rows
	default:
		f.Layout, f.Stacked, f.Panes = "columns", t.Root.Stacked, t.Root.Columns
	}
	return f
}

// Dir is where the templates are: the templates folder beside noct's config file.
func Dir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "noct", "templates"), nil
}

func path(dir, name string) string { return filepath.Join(dir, name+".yaml") }

// Load reads the template with that name.
func Load(dir, name string) (Template, error) {
	content, err := os.ReadFile(path(dir, name))
	if err != nil {
		return Template{}, err
	}
	var f file
	if err := yaml.Unmarshal(content, &f); err != nil {
		return Template{}, fmt.Errorf("template %s.yaml: %w", name, err)
	}
	t, err := f.template(name)
	if err != nil {
		return Template{}, fmt.Errorf("template %s.yaml: %w", name, err)
	}
	return t, nil
}

// LoadAll reads every template in dir, by name. One that cannot be read is left out, with an
// error naming its file; a missing folder has none.
func LoadAll(dir string) ([]Template, []error) {
	paths, _ := filepath.Glob(filepath.Join(dir, "*.yaml"))
	sort.Strings(paths)
	var templates []Template
	var errs []error
	for _, p := range paths {
		name := strings.TrimSuffix(filepath.Base(p), ".yaml")
		if CheckName(name) != nil {
			continue // not a name noct gives, such as a file being written
		}
		t, err := Load(dir, name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		templates = append(templates, t)
	}
	return templates, errs
}

// Save writes t through a temporary file, so a template being read is never half written.
func Save(dir string, t Template) error {
	content, err := yaml.Marshal(fileOf(t))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path(dir, t.Name)+".tmp", content, 0o644); err != nil {
		return err
	}
	return os.Rename(path(dir, t.Name)+".tmp", path(dir, t.Name))
}

func Delete(dir, name string) error { return os.Remove(path(dir, name)) }

// Rename gives a template another name, never over one that exists.
func Rename(dir, from, to string) error {
	if err := CheckName(to); err != nil {
		return err
	}
	if _, err := os.Stat(path(dir, to)); err == nil {
		return fmt.Errorf("there is a template %s already", to)
	}
	return os.Rename(path(dir, from), path(dir, to))
}

// validName is the rule scripts and flows use: safe as a file name and easy to type.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func CheckName(name string) error {
	if !validName.MatchString(name) {
		return errors.New("a template name has letters, digits, '.', '_' or '-', starting with a letter or digit")
	}
	return nil
}
