package flow

import (
	"fmt"
	"strconv"
	"strings"
)

// SplitPath is the other way of Path: where the list holding the step is, and the step's index.
func SplitPath(path string) (parent, branch string, index int) {
	cut := strings.LastIndex(path, "/")
	if cut < 0 {
		index, _ = strconv.Atoi(path)
		return "", "", index
	}
	index, _ = strconv.Atoi(path[cut+1:])
	rest := path[:cut]
	cut = strings.LastIndex(rest, "/")
	return rest[:cut], rest[cut+1:], index
}

// list is the list of steps at parent's branch: the flow's own steps when parent is "".
func (f *Flow) list(parent, branch string) *[]Step {
	if parent == "" {
		return &f.Steps
	}
	s := f.step(parent)
	if s == nil {
		return nil
	}
	switch branch {
	case "then":
		return &s.Then
	case "else":
		return &s.Else
	case "steps":
		return &s.Steps
	}
	return nil
}

func (f *Flow) step(path string) *Step {
	parent, branch, index := SplitPath(path)
	list := f.list(parent, branch)
	if list == nil || index < 0 || index >= len(*list) {
		return nil
	}
	return &(*list)[index]
}

// Step is the step at path.
func (f *Flow) Step(path string) (Step, bool) {
	s := f.step(path)
	if s == nil {
		return Step{}, false
	}
	return *s, true
}

// Insert puts s at index of parent's branch and returns its path.
func (f *Flow) Insert(parent, branch string, index int, s Step) (string, error) {
	list := f.list(parent, branch)
	if list == nil {
		return "", fmt.Errorf("no place for a step at %s %s", parent, branch)
	}
	index = min(max(index, 0), len(*list))
	*list = append((*list)[:index], append([]Step{s}, (*list)[index:]...)...)
	return Path(parent, branch, index), nil
}

// Replace puts s in place of the step at path.
func (f *Flow) Replace(path string, s Step) error {
	target := f.step(path)
	if target == nil {
		return fmt.Errorf("no step at %s", path)
	}
	*target = s
	return nil
}

// Remove takes the step at path out, with the steps inside it.
func (f *Flow) Remove(path string) error {
	parent, branch, index := SplitPath(path)
	list := f.list(parent, branch)
	if list == nil || index < 0 || index >= len(*list) {
		return fmt.Errorf("no step at %s", path)
	}
	*list = append((*list)[:index], (*list)[index+1:]...)
	return nil
}

// Move swaps the step at path with its neighbor by step (-1 up, 1 down) in the same list, and
// returns its new path; at an end it stays.
func (f *Flow) Move(path string, step int) string {
	parent, branch, index := SplitPath(path)
	list := f.list(parent, branch)
	other := index + step
	if list == nil || index < 0 || index >= len(*list) || other < 0 || other >= len(*list) {
		return path
	}
	(*list)[index], (*list)[other] = (*list)[other], (*list)[index]
	return Path(parent, branch, other)
}

// UniqueID is base made into an id no step uses yet: base, base2, base3...
func (f *Flow) UniqueID(base string) string {
	used := map[string]bool{}
	var collect func([]Step)
	collect = func(steps []Step) {
		for _, s := range steps {
			used[s.ID] = true
			collect(s.Then)
			collect(s.Else)
			collect(s.Steps)
		}
	}
	collect(f.Steps)
	base = idFrom(base)
	id := base
	for n := 2; used[id]; n++ {
		id = base + strconv.Itoa(n)
	}
	return id
}

// idFrom makes a name into an identifier: a script's name, tests-noct/ls-path, gives ls_path.
func idFrom(name string) string {
	if cut := strings.LastIndex(name, "/"); cut >= 0 {
		name = name[cut+1:]
	}
	var id strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r >= '0' && r <= '9' && id.Len() > 0:
			id.WriteRune(r)
		default:
			id.WriteRune('_')
		}
	}
	if id.Len() == 0 {
		return "step"
	}
	return id.String()
}
