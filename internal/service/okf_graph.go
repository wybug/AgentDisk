package service

import (
	"context"
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// materializeEdges is the WriteMarkdown transaction's edge-materialization step.
// It re-derives the source node's outgoing edges from the freshly-written body
// and replaces the node's existing edge set atomically.
//
// The function:
//  1. Re-runs ExtractLinks on the body (the same scan the broken-link path
//     uses), which classifies each link as bundle / external / anchor.
//  2. Resolves bundle-relative links to dst_node_id via the node repo; a miss
//     produces a dead-link edge (dst_exists=0, dst_node_id=0).
//  3. Diffs the new edge set against the previous one to compute the smallest
//     set of inserts/deletes — so a write that touches one paragraph does not
//     churn backlink counts on nodes whose edges did not change.
//  4. Updates backlink_count on every dst node that gained or lost an incoming
//     live edge.
//  5. Recomputes the source node's link_count and has_broken_link from the
//     new edge set, and persists them in the same transaction.
//
// The function runs inside the caller's transaction (tx non-nil) and is
// non-fatal to the caller only when wrapped with the same error handling as
// the rest of the materialization step — a failure here rolls the whole
// WriteMarkdown back, which is the right behavior (a half-materialized graph
// is worse than a failed write).
func (s *OkfService) materializeEdges(_ context.Context, tx *gorm.DB, bundle *model.OkfBundle, node *model.OkfNode, content []byte) error {
	if s.edges == nil {
		// Tests that exercise only the node path can skip wiring an edge repo.
		// Production always wires one.
		return nil
	}
	links := ExtractLinks(content, node.RelPath)
	newEdges := s.buildEdgesForLinks(bundle, node, links)

	oldEdges, err := s.edges.ListBySrc(bundle.PublicDirectoryID, node.ID, 0)
	if err != nil {
		return fmt.Errorf("list old edges: %w", err)
	}

	toInsert, toDelete, inc, dec := diffEdges(oldEdges, newEdges)

	if len(toDelete) > 0 || len(toInsert) > 0 {
		// ReplaceForSrc wipes the source node's edges and re-inserts the
		// new set. We pass the *post-diff* new set so identical edges land
		// in their original rows (the unique key on (src_node_id,
		// dst_rel_path, src_line) makes a re-insert a no-op on reuse).
		if err := s.edges.ReplaceForSrc(tx, bundle.PublicDirectoryID, node.ID, newEdges); err != nil {
			return fmt.Errorf("replace edges: %w", err)
		}
	}
	if len(inc) > 0 || len(dec) > 0 {
		if err := s.edges.AdjustBacklinks(tx, inc, dec); err != nil {
			return fmt.Errorf("adjust backlinks: %w", err)
		}
	}

	// Recompute source-node aggregates from the new edge set so they stay in
	// sync with the persisted rows. has_broken_link flips on as soon as one
	// dead-link edge exists; link_count counts every outgoing edge regardless
	// of kind so external/anchor links are visible in the count too. Clamp at
	// uint32 max as a defensive guard — a single markdown body cannot hold that
	// many links but gosec G115 insists on the cast being bounded.
	linkCount := uint64(len(newEdges))
	if linkCount > uint64(^uint32(0)) {
		linkCount = uint64(^uint32(0))
	}
	node.LinkCount = uint32(linkCount)
	node.HasBrokenLink = anyBroken(newEdges)
	return s.nodes.Upsert(tx, node)
}

// resolveBundleLinkTargets batch-resolves the destination nodes for every
// bundle-relative link in a single query (WHERE bundle_id = ? AND rel_path IN
// (?)), returning a relPath→node map. This replaces the per-link
// GetByBundleAndRelPath the edge materializer used to issue — an N+1 where K
// links cost K round trips. Best-effort: a lookup failure yields an empty map,
// which callers treat as "no targets resolved" (every bundle link broken), the
// same outcome as each per-link lookup failing.
func (s *OkfService) resolveBundleLinkTargets(bundleID uint64, links []LinkInfo) map[string]*model.OkfNode {
	out := map[string]*model.OkfNode{}
	relPaths := make([]string, 0, len(links))
	seen := map[string]bool{}
	for i := range links {
		li := links[i]
		if li.LinkKind != LinkKindBundle || seen[li.DstRelPath] {
			continue
		}
		seen[li.DstRelPath] = true
		relPaths = append(relPaths, li.DstRelPath)
	}
	if len(relPaths) == 0 {
		return out
	}
	nodes, err := s.nodes.ListByBundleAndRelPaths(bundleID, relPaths)
	if err != nil {
		return out
	}
	for i := range nodes {
		out[nodes[i].RelPath] = &nodes[i]
	}
	return out
}

// buildEdgesForLinks turns a slice of ExtractLinks output into OkfEdge rows
// ready for insert. Bundle links are resolved against the node repo (one batched
// query via resolveBundleLinkTargets) to get dst_node_id + dst_exists; external
// and anchor links land as-is with dst_exists=false (they have no in-bundle
// target and are not broken-link candidates by themselves — broken-link scans
// only apply to LinkKindBundle).
func (s *OkfService) buildEdgesForLinks(bundle *model.OkfBundle, node *model.OkfNode, links []LinkInfo) []model.OkfEdge {
	if len(links) == 0 {
		return nil
	}
	// Resolve every bundle-link target in one query instead of one per link.
	resolved := s.resolveBundleLinkTargets(bundle.ID, links)
	out := make([]model.OkfEdge, 0, len(links))
	for i := range links {
		li := links[i]
		edge := model.OkfEdge{
			PublicDirID: bundle.PublicDirectoryID,
			SrcNodeID:   node.ID,
			DstRelPath:  li.DstRelPath,
			LinkText:    truncateForColumn(li.LinkText, 512),
			SrcLine:     li.SrcLine,
			LinkKind:    li.LinkKind,
		}
		if li.LinkKind == LinkKindBundle {
			if dst := resolved[li.DstRelPath]; dst != nil {
				edge.DstNodeID = dst.ID
				edge.DstExists = true
			}
		}
		out = append(out, edge)
	}
	return out
}

// diffEdges partitions the new edge set against the old one. Returns:
//   - toInsert: edges in new that are not in old (by (dst_rel_path, src_line)).
//   - toDelete: edges in old that are not in new.
//   - increment: dst_node_id values whose live incoming edge count grew.
//   - decrement: dst_node_id values whose live incoming edge count shrank.
//
// Diffing on (dst_rel_path, src_line) mirrors the unique key, so "same target
// on the same line" is treated as unchanged even if the link text moved. We
// deliberately do not treat external/anchor edges as backlink-affecting —
// only live bundle edges (dst_exists=true) move the count.
func diffEdges(old, newEdges []model.OkfEdge) (toInsert, toDelete []model.OkfEdge, increment, decrement []uint64) {
	oldByKey := map[string]model.OkfEdge{}
	for _, e := range old {
		oldByKey[edgeKey(e)] = e
	}
	newByKey := map[string]model.OkfEdge{}
	for _, e := range newEdges {
		newByKey[edgeKey(e)] = e
	}

	for k, ne := range newByKey {
		oe, ok := oldByKey[k]
		if !ok {
			toInsert = append(toInsert, ne)
			// Brand-new live edge: dst gains an incoming edge.
			if ne.DstExists && ne.DstNodeID != 0 {
				increment = append(increment, ne.DstNodeID)
			}
			continue
		}
		// Edge existed before and still exists. If liveness flipped (target
		// was created/deleted between writes) the dst node's backlink count
		// moves accordingly.
		if ne.DstExists != oe.DstExists {
			if ne.DstExists && ne.DstNodeID != 0 {
				increment = append(increment, ne.DstNodeID)
			} else if oe.DstExists && oe.DstNodeID != 0 {
				decrement = append(decrement, oe.DstNodeID)
			}
		}
	}
	for k, oe := range oldByKey {
		if _, ok := newByKey[k]; !ok {
			toDelete = append(toDelete, oe)
			if oe.DstExists && oe.DstNodeID != 0 {
				decrement = append(decrement, oe.DstNodeID)
			}
		}
	}
	return toInsert, toDelete, increment, decrement
}

// edgeKey is the (dst_rel_path, src_line) composite that mirrors the unique
// index. Two edges with the same key are the "same" link for diffing.
func edgeKey(e model.OkfEdge) string {
	return fmt.Sprintf("%s\x00%d", e.DstRelPath, e.SrcLine)
}

// anyBroken reports whether any edge in the set is a dead bundle link. Used to
// recompute has_broken_link from the materialized edge set in O(n).
func anyBroken(edges []model.OkfEdge) bool {
	for i := range edges {
		if edges[i].LinkKind == LinkKindBundle && !edges[i].DstExists {
			return true
		}
	}
	return false
}

// truncateForColumn clamps s to maxLen bytes so the value fits the column
// without a silent DB-side truncation. Link text over 512 bytes is exceedingly
// rare (markdown link text is usually a short phrase) so this is purely
// defensive.
func truncateForColumn(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
