package okf

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseStandardRoundTrip(t *testing.T) {
	t.Parallel()
	src := []byte("---\ntype: concept\ntitle: Gemma\n---\n\nBody content here.\n")
	fm, body, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Type != "concept" {
		t.Fatalf("type = %q, want concept", fm.Type)
	}
	if fm.Title != "Gemma" {
		t.Fatalf("title = %q, want Gemma", fm.Title)
	}
	wantBody := "Body content here.\n"
	if string(body) != wantBody {
		t.Fatalf("body = %q, want %q", string(body), wantBody)
	}

	// Serialize back and confirm the structure is preserved.
	out, err := Serialize(fm, body)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	fm2, body2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if fm2.Type != fm.Type || fm2.Title != fm.Title {
		t.Fatalf("frontmatter changed after round-trip: %+v vs %+v", fm2, fm)
	}
	if string(body2) != wantBody {
		t.Fatalf("body changed after round-trip: %q vs %q", string(body2), wantBody)
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	t.Parallel()
	src := []byte("Just markdown, no frontmatter.\n\nSecond line.\n")
	fm, body, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Type != "" {
		t.Fatalf("expected empty frontmatter, got type=%q", fm.Type)
	}
	if !bytes.Equal(body, src) {
		t.Fatalf("body should equal input when no frontmatter, got %q", string(body))
	}
}

func TestParseMultilineBody(t *testing.T) {
	t.Parallel()
	src := []byte("---\ntype: doc\n---\nLine 1\nLine 2\n\nLine 4 after blank\n")
	fm, body, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Type != "doc" {
		t.Fatalf("type = %q", fm.Type)
	}
	want := "Line 1\nLine 2\n\nLine 4 after blank\n"
	if string(body) != want {
		t.Fatalf("body = %q, want %q", string(body), want)
	}
}

func TestParseCJKContent(t *testing.T) {
	t.Parallel()
	src := []byte("---\ntype: 概念\ntitle: 智能体云盘\n---\n\n正文：这是一段中文内容。\n")
	fm, body, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Type != "概念" {
		t.Fatalf("type = %q, want 概念", fm.Type)
	}
	if fm.Title != "智能体云盘" {
		t.Fatalf("title = %q", fm.Title)
	}
	wantBody := "正文：这是一段中文内容。\n"
	if string(body) != wantBody {
		t.Fatalf("body = %q, want %q", string(body), wantBody)
	}
}

func TestParseUnknownKeysPreserved(t *testing.T) {
	t.Parallel()
	src := []byte("---\ntype: concept\ncustom.author: Google\ncustom.score: 9\n---\n\nBody\n")
	fm, body, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Extra["custom.author"] != "Google" {
		t.Fatalf("custom.author = %#v", fm.Extra["custom.author"])
	}
	if fm.Extra["custom.score"] != 9 {
		t.Fatalf("custom.score = %#v", fm.Extra["custom.score"])
	}

	// Re-serialize and confirm the unknown keys come back.
	out, err := Serialize(fm, body)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), "custom.author: Google") {
		t.Fatalf("unknown key lost on serialize:\n%s", string(out))
	}
	if !strings.Contains(string(out), "custom.score: 9") {
		t.Fatalf("unknown key lost on serialize:\n%s", string(out))
	}
}

func TestParseEmptyBody(t *testing.T) {
	t.Parallel()
	src := []byte("---\ntype: concept\n---\n")
	fm, body, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Type != "concept" {
		t.Fatalf("type = %q", fm.Type)
	}
	if len(body) != 0 {
		t.Fatalf("body should be empty, got %q", string(body))
	}
}

func TestParseEmptyFrontmatter(t *testing.T) {
	t.Parallel()
	// Two fences with nothing between them is a legal but empty frontmatter.
	src := []byte("---\n---\n\nbody\n")
	fm, body, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Type != "" {
		t.Fatalf("type should be empty, got %q", fm.Type)
	}
	if string(body) != "body\n" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestParseUnclosedFrontmatter(t *testing.T) {
	t.Parallel()
	src := []byte("---\ntype: concept\ntitle: no close\n")
	_, _, err := Parse(src)
	if err == nil {
		t.Fatalf("expected error for unclosed frontmatter")
	}
	if !strings.Contains(err.Error(), "not closed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseInvalidYAML(t *testing.T) {
	t.Parallel()
	// A malformed mapping (tab/colon issues) should surface a YAML error.
	src := []byte("---\n: no key here\n  bad indent\n---\n\nbody\n")
	_, _, err := Parse(src)
	if err == nil {
		t.Fatalf("expected YAML parse error")
	}
}

func TestParseLeadingDashesNotFence(t *testing.T) {
	t.Parallel()
	// A first line of "---foo" is not a fence, so the whole thing is body.
	src := []byte("---foo\ntype: not-fm\n")
	fm, body, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm.Type != "" {
		t.Fatalf("should not parse as frontmatter, got type=%q", fm.Type)
	}
	if !bytes.Equal(body, src) {
		t.Fatalf("body should be entire input")
	}
}

func TestSerializeAlwaysEmitsFence(t *testing.T) {
	t.Parallel()
	out, err := Serialize(Frontmatter{}, nil)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.HasPrefix(string(out), "---\n") {
		t.Fatalf("output should start with fence, got %q", string(out))
	}
	// Must contain exactly two fence lines.
	if c := strings.Count(string(out), "\n---\n"); c != 1 {
		t.Fatalf("expected one closing fence, got %d in %q", c, string(out))
	}
}

func TestSerializeEmptyFrontmatterAndBody(t *testing.T) {
	t.Parallel()
	out, err := Serialize(Frontmatter{}, []byte{})
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	// Empty body should produce just the fenced (empty) frontmatter and a
	// trailing newline.
	want := "---\ntype: \"\"\n---\n"
	if string(out) != want {
		t.Fatalf("output = %q, want %q", string(out), want)
	}
}

func TestSerializeBodyWithoutTrailingNewline(t *testing.T) {
	t.Parallel()
	out, err := Serialize(Frontmatter{Type: "doc"}, []byte("no newline"))
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.HasSuffix(string(out), "no newline\n") {
		t.Fatalf("output should end with body + newline, got %q", string(out))
	}
}
