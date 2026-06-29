package okf

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

// These tests exercise the unexported helpers and error paths so the package
// reaches the 100% coverage required by CLAUDE.md §4.2 for utility packages.

func TestTrimOneLeadingNewline(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want []byte
	}{
		{[]byte("\nbody"), []byte("body")},
		{[]byte("\r\nbody"), []byte("body")},
		{[]byte("body"), []byte("body")}, // no leading newline
		{[]byte(""), []byte("")},         // empty
		{[]byte("\n"), []byte("")},       // single newline
		{[]byte("\r"), []byte("\r")},     // lone CR stays
		{[]byte("\r\n"), []byte("")},     // CRLF only
		{[]byte("no change"), []byte("no change")},
	}
	for _, tc := range cases {
		got := trimOneLeadingNewline(tc.in)
		if !bytes.Equal(got, tc.want) {
			t.Fatalf("trimOneLeadingNewline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStartsWithFence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},           // too short
		{"--", false},         // too short
		{"---", true},         // bare fence, no newline
		{"---\n", true},       // standard
		{"---\r\n", true},     // Windows
		{"---foo", false},     // fence + trailing text
		{"--- ", false},       // fence + space (not a clean fence)
		{"foo\n---\n", false}, // fence not on first line
		{"abcd", false},       // wrong content
		{"日本", false},         // multibyte, no fence
	}
	for _, tc := range cases {
		got := startsWithFence([]byte(tc.in))
		if got != tc.want {
			t.Fatalf("startsWithFence(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestFindFenceLine(t *testing.T) {
	t.Parallel()
	// Found: returns offset 10 (after "type: doc\n", which is 10 bytes).
	if got := findFenceLine([]byte("type: doc\n---\nbody")); got != 10 {
		t.Fatalf("offset = %d, want 10", got)
	}
	// Found with trailing spaces/CR (still a fence).
	if got := findFenceLine([]byte("x\n---  \r\nbody")); got != 2 {
		t.Fatalf("offset = %d, want 2", got)
	}
	// Not found.
	if got := findFenceLine([]byte("no fence here\n")); got != -1 {
		t.Fatalf("offset = %d, want -1", got)
	}
	// Fence at end with no trailing newline is still detected.
	if got := findFenceLine([]byte("x\n---")); got != 2 {
		t.Fatalf("offset = %d, want 2", got)
	}
	// Empty input.
	if got := findFenceLine([]byte("")); got != -1 {
		t.Fatalf("offset = %d, want -1", got)
	}
	// No newline anywhere and no fence: hits the nl<0 break branch.
	if got := findFenceLine([]byte("nofence")); got != -1 {
		t.Fatalf("offset = %d, want -1", got)
	}
	// Final line (no trailing newline) is the fence: hits the nl<0 line branch.
	if got := findFenceLine([]byte("a\n---")); got != 2 {
		t.Fatalf("offset = %d, want 2", got)
	}
	// A line that is "---foo" is not a fence (too long).
	if got := findFenceLine([]byte("---foo\n---\n")); got != 7 {
		t.Fatalf("offset = %d, want 7", got)
	}
}

func TestDropFirstLine(t *testing.T) {
	t.Parallel()
	got := dropFirstLine([]byte("first\nsecond"))
	want := []byte("second")
	if !bytes.Equal(got, want) {
		t.Fatalf("dropFirstLine = %q, want %q", got, want)
	}
	// No newline at all -> nil.
	if got := dropFirstLine([]byte("no newline")); got != nil {
		t.Fatalf("dropFirstLine = %q, want nil", got)
	}
	// Empty input -> nil.
	if got := dropFirstLine([]byte("")); got != nil {
		t.Fatalf("dropFirstLine = %q, want nil", got)
	}
}

func TestIsSchemeLike(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    string
		want bool
	}{
		{"", false},
		{"http", true},
		{"H", true}, // single letter
		{"a1+b-c.d", true},
		{"1abc", false},    // digit first
		{"_foo", false},    // underscore first
		{"ht tp", false},   // space invalid
		{"foo/bar", false}, // slash invalid
		{"日本", false},      // non-ASCII
	}
	for _, tc := range cases {
		if got := isSchemeLike(tc.s); got != tc.want {
			t.Fatalf("isSchemeLike(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestIsExternalOrFragment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    string
		want bool
	}{
		{"", true},
		{"#anchor", true},
		{"https://x", true},
		{"mailto:x@y", true},
		{"tel:+1", true},
		{"a:b", true},       // scheme-like
		{"./rel.md", false}, // relative
		{"../parent.md", false},
		{"plain.md", false},
		{"/bundle/x", false}, // not external
		{"1:2", false},       // digit-first "scheme" not scheme-like
	}
	for _, tc := range cases {
		if got := isExternalOrFragment(tc.s); got != tc.want {
			t.Fatalf("isExternalOrFragment(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

func TestSerializeMarshalError(t *testing.T) {
	t.Parallel()
	// A channel cannot be encoded by yaml.v3, so Serialize's marshal error
	// branch is exercised. The error must be wrapped with the "okf:" prefix.
	fm := Frontmatter{
		Type:  "doc",
		Extra: map[string]any{"bad": make(chan int)},
	}
	_, err := Serialize(fm, []byte("body"))
	if err == nil {
		t.Fatalf("expected marshal error for channel value, got nil")
	}
	if !contains(err.Error(), "okf:") {
		t.Fatalf("error should be wrapped with okf: prefix, got %q", err.Error())
	}
}

func TestOrderedMapMarshalError(t *testing.T) {
	t.Parallel()
	// Drive orderedMap.MarshalYAML directly with an unencodable value to hit
	// the valNode.Encode error path.
	m := orderedMap{{Key: "bad", Value: make(chan int)}}
	_, err := m.MarshalYAML()
	if err == nil {
		t.Fatalf("expected Encode error for channel value, got nil")
	}
}

func TestFrontmatterUnmarshalExtraRouteKnownKeys(t *testing.T) {
	t.Parallel()
	// A frontmatter that mixes known keys with a node that has an empty key
	// value exercises the "keyNode.Value == '' -> skip" branch in UnmarshalYAML.
	// yaml.v3 will parse '': value as a mapping entry with an empty key.
	src := []byte("type: doc\n\"\": skipped\ncustom: yes\n")
	var fm Frontmatter
	if err := yaml.Unmarshal(src, &fm); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fm.Type != "doc" {
		t.Fatalf("type = %q", fm.Type)
	}
	if v, ok := fm.Extra["custom"]; !ok || v != "yes" {
		t.Fatalf("custom not preserved: %#v", fm.Extra)
	}
	if _, leaked := fm.Extra[""]; leaked {
		t.Fatalf("empty key should be skipped, not stored in Extra")
	}
}

func TestFrontmatterUnmarshalNonMappingValueNode(t *testing.T) {
	t.Parallel()
	// UnmarshalYAML returns early for non-mapping nodes after populating known
	// fields; ensure a scalar node doesn't populate Extra but also doesn't panic.
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "just-a-string"}
	var fm Frontmatter
	if err := fm.UnmarshalYAML(scalar); err != nil {
		t.Fatalf("unmarshal scalar: %v", err)
	}
	if fm.Extra != nil {
		t.Fatalf("Extra should be nil for scalar node, got %#v", fm.Extra)
	}
}

func TestFrontmatterMarshalEmitsAllKnownFields(t *testing.T) {
	t.Parallel()
	// Exercise every known-field branch in MarshalYAML so all the omitempty
	// emits are covered.
	fm := Frontmatter{
		Type:        "doc",
		Title:       "T",
		Description: "D",
		Resource:    "R",
		Tags:        []string{"a"},
		Timestamp:   "2026-06-27T00:00:00Z",
		OkfVersion:  "0.1",
		Extra:       map[string]any{"custom": 1},
	}
	out, err := yaml.Marshal(fm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{"type:", "title:", "description:", "resource:", "tags:", "timestamp:", "okf_version:", "custom:"} {
		if !contains(string(out), want) {
			t.Fatalf("missing %q in output:\n%s", want, string(out))
		}
	}
}

func TestResolveWalksIntoCleanPaths(t *testing.T) {
	t.Parallel()
	// "./" + nested current path.
	if got := Resolve("./a/b.md", "x/y.md"); got != "x/a/b.md" {
		t.Fatalf("got %q", got)
	}
	// currentPath that is "." (bundle root, no dir).
	if got := Resolve("sibling.md", "."); got != "sibling.md" {
		t.Fatalf("got %q", got)
	}
}

// contains is a tiny helper so the test file does not depend on strings (which
// would otherwise be the only import). Avoids an unused-import lint if a case
// above ever changes.
func contains(haystack, needle string) bool {
	return bytes.Contains([]byte(haystack), []byte(needle))
}
