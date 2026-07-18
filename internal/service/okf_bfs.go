package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"gorm.io/gorm"
)

// ErrOkfNodeNotFound is returned by graph queries when the requested node ID
// does not resolve to a row. Handlers map this to 404.
var ErrOkfNodeNotFound = errors.New("okf: node not found")

// ErrOkfInvalidDirection is returned when a BFS direction is neither out, in,
// nor both. Handlers map this to 400.
var ErrOkfInvalidDirection = errors.New("okf: invalid direction (must be out, in, or both)")

// BFS direction constants. The values are part of the handler query-string
// contract (neighbors?dir=out|in|both), so renames need a wire-compat shim.
const (
	BFSOut  = "out"
	BFSIn   = "in"
	BFSBoth = "both"
)

// MaxBFSDepth caps how deep Reachable / ShortestPath will walk. The plan's
// performance budget targets 3 hops; deeper walks blow past the response
// budget without delivering useful signal. Callers asking for more get
// clamped, not errored.
const MaxBFSDepth = 3

// DefaultBFSMaxNodes is the cap applied when the caller does not pass one.
// Large enough to cover typical bundle graphs (most are <500 nodes), small
// enough that a runaway query can never return a multi-MB response.
const DefaultBFSMaxNodes = 500

// HardBFSMaxNodes is the absolute ceiling. Callers that ask for more get
// clamped to this — it protects the JSON encoder and the network from a
// pathological bundle shape.
const HardBFSMaxNodes = 1000

// NeighborsRequest is the input shape for the 1-hop neighbors query.
type NeighborsRequest struct {
	NodeID     uint64
	Direction  string // one of BFSOut / BFSIn / BFSBoth; "" defaults to BFSOut
	TypeFilter string // optional: only return neighbors of this OKF type
	UserID     string // ACL identity
	Department string // ACL identity
}

// NeighborsResponse is the 1-hop output. Nodes is the deduped neighbor list;
// Edges carries the rows that produced them so the client can render labels.
type NeighborsResponse struct {
	Nodes []model.OkfNode
	Edges []model.OkfEdge
}

// ReachableRequest is the input shape for the N-hop reachable query.
type ReachableRequest struct {
	NodeID     uint64
	Depth      int      // 1..MaxBFSDepth, clamped if over
	Types      []string // optional: only count nodes whose type is in this set
	MaxNodes   int      // optional: total nodes cap; defaults to DefaultBFSMaxNodes
	Direction  string   // optional; defaults to BFSOut
	UserID     string
	Department string
}

// ReachableResponse is the N-hop output. Nodes excludes the start node.
type ReachableResponse struct {
	Nodes []model.OkfNode
}

// ShortestPathRequest is the input shape for the bidirectional shortest
// path query. The walk terminates as soon as the two frontiers meet.
type ShortestPathRequest struct {
	SrcNodeID  uint64
	DstNodeID  uint64
	MaxDepth   int // per-side cap; total path length ≤ 2 * MaxDepth
	UserID     string
	Department string
}

// ShortestPathResponse is the shortest-path output. Nodes is ordered from
// src to dst (inclusive of both); empty when no path was found within the
// depth budget.
type ShortestPathResponse struct {
	Nodes []model.OkfNode
	Found bool
}

// SubgraphRequest is the input shape for the type-filtered subgraph query.
type SubgraphRequest struct {
	BundleID   uint64
	Types      []string // nodes whose type is in this set (empty = all)
	MaxNodes   int      // total node cap
	UserID     string
	Department string
}

// SubgraphResponse carries the subgraph's nodes and the edges between them.
// Edges whose src or dst falls outside the type filter (or the MaxNodes cap)
// are excluded so the response is a self-contained graph slice.
type SubgraphResponse struct {
	Nodes []model.OkfNode
	Edges []model.OkfEdge
}

