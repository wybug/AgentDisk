package okf

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// kv is a single key/value entry of an orderedMap. It exists so we can emit
// known OKF fields first and then unknown keys in a deterministic order while
// still feeding the whole thing to yaml.v3 as a single mapping.
type kv struct {
	Key   string
	Value any
}

// orderedMap is a stable, slice-backed map used by Frontmatter.MarshalYAML.
// yaml.v3 marshals a yaml.Marshaler's return value, and a slice of kv structs
// with the MarshalYAML method below is rendered as a YAML mapping that
// preserves insertion order.
type orderedMap []kv

// Sort orders entries by key so that unknown-key output is deterministic across
// runs. Known-key order is fixed by MarshalYAML before this is called.
func (m orderedMap) Sort() {
	sort.Slice(m, func(i, j int) bool {
		// Known fields keep their place; this is only reached for the Extra
		// tail, so a plain alphabetical comparison is sufficient.
		return m[i].Key < m[j].Key
	})
}

// MarshalYAML renders the orderedMap as a YAML mapping node, preserving the
// slice's order instead of the random order yaml.v3 would apply to a Go map.
//
// yaml.v3 signals encode failures (e.g. unsupported types like chan) by
// panicking out of Node.Encode rather than returning an error. We recover here
// and surface it as a normal error so callers' yaml.Marshal never observes a
// raw panic, and so the error branch is testable.
func (m orderedMap) MarshalYAML() (any, error) {
	node := &yaml.Node{
		Kind:    yaml.MappingNode,
		Content: make([]*yaml.Node, 0, len(m)*2),
	}
	for _, e := range m {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: e.Key,
			Tag:   "!!str",
		}
		valNode, err := encodeValue(e.Value)
		if err != nil {
			return nil, err
		}
		node.Content = append(node.Content, keyNode, valNode)
	}
	return node, nil
}

// encodeValue wraps Node.Encode with a panic-safe recover so unsupported
// runtime types become ordinary errors instead of crashing the marshaller.
// yaml.v3 signals encode failures by panicking rather than returning an error,
// so the return value of Encode is intentionally ignored — failures surface
// through the deferred recover below.
func encodeValue(v any) (node *yaml.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("okf: cannot encode value of type %T: %v", v, r)
			node = nil
		}
	}()
	n := &yaml.Node{}
	_ = n.Encode(v)
	return n, nil
}
