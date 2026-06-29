package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// TestExtractLinks_ClassifiesBundleRelative verifies that ./relative and
// /bundle/ absolute paths are tagged as bundle links, while http(s)/mailto and
// #anchor are isolated into their own buckets. CJK link text is exercised so
// the regex's UTF-8 byte semantics are covered.
func TestExtractLinks_ClassifiesBundleRelative(t *testing.T) {
	body := []byte("# Title\n" +
		"see [Gemma](./gemma.md) and [Glossary](/bundle/glossary.md)\n" +
		"external [Google](https://google.com) and mail [me](mailto:a@b.com)\n" +
		"jump to [intro](#intro) and an image ![pic](./pic.png)\n" +
		"CJK [腾讯](./tencent.md)\n")
	links := ExtractLinks(body, "concepts/overview.md")

	// Build a map keyed by link text. The test's expected-kinds table is in
	// terms of (text → kind), since DstRelPath is resolved for bundle links.
	got := map[string]string{}
	for _, l := range links {
		got[l.LinkText] = l.LinkKind
	}
	wantKinds := map[string]string{
		"Gemma":    LinkKindBundle,
		"Glossary": LinkKindBundle,
		"Google":   LinkKindExternal,
		"me":       LinkKindExternal,
		"intro":    LinkKindAnchor,
		"pic":      LinkKindBundle,
		"腾讯":       LinkKindBundle,
	}
	for text, kind := range wantKinds {
		if got[text] != kind {
			t.Errorf("kind for %q = %q, want %q", text, got[text], kind)
		}
	}
}

// TestExtractLinks_LineNumbers checks that SrcLine tracks the source line for
// each match. The first link sits on line 2; the CJK link sits on line 5.
func TestExtractLinks_LineNumbers(t *testing.T) {
	body := []byte("line1\n[a](./a.md)\nline3\n[b](./b.md)\n")
	links := ExtractLinks(body, "x.md")
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2", len(links))
	}
	if links[0].SrcLine != 2 {
		t.Errorf("link[0].SrcLine = %d, want 2", links[0].SrcLine)
	}
	if links[1].SrcLine != 4 {
		t.Errorf("link[1].SrcLine = %d, want 4", links[1].SrcLine)
	}
}

// TestExtractLinks_RelativeResolution walks the okf.Resolve path so "../x" and
// bare "y.md" resolve against the source document's folder.
func TestExtractLinks_RelativeResolution(t *testing.T) {
	body := []byte("[up](../parent.md) [sibling](sibling.md) [self](./self.md)")
	links := ExtractLinks(body, "concepts/gemma.md")
	if len(links) != 3 {
		t.Fatalf("len = %d, want 3", len(links))
	}
	want := map[string]string{
		"up":      "parent.md",
		"sibling": "concepts/sibling.md",
		"self":    "concepts/self.md",
	}
	for _, l := range links {
		if want[l.LinkText] != l.DstRelPath {
			t.Errorf("%q resolved to %q, want %q", l.LinkText, l.DstRelPath, want[l.LinkText])
		}
	}
}

// TestExtractLinks_EmailNotALink guards against a bare email being mistaken
// for a relative path because of the "@" character.
func TestExtractLinks_EmailNotALink(t *testing.T) {
	body := []byte("reach [team](team@example.com)")
	links := ExtractLinks(body, "x.md")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if links[0].LinkKind != LinkKindExternal {
		t.Errorf("kind = %q, want external", links[0].LinkKind)
	}
}

// TestExtractLinks_WindowsBackslashes verifies that backslashes are normalized
// to forward slashes during resolution, since OKF mandates "/" on the wire.
func TestExtractLinks_WindowsBackslashes(t *testing.T) {
	body := []byte(`[x](..\topic.md)`)
	links := ExtractLinks(body, "concepts/gemma.md")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if links[0].DstRelPath != "topic.md" {
		t.Errorf("DstRelPath = %q, want topic.md", links[0].DstRelPath)
	}
}

// TestExtractLinks_FragmentStripped confirms that a "#section" suffix on a
// bundle link does not leak into the resolved path used for the existence
// check.
func TestExtractLinks_FragmentStripped(t *testing.T) {
	body := []byte("[x](./topic.md#section)")
	links := ExtractLinks(body, "concepts/gemma.md")
	if len(links) != 1 {
		t.Fatalf("len = %d, want 1", len(links))
	}
	if links[0].DstRelPath != "concepts/topic.md" {
		t.Errorf("DstRelPath = %q, want concepts/topic.md", links[0].DstRelPath)
	}
}

