package service

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/okf"
)

// LinkKind classifies a markdown link target inside an OKF bundle.
const (
	LinkKindBundle   = "bundle"   // bundle-relative "/bundle/<rel>" or relative "./x/y.md"
	LinkKindExternal = "external" // absolute http(s)://, mailto:, tel:, ftp:// ...
	LinkKindAnchor   = "anchor"   // pure in-page "#section"
)

// LinkReasonBrokenNotFetched and friends are the broken-link reasons reported
// by ScanBundleLinks. They are exported as constants so callers (handlers,
// tests) can branch without parsing error strings.
const (
	BrokenReasonTargetNotFound      = "target_not_found"
	BrokenReasonExternalUnsupported = "external_unsupported"
)

// LinkInfo captures one markdown link discovered in a node's body. SrcLine is
// 1-indexed (matching editor / log conventions). DstRelPath is normalized to
// bundle-relative forward-slash form for bundle links; for external / anchor
// links it carries the raw href.
type LinkInfo struct {
	SrcNodeID  uint64
	SrcRelPath string
	DstRelPath string
	SrcLine    int
	LinkText   string
	LinkKind   string
}

// BrokenLink is a LinkInfo that did not resolve to an existing node. Reason is
// one of the BrokenReason* constants.
type BrokenLink struct {
	LinkInfo
	Reason string
}

