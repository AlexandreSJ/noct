package flow

import (
	"maps"
	"slices"

	"go.yaml.in/yaml/v3"
)

// A flow's params keep the order they are written in, which the run form and the inspect show and
// the inspect can change. Params stay a map for every lookup; ParamOrder holds the order.

// flowFields is Flow without its YAML methods, to decode and encode its mapping.
type flowFields Flow

func (f *Flow) UnmarshalYAML(node *yaml.Node) error {
	if err := node.Decode((*flowFields)(f)); err != nil {
		return err
	}
	f.ParamOrder = nil
	if params := mappingValue(node, "params"); params != nil && params.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(params.Content); i += 2 {
			f.ParamOrder = append(f.ParamOrder, params.Content[i].Value)
		}
	}
	return nil
}

func (f Flow) MarshalYAML() (any, error) {
	var node yaml.Node
	if err := node.Encode(flowFields(f)); err != nil {
		return nil, err
	}
	if params := mappingValue(&node, "params"); params != nil {
		params.Content = orderedPairs(params.Content, f.ParamNames())
	}
	return &node, nil
}

// ParamNames are the params' names in their order: as written, or as ParamOrder was set, with
// any param it leaves out after them by name.
func (f Flow) ParamNames() []string {
	names := make([]string, 0, len(f.Params))
	for _, name := range f.ParamOrder {
		if _, ok := f.Params[name]; ok && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(f.Params)) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// mappingValue is the value under key in a mapping node, or nil.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// orderedPairs are a mapping's key and value nodes with the keys in the order of names.
func orderedPairs(pairs []*yaml.Node, names []string) []*yaml.Node {
	ordered := make([]*yaml.Node, 0, len(pairs))
	for _, name := range names {
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i].Value == name {
				ordered = append(ordered, pairs[i], pairs[i+1])
			}
		}
	}
	return ordered
}
