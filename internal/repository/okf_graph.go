package repository

import (
	"fmt"
	"math"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// OkfEdgeRepo provides data access for the OKF edge graph. All reads and
// writes carry the bundle's public_dir_id so a bundle's graph slice stays on
// the composite indexes; cross-bundle queries are explicitly not supported.
type OkfEdgeRepo struct {
	db *gorm.DB
}

// NewOkfEdgeRepo creates a new OkfEdgeRepo bound to the given GORM handle.
func NewOkfEdgeRepo(db *gorm.DB) *OkfEdgeRepo {
	return &OkfEdgeRepo{db: db}
}

// ReplaceForSrc swaps the outgoing edges of one source node in a single call.
// It deletes every existing edge with the given (public_dir_id, src_node_id)
// and inserts the new set. The whole operation must run inside the caller's
// transaction (tx non-nil) so the edge replacement commits atomically with the
// source node's own upsert; passing nil falls back to the repo handle, which
// is useful for tests but should not be used from WriteMarkdown.
//
// The slice may be empty — that is the legitimate "node has no outgoing
// links" case, and the function still performs the delete so stale edges from
// a previous version of the document are cleared.
func (r *OkfEdgeRepo) ReplaceForSrc(tx *gorm.DB, publicDirID, srcNodeID uint64, edges []model.OkfEdge) error {
	exec := tx
	if exec == nil {
		exec = r.db
	}
	if err := exec.Where("public_dir_id = ? AND src_node_id = ?", publicDirID, srcNodeID).
		Delete(&model.OkfEdge{}).Error; err != nil {
		return fmt.Errorf("delete old edges: %w", err)
	}
	if len(edges) == 0 {
		return nil
	}
	// Batch insert in one statement. GORM creates one INSERT per row when
	// given a slice, which is fine — edge counts per source are small (a
	// markdown body rarely has more than a few dozen links). The unique key
	// on (src_node_id, dst_rel_path, src_line) makes a duplicate insert
	// surface as an error, which the caller should treat as a bug rather
	// than silently swallow.
	if err := exec.Create(&edges).Error; err != nil {
		return fmt.Errorf("insert new edges: %w", err)
	}
	return nil
}

// ListBySrc returns the outgoing edges of one node, ordered by source line
// for stable output. limit <= 0 means "no limit" — callers that stream the
// full edge set (e.g. broken-link rebuild) pass 0.
func (r *OkfEdgeRepo) ListBySrc(publicDirID, srcNodeID uint64, limit int) ([]model.OkfEdge, error) {
	q := r.db.Where("public_dir_id = ? AND src_node_id = ?", publicDirID, srcNodeID).
		Order("src_line ASC, id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.OkfEdge
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ListByDst returns the incoming edges of one node — used for backlink
// traversal. dst_node_id is NULL for dead links, so this only returns live
// edges.
func (r *OkfEdgeRepo) ListByDst(publicDirID, dstNodeID uint64, limit int) ([]model.OkfEdge, error) {
	q := r.db.Where("public_dir_id = ? AND dst_node_id = ?", publicDirID, dstNodeID).
		Order("src_node_id ASC, src_line ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.OkfEdge
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ListOutBySrcBatch returns every live outgoing edge for a set of source
// nodes in a single query. P3b BFS expands a frontier of N nodes per hop;
// without this method the expansion would issue one query per frontier node
// (10 ms each at the index, 50 nodes = 500 ms per hop).
//
// Edges with dst_exists=false are excluded — the BFS layer treats a dead
// link as a leaf, so returning them would only inflate the response.
//
// limit caps each source node's contribution. Pass 0 for "no per-source
// cap"; production callers usually pass 0 because the per-node fan-out is
// naturally bounded by the markdown body's link count.
func (r *OkfEdgeRepo) ListOutBySrcBatch(publicDirID uint64, srcIDs []uint64, limit int) ([]model.OkfEdge, error) {
	if len(srcIDs) == 0 {
		return nil, nil
	}
	q := r.db.Where("public_dir_id = ? AND src_node_id IN ? AND dst_exists = ?", publicDirID, srcIDs, true).
		Order("src_node_id ASC, src_line ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.OkfEdge
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ListInByDstBatch is the reverse-direction counterpart of ListOutBySrcBatch
// — every live incoming edge for a set of dst nodes. Used by the "in" and
// "both" BFS directions and by bidirectional shortest-path.
func (r *OkfEdgeRepo) ListInByDstBatch(publicDirID uint64, dstIDs []uint64, limit int) ([]model.OkfEdge, error) {
	if len(dstIDs) == 0 {
		return nil, nil
	}
	q := r.db.Where("public_dir_id = ? AND dst_node_id IN ? AND dst_exists = ?", publicDirID, dstIDs, true).
		Order("dst_node_id ASC, src_node_id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var out []model.OkfEdge
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// EdgeStats summarizes a bundle's edge table for the stats endpoint.
// Live + Broken sum to Total. The Live count is what the BFS layer can walk
// because broken edges are excluded from ListOutBySrcBatch.
type EdgeStats struct {
	Total  uint32
	Live   uint32
	Broken uint32
}

// StatsByBundle returns total / live / broken edge counts for a bundle in
// one round-trip via a GROUP BY on dst_exists. The bundleID arg is unused
// but kept for symmetry with the rest of the bundle-scoped API.
func (r *OkfEdgeRepo) StatsByBundle(_, publicDirID uint64) (EdgeStats, error) {
	type row struct {
		DstExists bool
		N         int64
	}
	var rows []row
	if err := r.db.Model(&model.OkfEdge{}).
		Select("dst_exists, count(*) as n").
		Where("public_dir_id = ?", publicDirID).
		Group("dst_exists").
		Scan(&rows).Error; err != nil {
		return EdgeStats{}, err
	}
	var stats EdgeStats
	for _, r := range rows {
		// Edge counts are bounded by the table size; clamping into uint32
		// keeps the cast well-defined even if a future migration raised the
		// column type. Realistic bundles are < 1M edges, far below MaxUint32.
		n := clampCountToUint32(r.N)
		stats.Total += n
		if r.DstExists {
			stats.Live += n
		} else {
			stats.Broken += n
		}
	}
	return stats, nil
}

// clampCountToUint32 keeps an int64 count inside the uint32 range so the
// caller can sum without a cast. Negative counts come from a misbehaving
// aggregate and are treated as zero.
func clampCountToUint32(n int64) uint32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(n)
}

// ListBrokenByBundle returns the dead-link edges of a bundle (dst_exists=0)
// paged by edge id. cursor is the last edge id of the previous page; pass 0
// for the first page. The returned nextCursor is 0 when the bundle has been
// fully enumerated.
func (r *OkfEdgeRepo) ListBrokenByBundle(publicDirID, cursor uint64, limit int) ([]model.OkfEdge, uint64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := r.db.Where("public_dir_id = ? AND dst_exists = ?", publicDirID, false)
	if cursor > 0 {
		q = q.Where("id > ?", cursor)
	}
	var out []model.OkfEdge
	if err := q.Order("id ASC").Limit(limit + 1).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	nextCursor := uint64(0)
	if len(out) > limit {
		nextCursor = out[limit-1].ID
		out = out[:limit]
	}
	return out, nextCursor, nil
}

// CountBrokenByBundle returns the total dead-link edge count (dst_exists=false)
// for a bundle's public directory. It is a cheap COUNT over the materialized
// edge table so the broken-links response can carry a real total for pagination
// without re-scanning every node body on each page request.
func (r *OkfEdgeRepo) CountBrokenByBundle(publicDirID uint64) (int64, error) {
	var count int64
	if err := r.db.Model(&model.OkfEdge{}).
		Where("public_dir_id = ? AND dst_exists = ?", publicDirID, false).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// BrokenLinkRow is a dead bundle-relative edge joined with its source node's
// rel_path, in the shape the service projects onto its BrokenLink type. EdgeID
// is the cursor key (the edge primary key); it is not part of the wire response.
type BrokenLinkRow struct {
	EdgeID     uint64
	SrcNodeID  uint64
	SrcRelPath string
	DstRelPath string
	SrcLine    int
	LinkText   string
	LinkKind   string
}

// ListBrokenBundleLinks returns the dead bundle-relative edges (dst_exists=false
// AND link_kind='bundle') for a bundle, paged by edge id, joining the source
// node for its rel_path. It replaces the per-node OSS body scan ListBrokenLinks
// used to run — the materialized edge table is the authoritative broken-link
// store (kept current by every WriteMarkdown), so reading it is O(matches)
// instead of O(nodes). cursor is the last edge id of the previous page; pass 0
// for the first page. The returned nextCursor is 0 when fully enumerated.
func (r *OkfEdgeRepo) ListBrokenBundleLinks(publicDirID, cursor uint64, limit int) ([]BrokenLinkRow, uint64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := r.db.Table("disk_okf_edge AS e").
		Select("e.id AS edge_id, e.src_node_id, n.rel_path AS src_rel_path, e.dst_rel_path, e.src_line, e.link_text, e.link_kind").
		Joins("JOIN disk_okf_node n ON n.id = e.src_node_id").
		Where("e.public_dir_id = ? AND e.dst_exists = ? AND e.link_kind = ?", publicDirID, false, "bundle")
	if cursor > 0 {
		q = q.Where("e.id > ?", cursor)
	}
	var rows []BrokenLinkRow
	if err := q.Order("e.id ASC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	next := uint64(0)
	if len(rows) > limit {
		next = rows[limit-1].EdgeID
		rows = rows[:limit]
	}
	return rows, next, nil
}

// CountByBundle returns the total edge count for a bundle. The bundleID arg
// is accepted for symmetry with OkfNodeRepo.CountByBundle but the lookup goes
// through public_dir_id (the edge table's clustering key). tx is the caller's
// in-progress transaction or nil to use the repo handle.
func (r *OkfEdgeRepo) CountByBundle(tx *gorm.DB, _, publicDirID uint64) (uint32, error) {
	exec := tx
	if exec == nil {
		exec = r.db
	}
	var count int64
	if err := exec.Model(&model.OkfEdge{}).
		Where("public_dir_id = ?", publicDirID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	if count < 0 {
		return 0, nil
	}
	if count > int64(^uint32(0)) {
		return ^uint32(0), nil
	}
	return uint32(count), nil
}

// DeleteByBundle wipes every edge belonging to a bundle. Intended for the
// unregister path, where the bundle row is being removed and leaving orphan
// edges would corrupt future re-registrations. tx should be the caller's
// transaction so the delete commits atomically with the bundle delete.
func (r *OkfEdgeRepo) DeleteByBundle(tx *gorm.DB, publicDirID uint64) error {
	exec := tx
	if exec == nil {
		exec = r.db
	}
	return exec.Where("public_dir_id = ?", publicDirID).Delete(&model.OkfEdge{}).Error
}

// AdjustBacklinks bumps backlink_count on a set of node ids. increment ids
// each get +1, decrement ids each get -1. The function clamps at zero on
// decrement so a miscount can never push a node negative. Both slices are
// applied in a single UPDATE pass per direction; tx should be the caller's
// transaction.
//
// We accept the raw id slices (rather than deriving them inside the repo)
// because the diff that computes "which dst nodes gained/lost an incoming
// edge" lives at the service layer where the old and new edge sets are both
// in hand.
func (r *OkfEdgeRepo) AdjustBacklinks(tx *gorm.DB, increment, decrement []uint64) error {
	exec := tx
	if exec == nil {
		exec = r.db
	}
	if len(increment) > 0 {
		if err := exec.Model(&model.OkfNode{}).
			Where("id IN ?", increment).
			UpdateColumn("backlink_count", gorm.Expr("backlink_count + 1")).Error; err != nil {
			return fmt.Errorf("increment backlinks: %w", err)
		}
	}
	if len(decrement) > 0 {
		// Clamp at 0 — a buggy diff should never produce a negative count,
		// but GREATEST guards against it without failing the transaction.
		if err := exec.Model(&model.OkfNode{}).
			Where("id IN ? AND backlink_count > 0", decrement).
			UpdateColumn("backlink_count", gorm.Expr("backlink_count - 1")).Error; err != nil {
			return fmt.Errorf("decrement backlinks: %w", err)
		}
	}
	return nil
}
