// Package script loads the scripts users keep in noct: a command line with parameters and notes,
// stored as plain files so they can be edited, versioned and shared.
package script

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"text/template/parse"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	FileName  = "script.yaml"
	NotesName = "notes.md"
)

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// validName allows names that are safe as directory names and easy to type in `noct run`.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Param struct {
	Name    string   `yaml:"name"`
	Prompt  string   `yaml:"prompt,omitempty"`
	Choices []string `yaml:"choices,omitempty"`
	Default string   `yaml:"default,omitempty"`
	// Remember offers the last value used instead of the default.
	Remember bool `yaml:"remember,omitempty"`
	// Secret values are masked while typed and never remembered.
	Secret bool `yaml:"secret,omitempty"`
	// From reads the value from the environment (env:NAME) instead of asking for it.
	From string `yaml:"from,omitempty"`
	// Path values are file paths, which tab completes in the form.
	Path bool `yaml:"path,omitempty"`
}

// Label is what the form shows for the parameter.
func (p Param) Label() string {
	if p.Prompt != "" {
		return p.Prompt
	}
	return p.Name
}

// Script is one saved command. Name and Dir come from where it is stored, not from the file.
type Script struct {
	Name string `yaml:"-"`
	Dir  string `yaml:"-"`
	// Folder is the name of the folder the script is in, or "" outside folders.
	Folder string `yaml:"-"`
	// Edited is when script.yaml last changed.
	Edited time.Time `yaml:"-"`
	// Problem is why the script cannot be loaded; a broken script is listed so it can be fixed.
	Problem     error   `yaml:"-"`
	Description string  `yaml:"description,omitempty"`
	Program     string  `yaml:"program,omitempty"`
	VersionCmd  string  `yaml:"version_cmd,omitempty"`
	Run         string  `yaml:"run"`
	Workdir     string  `yaml:"workdir,omitempty"`
	Params      []Param `yaml:"params,omitempty"`
	// Outputs are what the script gives back to a flow: the first line name=value of its output
	// becomes {{.steps.<id>.name}}.
	Outputs []string `yaml:"outputs,omitempty"`
	// Draft marks a script an AI wrote: it does not run until the user reviews and promotes it.
	Draft bool `yaml:"draft,omitempty"`
}

// ID names the script among all others: folder/name inside a folder, name outside.
func (s Script) ID() string {
	if s.Folder == "" {
		return s.Name
	}
	return s.Folder + "/" + s.Name
}

// FromCommand is a script for a command line typed by hand, as a custom bind's: each {{.name}}
// it reads is a parameter asked as a path, and it runs in dir.
func FromCommand(name, command, dir string) (Script, error) {
	s := Script{Name: name, Run: command, Workdir: dir}
	if fields := strings.Fields(command); len(fields) > 0 {
		s.Program = fields[0]
	}
	t, err := s.template()
	if err != nil {
		return Script{}, err
	}
	for _, field := range fieldsUsed(t.Root) {
		if !slices.ContainsFunc(s.Params, func(p Param) bool { return p.Name == field }) {
			s.Params = append(s.Params, Param{Name: field, Path: true})
		}
	}
	return s, nil
}

// Load reads the script stored in dir.
func Load(dir string) (Script, error) {
	content, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		return Script{}, fmt.Errorf("reading script %s: %w", filepath.Base(dir), err)
	}
	var s Script
	if err := yaml.Unmarshal(content, &s); err != nil {
		return Script{}, fmt.Errorf("script %s: %w", filepath.Base(dir), err)
	}
	s.Name, s.Dir, s.Edited = filepath.Base(dir), dir, modTime(filepath.Join(dir, FileName))
	if err := s.Validate(); err != nil {
		return Script{}, fmt.Errorf("script %s: %w", s.Name, err)
	}
	return s, nil
}

func modTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// Validate reports the first problem that would make the script fail when run.
func (s Script) Validate() error {
	if strings.TrimSpace(s.Run) == "" {
		return errors.New("run is empty")
	}
	seen := map[string]bool{}
	for _, p := range s.Params {
		if err := p.validate(); err != nil {
			return err
		}
		if seen[p.Name] {
			return fmt.Errorf("parameter %q appears twice", p.Name)
		}
		seen[p.Name] = true
	}
	if err := validateOutputs(s.Outputs); err != nil {
		return err
	}
	t, err := s.template()
	if err != nil {
		return err
	}
	for _, name := range fieldsUsed(t.Root) {
		if !seen[name] {
			return fmt.Errorf("run uses {{.%s}}, which is not a parameter", name)
		}
	}
	return nil
}

func validateOutputs(outputs []string) error {
	seen := map[string]bool{}
	for _, name := range outputs {
		if !identifier.MatchString(name) {
			return fmt.Errorf("output %q must be letters, digits and _ only", name)
		}
		if seen[name] {
			return fmt.Errorf("output %q appears twice", name)
		}
		seen[name] = true
	}
	return nil
}

