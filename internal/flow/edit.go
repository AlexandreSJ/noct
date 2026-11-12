package flow

import (
	"fmt"
	"slices"
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
	if cut < 0 {
		return "", "", -1 // a parent with a branch but no step: no step is there
	}
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
	return s.branch(branch)
}

func (f *Flow) step(path string) *Step {
	parent, branch, index := SplitPath(path)
	list := f.list(parent, branch)
	if list == nil || index < 0 || index >= len(*list) {
		return nil
	}
	return &(*list)[index]
}

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
			for _, b := range s.Branches() {
				collect(b.Steps)
			}
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

// Shift moves the step at path one place down (or up) the flow as a line moves through code: past
// a step next to it, into a block next to it (an if, a switch or a repeat), from one of a block's
// lists to the next (an if's yes to its no, a case to the next), and out of the block at its end.
// It returns the step's new path; at an end of the flow it stays.
func (f *Flow) Shift(path string, down bool) string {
	parent, branch, index := SplitPath(path)
	list := f.list(parent, branch)
	if list == nil || index < 0 || index >= len(*list) {
		return path
	}
	step := 1
	if !down {
		step = -1
	}
	if next := index + step; next >= 0 && next < len(*list) {
		branches := (*list)[next].Branches()
		if len(branches) == 0 {
			return f.Move(path, step)
		}
		into, at := Path(parent, branch, next), 0
		intoBranch := branches[0].Name
		if !down {
			last := branches[len(branches)-1]
			intoBranch, at = last.Name, len(last.Steps)
		}
		to, _ := f.MoveTo(path, into, intoBranch, at)
		return to
	}
	if parent == "" {
		return path
	}
	outer, outerBranch, at := SplitPath(parent)
	block, _ := f.Step(parent)
	branches := block.Branches()
	k := slices.IndexFunc(branches, func(b Branch) bool { return b.Name == branch })
	switch {
	case down && k >= 0 && k+1 < len(branches):
		outer, outerBranch, at = parent, branches[k+1].Name, 0
	case !down && k > 0:
		outer, outerBranch, at = parent, branches[k-1].Name, len(branches[k-1].Steps)
	case down:
		at++
	}
	to, _ := f.MoveTo(path, outer, outerBranch, at)
	return to
}

// MoveTo takes the step at path out and puts it at index of parent's branch, both as the flow is
// before the step leaves its place, and returns its new path. A block cannot go inside itself.
func (f *Flow) MoveTo(path, parent, branch string, index int) (string, error) {
	if parent == path || strings.HasPrefix(parent, path+"/") {
		return path, fmt.Errorf("a step cannot go inside itself")
	}
	s, ok := f.Step(path)
	if !ok || f.list(parent, branch) == nil {
		return path, fmt.Errorf("no step at %s, or no place at %s %s", path, parent, branch)
	}
	from, fromBranch, fromIndex := SplitPath(path)
	if err := f.Remove(path); err != nil {
		return path, err
	}
	if from == parent && fromBranch == branch && fromIndex < index {
		index-- // the list closed up where the step was
	}
	parent = shiftedPath(parent, path)
	return f.Insert(parent, branch, index, s)
}

// shiftedPath is path once the step at removed left its list: a later sibling of removed, or a
// step inside one, has an index one less.
func shiftedPath(path, removed string) string {
	parent, branch, index := SplitPath(removed)
	prefix := Path(parent, branch, 0)
	prefix = prefix[:len(prefix)-1] // the list's path, up to the index
	rest, ok := strings.CutPrefix(path, prefix)
	if !ok {
		return path
	}
	digits, tail, _ := strings.Cut(rest, "/")
	n, err := strconv.Atoi(digits)
	if err != nil || n <= index {
		return path
	}
	if tail != "" {
		tail = "/" + tail
	}
	return prefix + strconv.Itoa(n-1) + tail
}
