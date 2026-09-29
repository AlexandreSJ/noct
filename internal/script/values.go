package script

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Asked tells whether the user fills the parameter in, rather than the environment.
func (p Param) Asked() bool {
	return p.From == ""
}

func (p Param) envName() string {
	return strings.TrimPrefix(p.From, "env:")
}

// StartingValues is what each parameter holds before the user changes anything: the environment
// for env parameters, then the last value used when the parameter is remembered, then its default.
func (s Script) StartingValues(remembered map[string]string, getenv func(string) string) map[string]string {
	values := make(map[string]string, len(s.Params))
	for _, p := range s.Params {
		switch last, ok := remembered[p.Name]; {
		case !p.Asked():
			values[p.Name] = getenv(p.envName())
		case p.Remember && !p.Secret && ok:
			values[p.Name] = last
		case p.Default == "" && len(p.Choices) > 0:
			values[p.Name] = p.Choices[0]
		default:
			values[p.Name] = p.Default
		}
	}
	return values
}

// CheckValues reports values the script cannot run with.
func (s Script) CheckValues(values map[string]string) error {
	for _, p := range s.Params {
		value := values[p.Name]
		if !p.Asked() && value == "" {
			return fmt.Errorf("%s is not set in the environment (parameter %s)", p.envName(), p.Name)
		}
		if len(p.Choices) > 0 && !containsString(p.Choices, value) {
			return fmt.Errorf("%s must be one of %s", p.Name, strings.Join(p.Choices, ", "))
		}
	}
	return nil
}

// MemoryFile is the name of the remembered values' file in noct's state directory.
const MemoryFile = "params.json"

// Memory keeps the last values used for remembered parameters, in one JSON file shared by the
// TUI and the CLI. Secrets are never written.
type Memory struct {
	path   string
	values map[string]map[string]string
}

// LoadMemory reads the file at path; a missing file is an empty memory.
func LoadMemory(path string) (*Memory, error) {
	m := &Memory{path: path, values: map[string]map[string]string{}}
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, fmt.Errorf("reading remembered values: %w", err)
	}
	if err := json.Unmarshal(content, &m.values); err != nil {
		return m, fmt.Errorf("reading remembered values %s: %w", path, err)
	}
	return m, nil
}

// Values are the remembered values of the script.
func (m *Memory) Values(s Script) map[string]string {
	return m.values[s.ID()]
}

// FlowValues are the values last given to a flow's parameters.
func (m *Memory) FlowValues(name string) map[string]string {
	return m.values[flowKey+name]
}

// RememberFlow keeps the values given to a flow's parameters and saves the file.
func (m *Memory) RememberFlow(name string, values map[string]string) error {
	return m.keep(flowKey+name, values)
}

// flowKey starts a flow's key in the file, apart from the scripts' IDs.
const flowKey = "flow:"

// Remember keeps the values of the script's remembered parameters and saves the file. Values
// saved meanwhile by another noct are kept.
func (m *Memory) Remember(s Script, values map[string]string) error {
	kept := map[string]string{}
	for _, p := range s.Params {
		if p.Remember && !p.Secret && p.Asked() {
			kept[p.Name] = values[p.Name]
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return m.keep(s.ID(), kept)
}

// keep saves values under key, keeping values saved meanwhile by another noct.
func (m *Memory) keep(key string, values map[string]string) error {
	if latest, err := LoadMemory(m.path); err == nil {
		m.values = latest.values
	}
	m.values[key] = values
	return m.save()
}

// Rename moves a script's remembered values to its new ID.
func (m *Memory) Rename(oldName, newName string) error {
	if latest, err := LoadMemory(m.path); err == nil {
		m.values = latest.values
	}
	values, ok := m.values[oldName]
	if !ok {
		return nil
	}
	delete(m.values, oldName)
	m.values[newName] = values
	return m.save()
}

func (m *Memory) save() error {
	content, err := json.MarshalIndent(m.values, "", "  ")
	if err != nil {
		return fmt.Errorf("saving remembered values: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("saving remembered values: %w", err)
	}
	temp := m.path + ".tmp"
	if err := os.WriteFile(temp, content, 0o600); err != nil {
		return fmt.Errorf("saving remembered values: %w", err)
	}
	if err := os.Rename(temp, m.path); err != nil {
		return fmt.Errorf("saving remembered values: %w", err)
	}
	return nil
}