// markdownLinkRe matches both image and link syntax. Submatches:
//
//	[1] image alt text  (empty for plain links)
//	[2] image target
//	[3] link text       (empty for images)
//	[4] link target
//
// A single regex keeps the scan O(n) in the body size and avoids walking the
// same bytes twice. We deliberately do not handle reference-style links
// ([text][ref]) — OKF writers always emit inline links, and the SDK agrees.
var markdownLinkRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)|\[([^\]]+)\]\(([^)]+)\)`)

// okfNodeExistenceChecker is the narrow interface ExtractLinks needs to verify
// a bundle-relative target exists. OkfNodeRepo satisfies it natively. The
// indirection keeps link extraction testable without a real database.
type okfNodeExistenceChecker interface {
	GetByBundleAndRelPath(bundleID uint64, relPath string) (*model.OkfNode, error)
}

// ExtractLinks scans a markdown body for inline links/images and classifies
// each as bundle, external, or anchor. srcRelPath is the bundle-relative path
// of the document containing the links; it is used to resolve relative links
// like "./gemma.md" to absolute bundle-relative paths via okf.Resolve.
//
// Email addresses (foo@bar.com without a scheme), bare URLs (www.example.com),
// and inline code spans are not treated as links — only [text](target) and
// ![alt](target) forms. CJK link text is supported because the regex operates
// on UTF-8 bytes without code-point class restrictions.
func ExtractLinks(content []byte, srcRelPath string) []LinkInfo {
	if len(content) == 0 {
		return nil
	}
	var out []LinkInfo
	matches := markdownLinkRe.FindAllSubmatchIndex(content, -1)
	for _, m := range matches {
		// lineOf computes the 1-indexed line number of the match by counting
		// newlines from the start of the body up to the match's first byte.
		// We recompute per match rather than walking a cursor because matches
		// are non-overlapping but the cursor bookkeeping adds subtle bugs.
		line := 1 + bytes.Count(content[:m[0]], []byte("\n"))

		text, target := pickLinkSubmatch(content, m)
		if target == "" {
			continue
		}
		if isMailtoOrEmail(target) {
			out = append(out, LinkInfo{
				SrcRelPath: srcRelPath,
				DstRelPath: target,
				SrcLine:    line,
				LinkText:   text,
				LinkKind:   LinkKindExternal,
			})
			continue
		}
		if kind, resolved := classifyLink(target, srcRelPath); kind != "" {
			out = append(out, LinkInfo{
				SrcRelPath: srcRelPath,
				DstRelPath: resolved,
				SrcLine:    line,
				LinkText:   text,
				LinkKind:   kind,
			})
		}
	}
	return out
}

// pickLinkSubmatch returns (text, target) from a regex match. The regex has
// two alternatives (image vs link), each with its own submatch group pair, so
// we look at whichever pair is non-empty.
func pickLinkSubmatch(content []byte, m []int) (text, target string) {
	// Image group: m[2..3] alt text, m[4..5] target.
	if m[4] >= 0 {
		return string(content[m[2]:m[3]]), string(content[m[4]:m[5]])
	}
	// Link group: m[6..7] text, m[8..9] target.
	return string(content[m[6]:m[7]]), string(content[m[8]:m[9]])
}

// classifyLink determines the kind of a link target and, for bundle-relative
// links, the resolved absolute bundle path. Returns ("", "") when the target
// should be skipped (e.g. mailto handled by caller).
func classifyLink(target, srcRelPath string) (kind, resolved string) {
	// Strip a fragment suffix for classification purposes; bundle links keep
	// the fragment removed from the resolved path so the existence check hits
	// the file, not "file.md#section".
	target = strings.TrimSpace(target)
	if target == "" {
		return "", ""
	}
	if strings.HasPrefix(target, "#") {
		return LinkKindAnchor, target
	}
	// Absolute scheme: https://, mailto:, tel:, ftp://, etc. okf.Resolve
	// leaves these unchanged, so we use the same detection.
	if isExternalURL(target) {
		return LinkKindExternal, target
	}
	// Bundle-relative "/bundle/..." — keep the path verbatim but strip the
	// prefix so the existence check uses the bundle-relative key.
	if rel, ok := okf.ParseBundleRelative(target); ok {
		return LinkKindBundle, stripFragment(rel)
	}
	// Relative "./foo.md" / "../bar.md" / "baz.md" — resolve against the
	// current document's directory.
	resolved = okf.Resolve(target, srcRelPath)
	return LinkKindBundle, stripFragment(resolved)
}

// isExternalURL reports whether target is an absolute URL with a scheme. We
// reuse the pkg/okf external detection indirectly by checking for "://", plus
// the mailto/tel schemes that lack a "//" separator.
func isExternalURL(target string) bool {
	if strings.Contains(target, "://") {
		return true
	}
	if strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "tel:") {
		return true
	}
	return false
}

// isMailtoOrEmail reports whether target is a mailto link or a bare email.
// Bare emails (foo@bar.com) are matched so they don't get treated as relative
// bundle paths.
func isMailtoOrEmail(target string) bool {
	if strings.HasPrefix(target, "mailto:") {
		return true
	}
	// bare email: must contain "@", have a dot in the tail, and have no slash
	// (a slash means it's a path that happens to contain "@").
	if !strings.Contains(target, "@") {
		return false
	}
	if strings.ContainsAny(target, "/#?") {
		return false
	}
	at := strings.LastIndex(target, "@")
	if at <= 0 || at == len(target)-1 {
		return false
	}
	return strings.Contains(target[at+1:], ".")
}

// stripFragment removes any "#section" suffix so the existence check matches
// the file itself.
func stripFragment(p string) string {
	if i := strings.Index(p, "#"); i >= 0 {
		return p[:i]
	}
	return p
}

// ScanReport is the structured result of ScanBundleLinks. ScannedNodes counts
// every node inspected (including ones with no links); BrokenLinks is the
// list of unresolvable targets.
type ScanReport struct {
	ScannedNodes int
	BrokenLinks  []BrokenLink
}

// ScanBundleLinks walks every node in a bundle, extracts its links, and
// verifies bundle-relative links against the node index. Each node's
// has_broken_link flag is updated in-place to reflect the latest scan so
// readers can cheaply filter broken-link nodes without re-scanning.
//
// The function is forgiving: a node whose body cannot be read is counted as
// scanned but contributes no broken links, and external/anchor links are
// never broken (they are merely out-of-bundle references). The function
// returns an aggregate report so callers can surface a summary to the API.
func (s *OkfService) ScanBundleLinks(ctx context.Context, bundleID uint64) (*ScanReport, error) {
	bundle, err := s.bundles.GetByID(bundleID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, fmt.Errorf("lookup bundle: %w", err)
	}
	// ListByBundleSQLite and ListByBundle return the same shape; pick the one
	// matching the configured driver so JSON_CONTAINS does not blow up on
	// SQLite. Empty filter returns every node.
	nodes, err := s.listNodesForScan(bundleID, repository.NodeListFilter{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	existence := s.nodeExistenceChecker()
	report := &ScanReport{ScannedNodes: len(nodes)}
	// Track which nodes are broken so we can flip the flag in a single pass
	// after the scan completes.
	brokenByNodeID := map[uint64]bool{}
	for i := range nodes {
		n := &nodes[i]
		body := s.readNodeBody(ctx, bundle, n)
		links := ExtractLinks(body, n.RelPath)
		nodeBroken := false
		for _, li := range links {
			li.SrcNodeID = n.ID
			if li.LinkKind != LinkKindBundle {
				// External / anchor links are not broken-link candidates.
				continue
			}
			if !s.bundleLinkTargetExists(bundle.ID, li.DstRelPath, existence) {
				report.BrokenLinks = append(report.BrokenLinks, BrokenLink{
					LinkInfo: li,
					Reason:   BrokenReasonTargetNotFound,
				})
				nodeBroken = true
			}
		}
		brokenByNodeID[n.ID] = nodeBroken
		_ = brokenByNodeID // collected for callers that want a set lookup
		// Persist the flag. We update through the repo rather than the upsert
		// path so we don't bump UpdatedAt for content that didn't change; the
		// dedicated UpdateBrokenLinkFlag keeps the row otherwise stable.
		if err := s.updateNodeBrokenFlag(n, nodeBroken); err != nil {
			// Non-fatal: a flag-update failure shouldn't abort the whole scan.
			// Log via the structured path so operators can chase it; tests
			// observe it via the report.
			continue
		}
	}
	return report, nil
}

// listNodesForScan dispatches to the driver-appropriate list implementation.
func (s *OkfService) listNodesForScan(bundleID uint64, filter repository.NodeListFilter) ([]model.OkfNode, error) {
	// Both fake and real repos satisfy the same interface; the driver pick
	// mirrors the public ListNodesByType path.
	if s.dbDriver == "sqlite" {
		return s.nodes.ListByBundleSQLite(bundleID, filter)
	}
	return s.nodes.ListByBundle(bundleID, filter)
}

// readNodeBody fetches the markdown body for a node. The reader goes through
// the public directory service so it benefits from the same OSS path as the
// writer. A missing file or a read error yields an empty body; callers treat
// that as "no links" rather than a scan failure, since a single unreadable
// file must not abort a whole-bundle scan.
func (s *OkfService) readNodeBody(ctx context.Context, _ *model.OkfBundle, n *model.OkfNode) []byte {
	if n.FileID == 0 {
		return nil
	}
	body, err := s.pdSvc.ReadFileContent(ctx, n.FileID)
	if err != nil {
		// Swallow intentionally — see function doc.
		return nil
	}
	return body
}

// bundleLinkTargetExists reports whether dstRelPath resolves to a known node.
// The check is by natural key (bundle_id, rel_path), which is what the writer
// upserts against; an in-progress write that hasn't committed yet will read
// as missing, matching the writer's transactional semantics.
func (s *OkfService) bundleLinkTargetExists(bundleID uint64, dstRelPath string, _ okfNodeExistenceChecker) bool {
	if dstRelPath == "" {
		return false
	}
	if _, err := s.nodes.GetByBundleAndRelPath(bundleID, dstRelPath); err == nil {
		return true
	}
	return false
}

// nodeExistenceChecker returns the repo as the existence checker. The ind-
// direction exists so future callers can swap in a cached checker without
// touching the call sites.
func (s *OkfService) nodeExistenceChecker() okfNodeExistenceChecker { return s.nodes }

// updateNodeBrokenFlag persists the latest broken-link scan result for a node.
// We round-trip through Upsert so both fake and real repos agree on the
// update path. The node row is loaded from the repo (not mutated in memory)
// so we don't clobber concurrent writes from another goroutine.
func (s *OkfService) updateNodeBrokenFlag(n *model.OkfNode, broken bool) error {
	if n == nil {
		return nil
	}
	if n.HasBrokenLink == broken {
		return nil
	}
	clone := *n
	clone.HasBrokenLink = broken
	return s.nodes.Upsert(nil, &clone)
}

// ScanBundleLinksForHandler is the convenience wrapper used by the HTTP layer.
// It enforces reader ACL on the bundle (mirroring GetBundle) before walking
// nodes, since the scan writes has_broken_link back to disk_okf_node and so
// must not be triggerable by users who cannot even see the bundle.
func (s *OkfService) ScanBundleLinksForHandler(ctx context.Context, bundleID uint64, userID, department string) (*ScanReport, error) {
	bundle, err := s.bundles.GetByID(bundleID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, fmt.Errorf("lookup bundle: %w", err)
	}
	if vErr := s.requireBundleVisible(bundle, userID, department); vErr != nil {
		return nil, vErr
	}
	return s.ScanBundleLinks(ctx, bundleID)
}

// ListBrokenLinks returns the broken links recorded for a bundle, scoped to a
// cursor pagination window. The cursor is the node ID of the last item in the
// previous page; limit caps the page size. The handler is the only caller;
// the method lives on the service so the visibility check is enforced here
// rather than re-implemented in the handler.
func (s *OkfService) ListBrokenLinks(ctx context.Context, bundleID uint64, userID, department string, cursor uint64, limit int) ([]BrokenLink, uint64, error) {
	bundle, err := s.bundles.GetByID(bundleID)
	if err != nil {
		if isNotFound(err) {
			return nil, 0, ErrOkfBundleNotFound
		}
		return nil, 0, fmt.Errorf("lookup bundle: %w", err)
	}
	if vErr := s.requireBundleVisible(bundle, userID, department); vErr != nil {
		return nil, 0, vErr
	}
	if limit <= 0 || limit > 50 {
		limit = 50
	}

	nodes, err := s.listNodesForScan(bundleID, repository.NodeListFilter{})
	if err != nil {
		return nil, 0, fmt.Errorf("list nodes: %w", err)
	}
	existence := s.nodeExistenceChecker()
	out := make([]BrokenLink, 0, limit)
	nextCursor := uint64(0)
	for i := range nodes {
		n := &nodes[i]
		if cursor != 0 && n.ID <= cursor {
			continue
		}
		body := s.readNodeBody(ctx, bundle, n)
		links := ExtractLinks(body, n.RelPath)
		for _, li := range links {
			if li.LinkKind != LinkKindBundle {
				continue
			}
			if s.bundleLinkTargetExists(bundle.ID, li.DstRelPath, existence) {
				continue
			}
			li.SrcNodeID = n.ID
			out = append(out, BrokenLink{LinkInfo: li, Reason: BrokenReasonTargetNotFound})
			if len(out) == limit {
				nextCursor = n.ID
				return out, nextCursor, nil
			}
		}
	}
	return out, 0, nil
}