// Neighbors returns the 1-hop neighbors of a node. The src node itself is
// excluded from the result. dir controls whether the walk follows outgoing
// edges, incoming edges, or both. The Type filter narrows the returned
// neighbor set; it does not affect which edges are walked.
//
// ACL: the source node's bundle must be visible to the caller. Cross-bundle
// edges are not a thing — the edge table is bundle-scoped — so this check
// is sufficient.
func (s *OkfService) Neighbors(_ context.Context, req NeighborsRequest) (*NeighborsResponse, error) {
	if req.Direction == "" {
		req.Direction = BFSOut
	}
	switch req.Direction {
	case BFSOut, BFSIn, BFSBoth:
	default:
		return nil, ErrOkfInvalidDirection
	}

	node, bundle, err := s.loadNodeAndBundle(req.NodeID)
	if err != nil {
		return nil, err
	}
	if visErr := s.requireBundleVisible(bundle, req.UserID, req.Department); visErr != nil {
		return nil, visErr
	}

	pdID := bundle.PublicDirectoryID
	var out, in []model.OkfEdge
	switch req.Direction {
	case BFSOut, BFSBoth:
		out, err = s.edges.ListOutBySrcBatch(pdID, []uint64{node.ID}, 0)
	case BFSIn:
		in, err = s.edges.ListInByDstBatch(pdID, []uint64{node.ID}, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("load edges: %w", err)
	}
	if req.Direction == BFSBoth {
		in, err = s.edges.ListInByDstBatch(pdID, []uint64{node.ID}, 0)
		if err != nil {
			return nil, fmt.Errorf("load in edges: %w", err)
		}
	}
	neighborIDs := edgesToNeighborIDs(node.ID, out, in, req.Direction)
	if len(neighborIDs) == 0 {
		return &NeighborsResponse{Nodes: []model.OkfNode{}, Edges: append(out, in...)}, nil
	}

	loaded, err := s.nodes.ListByIDs(neighborIDs)
	if err != nil {
		return nil, fmt.Errorf("load neighbors: %w", err)
	}
	loaded = filterNodesByType(loaded, req.TypeFilter)
	return &NeighborsResponse{Nodes: loaded, Edges: append(out, in...)}, nil
}

// Reachable runs an N-hop BFS from the start node and returns every reachable
// node (excluding the start). The walk is bounded by both Depth (≤MaxBFSDepth)
// and MaxNodes so a hyper-connected bundle cannot blow the response budget.
//
// The cache is consulted hop-by-hop: a node whose adjacency is cached skips
// the batched edge lookup. Cache misses are backfilled after the DB lookup so
// the next walk benefits.
func (s *OkfService) Reachable(ctx context.Context, req ReachableRequest) (*ReachableResponse, error) {
	if req.Direction == "" {
		req.Direction = BFSOut
	}
	if req.Direction != BFSOut && req.Direction != BFSIn && req.Direction != BFSBoth {
		return nil, ErrOkfInvalidDirection
	}
	if req.Depth <= 0 {
		req.Depth = 1
	}
	if req.Depth > MaxBFSDepth {
		req.Depth = MaxBFSDepth
	}
	if req.MaxNodes <= 0 {
		req.MaxNodes = DefaultBFSMaxNodes
	}
	if req.MaxNodes > HardBFSMaxNodes {
		req.MaxNodes = HardBFSMaxNodes
	}

	// "both" runs two single-direction walks and unions them. This sidesteps
	// a refactor of batchAdjacency (shared with ShortestPath) and keeps the
	// cache key single-direction. The cost is 2x BFS, acceptable because
	// MaxBFSDepth=3 bounds each walk.
	if req.Direction == BFSBoth {
		return s.reachableBoth(ctx, req)
	}

	node, bundle, err := s.loadNodeAndBundle(req.NodeID)
	if err != nil {
		return nil, err
	}
	if err := s.requireBundleVisible(bundle, req.UserID, req.Department); err != nil {
		return nil, err
	}

	typeFilter := newTypeSet(req.Types)
	visited := map[uint64]bool{node.ID: true}
	frontier := []uint64{node.ID}
	reachable := []uint64{}

	for depth := 0; depth < req.Depth && len(frontier) > 0; depth++ {
		adjacency, missing := s.batchAdjacency(ctx, bundle, frontier, req.Direction)
		// Backfill misses so the next walk hits the cache.
		for _, nodeID := range missing {
			if adj, ok := adjacency[nodeID]; ok {
				_ = s.cache.SetAdj(ctx, bundle.ID, nodeID, adj)
			}
		}
		var next []uint64
		for _, srcID := range frontier {
			for _, dstID := range adjacency[srcID] {
				if visited[dstID] {
					continue
				}
				visited[dstID] = true
				reachable = append(reachable, dstID)
				if len(reachable) >= req.MaxNodes {
					return s.buildReachableResponse(reachable, typeFilter)
				}
				next = append(next, dstID)
			}
		}
		frontier = next
	}
	return s.buildReachableResponse(reachable, typeFilter)
}

// reachableBoth unions a BFSOut walk and a BFSIn walk from the same start
// node. Used by Reachable when Direction==BFSBoth. The two walks run
// sequentially (each is bounded by MaxBFSDepth); the union is dedup'd by
// node ID and capped at req.MaxNodes.
//
// Ordering: out-walk nodes first, then in-walk nodes that weren't already
// in the out set. This is deterministic but not ranked — callers needing
// "closest first" semantics should run a single-direction walk instead.
//
// The MaxNodes cap is applied per walk AND at the union (worst case the
// two walks each return MaxNodes and the union has 2×MaxNodes, which the
// final truncation brings back to req.MaxNodes). This means in well-fed
// graphs the in-walk may contribute 0 nodes after truncation — an
// acceptable tradeoff vs. complicating the cache key with a direction
// suffix.
func (s *OkfService) reachableBoth(ctx context.Context, req ReachableRequest) (*ReachableResponse, error) {
	outReq := req
	outReq.Direction = BFSOut
	out, err := s.Reachable(ctx, outReq)
	if err != nil {
		return nil, err
	}
	inReq := req
	inReq.Direction = BFSIn
	in, err := s.Reachable(ctx, inReq)
	if err != nil {
		return nil, err
	}

	seen := make(map[uint64]bool, len(out.Nodes)+len(in.Nodes))
	union := make([]model.OkfNode, 0, len(out.Nodes)+len(in.Nodes))
	for i := range out.Nodes {
		n := out.Nodes[i]
		if !seen[n.ID] {
			seen[n.ID] = true
			union = append(union, n)
		}
	}
	for i := range in.Nodes {
		n := in.Nodes[i]
		if !seen[n.ID] {
			seen[n.ID] = true
			union = append(union, n)
		}
	}
	if req.MaxNodes > 0 && len(union) > req.MaxNodes {
		union = union[:req.MaxNodes]
	}
	return &ReachableResponse{Nodes: union}, nil
}

// ShortestPath runs a bidirectional BFS from src and dst. The walk terminates
// as soon as the two frontiers meet, so the cost is O(b^(d/2)) rather than
// O(b^d) for a unidirectional walk — important on well-connected graphs where
// the path length is small but the branching factor is high.
//
// Returns Found=false with an empty node slice when no path exists within the
// depth budget. ACL: both src and dst must be in bundles visible to the
// caller; in practice they are always in the same bundle (the edge table is
// bundle-scoped), but we check both to fail closed on a cross-bundle request.
func (s *OkfService) ShortestPath(ctx context.Context, req ShortestPathRequest) (*ShortestPathResponse, error) {
	if req.MaxDepth <= 0 {
		req.MaxDepth = MaxBFSDepth
	}
	if req.MaxDepth > MaxBFSDepth {
		req.MaxDepth = MaxBFSDepth
	}
	if req.SrcNodeID == req.DstNodeID {
		// Trivial path: same node. Still subject to ACL.
		node, bundle, err := s.loadNodeAndBundle(req.SrcNodeID)
		if err != nil {
			return nil, err
		}
		if err := s.requireBundleVisible(bundle, req.UserID, req.Department); err != nil {
			return nil, err
		}
		return &ShortestPathResponse{Nodes: []model.OkfNode{*node}, Found: true}, nil
	}

	srcNode, srcBundle, err := s.loadNodeAndBundle(req.SrcNodeID)
	if err != nil {
		return nil, err
	}
	if visErr := s.requireBundleVisible(srcBundle, req.UserID, req.Department); visErr != nil {
		return nil, visErr
	}
	_, dstBundle, err := s.loadNodeAndBundle(req.DstNodeID)
	if err != nil {
		return nil, err
	}
	if visErr := s.requireBundleVisible(dstBundle, req.UserID, req.Department); visErr != nil {
		return nil, visErr
	}
	if srcBundle.ID != dstBundle.ID {
		// Cross-bundle edges do not exist; the path cannot either.
		return &ShortestPathResponse{Nodes: []model.OkfNode{}, Found: false}, nil
	}

	// Parent maps let us reconstruct the path once the two walks meet. A nil
	// parent marks a start node. *uint64 (rather than uint64 + a sentinel)
	// keeps the "is this the start?" check a simple nil test.
	fwdVisited := map[uint64]*uint64{srcNode.ID: nil}
	bwdVisited := map[uint64]*uint64{req.DstNodeID: nil}
	fwdFrontier := []uint64{srcNode.ID}
	bwdFrontier := []uint64{req.DstNodeID}

	var meet uint64
	var found bool
	for depth := 0; depth < req.MaxDepth && len(fwdFrontier) > 0 && len(bwdFrontier) > 0 && !found; depth++ {
		// Expand the smaller frontier first — classic bidirectional optimization.
		var nextFwd, nextBwd []uint64
		if len(fwdFrontier) <= len(bwdFrontier) {
			nextFwd, meet, found = s.expandFrontier(ctx, srcBundle, fwdFrontier, fwdVisited, bwdVisited, BFSOut)
			nextBwd = bwdFrontier
		} else {
			nextBwd, meet, found = s.expandFrontier(ctx, srcBundle, bwdFrontier, bwdVisited, fwdVisited, BFSIn)
			nextFwd = fwdFrontier
		}
		fwdFrontier = nextFwd
		bwdFrontier = nextBwd
	}
	if !found {
		return &ShortestPathResponse{Nodes: []model.OkfNode{}, Found: false}, nil
	}

	path := reconstructPath(meet, fwdVisited, bwdVisited)
	loaded, err := s.nodes.ListByIDs(path)
	if err != nil {
		return nil, fmt.Errorf("load path: %w", err)
	}
	byID := map[uint64]model.OkfNode{}
	for i := range loaded {
		byID[loaded[i].ID] = loaded[i]
	}
	ordered := make([]model.OkfNode, 0, len(path))
	for _, id := range path {
		if n, ok := byID[id]; ok {
			ordered = append(ordered, n)
		}
	}
	return &ShortestPathResponse{Nodes: ordered, Found: true}, nil
}

// Subgraph returns the nodes (and edges between them) of a bundle filtered by
// type. The MaxNodes cap applies to nodes; edges are pruned to match. This is
// the bulk-extract path used by the front-end's graph visualizer.
//
// ACL: the bundle must be visible.
func (s *OkfService) Subgraph(_ context.Context, req SubgraphRequest) (*SubgraphResponse, error) {
	if req.MaxNodes <= 0 {
		req.MaxNodes = DefaultBFSMaxNodes
	}
	if req.MaxNodes > HardBFSMaxNodes {
		req.MaxNodes = HardBFSMaxNodes
	}
	bundle, err := s.bundles.GetByID(req.BundleID)
	if err != nil {
		return nil, err
	}
	if visErr := s.requireBundleVisible(bundle, req.UserID, req.Department); visErr != nil {
		return nil, visErr
	}

	// Node filter lives at the service layer because the repo's ListByBundle
	// takes a single type. A multi-type filter is the common case for the
	// visualizer (e.g. "concepts + guides"), so we accept a slice.
	listFn := s.nodes.ListByBundle
	if s.dbDriver == "sqlite" {
		listFn = s.nodes.ListByBundleSQLite
	}
	all, err := listFn(bundle.ID, nodeListFilterForSubgraph(req.Types), 0, 0)
	if err != nil {
		return nil, fmt.Errorf("load nodes: %w", err)
	}
	all = filterNodesByTypes(all, req.Types)
	if len(all) > req.MaxNodes {
		all = all[:req.MaxNodes]
	}
	if len(all) == 0 {
		return &SubgraphResponse{Nodes: []model.OkfNode{}, Edges: []model.OkfEdge{}}, nil
	}

	idSet := map[uint64]bool{}
	ids := make([]uint64, 0, len(all))
	for i := range all {
		ids = append(ids, all[i].ID)
		idSet[all[i].ID] = true
	}
	edges, err := s.edges.ListOutBySrcBatch(bundle.PublicDirectoryID, ids, 0)
	if err != nil {
		return nil, fmt.Errorf("load edges: %w", err)
	}
	// Drop edges whose dst is outside the slice (e.g. a type the caller did
	// not ask for) — they would dangle in the visualizer.
	pruned := edges[:0]
	for i := range edges {
		if idSet[edges[i].DstNodeID] {
			pruned = append(pruned, edges[i])
		}
	}
	return &SubgraphResponse{Nodes: all, Edges: pruned}, nil
}

// loadNodeAndBundle is the shared preamble for every BFS entry point: resolve
// the node, then resolve its bundle. The two lookups are sequential because
// the bundle lookup needs the node's BundleID; batching them would require a
// JOIN we don't have an index for.
func (s *OkfService) loadNodeAndBundle(nodeID uint64) (*model.OkfNode, *model.OkfBundle, error) {
	node, err := s.nodes.GetByID(nodeID)
	if err != nil {
		return nil, nil, mapNodeLookupErr(err)
	}
	bundle, err := s.bundles.GetByID(node.BundleID)
	if err != nil {
		return nil, nil, ErrOkfBundleNotFound
	}
	return node, bundle, nil
}

// batchAdjacency returns the live outgoing (or incoming) adjacency for every
// node in frontier. Cache hits are pulled straight from the cache; misses are
// batched into a single edge lookup and merged in.
//
// Returns:
//   - adjacency: the full per-node id→neighbors map
//   - missing: the subset of frontier nodes that were not in cache (so the
//     caller can backfill).
func (s *OkfService) batchAdjacency(ctx context.Context, bundle *model.OkfBundle, frontier []uint64, dir string) (adjacency map[uint64][]uint64, missing []uint64) {
	adjacency = map[uint64][]uint64{}
	for _, id := range frontier {
		cached, _ := s.cache.GetAdj(ctx, bundle.ID, id)
		if cached != nil {
			adjacency[id] = dedupAdj(cached)
			continue
		}
		missing = append(missing, id)
	}
	if len(missing) == 0 {
		return adjacency, missing
	}

	var edges []model.OkfEdge
	var err error
	if dir == BFSIn {
		edges, err = s.edges.ListInByDstBatch(bundle.PublicDirectoryID, missing, 0)
	} else {
		edges, err = s.edges.ListOutBySrcBatch(bundle.PublicDirectoryID, missing, 0)
	}
	if err != nil {
		// On error, treat the missing nodes as leaves — the BFS will still
		// complete, just with a smaller result. Cache backfill is skipped.
		return adjacency, nil
	}
	if dir == BFSIn {
		for _, e := range edges {
			adjacency[e.DstNodeID] = append(adjacency[e.DstNodeID], e.SrcNodeID)
		}
	} else {
		for _, e := range edges {
			adjacency[e.SrcNodeID] = append(adjacency[e.SrcNodeID], e.DstNodeID)
		}
	}
	for _, id := range missing {
		adjacency[id] = dedupAdj(adjacency[id])
	}
	return adjacency, missing
}

// expandFrontier advances one BFS frontier one hop. sharedVisited is the
// other direction's visited set; the function returns meet=0, found=false
// until a node in the expansion also appears in sharedVisited.
//
// The function does NOT mutate frontier — it returns the next-frontier
// slice instead, because mutating a slice header in place cannot shrink the
// backing array. The caller reassigns.
func (s *OkfService) expandFrontier(ctx context.Context, bundle *model.OkfBundle, frontier []uint64, visited, sharedVisited map[uint64]*uint64, dir string) (next []uint64, meet uint64, found bool) {
	adjacency, _ := s.batchAdjacency(ctx, bundle, frontier, dir)
	for _, src := range frontier {
		for _, dst := range adjacency[src] {
			if _, seen := visited[dst]; seen {
				continue
			}
			srcCopy := src
			visited[dst] = &srcCopy
			if _, isMeet := sharedVisited[dst]; isMeet {
				return next, dst, true
			}
			next = append(next, dst)
		}
	}
	return next, 0, false
}

// reconstructPath rebuilds the src→dst path from the two parent maps.
// fwdVisited walks backward from meet to src; bwdVisited walks backward from
// meet to dst. The two halves are concatenated.
func reconstructPath(meet uint64, fwdVisited, bwdVisited map[uint64]*uint64) []uint64 {
	// Forward half: src → meet (build then reverse).
	var fwd []uint64
	cur := meet
	for cur != 0 {
		fwd = append(fwd, cur)
		p, ok := fwdVisited[cur]
		if !ok || p == nil {
			break
		}
		cur = *p
	}
	for i, j := 0, len(fwd)-1; i < j; i, j = i+1, j-1 {
		fwd[i], fwd[j] = fwd[j], fwd[i]
	}
	// Backward half: meet → dst. Skip meet (already in fwd) and walk parents.
	var bwd []uint64
	cur = meet
	for {
		p, ok := bwdVisited[cur]
		if !ok || p == nil {
			break
		}
		cur = *p
		bwd = append(bwd, cur)
	}
	return append(fwd, bwd...)
}

// buildReachableResponse loads the reachable node rows and applies the type
// filter. Returns a response with non-nil slices so the JSON encoder always
// emits an array (never null).
func (s *OkfService) buildReachableResponse(ids []uint64, typeFilter map[string]bool) (*ReachableResponse, error) {
	if len(ids) == 0 {
		return &ReachableResponse{Nodes: []model.OkfNode{}}, nil
	}
	loaded, err := s.nodes.ListByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("load reachable: %w", err)
	}
	out := make([]model.OkfNode, 0, len(loaded))
	for i := range loaded {
		if typeFilter != nil && !typeFilter[loaded[i].Type] {
			continue
		}
		out = append(out, loaded[i])
	}
	return &ReachableResponse{Nodes: out}, nil
}

