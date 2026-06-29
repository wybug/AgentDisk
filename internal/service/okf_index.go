package service

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

// LogAction* constants are the action tags written to log.md. They are
// exported so callers (handlers, tests) can pass them by name without typos.
const (
	LogActionCreate     = "create"
	LogActionUpdate     = "update"
	LogActionDelete     = "delete"
	LogActionRegister   = "register"
	LogActionUnregister = "unregister"
	LogActionScan       = "scan"
)

// indexLogSeparator is the field separator for log.md rows. We use a tab
// (rather than comma) because bundle relPaths and user IDs never contain a
// tab, but can contain commas; this keeps the rowsplit unambiguous.
const indexLogSeparator = "\t"

// GenerateIndexMarkdown renders the bundle's auto-generated index.md. The
// index has NO frontmatter (OKF mandates that auto-generated reserved files
// stay schema-less), and groups nodes by type with bundle-relative links so
// the reader can click through. The bundle title (or "Untitled Bundle")
// anchors the page; a header line records the generation time and counts so
// a human reader can tell at a glance whether the index is current.
//
// The output is deterministic: types are sorted alphabetically and nodes
// within a type are sorted by title, so identical bundle state produces
// identical bytes. This is essential for diff-stable commits when the index
// is regenerated on every write.
func (s *OkfService) GenerateIndexMarkdown(_ context.Context, bundleID uint64) ([]byte, error) {
	bundle, err := s.bundles.GetByID(bundleID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, fmt.Errorf("lookup bundle: %w", err)
	}

	// Gather all nodes. We do not apply a type or tag filter because the index
	// must reflect the full bundle, not a slice.
	nodes, err := s.listNodesForScan(bundleID, repository.NodeListFilter{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	// Skip the root index.md and log.md nodes — they are the auto-generated
	// files themselves and including them would self-reference. We keep them
	// in the bundle, but elide them from the index body.
	filtered := make([]model.OkfNode, 0, len(nodes))
	for i := range nodes {
		base := pathBase(nodes[i].RelPath)
		if base == model.OkfReservedRootRelPath || base == model.OkfReservedLogRelPath {
			continue
		}
		filtered = append(filtered, nodes[i])
	}

	// Group by type → sorted title → node. types sorted for determinism.
	byType := groupNodesByType(filtered)
	now := time.Now().UTC().Format(time.RFC3339)

	var buf bytes.Buffer
	title := bundle.Title
	if strings.TrimSpace(title) == "" {
		title = "Untitled Bundle"
	}
	fmt.Fprintf(&buf, "# %s\n\n", title)
	fmt.Fprintf(&buf, "> Generated at %s. %d nodes, %d types.\n\n", now, len(filtered), len(byType))

	if len(filtered) == 0 {
		// Empty bundle: still emit a header so the file is non-empty.
		buf.WriteString("_No nodes yet._\n")
		return buf.Bytes(), nil
	}

	for _, t := range sortedTypeKeys(byType) {
		group := byType[t]
		fmt.Fprintf(&buf, "## Type: %s (%d nodes)\n", t, len(group))
		for _, n := range group {
			line := renderIndexNodeLine(n)
			buf.WriteString(line)
		}
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

// renderIndexNodeLine formats one bullet for the index. The title defaults to
// the relPath when empty (OKF allows untitled nodes) and the description, if
// present, is appended after an em-dash so a reader gets a one-line summary.
func renderIndexNodeLine(n model.OkfNode) string {
	title := n.Title
	if strings.TrimSpace(title) == "" {
		title = n.RelPath
	}
	href := "/bundle/" + n.RelPath
	if strings.TrimSpace(n.Description) != "" {
		return fmt.Sprintf("- [%s](%s) — %s\n", title, href, singleLine(n.Description))
	}
	return fmt.Sprintf("- [%s](%s)\n", title, href)
}

// singleLine collapses newlines and trims surrounding whitespace so a multi-
// line description fits on a single index bullet.
func singleLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// groupNodesByType buckets nodes by Type. Nodes with an empty type are placed
// under the "" bucket — the caller can choose to render them as "untitled" or
// leave them out; the renderer keeps them under an empty heading for honesty.
func groupNodesByType(nodes []model.OkfNode) map[string][]model.OkfNode {
	out := map[string][]model.OkfNode{}
	for i := range nodes {
		t := nodes[i].Type
		out[t] = append(out[t], nodes[i])
	}
	for t := range out {
		g := out[t]
		sort.Slice(g, func(a, b int) bool {
			return g[a].Title < g[b].Title
		})
		out[t] = g
	}
	return out
}

// sortedTypeKeys returns the keys of m sorted alphabetically. Deterministic
// ordering keeps the rendered index stable across regenerations.
func sortedTypeKeys(m map[string][]model.OkfNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// pathBase returns the last path segment. We avoid importing path/filepath
// here because bundle relPaths always use forward slashes; a small helper
// keeps this file dependency-light.
func pathBase(p string) string {
	if p == "" {
		return ""
	}
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// AppendLogEntry appends a single row to the bundle's log.md. The row format
// is "ISO8601\taction\trelPath\tuserID\n". log.md must not have frontmatter
// (OKF §reserved-file-names); this function preserves the existing body and
// appends, creating the file if it does not exist yet.
//
// Failures here are non-fatal to the caller (a missing log row is preferable
// to a failed write), but the error is returned so the caller can decide
// whether to log it.
func (s *OkfService) AppendLogEntry(ctx context.Context, bundleID uint64, action, relPath, userID string) error {
	bundle, err := s.bundles.GetByID(bundleID)
	if err != nil {
		if isNotFound(err) {
			return ErrOkfBundleNotFound
		}
		return fmt.Errorf("lookup bundle: %w", err)
	}

	// Read the existing log.md body so we can append without clobbering. A
	// missing file is fine — we start from an empty body. ReadFileContent
	// going through the public directory service guarantees the path matches
	// what the writer uses.
	var body []byte
	if existing, fErr := s.pdSvc.FindFileByRelPath(bundle.PublicDirectoryID, model.OkfReservedLogRelPath); fErr == nil {
		if cur, rErr := s.pdSvc.ReadFileContent(ctx, existing.ID); rErr == nil {
			body = cur
		}
	}

	row := formatLogRow(action, relPath, userID)
	updated := appendLogRow(body, row)

	// Always write through UploadFileAt so the OSS path and versioning match
	// the regular writer flow. Plain text body, no frontmatter.
	if _, err := s.pdSvc.UploadFileAt(ctx, bundle.PublicDirectoryID, model.OkfReservedLogRelPath, "text/markdown; charset=utf-8", updated); err != nil {
		return fmt.Errorf("write log.md: %w", err)
	}
	return nil
}

// formatLogRow builds a single tab-separated row with a trailing newline.
// relPath may be empty for bundle-level actions (register / unregister / scan).
func formatLogRow(action, relPath, userID string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("%s%s%s%s%s%s%s\n", now, indexLogSeparator, action, indexLogSeparator, relPath, indexLogSeparator, userID)
}

// appendLogRow concatenates the existing body with the new row, inserting a
// trailing newline when the body does not end with one so each row lands on
// its own line.
func appendLogRow(body []byte, row string) []byte {
	if len(body) == 0 {
		return []byte(row)
	}
	if !bytes.HasSuffix(body, []byte("\n")) {
		body = append(body, '\n')
	}
	return append(body, []byte(row)...)
}

// RegenerateIndex is the service entry-point for POST /bundles/:id/regenerate-
// index. It renders the index markdown and writes it to OSS via the same
// UploadFileAt path as the writer, then returns the new content version
// identifier (the OSS object version, approximated by the file row's
// Version) and the regeneration timestamp. Reader ACL is enforced because
// regeneration overwrites the bundle's root index.md in OSS — letting a
// user who cannot even see the bundle overwrite its index would be a
// privilege escalation.
func (s *OkfService) RegenerateIndex(ctx context.Context, bundleID uint64, userID, department string) (indexVersion uint32, regeneratedAt time.Time, err error) {
	bundle, err := s.bundles.GetByID(bundleID)
	if err != nil {
		if isNotFound(err) {
			return 0, time.Time{}, ErrOkfBundleNotFound
		}
		return 0, time.Time{}, fmt.Errorf("lookup bundle: %w", err)
	}
	if vErr := s.requireBundleVisible(bundle, userID, department); vErr != nil {
		return 0, time.Time{}, vErr
	}
	return s.regenerateIndexInternal(ctx, bundle)
}

// regenerateIndexInternal is the ACL-free inner path used by callers that have
// already proven their right to touch the bundle (e.g. the async post-write
// index regen triggered by WriteMarkdown, which already passed the writer's
// own ACL at the public-directory layer).
func (s *OkfService) regenerateIndexInternal(ctx context.Context, bundle *model.OkfBundle) (indexVersion uint32, regeneratedAt time.Time, err error) {
	body, err := s.GenerateIndexMarkdown(ctx, bundle.ID)
	if err != nil {
		return 0, time.Time{}, err
	}
	file, err := s.pdSvc.UploadFileAt(ctx, bundle.PublicDirectoryID, model.OkfReservedRootRelPath, "text/markdown; charset=utf-8", body)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("write index.md: %w", err)
	}
	// file.Version is a small monotonic counter (one per re-write); a negative
	// value is a defensive guard against an unknown storage backend that
	// misbehaves. We clamp to uint32 by capping at math.MaxUint32 to satisfy
	// gosec G115, even though in practice the version never approaches it.
	version := uint32(0)
	if file.Version > 0 {
		v := uint64(file.Version)
		if v > uint64(^uint32(0)) {
			v = uint64(^uint32(0))
		}
		version = uint32(v)
	}
	return version, time.Now().UTC(), nil
}