// fieldsUsed lists the fields a template reads, such as name in {{.name}}.
func fieldsUsed(node parse.Node) []string {
	var names []string
	switch n := node.(type) {
	case *parse.ListNode:
		for _, child := range n.Nodes {
			names = append(names, fieldsUsed(child)...)
		}
	case *parse.ActionNode:
		names = fieldsUsed(n.Pipe)
	case *parse.PipeNode:
		for _, command := range n.Cmds {
			names = append(names, fieldsUsed(command)...)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			names = append(names, fieldsUsed(arg)...)
		}
	case *parse.FieldNode:
		names = append(names, n.Ident[0])
	case *parse.IfNode:
		names = append(names, branchFields(&n.BranchNode)...)
	case *parse.RangeNode:
		names = append(names, branchFields(&n.BranchNode)...)
	case *parse.WithNode:
		names = append(names, branchFields(&n.BranchNode)...)
	}
	return names
}

func branchFields(n *parse.BranchNode) []string {
	names := fieldsUsed(n.Pipe)
	names = append(names, fieldsUsed(n.List)...)
	if n.ElseList != nil {
		names = append(names, fieldsUsed(n.ElseList)...)
	}
	return names
}

func (p Param) validate() error {
	if !identifier.MatchString(p.Name) {
		return fmt.Errorf("parameter name %q must be letters, digits and _ only", p.Name)
	}
	if p.From != "" && !strings.HasPrefix(p.From, "env:") {
		return fmt.Errorf("parameter %q: from must look like env:NAME", p.Name)
	}
	if p.Default != "" && len(p.Choices) > 0 && !containsString(p.Choices, p.Default) {
		return fmt.Errorf("parameter %q: default %q is not one of its choices", p.Name, p.Default)
	}
	return nil
}

func (s Script) template() (*template.Template, error) {
	t, err := template.New(s.Name).Option("missingkey=error").Parse(s.Run)
	if err != nil {
		return nil, fmt.Errorf("run: %w", err)
	}
	return t, nil
}

// Command renders run with the parameter values into a shell command line.
func (s Script) Command(values map[string]string) (string, error) {
	t, err := s.template()
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, values); err != nil {
		return "", fmt.Errorf("script %s: %w", s.Name, err)
	}
	return out.String(), nil
}

// WorkingDir is where the script runs: workdir relative to the script's own directory, or base
// (where noct was started) when workdir is not set.
func (s Script) WorkingDir(base string) string {
	switch {
	case s.Workdir == "":
		return base
	case filepath.IsAbs(s.Workdir):
		return s.Workdir
	}
	return filepath.Join(s.Dir, s.Workdir)
}

// Notes is the script's notes.md, or "" when it has none.
func (s Script) Notes() string {
	content, err := os.ReadFile(filepath.Join(s.Dir, NotesName))
	if err != nil {
		return ""
	}
	return string(content)
}

func (s Script) File() string {
	return filepath.Join(s.Dir, FileName)
}

func (s Script) NotesFile() string {
	return filepath.Join(s.Dir, NotesName)
}

// Rename gives the script a new name, which is the name of its directory.
func Rename(s Script, name string) (Script, error) {
	if !validName.MatchString(name) {
		return Script{}, fmt.Errorf("script name %q: use letters, digits, '.', '-' and '_'", name)
	}
	dir := filepath.Join(filepath.Dir(s.Dir), name)
	if _, err := os.Stat(dir); err == nil {
		return Script{}, fmt.Errorf("script %s already exists", name)
	}
	if err := os.Rename(s.Dir, dir); err != nil {
		return Script{}, fmt.Errorf("renaming script %s: %w", s.Name, err)
	}
	s.Name, s.Dir = name, dir
	return s, nil
}

// BackupName is the copy of script.yaml kept from before the last edit.
const BackupName = FileName + ".bak"

// BackupFile is where the script's last working version is kept.
func (s Script) BackupFile() string {
	return filepath.Join(s.Dir, BackupName)
}

// Backup keeps a copy of a working script before it is edited, so a mistake can be undone. A
// broken script is not copied: that would replace the last working version.
func Backup(s Script) error {
	if s.Problem != nil {
		return nil
	}
	content, err := os.ReadFile(s.File())
	if err != nil {
		return fmt.Errorf("backing up script %s: %w", s.Name, err)
	}
	if err := os.WriteFile(s.BackupFile(), content, 0o644); err != nil {
		return fmt.Errorf("backing up script %s: %w", s.Name, err)
	}
	return nil
}

// Delete removes the script and its files.
func Delete(s Script) error {
	if s.Dir == "" || !validName.MatchString(filepath.Base(s.Dir)) {
		return fmt.Errorf("refusing to delete %q", s.Dir)
	}
	if err := os.RemoveAll(s.Dir); err != nil {
		return fmt.Errorf("deleting script %s: %w", s.Name, err)
	}
	return nil
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
