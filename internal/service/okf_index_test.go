package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestGenerateIndexMarkdown_GroupsByType seeds a bundle with multiple node
// types and asserts the rendered markdown groups them under "## Type:"
// headings, in alphabetical order, with bundle-relative links.
func TestGenerateIndexMarkdown_GroupsByType(t *testing.T) {
	_, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", "My KB"))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	for _, c := range []struct {
		rel, typ, title string
	}{
		{"concepts/gemma.md", "concept", "Gemma"},
		{"concepts/transformer.md", "concept", "Transformer"},
		{"guides/quickstart.md", "guide", "Quickstart"},
	} {
		body := []byte("---\ntype: " + c.typ + "\ntitle: " + c.title + "\n---\nbody")
		if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
			PublicDirectoryID: 7, RelPath: c.rel, Content: body,
		}); err != nil {
			t.Fatalf("WriteMarkdown %s: %v", c.rel, err)
		}
	}

	body, err := svc.GenerateIndexMarkdown(context.Background(), 1)
	if err != nil {
		t.Fatalf("GenerateIndexMarkdown: %v", err)
	}
	s := string(body)
	// No frontmatter for the generated index (the bundle's actual index.md
	// has its own frontmatter, but the rendered output the auto-index path
	// writes is body-only).
	if strings.HasPrefix(s, "---") {
		t.Errorf("index markdown starts with frontmatter, want none")
	}
	if !strings.Contains(s, "# My KB") {
		t.Errorf("missing title heading: %q", s)
	}
	if !strings.Contains(s, "## Type: concept (2 nodes)") {
		t.Errorf("missing concept heading: %q", s)
	}
	if !strings.Contains(s, "## Type: guide (1 nodes)") {
		t.Errorf("missing guide heading: %q", s)
	}
	if !strings.Contains(s, "[Gemma](/bundle/concepts/gemma.md)") {
		t.Errorf("missing Gemma link: %q", s)
	}
	// Concept group must be sorted alphabetically by title: Gemma before Transformer.
	gemmaIdx := strings.Index(s, "Gemma")
	transformerIdx := strings.Index(s, "Transformer")
	if gemmaIdx < 0 || transformerIdx < 0 || gemmaIdx > transformerIdx {
		t.Errorf("concepts not alphabetical: gemma=%d transformer=%d", gemmaIdx, transformerIdx)
	}
}

// TestGenerateIndexMarkdown_EmptyBundle verifies the renderer emits a
// non-empty placeholder body when the bundle has zero typed nodes.
func TestGenerateIndexMarkdown_EmptyBundle(t *testing.T) {
	_, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", "Empty"))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	body, err := svc.GenerateIndexMarkdown(context.Background(), 1)
	if err != nil {
		t.Fatalf("GenerateIndexMarkdown: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "0 nodes, 0 types") {
		t.Errorf("missing empty header: %q", s)
	}
	if !strings.Contains(s, "_No nodes yet._") {
		t.Errorf("missing placeholder: %q", s)
	}
}

// TestGenerateIndexMarkdown_CJKTitle verifies the title is rendered verbatim
// even when it contains multibyte characters.
func TestGenerateIndexMarkdown_CJKTitle(t *testing.T) {
	_, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", "知识库"))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	body, err := svc.GenerateIndexMarkdown(context.Background(), 1)
	if err != nil {
		t.Fatalf("GenerateIndexMarkdown: %v", err)
	}
	if !strings.Contains(string(body), "# 知识库") {
		t.Errorf("missing CJK title: %q", string(body))
	}
}

// TestGenerateIndexMarkdown_DescriptionTrailer verifies a node's description
// is appended after an em-dash on the index bullet, collapsed to a single
// line.
func TestGenerateIndexMarkdown_DescriptionTrailer(t *testing.T) {
	_, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", "Desc"))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	body := []byte("---\ntype: concept\ntitle: WithDesc\ndescription: |\n  First line\n  Second line\n---\nbody")
	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "concepts/x.md", Content: body,
	}); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out, err := svc.GenerateIndexMarkdown(context.Background(), 1)
	if err != nil {
		t.Fatalf("GenerateIndexMarkdown: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "First line Second line") {
		t.Errorf("missing collapsed description: %q", s)
	}
}

// TestGenerateIndexMarkdown_NotFound covers the missing-bundle path.
func TestGenerateIndexMarkdown_NotFound(t *testing.T) {
	_, _, _, svc := okfTestHarness(t)
	_, err := svc.GenerateIndexMarkdown(context.Background(), 999)
	if !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}