// edgesToNeighborIDs returns the deduped, sorted set of neighbor IDs for a
// 1-hop walk. The src node's own ID is excluded to handle self-loops.
func edgesToNeighborIDs(srcID uint64, out, in []model.OkfEdge, dir string) []uint64 {
	set := map[uint64]bool{}
	if dir == BFSOut || dir == BFSBoth {
		for i := range out {
			if out[i].DstNodeID != 0 && out[i].DstNodeID != srcID {
				set[out[i].DstNodeID] = true
			}
		}
	}
	if dir == BFSIn || dir == BFSBoth {
		for i := range in {
			if in[i].SrcNodeID != 0 && in[i].SrcNodeID != srcID {
				set[in[i].SrcNodeID] = true
			}
		}
	}
	result := make([]uint64, 0, len(set))
	for id := range set {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// filterNodesByType drops rows whose Type does not match. Empty typeFilter
// returns the input unchanged.
func filterNodesByType(nodes []model.OkfNode, typeFilter string) []model.OkfNode {
	if typeFilter == "" {
		return nodes
	}
	out := nodes[:0]
	for i := range nodes {
		if nodes[i].Type == typeFilter {
			out = append(out, nodes[i])
		}
	}
	return out
}

// filterNodesByTypes is the multi-type version. Empty types returns the
// input unchanged.
func filterNodesByTypes(nodes []model.OkfNode, types []string) []model.OkfNode {
	if len(types) == 0 {
		return nodes
	}
	set := newTypeSet(types)
	out := nodes[:0]
	for i := range nodes {
		if set[nodes[i].Type] {
			out = append(out, nodes[i])
		}
	}
	return out
}

// newTypeSet turns a slice of type strings into a set lookup.
func newTypeSet(types []string) map[string]bool {
	if len(types) == 0 {
		return nil
	}
	out := make(map[string]bool, len(types))
	for _, t := range types {
		out[t] = true
	}
	return out
}

// dedupAdj returns the input with duplicate ids removed, preserving order.
// Adjacency lists sourced from the edge table are already unique on
// (src, dst) but cache hits could be served stale if a writer race produced
// a transient duplicate — defensive dedup keeps the BFS loop correct.
func dedupAdj(ids []uint64) []uint64 {
	if len(ids) <= 1 {
		return ids
	}
	seen := make(map[uint64]bool, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// nodeListFilterForSubgraph builds the repo-level filter for the Subgraph
// query. When the caller passes exactly one type, the filter is pushed down
// into the SQL query. With zero or many types, we fetch all and filter in
// Go — the type column is low-cardinality and the visualizer's typical
// maxNodes cap (500) keeps the slice bounded.
func nodeListFilterForSubgraph(types []string) repository.NodeListFilter {
	if len(types) == 1 {
		return repository.NodeListFilter{Type: types[0]}
	}
	return repository.NodeListFilter{}
}

// mapNodeLookupErr converts a repo miss into ErrOkfNodeNotFound. Other
// errors pass through unchanged so a real DB failure does not get masked as
// a 404.
func mapNodeLookupErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOkfNodeNotFound
	}
	return err
}
