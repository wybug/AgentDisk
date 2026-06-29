// Package okf implements Open Knowledge Format (OKF) v0.1 utilities.
//
// OKF is a draft specification by Google for representing knowledge artifacts as
// markdown files with YAML frontmatter. The only required field is "type". All
// other fields are optional and unknown keys are preserved per the OKF §9
// tolerance rules, enabling forward/backward compatibility.
//
// Reference: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
package okf

import (
	"gopkg.in/yaml.v3"
)

// Frontmatter represents the YAML frontmatter of an OKF file.
//
// Type is the only required field per the OKF specification. The remaining
// fields are common recommended keys. Extra preserves all unknown keys so that
// round-trip serialization is lossless; it is handled by the custom
// MarshalYAML/UnmarshalYAML methods and is not exported as a YAML field itself.
type Frontmatter struct {
	Type        string         `yaml:"type"`
	Title       string         `yaml:"title,omitempty"`
	Description string         `yaml:"description,omitempty"`
	Resource    string         `yaml:"resource,omitempty"`
	Tags        []string       `yaml:"tags,omitempty"`
	Timestamp   string         `yaml:"timestamp,omitempty"`   // ISO 8601
	OkfVersion  string         `yaml:"okf_version,omitempty"` // allowed only on root index.md
	Extra       map[string]any `yaml:"-"`                     // inline; handled by MarshalYAML/UnmarshalYAML
}

// knownFrontmatterKeys is the set of fields declared on Frontmatter so the
// custom (un)marshalers can route everything else into Extra.
var knownFrontmatterKeys = map[string]struct{}{
	"type":        {},
	"title":       {},
	"description": {},
	"resource":    {},
	"tags":        {},
	"timestamp":   {},
	"okf_version": {},
}

// MarshalYAML implements yaml.Marshaler. Known fields are emitted from the
// struct; any entries in Extra are emitted alongside them so unknown keys
// survive a serialize/parse round trip.
func (f Frontmatter) MarshalYAML() (any, error) {
	out := orderedMap{
		{"type", f.Type},
	}
	// Optional known fields are only emitted when non-zero to keep the output
	// stable and minimal, matching the omitempty semantics on the struct tags.
	if f.Title != "" {
		out = append(out, kv{"title", f.Title})
	}
	if f.Description != "" {
		out = append(out, kv{"description", f.Description})
	}
	if f.Resource != "" {
		out = append(out, kv{"resource", f.Resource})
	}
	if len(f.Tags) > 0 {
		out = append(out, kv{"tags", f.Tags})
	}
	if f.Timestamp != "" {
		out = append(out, kv{"timestamp", f.Timestamp})
	}
	if f.OkfVersion != "" {
		out = append(out, kv{"okf_version", f.OkfVersion})
	}
	// Unknown keys are appended after known ones, sorted for deterministic
	// output. Nested values flow through yaml.Marshal unchanged.
	for k, v := range f.Extra {
		out = append(out, kv{k, v})
	}
	out.Sort()
	return out, nil
}

// UnmarshalYAML implements yaml.Unmarshaler. Known fields populate the struct;
// any other keys are placed into Extra so they are not lost on parse.
func (f *Frontmatter) UnmarshalYAML(value *yaml.Node) error {
	// A non-mapping frontmatter (e.g. a stray scalar) cannot populate any known
	// field; treat it as an empty Frontmatter rather than erroring, so a
	// malformed-but-tolerant reader still produces a usable value. The OKF
	// §9 tolerance rule favors forward compatibility over strictness here.
	if value.Kind != yaml.MappingNode {
		return nil
	}
	// Decode into a temporary alias to avoid infinite recursion into our own
	// UnmarshalYAML. The alias shares the same field layout, so yaml.v3 fills
	// the known fields directly.
	alias := struct {
		Type        string   `yaml:"type"`
		Title       string   `yaml:"title"`
		Description string   `yaml:"description"`
		Resource    string   `yaml:"resource"`
		Tags        []string `yaml:"tags"`
		Timestamp   string   `yaml:"timestamp"`
		OkfVersion  string   `yaml:"okf_version"`
	}{}
	if err := value.Decode(&alias); err != nil {
		return err
	}
	f.Type = alias.Type
	f.Title = alias.Title
	f.Description = alias.Description
	f.Resource = alias.Resource
	f.Tags = alias.Tags
	f.Timestamp = alias.Timestamp
	f.OkfVersion = alias.OkfVersion

	// Walk the mapping node and capture any keys that are not known, decoding
	// them into generic any values for Extra. yaml.Node only exposes mappings as
	// a flat slice of [key, value, key, value, ...]. The MappingNode check above
	// guarantees value.Content is a key/value sequence here.
	for i := 0; i+1 < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		valNode := value.Content[i+1]
		if keyNode.Value == "" {
			continue
		}
		if _, known := knownFrontmatterKeys[keyNode.Value]; known {
			continue
		}
		// Lazily allocate Extra so a frontmatter with no unknown keys keeps a
		// nil Extra map (matching a freshly-constructed Frontmatter value and
		// keeping round-trip reflect.DeepEqual checks exact).
		if f.Extra == nil {
			f.Extra = make(map[string]any)
		}
		var v any
		if err := valNode.Decode(&v); err != nil {
			return err
		}
		f.Extra[keyNode.Value] = v
	}
	return nil
}