// TestExtractLinks_ImageLinks verifies that image links ![alt](target) are
// captured alongside plain links.
func TestExtractLinks_ImageLinks(t *testing.T) {
	body := []byte("![cover](/bundle/images/cover.png) and text [doc](./doc.md)")
	links := ExtractLinks(body, "index.md")
	if len(links) != 2 {
		t.Fatalf("len = %d, want 2", len(links))
	}
}

// TestExtractLinks_EmptyBody verifies the function tolerates nil/empty input.
func TestExtractLinks_EmptyBody(t *testing.T) {
	if got := ExtractLinks(nil, "x.md"); got != nil {
		t.Errorf("nil body = %v, want nil", got)
	}
	if got := ExtractLinks([]byte(""), "x.md"); got != nil {
		t.Errorf("empty body = %v, want nil", got)
	}
	if got := ExtractLinks([]byte("no links here"), "x.md"); len(got) != 0 {
		t.Errorf("plain text = %v, want 0 links", got)
	}
}

// TestScanBundleLinks_DetectsBrokenLink seeds a bundle with two nodes: one
// referencing a missing target. After ScanBundleLinks the source node's
// has_broken_link flag must be true and the missing target reported.
func TestScanBundleLinks_DetectsBrokenLink(t *testing.T) {
	_, nodes, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	// Source node links to ./missing.md, which does not exist.
	body := []byte("---\ntype: concept\ntitle: Source\n---\n[miss](./missing.md)\n")
	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "concepts/source.md",
		Content:           body,
	}); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}

	report, err := svc.ScanBundleLinks(context.Background(), 1)
	if err != nil {
		t.Fatalf("ScanBundleLinks: %v", err)
	}
	if report.ScannedNodes < 2 {
		t.Errorf("ScannedNodes = %d, want >= 2 (index + source)", report.ScannedNodes)
	}
	if len(report.BrokenLinks) == 0 {
		t.Fatalf("expected broken link, got none")
	}
	bl := report.BrokenLinks[0]
	if bl.Reason != BrokenReasonTargetNotFound {
		t.Errorf("Reason = %q, want %q", bl.Reason, BrokenReasonTargetNotFound)
	}
	if !strings.HasSuffix(bl.DstRelPath, "missing.md") {
		t.Errorf("DstRelPath = %q, want suffix missing.md", bl.DstRelPath)
	}
	// The flag must be persisted on the source node.
	src, _ := nodes.GetByBundleAndRelPath(1, "concepts/source.md")
	if !src.HasBrokenLink {
		t.Errorf("source node HasBrokenLink = false, want true")
	}
}

// TestScanBundleLinks_TargetExists verifies that a link to an existing node is
// not flagged, and that the source node's flag stays false.
func TestScanBundleLinks_TargetExists(t *testing.T) {
	_, nodes, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "concepts/target.md",
		Content: mustTypedMD("concept", "Target"),
	}); err != nil {
		t.Fatalf("WriteMarkdown target: %v", err)
	}
	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "concepts/source.md",
		Content: []byte("---\ntype: concept\ntitle: Source\n---\n[t](./target.md)\n"),
	}); err != nil {
		t.Fatalf("WriteMarkdown source: %v", err)
	}

	report, err := svc.ScanBundleLinks(context.Background(), 1)
	if err != nil {
		t.Fatalf("ScanBundleLinks: %v", err)
	}
	for _, bl := range report.BrokenLinks {
		if strings.Contains(bl.DstRelPath, "target.md") {
			t.Errorf("target.md flagged broken: %+v", bl)
		}
	}
	src, _ := nodes.GetByBundleAndRelPath(1, "concepts/source.md")
	if src.HasBrokenLink {
		t.Errorf("source node HasBrokenLink = true, want false")
	}
}

// TestScanBundleLinks_NotFound asserts the bundle-not-found sentinel is
// returned for a missing bundle.
func TestScanBundleLinks_NotFound(t *testing.T) {
	_, _, _, _, svc := okfTestHarness(t)
	_, err := svc.ScanBundleLinks(context.Background(), 999)
	if !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}

// TestScanBundleLinks_ClearsFlagWhenFixed verifies that fixing a previously
// broken link flips has_broken_link back to false on a subsequent scan.
func TestScanBundleLinks_ClearsFlagWhenFixed(t *testing.T) {
	_, nodes, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "concepts/source.md",
		Content: []byte("---\ntype: concept\ntitle: Source\n---\n[t](./target.md)\n"),
	}); err != nil {
		t.Fatalf("WriteMarkdown source: %v", err)
	}

	// First scan: link is broken.
	if _, err := svc.ScanBundleLinks(context.Background(), 1); err != nil {
		t.Fatalf("first scan: %v", err)
	}
	src, _ := nodes.GetByBundleAndRelPath(1, "concepts/source.md")
	if !src.HasBrokenLink {
		t.Fatalf("HasBrokenLink after first scan = false, want true")
	}

	// Create the target node and re-scan.
	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "concepts/target.md",
		Content: mustTypedMD("concept", "Target"),
	}); err != nil {
		t.Fatalf("WriteMarkdown target: %v", err)
	}
	if _, err := svc.ScanBundleLinks(context.Background(), 1); err != nil {
		t.Fatalf("second scan: %v", err)
	}
	src, _ = nodes.GetByBundleAndRelPath(1, "concepts/source.md")
	if src.HasBrokenLink {
		t.Errorf("HasBrokenLink after fix = true, want false")
	}
}

