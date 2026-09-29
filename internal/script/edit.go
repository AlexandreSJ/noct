package script

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Fields are the parts of a script edited in noct's form.
type Fields struct {
	Name        string
	Program     string
	Description string
	Run         string
	Params      []Param
	Notes       string
}

// FieldsOf are a script's fields as they are now.
func FieldsOf(s Script) Fields {
	return Fields{Name: s.Name, Program: s.Program, Description: s.Description, Run: s.Run, Params: s.Params, Notes: s.Notes()}
}

// hints explain, at the top of a new script.yaml, what each setting does.
const hints = `# description: what the script does, shown in the list.
# program: the program it runs, to show its version and help (optional).
# run: the command line; {{.name}} inserts the value of the parameter called name.
# params: values asked before running: name, prompt, choices, default, remember, secret, from.
`

const notesSkeleton = `# %s

How and when to run it, what to check afterwards, known problems.
`

// Create starts a new script in root from the fields, with the hints comment on top when asked.
func Create(root string, f Fields, withHints bool) (Script, error) {
	if err := f.check(); err != nil {
		return Script{}, err
	}
	dir := filepath.Join(root, f.Name)
	if _, err := os.Stat(dir); err == nil {
		return Script{}, fmt.Errorf("script %s already exists", f.Name)
	}
	body, err := yaml.Marshal(Script{Description: f.Description, Program: f.Program, Run: f.Run, Params: f.Params})
	if err != nil {
		return Script{}, fmt.Errorf("creating script %s: %w", f.Name, err)
	}
	if withHints {
		body = append([]byte(hints), body...)
	}
	notes := f.Notes
	if strings.TrimSpace(notes) == "" {
		notes = fmt.Sprintf(notesSkeleton, f.Name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Script{}, fmt.Errorf("creating script %s: %w", f.Name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), body, 0o644); err != nil {
		return Script{}, fmt.Errorf("creating script %s: %w", f.Name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, NotesName), []byte(notes), 0o644); err != nil {
		return Script{}, fmt.Errorf("creating script %s: %w", f.Name, err)
	}
	return Load(dir)
}

// Update saves the fields into the script. Only what changed is rewritten: the rest of
// script.yaml, comments included, stays as it was.
func Update(s Script, f Fields) (Script, error) {
	if err := f.check(); err != nil {
		return Script{}, err
	}
	changed := s
	changed.Program, changed.Description, changed.Run, changed.Params = f.Program, f.Description, f.Run, f.Params
	if err := changed.Validate(); err != nil {
		return Script{}, err
	}
	values := map[string]any{"program": f.Program, "description": f.Description, "run": f.Run}
	if !slices.EqualFunc(s.Params, f.Params, paramsEqual) {
		values["params"] = f.Params
	}
	if err := rewrite(s.File(), values); err != nil {
		return Script{}, fmt.Errorf("saving script %s: %w", s.Name, err)
	}
	if f.Notes != s.Notes() {
		if err := os.WriteFile(s.NotesFile(), []byte(f.Notes), 0o644); err != nil {
			return Script{}, fmt.Errorf("saving the notes of %s: %w", s.Name, err)
		}
	}
	if f.Name != s.Name {
		return Rename(changed, f.Name)
	}
	saved, err := Load(s.Dir)
	saved.Folder = s.Folder
	return saved, err
}

func paramsEqual(a, b Param) bool {
	return a.Name == b.Name && a.Prompt == b.Prompt && slices.Equal(a.Choices, b.Choices) && a.Default == b.Default &&
		a.Remember == b.Remember && a.Secret == b.Secret && a.From == b.From
}

// paramsExample is shown when a script without parameters has them edited.
const paramsExample = `# One entry per parameter, for example:
# - name: who
#   prompt: Who to greet
#   choices: [ana, bia]
#   default: ana
#   remember: true
`

// FormatParams writes parameters as the YAML list edited in the editor.
func FormatParams(params []Param) string {
	if len(params) == 0 {
		return paramsExample
	}
	out, err := yaml.Marshal(params)
	if err != nil {
		return paramsExample
	}
	return string(out)
}

// ParseParams reads parameters from the YAML list edited in the editor; comments alone are none.
func ParseParams(text string) ([]Param, error) {
	var params []Param
	if err := yaml.Unmarshal([]byte(text), &params); err != nil {
		return nil, fmt.Errorf("params: %w", err)
	}
	return params, nil
}

func (f Fields) check() error {
	if !validName.MatchString(f.Name) {
		return fmt.Errorf("script name %q: use letters, digits, '.', '-' and '_'", f.Name)
	}
	if strings.TrimSpace(f.Run) == "" {
		return errors.New("the script is empty")
	}
	return nil
}

// Promote clears a draft's mark, keeping the rest of its file: the user reviewed it and lets it run.
func Promote(s Script) error {
	if err := rewrite(s.File(), map[string]any{"draft": ""}); err != nil {
		return fmt.Errorf("promoting script %s: %w", s.ID(), err)
	}
	return nil
}

// rewrite sets top-level keys of a YAML file, keeping everything else in it. Values are strings or
// anything YAML can encode.
func rewrite(path string, values map[string]any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return err
	}
	if len(document.Content) == 0 {
		document = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return errors.New("script.yaml is not a mapping")
	}
	for key, value := range values {
		if err := setKey(mapping, key, value); err != nil {
			return err
		}
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, out.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// setKey sets a key's value, adding the key when it is missing. An empty value removes the key.
func setKey(mapping *yaml.Node, key string, value any) error {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return err
	}
	empty := value == "" || (node.Kind == yaml.SequenceNode && len(node.Content) == 0)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}
		if empty {
			mapping.Content = slices.Delete(mapping.Content, i, i+2)
			return nil
		}
		comments := mapping.Content[i+1]
		node.HeadComment, node.LineComment, node.FootComment = comments.HeadComment, comments.LineComment, comments.FootComment
		mapping.Content[i+1] = &node
		return nil
	}
	if !empty {
		mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, &node)
	}
	return nil
}
