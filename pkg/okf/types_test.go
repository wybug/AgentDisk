package okf

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFrontmatterMarshalRoundTrip(t *testing.T) {
	t.Parallel()
	original := Frontmatter{
		Type:        "concept",
		Title:       "Gemma",
		Description: "Open LLM",
		Resource:    "gemma.md",
		Tags:        []string{"llm", "google"},
		Timestamp:   "2026-06-27T00:00:00Z",
	}
	out, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Frontmatter
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Tags round-trip as a non-nil slice; compare by value semantics rather
	// than nil-ness to keep the assertion readable.
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", got, original)
	}
}

func TestFrontmatterExtraPreserved(t *testing.T) {
	t.Parallel()
	src := []byte(`type: concept
title: Gemma
custom.author: Google
custom.score: 7
extra.nested:
  a: 1
  b: 2
`)
	var fm Frontmatter
	if err := yaml.Unmarshal(src, &fm); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fm.Type != "concept" {
		t.Fatalf("type = %q, want concept", fm.Type)
	}
	if fm.Title != "Gemma" {
		t.Fatalf("title = %q, want Gemma", fm.Title)
	}
	wantExtra := map[string]any{
		"custom.author": "Google",
		"custom.score":  7,
		"extra.nested": map[string]any{
			"a": 1,
			"b": 2,
		},
	}
	if !reflect.DeepEqual(fm.Extra, wantExtra) {
		t.Fatalf("extra mismatch:\n got  %+v\n want %+v", fm.Extra, wantExtra)
	}

	// Serialize and re-parse to confirm unknown keys survive the round trip.
	out, err := yaml.Marshal(fm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fm2 Frontmatter
	if err := yaml.Unmarshal(out, &fm2); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if fm2.Type != fm.Type || fm2.Title != fm.Title {
		t.Fatalf("known fields lost after round-trip: %+v", fm2)
	}
	if !reflect.DeepEqual(fm2.Extra, wantExtra) {
		t.Fatalf("extra lost after round-trip:\n got  %+v\n want %+v", fm2.Extra, wantExtra)
	}
}

func TestFrontmatterEmptyMarshal(t *testing.T) {
	t.Parallel()
	out, err := yaml.Marshal(Frontmatter{})
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}
	// An empty Frontmatter still emits type (the required key) but nothing
	// else; we only assert that no extra keys leaked out.
	var fm Frontmatter
	if err := yaml.Unmarshal(out, &fm); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fm.Type != "" {
		t.Fatalf("type should be empty, got %q", fm.Type)
	}
	if len(fm.Extra) != 0 {
		t.Fatalf("no extra expected, got %+v", fm.Extra)
	}
}

func TestFrontmatterUnmarshalIgnoresUnknownYAMLTypes(t *testing.T) {
	t.Parallel()
	// yaml.v3 supports !!seq and !!map nodes for arbitrary keys; ensure
	// scalars, sequences, and mappings all land in Extra unmodified.
	src := []byte(`type: doc
aliases:
  - foo
  - bar
weights:
  x: 1
  y: 2
inline_scalar: hello
`)
	var fm Frontmatter
	if err := yaml.Unmarshal(src, &fm); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := fm.Extra["aliases"]; !reflect.DeepEqual(got, []any{"foo", "bar"}) {
		t.Fatalf("aliases = %#v, want [foo bar]", got)
	}
	if got := fm.Extra["weights"]; !reflect.DeepEqual(got, map[string]any{"x": 1, "y": 2}) {
		t.Fatalf("weights = %#v, want map[x:1 y:2]", got)
	}
	if got := fm.Extra["inline_scalar"]; got != "hello" {
		t.Fatalf("inline_scalar = %#v, want hello", got)
	}
}

func TestFrontmatterUnmarshalTypeTypeError(t *testing.T) {
	t.Parallel()
	// A mapping where a known string field receives a sequence triggers a
	// Decode error inside UnmarshalYAML; we surface it as-is.
	src := []byte("type:\n  - not-a-string\n")
	var fm Frontmatter
	if err := yaml.Unmarshal(src, &fm); err == nil {
		t.Fatalf("expected decode error for sequence-typed string field, got nil with fm=%+v", fm)
	}
}

func TestFrontmatterUnmarshalExtraValueDecodeError(t *testing.T) {
	t.Parallel()
	// Manually build a mapping node whose unknown-key value is a !!binary node
	// carrying invalid base64. valNode.Decode(&any) fails inside the Extra
	// walk, and UnmarshalYAML must surface that error rather than swallow it.
	// (Constructing the node directly is the only way to reach this branch,
	// because a top-level yaml.Unmarshal rejects the bad binary earlier.)
	root := &yaml.Node{Kind: yaml.MappingNode}
	root.Content = []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "type", Tag: "!!str"},
		{Kind: yaml.ScalarNode, Value: "doc", Tag: "!!str"},
		{Kind: yaml.ScalarNode, Value: "custom", Tag: "!!str"},
		{Kind: yaml.ScalarNode, Value: "not-base64!!!", Tag: "!!binary"},
	}
	var fm Frontmatter
	if err := fm.UnmarshalYAML(root); err == nil {
		t.Fatalf("expected decode error for invalid binary Extra value, got nil")
	}
}

func TestFrontmatterUnmarshalNonMappingNode(t *testing.T) {
	t.Parallel()
	// A non-mapping node at the top level is tolerated per OKF §9: no known
	// field can be populated, so the result is a zero Frontmatter and no error.
	// This keeps a lenient reader from rejecting an unusual but harmless file.
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(`just a string`), &fm); err != nil {
		t.Fatalf("expected nil error for scalar frontmatter, got %v", err)
	}
	if fm.Type != "" || fm.Extra != nil {
		t.Fatalf("expected zero Frontmatter, got %+v", fm)
	}
}