// TestListBrokenLinks_Pagination exercises the cursor pagination path by
// exceeding the limit and walking two pages.
func TestListBrokenLinks_Pagination(t *testing.T) {
	_, _, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	// Write 3 source nodes, each pointing at a distinct missing target.
	for i, name := range []string{"a", "b", "c"} {
		body := []byte("---\ntype: concept\ntitle: " + name + "\n---\n[x](./miss-" + name + ".md)\n")
		if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
			PublicDirectoryID: 7, RelPath: "concepts/" + name + ".md",
			Content: body,
		}); err != nil {
			t.Fatalf("WriteMarkdown %s: %v", name, err)
		}
		_ = i
	}

	// Page 1: limit=2
	page1, cursor, err := svc.ListBrokenLinks(context.Background(), 1, "", "", 0, 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("page1 len = %d, want 2", len(page1))
	}
	if cursor == 0 {
		t.Errorf("page1 cursor = 0, want non-zero (more pages)")
	}
	// Page 2: use cursor from page1
	page2, cursor2, err := svc.ListBrokenLinks(context.Background(), 1, "", "", cursor, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) == 0 {
		t.Errorf("page2 len = 0, want >= 1")
	}
	if cursor2 != 0 {
		t.Errorf("page2 cursor = %d, want 0 (last page)", cursor2)
	}
}

// TestListBrokenLinks_Forbidden covers the visibility check: an opaque
// visibility stub that denies the bundle forces ErrOkfForbidden.
func TestListBrokenLinks_Forbidden(t *testing.T) {
	_, _, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	// Inject a visibility that returns an empty allow-set for any caller.
	svc.SetVisibility(stubVisibility{visible: map[uint64]bool{}})
	_, _, err = svc.ListBrokenLinks(context.Background(), bundle.ID, "user-x", "dept-y", 0, 50)
	if !errors.Is(err, ErrOkfForbidden) {
		t.Errorf("expected ErrOkfForbidden, got %v", err)
	}
}

// TestListBrokenLinks_NotFound covers the missing-bundle path.
func TestListBrokenLinks_NotFound(t *testing.T) {
	_, _, _, _, svc := okfTestHarness(t)
	_, _, err := svc.ListBrokenLinks(context.Background(), 999, "", "", 0, 50)
	if !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}

// TestScanBundleLinksForHandler_Forbidden covers the reader ACL on the
// HTTP-layer scan wrapper: a caller whose visibility set excludes the bundle
// must get ErrOkfForbidden rather than a scan running over data they cannot
// otherwise read. The handler maps this to 403.
func TestScanBundleLinksForHandler_Forbidden(t *testing.T) {
	_, _, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	svc.SetVisibility(stubVisibility{visible: map[uint64]bool{}})
	if _, err := svc.ScanBundleLinksForHandler(context.Background(), bundle.ID, "user-x", "dept-y"); !errors.Is(err, ErrOkfForbidden) {
		t.Errorf("expected ErrOkfForbidden, got %v", err)
	}
}

// TestScanBundleLinksForHandler_NotFound covers the missing-bundle path so
// the wrapper surfaces the same sentinel as the underlying scan.
func TestScanBundleLinksForHandler_NotFound(t *testing.T) {
	_, _, _, _, svc := okfTestHarness(t)
	if _, err := svc.ScanBundleLinksForHandler(context.Background(), 999, "user-x", "dept-y"); !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}

// okfTestHarness wires a fresh OkfService against in-memory fakes. Returns
// the bundle repo, node repo, edge repo, public dir stub, and service so
// individual tests can drive whichever layer they need.
func okfTestHarness(t *testing.T) (*fakeOkfBundleRepo, *fakeOkfNodeRepo, *fakeOkfEdgeRepo, *fakeOkfPublicDir, *OkfService) {
	t.Helper()
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")
	return bundles, nodes, edges, pub, svc
}

// Ensure gorm import is exercised by helpers that check ErrRecordNotFound in
// assertions; the lint pass complains about unused imports otherwise.
var _ = gorm.ErrRecordNotFound
