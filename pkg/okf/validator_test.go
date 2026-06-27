package okf

import (
	"errors"
	"testing"
)

func TestValidateTypePresent(t *testing.T) {
	t.Parallel()
	fm := Frontmatter{Type: "concept"}
	if err := Validate(fm); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateTypeMissing(t *testing.T) {
	t.Parallel()
	if err := Validate(Frontmatter{}); !errors.Is(err, ErrMissingType) {
		t.Fatalf("expected ErrMissingType, got %v", err)
	}
}

func TestValidateTypeWhitespaceOnly(t *testing.T) {
	t.Parallel()
	cases := []string{"", "   ", "\t\t", "\n\n"}
	for _, tc := range cases {
		fm := Frontmatter{Type: tc}
		err := Validate(fm)
		if !errors.Is(err, ErrMissingType) {
			t.Fatalf("type=%q expected ErrMissingType, got %v", tc, err)
		}
	}
}

func TestValidateIgnoresOtherFields(t *testing.T) {
	t.Parallel()
	// Type present is sufficient; all other fields may be anything or empty.
	fm := Frontmatter{
		Type:  "doc",
		Title: "",
		Extra: map[string]any{"arbitrary": 42},
		Tags:  nil,
	}
	if err := Validate(fm); err != nil {
		t.Fatalf("expected nil with non-empty type, got %v", err)
	}
}

func TestIsReservedName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want bool
	}{
		{"index.md", true},
		{"log.md", true},
		// Case-sensitive per spec.
		{"INDEX.md", false},
		{"Index.md", false},
		{"LOG.MD", false},
		// Non-reserved names.
		{"overview.md", false},
		{"gemma.md", false},
		{"index.txt", false},
		{"", false},
		// Path-like inputs are not reserved (callers check file basenames).
		{"foo/index.md", false},
	}
	for _, tc := range cases {
		if got := IsReservedName(tc.name); got != tc.want {
			t.Fatalf("IsReservedName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