// TestAppendLogEntry_CreatesAndAppends writes two entries against a fresh
// bundle and verifies both rows land in log.md, in order, tab-separated.
func TestAppendLogEntry_CreatesAndAppends(t *testing.T) {
	_, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	if err := svc.AppendLogEntry(context.Background(), bundle.ID, LogActionCreate, "a.md", "u1"); err != nil {
		t.Fatalf("AppendLogEntry create: %v", err)
	}
	if err := svc.AppendLogEntry(context.Background(), bundle.ID, LogActionUpdate, "a.md", "u1"); err != nil {
		t.Fatalf("AppendLogEntry update: %v", err)
	}

	file, err := pub.FindFileByRelPath(7, "log.md")
	if err != nil {
		t.Fatalf("log.md not found: %v", err)
	}
	body, _ := pub.ReadFileContent(context.Background(), file.ID)
	s := string(body)
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	// RegisterBundle itself appends a "register" row, so the first line is
	// the register entry; our explicit create / update rows follow.
	if len(lines) != 3 {
		t.Fatalf("log.md lines = %d, want 3 (register + create + update)", len(lines))
	}
	registerFields := strings.Split(lines[0], "\t")
	if registerFields[1] != LogActionRegister {
		t.Errorf("register action = %q, want %q", registerFields[1], LogActionRegister)
	}
	// Second row: create.
	createFields := strings.Split(lines[1], "\t")
	if len(createFields) != 4 {
		t.Fatalf("create row fields = %d, want 4", len(createFields))
	}
	if createFields[1] != LogActionCreate {
		t.Errorf("create action = %q, want %q", createFields[1], LogActionCreate)
	}
	if createFields[2] != "a.md" {
		t.Errorf("create relPath = %q, want a.md", createFields[2])
	}
	if createFields[3] != "u1" {
		t.Errorf("create userID = %q, want u1", createFields[3])
	}
	// Third row: update.
	updateFields := strings.Split(lines[2], "\t")
	if updateFields[1] != LogActionUpdate {
		t.Errorf("update action = %q, want %q", updateFields[1], LogActionUpdate)
	}
}

// TestAppendLogEntry_BundleNotFound verifies the helper short-circuits when
// the bundle is gone.
func TestAppendLogEntry_BundleNotFound(t *testing.T) {
	_, _, _, svc := okfTestHarness(t)
	err := svc.AppendLogEntry(context.Background(), 999, LogActionCreate, "x.md", "u1")
	if !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}

// TestAppendLogEntry_NoFrontmatter makes sure the auto-generated log.md body
// does not start with a YAML frontmatter delimiter — OKF §reserved names
// forbids it.
func TestAppendLogEntry_NoFrontmatter(t *testing.T) {
	_, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	bundle, _ := svc.RegisterBundle(context.Background(), 7)
	if err := svc.AppendLogEntry(context.Background(), bundle.ID, LogActionCreate, "a.md", "u1"); err != nil {
		t.Fatalf("AppendLogEntry: %v", err)
	}
	file, _ := pub.FindFileByRelPath(7, "log.md")
	body, _ := pub.ReadFileContent(context.Background(), file.ID)
	if strings.HasPrefix(string(body), "---") {
		t.Errorf("log.md starts with frontmatter: %q", string(body))
	}
}

// TestRegenerateIndex_ReturnsVersion verifies RegenerateIndex writes the
// bundle root index.md and returns its file version + timestamp.
func TestRegenerateIndex_ReturnsVersion(t *testing.T) {
	_, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", "Original"))
	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	version, at, err := svc.RegenerateIndex(context.Background(), bundle.ID)
	if err != nil {
		t.Fatalf("RegenerateIndex: %v", err)
	}
	if version < 1 {
		t.Errorf("version = %d, want >= 1", version)
	}
	if at.IsZero() {
		t.Errorf("regeneratedAt is zero")
	}

	// The auto-generated index replaces the user-authored one. Verify the new
	// body has the generated header rather than the original frontmatter.
	file, _ := pub.FindFileByRelPath(7, "index.md")
	body, _ := pub.ReadFileContent(context.Background(), file.ID)
	if !strings.Contains(string(body), "> Generated at") {
		t.Errorf("index.md not regenerated: %q", string(body))
	}
}

// TestRegenerateIndex_NotFound covers the missing-bundle path.
func TestRegenerateIndex_NotFound(t *testing.T) {
	_, _, _, svc := okfTestHarness(t)
	_, _, err := svc.RegenerateIndex(context.Background(), 999)
	if !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}
